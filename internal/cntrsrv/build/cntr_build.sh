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

# exit when a command fails
set -e

case $1 in
  "static")
      echo "Building static binary"
      CGO_ENABLED=0 go build ...
      ;;
  *)
      echo "Building dynamic binary"
      go build -ldflags '-s -w -I /lib64/ld-linux-x86-64.so.2 -extldflags=-Wl,--dynamic-linker,/lib64/ld-linux-x86-64.so.2,--strip-all'  ...
      ;;
esac

docker build -t cntr:latest -f Dockerfile.cntr .

echo "docker build complete. Have a nice day."
