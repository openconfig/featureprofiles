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

# This script is used to generate the Feature Profiles topology
# binding proto APIs.

set -e

# Set directory to hold symlink
mkdir -p protobuf-import
# Remove any existing symlinks & empty directories
find protobuf-import -type l -delete
find protobuf-import -type d -empty -delete
# Download the required dependencies
go mod download
# Get ondatra modules we use and create required directory structure
go list -f 'protobuf-import/{{ .Path }}' -m github.com/openconfig/ondatra | xargs -L1 dirname | sort | uniq | xargs mkdir -p
go list -f '{{ .Dir }} protobuf-import/{{ .Path }}' -m github.com/openconfig/ondatra | xargs -L1 -- ln -s

cd "$( dirname "${BASH_SOURCE[0]}" )"

protoc -I='../../protobuf-import' --proto_path=. --go_out=. --go_opt=module=github.com/openconfig/featureprofiles/topologies/proto *.proto
