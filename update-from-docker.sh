#!/bin/bash
THISDIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd $THISDIR || exit 1

echo "Building cmd/dump..."
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

echo -n "Checking for tags: "
echo $($CTOOL search --list-tags docker.io/library/mongo | grep ^NAME)
if [ $? -ne 0 ]; then
    echo "Failed to connect to docker.io/library/mongo"
    exit 1
fi

# Iterate over all MongoDB versions available on Docker Hub. 
# This will require docker.com credentials if you hit the anonymous rate limit.
for version in $($CTOOL search --list-tags docker.io/library/mongo --no-trunc --limit 99999999 | 
    awk '{print $2}' | 
    egrep '^[0-9]+\.[0-9]+\.[0-9]+$' |
    egrep -v '(4\.2\.4|4\.2\.2|4\.1\.8)$' |  # Exclude broken versions
    sort -rV); do
    if [ -d "data/${version}" ]; then
        # echo "Skipping existing data/${version}"
        continue
    fi
    echo "Generating for mongo:$version"
    $CTOOL stop mongodb >/dev/null 2>&1
    $CTOOL rm mongodb >/dev/null 2>&1
    $CTOOL images | grep mongo | awk '{print $3}' | xargs $CTOOL rmi --force >/dev/null 2>&1
    $CTOOL volume prune --force  >/dev/null 2>&1
    CID=$($CTOOL run -d --name mongodb -p 27017:27017 -e MONGO_INITDB_ROOT_USERNAME=admin -e MONGO_INITDB_ROOT_PASSWORD=changeme docker.io/library/mongo:$version)
   
    if [ $? -ne 0 ]; then
       echo "Failed to build pull $version"
       continue
    fi

    IMAGE_NAME="docker.io/library/mongo:$version"
    CONTAINER_ID=$($CTOOL ps --all --filter ancestor=$IMAGE_NAME --format="{{.ID}}" | head -n 1)
    CONTAINER_STATUS=$($CTOOL inspect --format "{{json .State.Status }}" $CONTAINER_ID)
    until [ $CONTAINER_STATUS == '"running"' ]
    do
        echo "Waiting for container to start..."
        sleep 1
    done
    echo "Container $CONTAINER_ID is $CONTAINER_STATUS"
    echo "Connecting to mongo:$version"
    ./cmd/dump/dump localhost:27017 data/${version}
    
    $CTOOL stop mongodb >/dev/null 2>&1
    $CTOOL rm mongodb >/dev/null 2>&1
    $CTOOL images | grep mongo | awk '{print $3}' | xargs $CTOOL rmi --force >/dev/null 2>&1
    $CTOOL volume prune --force  >/dev/null 2>&1
done


echo "Building cmd/crunch..."
( cd cmd/crunch && go build -buildvcs=false )
if [ $? -ne 0 ]; then
    echo "Failed to build cmd/crunch"
    exit 1
fi

if [[ -f cmd/crunch/crunch ]]; then
    echo "Built cmd/crunch/crunch successfully"
else
    echo "cmd/crunch/crunch binary not found after build"
    exit 1
fi

./cmd/crunch/crunch data/
if [ $? -ne 0 ]; then
    echo "Failed to crunch data"
    exit 1
fi
