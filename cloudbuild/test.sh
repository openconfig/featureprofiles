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

case $1 in
  arista_ceos)
    topology=arista_ixia.textproto
    internal_image=us-west1-docker.pkg.dev/gep-kne/arista/ceos:ga
    external_image=ceos:latest
    ;;
  juniper_cptx)
    topology=juniper_ixia.textproto
    internal_image=us-west1-docker.pkg.dev/gep-kne/juniper/cptx:ga
    external_image=cptx:latest
    ;;
  cisco_8000e)
    topology=cisco_ixia.textproto
    internal_image=us-west1-docker.pkg.dev/gep-kne/cisco/8000e:ga
    external_image=8000e:latest
    ;;
  nokia_srlinux)
    topology=nokia_ixia.textproto
    internal_image=us-west1-docker.pkg.dev/gep-kne/nokia/srlinux:ga
    external_image=ghcr.io/nokia/srlinux:latest
    ;;
  :)
    echo "Model $1 not valid"
    exit 1
    ;;
esac

export PATH=${PATH}:/usr/local/go/bin:$(/usr/local/go/bin/go env GOPATH)/bin

kne deploy kne-internal/deploy/kne/kind-bridge.yaml

docker pull $internal_image
docker tag $internal_image $external_image
kind load docker-image --name=kne $external_image

pushd /tmp/workspace
kne create topologies/kne/$topology
cat >/tmp/testbed.kne.yml << EOF
username: admin
password: admin
topology: ${PWD}/topologies/kne/$topology
skip_reset: true
EOF

go test -v ./feature/system/tests/... -kne-config /tmp/testbed.kne.yml -testbed "$PWD"/topologies/dut.testbed
popd
