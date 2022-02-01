#!/bin/bash

set -xe

docker pull gcr.io/disco-idea-817/ceos:latest
docker tag gcr.io/disco-idea-817/ceos ceos:latest
kind load docker-image --name=kne ceos:latest

pushd /tmp/workspace
# TODO(bstoll): Replace this with the proper test execution process
kne_cli create topologies/kne/arista_ceos.textproto
cat >topologies/kne/testbed.kne.yml << EOF
username: admin
password: admin
topology: ${PWD}/topologies/kne/arista_ceos.textproto
cli: ${HOME}/go/bin/kne_cli
EOF
go test -v feature/system/system_base/tests/*.go -config "$PWD"/topologies/kne/testbed.kne.yml -testbed "$PWD"/topologies/dut.testbed
go test -v feature/system/system_ntp/tests/*.go -config "$PWD"/topologies/kne/testbed.kne.yml -testbed "$PWD"/topologies/dut.testbed
popd
