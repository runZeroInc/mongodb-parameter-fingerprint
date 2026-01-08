package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gopkg.in/mgo.v2"            // Older wire protocol compatibility (<4.2)
	obson "gopkg.in/mgo.v2/bson" // Older wire protocol compatibility (<4.2)
)

type ListedCommand struct {
	Help                  string `json:"help"`
	RequiresAuth          bool   `json:"requiresAuth"`
	SecondaryOk           bool   `json:"secondaryOk"`
	AdminOnly             bool   `json:"adminOnly"`
	APIVersions           []any  `json:"apiVersions"`
	DeprecatedAPIVersions []any  `json:"deprecatedApiVersions"`
}

type ListCommands struct {
	Commands map[string]ListedCommand `json:"commands"`
	OK       float64                  `json:"ok"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mongo-bongo <mongodb_address> [output_directory]")
		return
	}
	addr := os.Args[1]
	ddir := os.Args[2]

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	mgoUri := fmt.Sprintf("mongodb://%s", addr)

	if _, err := os.Stat(ddir); err == nil {
		logrus.Printf("output %s already exists, skipping", ddir)
		os.Exit(0)
	}

	ms, err := mongo.Connect(options.Client().ApplyURI(mgoUri))
	if err != nil {
		logrus.Fatalf("failed to connect to MongoDB at %s: %v", addr, err)
	}
	defer func() {
		_ = ms.Disconnect(ctx)
	}()

	// Try to run listCommands with the official driver (wire protocol 8+)
	mdb := ms.Database("test")
	var result any
	nresult := bson.M{}
	err = mdb.RunCommand(ctx, bson.D{{Key: "listCommands", Value: 1}}).Decode(&nresult)
	if err != nil {
		if !strings.Contains(err.Error(), "but this version of the Go driver requires at least") {
			logrus.Fatalf("failed to run listCommands: %v", err)
		}

		// Fallback to the legacy driver for older MongoDB wire protocols
		ms, err := mgo.DialWithTimeout(addr, time.Second*10)
		if err != nil {
			logrus.Fatalf("failed to connect with old driver to MongoDB at %s: %v", addr, err)
		}

		defer ms.Close()
		ms.SetMode(mgo.Monotonic, true)

		oresult := obson.M{}
		mdb := ms.DB("test")
		if err = mdb.Run(obson.D{{Name: "listCommands", Value: 1}}, &oresult); err != nil {
			logrus.Fatalf("failed to run listCommands with old driver: %v", err)
		}
		result = oresult
	}

	if err := os.MkdirAll(ddir, 0o700); err != nil {
		logrus.Fatalf("failed to create output directory %s: %v", ddir, err)
	}

	jb, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		logrus.Fatalf("failed to marshal listCommands result: %v", err)
	}

	lc := ListCommands{}
	if err := json.Unmarshal(jb, &lc); err != nil {
		logrus.Fatalf("failed to unmarshal listCommands result: %v", err)
	}
	for cmdName, cmdDetails := range lc.Commands {
		fileName := filepath.Base(cmdName) + ".json"
		fpath := filepath.Join(ddir, fileName)
		f, err := os.Create(fpath)
		if err != nil {
			logrus.Errorf("failed to create file %s: %v", fpath, err)
			continue
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cmdDetails); err != nil {
			logrus.Errorf("failed to write command details to %s: %v", fpath, err)
		}
		f.Close()
	}
	logrus.Printf("dumped %d commands to %s", len(lc.Commands), ddir)
}
