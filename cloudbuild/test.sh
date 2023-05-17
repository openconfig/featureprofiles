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
    topology=arista_ceos.textproto
    vendor_creds=ARISTA/admin/admin
    deviations=
    ;;
  juniper_cptx)
    topology=juniper_cptx.textproto
    vendor_creds=JUNIPER/root/Google123
    deviations="-deviation_cli_takes_precedence_over_oc=true"
    ;;
  cisco_8000e)
    topology=cisco_8000e.textproto
    vendor_creds=CISCO/cisco/cisco123
    deviations=
    ;;
  cisco_xrd)
    topology=cisco_xrd.textproto
    vendor_creds=CISCO/cisco/cisco123
    deviations=
    ;;
  nokia_srlinux)
    topology=nokia_srlinux.textproto
    vendor_creds=NOKIA/admin/NokiaSrl1!
    deviations=
    ;;
  :)
    echo "Model $1 not valid"
    exit 1
    ;;
esac

export PATH=${PATH}:/usr/local/go/bin:$(/usr/local/go/bin/go env GOPATH)/bin

kne deploy kne-internal/deploy/kne/kind-bridge.yaml

pushd /tmp/workspace

cp -r "$PWD"/topologies/kne /tmp
sed -i "s/ceos:latest/us-west1-docker.pkg.dev\/gep-kne\/arista\/ceos:ga/g" /tmp/kne/"$topology"
sed -i "s/cptx:latest/us-west1-docker.pkg.dev\/gep-kne\/juniper\/cptx:ga/g" /tmp/kne/"$topology"
sed -i "s/8000e:latest/us-west1-docker.pkg.dev\/gep-kne\/cisco\/8000e:ga/g" /tmp/kne/"$topology"
sed -i "s/xrd:latest/us-west1-docker.pkg.dev\/gep-kne\/cisco\/xrd:ga/g" /tmp/kne/"$topology"
sed -i "s/ghcr.io\/nokia\/srlinux:latest/us-west1-docker.pkg.dev\/gep-kne\/nokia\/srlinux:ga/g" /tmp/kne/"$topology"

kne create /tmp/kne/"$topology"

go test -v ./feature/system/tests/... \
  -timeout 0 \
  -testbed "$PWD"/topologies/dut.testbed \
  -kne-topo /tmp/kne/"$topology" \
  -kne-skip-reset \
  -vendor_creds "$vendor_creds" \
  "$deviations"

popd
