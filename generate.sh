#!/bin/bash

( cd cmd/dump && go build )
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

if [[ 'which podman' != '' ]]; then
    echo "Using podman as a docker replacement..."
    alias docker=podman
fi

# Iterate over all MongoDB versions available on Docker Hub. 
# This will require docker.com credentials if you hit the anonymous rate limit.
for version in $(docker search --list-tags docker.io/library/mongo --no-trunc --limit 99999999 | awk '{print $2}' | egrep '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -rV); do
    if [ -d "data/${version}" ]; then
        echo "Skipping existing data/${version}"
        continue
    fi
    echo "Generating for mongo:$version"
    docker stop mongodb; docker rm mongodb;
    docker images | grep mongo | awk '{print $3}' | xargs docker rmi --force;
    docker volume prune --force;
    docker run -d --name mongodb -p 27017:27017 -e MONGO_INITDB_ROOT_USERNAME=admin -e MONGO_INITDB_ROOT_PASSWORD=changeme mongo:$version
    sleep 1
    echo "Connecting to mongo:$version"
    ./cmd/dump/dump localhost:27017 data/${version}
done