# Copyright 2024 Google LLC
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

#!/bin/bash

go install ./

filename=invalid_all_empty.md
if validate_readme_spec -alsologtostderr testdata/"${filename}"; then
  echo "Validation passed, but failure expected"
  exit 1
fi
filename=invalid_empty_rpcs.md
if validate_readme_spec -alsologtostderr testdata/"${filename}"; then
  echo "Validation passed, but failure expected"
  exit 1
fi
filename=invalid_heading.md
if validate_readme_spec -alsologtostderr testdata/"${filename}"; then
  echo "Validation passed, but failure expected"
  exit 1
fi
filename=invalid_path.md
if validate_readme_spec -alsologtostderr testdata/"${filename}"; then
  echo "Validation passed, but failure expected"
  exit 1
fi
filename=valid_empty_paths.md
if ! validate_readme_spec -alsologtostderr testdata/"${filename}"; then
  echo "Validation failed, but pass expected"
  exit 1
fi
echo "PASS"
