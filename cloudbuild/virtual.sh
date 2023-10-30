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

sudo tee /usr/bin/cleanup.sh >/dev/null <<'EOF'
#!/bin/bash
NAME="$(curl -X GET http://metadata.google.internal/computeMetadata/v1/instance/name -H 'Metadata-Flavor: Google')"
ZONE="$(curl -X GET http://metadata.google.internal/computeMetadata/v1/instance/zone -H 'Metadata-Flavor: Google')"
gcloud --quiet compute instances delete "${NAME}" --zone="${ZONE}"
EOF
sudo tee /etc/systemd/system/cleanup.service >/dev/null <<'EOF'
[Unit]
Description="Delete self after delay"
[Service]
Type=oneshot
ExecStart=/usr/bin/cleanup.sh
EOF
sudo tee /etc/systemd/system/cleanup.timer >/dev/null <<'EOF'
[Unit]
Description="Delete self after delay"
[Timer]
OnBootSec=9h
OnUnitActiveSec=9h
[Install]
WantedBy=default.target
EOF
sudo chmod 755 /usr/bin/cleanup.sh
sudo systemctl daemon-reload
sudo systemctl disable cleanup.service
sudo systemctl enable cleanup.timer
sudo systemctl start cleanup.timer

readonly platform="${1}"
readonly dut_tests="${2}"

case ${platform} in
  arista_ceos)
    vendor_creds=ARISTA/admin/admin
    ;;
  juniper_cptx)
    vendor_creds=JUNIPER/root/Google123
    ;;
  juniper_ncptx)
    vendor_creds=JUNIPER/root/Google123
    ;;
  cisco_8000e)
    vendor_creds=CISCO/cisco/cisco123
    ;;
  cisco_xrd)
    vendor_creds=CISCO/cisco/cisco123
    ;;
  nokia_srlinux)
    vendor_creds=NOKIA/admin/NokiaSrl1!
    ;;
  openconfig_lemming)
    vendor_creds=OPENCONFIG/admin/admin
    ;;
  :)
    echo "Model ${platform} not valid"
    exit 1
    ;;
esac

function metadata_kne_topology() {
  local metadata_test_path
  metadata_test_path="${1}"
  local topo_prefix
  topo_prefix=$(echo "${platform}" | tr "_" "/")
  declare -A kne_topology_file
  kne_topology_file["TESTBED_DUT"]="${topo_prefix}/dut.textproto"
  kne_topology_file["TESTBED_DUT_DUT_4LINKS"]="${topo_prefix}/dutdut.textproto"
  kne_topology_file["TESTBED_DUT_ATE_2LINKS"]="${topo_prefix}/dutate.textproto"
  kne_topology_file["TESTBED_DUT_ATE_4LINKS"]="${topo_prefix}/dutate.textproto"
  kne_topology_file["TESTBED_DUT_ATE_9LINKS_LAG"]="${topo_prefix}/dutate_lag.textproto"
  kne_topology_file["TESTBED_DUT_DUT_ATE_2LINKS"]="${topo_prefix}/dutdutate.textproto"
  for p in "${!kne_topology_file[@]}"; do
    if grep -q "testbed.*${p}$" "${metadata_test_path}"/metadata.textproto; then
      echo "${kne_topology_file[${p}]}"
      return
    fi
  done
  echo "UNKNOWN"
}

export PATH=${PATH}:/usr/local/go/bin:$(/usr/local/go/bin/go env GOPATH)/bin

for dut_test in ${dut_tests}; do
  test_badge=$(echo "${dut_test}" | awk '{split($0,a,",");print a[2]}')
  gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"${test_badge}\",\"status\":\"pending execution\"}"
done

kne deploy kne-internal/deploy/kne/kind-bridge.yaml

pushd /tmp/featureprofiles

cp -r "${PWD}"/topologies/kne /tmp

for dut_test in ${dut_tests}; do
  test_path=$(echo "${dut_test}" | awk '{split($0,a,",");print a[1]}')
  test_badge=$(echo "${dut_test}" | awk '{split($0,a,",");print a[2]}')
  kne_topology=$(metadata_kne_topology "${test_path}")
  sed -i "s/ceos:latest/us-west1-docker.pkg.dev\/gep-kne\/arista\/ceos:ga/g" /tmp/kne/"${kne_topology}"
  sed -i "s/cptx:latest/us-west1-docker.pkg.dev\/gep-kne\/juniper\/cptx:ga/g" /tmp/kne/"${kne_topology}"
  sed -i "s/ncptx:latest/us-west1-docker.pkg.dev\/gep-kne\/juniper\/ncptx:ga/g" /tmp/kne/"${kne_topology}"
  sed -i "s/8000e:latest/us-west1-docker.pkg.dev\/gep-kne\/cisco\/8000e:ga/g" /tmp/kne/"${kne_topology}"
  sed -i "s/xrd:latest/us-west1-docker.pkg.dev\/gep-kne\/cisco\/xrd:ga/g" /tmp/kne/"${kne_topology}"
  sed -i "s/srlinux:latest/us-west1-docker.pkg.dev\/gep-kne\/nokia\/srlinux:ga/g" /tmp/kne/"${kne_topology}"

  gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"${test_badge}\",\"status\":\"environment setup\"}"
  kne create /tmp/kne/"${kne_topology}"
  gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"${test_badge}\",\"status\":\"running\"}"
  go test -v ./"${test_path}"/... -timeout 0 \
  -kne-topo /tmp/kne/"${kne_topology}" \
  -kne-skip-reset \
  -vendor_creds "${vendor_creds}" \
  -alsologtostderr
  if [[ $? -eq 0 ]]; then
    gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"${test_badge}\",\"status\":\"success\"}"
  else
    gcloud pubsub topics publish featureprofiles-badge-status --message "{\"path\":\"${test_badge}\",\"status\":\"failure\"}"
  fi
  kne delete /tmp/kne/"${kne_topology}"
done

popd
