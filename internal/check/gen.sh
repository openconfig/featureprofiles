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
set -e

cd "$(dirname "$0")"

go run github.com/openconfig/ygnmi/app/ygnmi generator \
  --trim_module_prefix=openconfig \
  --base_package_path=github.com/openconfig/featureprofiles/internal/check/exampleoc \
  --output_dir=exampleoc \
  yang/openconfig-simple.yang \
  yang/openconfig-nested.yang

go install golang.org/x/tools/cmd/goimports@latest
go install github.com/google/addlicense@latest
goimports -w .
gofmt -w -s .
addlicense -c "Google LLC" -l apache .
