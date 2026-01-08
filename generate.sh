#!/bin/bash

( cd cmd/dump && go build -buildvcs=false )
if [ $? -ne 0 ]; then
    echo "Failed to build cmd/dump"
    exit 1
fi

if [[ -f cmd/dump/dump ]]; then
    echo "Built cmd/dump/dump successfully"
else
    echo "cmd/dump/dump binary not found after build"
    exit 1
fi

if [ ! -d "data" ]; then
    mkdir -p data
fi

CTOOL=$(which podman)
if [[ "$CTOOL" == "" ]]; then
    CTOOL=$(which docker)
    if [[ "$CTOOL" == "" ]]; then
        echo "Neither docker nor podman found. Please install one of them."
        exit 1
    fi
fi

# Iterate over all MongoDB versions available on Docker Hub. 
# This will require docker.com credentials if you hit the anonymous rate limit.
for version in $($CTOOL search --list-tags docker.io/library/mongo --no-trunc --limit 99999999 | awk '{print $2}' | egrep '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -rV); do
    if [ -d "data/${version}" ]; then
        echo "Skipping existing data/${version}"
        continue
    fi
    echo "Generating for mongo:$version"
    $CTOOL stop mongodb >/dev/null 2>&1
    $CTOOL rm mongodb >/dev/null 2>&1
    $CTOOL images | grep mongo | awk '{print $3}' | xargs $CTOOL rmi --force >/dev/null 2>&1
    $CTOOL volume prune --force  >/dev/null 2>&1
    $CTOOL run -d --name mongodb -p 27017:27017 -e MONGO_INITDB_ROOT_USERNAME=admin -e MONGO_INITDB_ROOT_PASSWORD=changeme docker.io/library/mongo:$version
   
    if [ $? -ne 0 ]; then
       echo "Failed to build pull $version"
       continue
    fi
   
    echo "Connecting to mongo:$version"
    ./cmd/dump/dump localhost:27017 data/${version}
done
