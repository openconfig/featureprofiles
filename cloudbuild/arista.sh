#!/bin/bash
# Copyright 2022 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -xe

export PATH=${PATH}:/usr/local/go/bin:$(/usr/local/go/bin/go env GOPATH)/bin

kne deploy kne-internal/deploy/kne/kind-bridge.yaml

docker pull us-west1-docker.pkg.dev/gep-kne/arista/ceos:ga
docker tag us-west1-docker.pkg.dev/gep-kne/arista/ceos:ga ceos:latest
kind load docker-image --name=kne ceos:latest

pushd /tmp/workspace
# TODO(bstoll): Replace this with the proper test execution process
kne create topologies/kne/arista_ceos.textproto
cat >/tmp/testbed.kne.yml << EOF
username: admin
password: admin
topology: ${PWD}/topologies/kne/arista_ceos.textproto
cli: /usr/local/bin/kne
EOF
go test -v feature/system/tests/*.go -kne-config /tmp/testbed.kne.yml -testbed "$PWD"/topologies/dut.testbed
go test -v feature/system/ntp/tests/*.go -kne-config /tmp/testbed.kne.yml -testbed "$PWD"/topologies/dut.testbed
popd
