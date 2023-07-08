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

nohup /tmp/featureprofiles/cloudbuild/cleanup.sh 2>/dev/null &

case $1 in
  arista_ceos)
    topology=arista/ceos/dutate.textproto
    vendor_creds=ARISTA/admin/admin
    ;;
  juniper_cptx)
    topology=juniper/cptx/dutate.textproto
    vendor_creds=JUNIPER/root/Google123
    ;;
  cisco_8000e)
    topology=cisco/8000e/dutate.textproto
    vendor_creds=CISCO/cisco/cisco123
    ;;
  cisco_xrd)
    topology=cisco/xrd/dutate.textproto
    vendor_creds=CISCO/cisco/cisco123
    ;;
  nokia_srlinux)
    topology=nokia/srlinux/dutate.textproto
    vendor_creds=NOKIA/admin/NokiaSrl1!
    ;;
  :)
    echo "Model $1 not valid"
    exit 1
    ;;
esac

export PATH=${PATH}:/usr/local/go/bin:$(/usr/local/go/bin/go env GOPATH)/bin

kne deploy kne-internal/deploy/kne/kind-bridge.yaml

pushd /tmp/featureprofiles

cp -r "$PWD"/topologies/kne /tmp
sed -i "s/ceos:latest/us-west1-docker.pkg.dev\/gep-kne\/arista\/ceos:ga/g" /tmp/kne/"$topology"
sed -i "s/cptx:latest/us-west1-docker.pkg.dev\/gep-kne\/juniper\/cptx:ga/g" /tmp/kne/"$topology"
sed -i "s/8000e:latest/us-west1-docker.pkg.dev\/gep-kne\/cisco\/8000e:ga/g" /tmp/kne/"$topology"
sed -i "s/xrd:latest/us-west1-docker.pkg.dev\/gep-kne\/cisco\/xrd:ga/g" /tmp/kne/"$topology"
sed -i "s/ghcr.io\/nokia\/srlinux:latest/us-west1-docker.pkg.dev\/gep-kne\/nokia\/srlinux:ga/g" /tmp/kne/"$topology"

kne create /tmp/kne/"$topology"

go test -v ./feature/system/tests/... \
  -timeout 0 \
  -kne-topo /tmp/kne/"$topology" \
  -vendor_creds "$vendor_creds" \
  -alsologtostderr

popd
