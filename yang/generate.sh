#!/bin/bash
#
# Copyright 2022 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script is used to generate the OpenConfig Go APIs.

set -e

go install github.com/openconfig/ygot/generator@latest
git clone https://github.com/openconfig/public.git

EXCLUDE_MODULES=ietf-interfaces,openconfig-bfd,openconfig-messages

COMMON_ARGS=(
  -path=public/release/models,public/third_party/ietf
  -generate_path_structs
  -compress_paths
  -exclude_modules="${EXCLUDE_MODULES}"
  -generate_fakeroot
  -fakeroot_name=device
  -ignore_shadow_schema_paths
  -generate_simple_unions
  -typedef_enum_with_defmod
  -enum_suffix_for_simple_union_enums
  -trim_enum_openconfig_prefix
  -generate_append
  -generate_getters
  -generate_rename
  -generate_delete
  -generate_leaf_getters
  -structs_split_files_count=5
)

YANG_FILES=(
  google-bgp-timers.yang
  public/release/models/acl/openconfig-acl.yang
  public/release/models/acl/openconfig-packet-match.yang
  public/release/models/bgp/openconfig-bgp-policy.yang
  public/release/models/bgp/openconfig-bgp-types.yang
  public/release/models/interfaces/openconfig-if-aggregate.yang
  public/release/models/interfaces/openconfig-if-ethernet.yang
  public/release/models/interfaces/openconfig-if-ip-ext.yang
  public/release/models/interfaces/openconfig-if-ip.yang
  public/release/models/interfaces/openconfig-if-sdn-ext.yang
  public/release/models/interfaces/openconfig-if-tunnel.yang
  public/release/models/interfaces/openconfig-interfaces.yang
  public/release/models/isis/openconfig-isis.yang
  public/release/models/lacp/openconfig-lacp.yang
  public/release/models/lldp/openconfig-lldp-types.yang
  public/release/models/lldp/openconfig-lldp.yang
  public/release/models/local-routing/openconfig-local-routing.yang
  public/release/models/macsec/openconfig-macsec.yang
  public/release/models/mpls/openconfig-mpls-types.yang
  public/release/models/network-instance/openconfig-network-instance.yang
  public/release/models/openconfig-extensions.yang
  public/release/models/p4rt/openconfig-p4rt.yang
  public/release/models/policy-forwarding/openconfig-policy-forwarding.yang
  public/release/models/policy/openconfig-policy-types.yang
  public/release/models/policy/openconfig-routing-policy.yang
  public/release/models/platform/openconfig-platform.yang
  public/release/models/platform/openconfig-platform-port.yang
  public/release/models/qos/openconfig-qos-elements.yang
  public/release/models/qos/openconfig-qos-interfaces.yang
  public/release/models/qos/openconfig-qos-types.yang
  public/release/models/qos/openconfig-qos.yang
  public/release/models/relay-agent/openconfig-relay-agent.yang
  public/release/models/sampling/openconfig-sampling-sflow.yang
  public/release/models/stp/openconfig-spanning-tree.yang
  public/release/models/system/openconfig-aaa.yang
  public/release/models/system/openconfig-aaa-types.yang
  public/release/models/system/openconfig-system.yang
  public/release/models/types/openconfig-inet-types.yang
  public/release/models/types/openconfig-types.yang
  public/release/models/types/openconfig-yang-types.yang
  public/release/models/vlan/openconfig-vlan.yang
  public/third_party/ietf/iana-if-type.yang
  public/third_party/ietf/ietf-inet-types.yang
  public/third_party/ietf/ietf-interfaces.yang
  public/third_party/ietf/ietf-yang-types.yang
)

mkdir -p fpoc
generator \
  -output_dir=fpoc \
  -package_name=fpoc \
  -generate_structs \
  -path_structs_split_files_count=10 \
  "${COMMON_ARGS[@]}" \
  "${YANG_FILES[@]}"

find fpoc -name "*.go" -exec goimports -w {} +
find fpoc -name "*.go" -exec gofmt -w -s {} +
rm -rf public
