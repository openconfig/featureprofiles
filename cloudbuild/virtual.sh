#!/bin/bash
# Copyright 2023 Google LLC
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

set -x

case $1 in
  arista_ceos)
    vendor_creds=ARISTA/admin/admin
    deviations=
    ;;
  juniper_cptx)
    vendor_creds=JUNIPER/root/Google123
    deviations="-deviation_cli_takes_precedence_over_oc=true"
    ;;
  cisco_8000e)
    vendor_creds=CISCO/cisco/cisco123
    deviations=
    ;;
  cisco_xrd)
    vendor_creds=CISCO/cisco/cisco123
    deviations=
    ;;
  nokia_srlinux)
    vendor_creds=NOKIA/admin/NokiaSrl1!
    deviations=
    ;;
  :)
    echo "Model $1 not valid"
    exit 1
    ;;
esac

function metadata_testbed() {
  patterns=("TESTBED_DUT" "TESTBED_DUT_DUT_4LINKS" "TESTBED_DUT_ATE_2LINKS" "TESTBED_DUT_ATE_4LINKS")
  declare -A testbed
  testbed["TESTBED_DUT"]="/tmp/workspace/topologies/dut.testbed"
  testbed["TESTBED_DUT_DUT_4LINKS"]="/tmp/workspace/topologies/dutdut.testbed"
  testbed["TESTBED_DUT_ATE_2LINKS"]="/tmp/workspace/topologies/atedut_2.testbed"
  testbed["TESTBED_DUT_ATE_4LINKS"]="/tmp/workspace/topologies/atedut_4.testbed"
  for p in "${patterns[@]}"; do
    if grep -q "testbed.*${p}$" "${2}"/metadata.textproto; then
      echo "${testbed[${p}]}"
      return
    fi
  done
  echo "UNKNOWN"
}

function metadata_topology() {
  patterns=("TESTBED_DUT" "TESTBED_DUT_DUT_4LINKS" "TESTBED_DUT_ATE_2LINKS" "TESTBED_DUT_ATE_4LINKS")
  declare -A topology
  topology["TESTBED_DUT"]="${1}.textproto"
  topology["TESTBED_DUT_DUT_4LINKS"]="${1}_dutdut.textproto"
  topology["TESTBED_DUT_ATE_2LINKS"]="${1}_lag.textproto"
  topology["TESTBED_DUT_ATE_4LINKS"]="${1}_lag.textproto"
  for p in "${patterns[@]}"; do
    if grep -q "testbed.*${p}$" "${2}"/metadata.textproto; then
      echo "${topology[${p}]}"
      return
    fi
  done
  echo "UNKNOWN"
}

export PATH=${PATH}:/usr/local/go/bin:$(/usr/local/go/bin/go env GOPATH)/bin

for dut_test in $2; do
  test_path=$(echo $dut_test | awk '{split($0,a,",");print a[1]}')
  test_badge=$(echo $dut_test | awk '{split($0,a,",");print a[2]}')
  gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"$test_badge\",\"status\":\"pending execution\"}"
done

kne deploy kne-internal/deploy/kne/kind-bridge.yaml

pushd /tmp/workspace

cp -r "$PWD"/topologies/kne /tmp

for dut_test in $2; do
  test_path=$(echo $dut_test | awk '{split($0,a,",");print a[1]}')
  test_badge=$(echo $dut_test | awk '{split($0,a,",");print a[2]}')
  testbed=$(metadata_testbed "$1" "$test_path")
  topology=$(metadata_topology "$1" "$test_path")
  sed -i "s/ceos:latest/us-west1-docker.pkg.dev\/gep-kne\/arista\/ceos:ga/g" /tmp/kne/"$topology"
  sed -i "s/cptx:latest/us-west1-docker.pkg.dev\/gep-kne\/juniper\/cptx:ga/g" /tmp/kne/"$topology"
  sed -i "s/8000e:latest/us-west1-docker.pkg.dev\/gep-kne\/cisco\/8000e:ga/g" /tmp/kne/"$topology"
  sed -i "s/xrd:latest/us-west1-docker.pkg.dev\/gep-kne\/cisco\/xrd:ga/g" /tmp/kne/"$topology"
  sed -i "s/ghcr.io\/nokia\/srlinux:latest/us-west1-docker.pkg.dev\/gep-kne\/nokia\/srlinux:ga/g" /tmp/kne/"$topology"

  gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"$test_badge\",\"status\":\"environment setup\"}"
  kne create /tmp/kne/"$topology"
  gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"$test_badge\",\"status\":\"running\"}"
  go test -v ./"$test_path"/... -timeout 0 -testbed "$testbed" \
  -kne-topo /tmp/kne/"$topology" \
  -kne-skip-reset \
  -vendor_creds "$vendor_creds" \
  "$deviations"
  if [[ $? -eq 0 ]]; then
    gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"$test_badge\",\"status\":\"success\"}"
  else
    gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"$test_badge\",\"status\":\"failure\"}"
  fi
  kne delete /tmp/kne/"$topology"
done

popd
