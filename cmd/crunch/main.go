package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

type MongoCommandHelp struct {
	Help                  string `json:"help,omitempty"`
	RequiresAuth          bool   `json:"requiresAuth,omitempty"`
	SecondaryOk           bool   `json:"secondaryOk,omitempty"`
	AdminOnly             bool   `json:"adminOnly,omitempty"`
	APIVersions           []any  `json:"apiVersions,omitempty"`
	DeprecatedAPIVersions []any  `json:"deprecatedApiVersions,omitempty"`
}

var patVersion = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

type MongoMatch struct {
	MinVersion      string   `json:"version_min,omitempty"`
	MaxVersion      string   `json:"version_max,omitempty"`
	AllVersions     []string `json:"versions,omitempty"`
	PreviousVersion string   `json:"version_prev,omitempty"`
	ParamsAdded     []string `json:"params_added,omitempty"`
	ParamsRemoved   []string `json:"params_removed,omitempty"`
	Count           int      `json:"count,omitempty"`
}

func main() {
	versions := make(map[string]map[string]*MongoCommandHelp)
	if len(os.Args) != 2 {
		logrus.Fatalf("Usage: %s <source-directory>", os.Args[0])
	}
	src := os.Args[1]
	dir, err := os.ReadDir(src)
	if err != nil {
		logrus.Fatalf("Failed to read directory %s: %v", src, err)
	}
	for _, entry := range dir {
		if !patVersion.MatchString(entry.Name()) {
			continue
		}
		commands, err := loadMongoCommands(filepath.Join(src, entry.Name()))
		if err != nil {
			logrus.Fatalf("Failed to load MongoDB commands from %s: %v", entry.Name(), err)
		}
		versions[entry.Name()] = commands
	}
	logrus.Printf("Loaded %d MongoDB versions", len(versions))
	sortedVersions := make([]string, 0, len(versions))
	for v := range versions {
		sortedVersions = append(sortedVersions, v)
	}
	sort.Strings(sortedVersions)

	lastParams := []string{}
	lastVersion := ""
	matches := make(map[string]*MongoMatch)
	matchOrder := []string{}
	for _, v := range sortedVersions {
		cmdHelp, ok := versions[v]["setParameter"]
		if !ok {
			logrus.Errorf("No setParameter command found for version %s: %v", v, versions[v])
			continue
		}
		_, vals, ok := strings.Cut(cmdHelp.Help, "supported:\n")
		if !ok {
			_, vals, ok = strings.Cut(cmdHelp.Help, "supported so far:\n") // 2.2.7
			if !ok {
				logrus.Errorf("No supported parameters found in setParameter help for version %s: %s", v, cmdHelp.Help)
				continue
			}
		}
		params := []string{}
		for _, line := range strings.Split(vals, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			params = append(params, line)
		}

		phash := md5.Sum([]byte(cmdHelp.Help))
		mhashKey := hex.EncodeToString(phash[:])
		m, ok := matches[mhashKey]
		if !ok {
			m = &MongoMatch{
				MinVersion:      v,
				MaxVersion:      v,
				AllVersions:     []string{v},
				PreviousVersion: lastVersion,
				Count:           len(params),
			}
			m.ParamsAdded, m.ParamsRemoved = diffParams(lastParams, params)
			matches[mhashKey] = m
			matchOrder = append(matchOrder, mhashKey)
		} else {
			m.AllVersions = append(m.AllVersions, v)
			m.MaxVersion = v
		}
		lastParams = params
		lastVersion = v
	}

	out := bytes.Buffer{}

	out.WriteString("{\n")
	for _, k := range matchOrder {
		d, err := json.MarshalIndent(matches[k], "  ", "  ")
		if err != nil {
			logrus.Fatalf("Failed to marshal match to JSON: %v", err)
		}
		out.WriteString("  \"" + k + "\": ")
		out.Write(d)
		out.WriteString(",\n")
	}
	final := out.Bytes()
	final[len(final)-2] = '\n' // Replace the trailing comma
	final[len(final)-1] = '}'  // Replace the newline with closing brace

	fd, err := os.Create(filepath.Join(src, "matches.json"))
	if err != nil {
		logrus.Fatalf("Failed to create output file: %v", err)
	}
	defer fd.Close()
	_, err = fd.Write(final)
	if err != nil {
		logrus.Fatalf("Failed to write output file: %v", err)
	}
	logrus.Printf("Wrote %d unique matches to %s", len(matches), filepath.Join(src, "matches.json"))
}

var patValidJSON = regexp.MustCompile(`^*\.json$`)

func loadMongoCommands(path string) (map[string]*MongoCommandHelp, error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		logrus.Fatalf("Failed to read directory %s: %v", path, err)
	}
	res := make(map[string]*MongoCommandHelp, len(dir))
	for _, entry := range dir {
		if !patValidJSON.MatchString(entry.Name()) {
			continue
		}
		cname, _, ok := strings.Cut(entry.Name(), ".json")
		if !ok {
			logrus.Errorf("Invalid JSON file name: %s", entry.Name())
			continue
		}
		cmdHelp, err := loadMongoCommandHelp(filepath.Join(path, entry.Name()))
		if err != nil {
			logrus.Fatalf("Failed to load MongoDB commands from %s: %v", entry.Name(), err)
		}
		res[cname] = cmdHelp
	}
	return res, nil
}

func loadMongoCommandHelp(path string) (*MongoCommandHelp, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var helpOutput MongoCommandHelp
	defer fd.Close()
	err = json.NewDecoder(fd).Decode(&helpOutput)
	if err != nil {
		return nil, err
	}
	return &helpOutput, nil
}

func diffParams(oldParams, newParams []string) (added, removed []string) {
	oldSet := make(map[string]struct{})
	newSet := make(map[string]struct{})
	for _, p := range oldParams {
		oldSet[p] = struct{}{}
	}
	for _, p := range newParams {
		newSet[p] = struct{}{}
	}
	for p := range newSet {
		if _, ok := oldSet[p]; !ok {
			added = append(added, p)
		}
	}
	for p := range oldSet {
		if _, ok := newSet[p]; !ok {
			removed = append(removed, p)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}
