#!/bin/bash

# exit when a command fails
set -e

CGO_ENABLED=0 go build ..

docker build -t cntr:latest -f Dockerfile.cntr .

echo "docker build complete. Have a nice day."
