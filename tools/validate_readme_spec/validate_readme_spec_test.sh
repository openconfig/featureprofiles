#!/bin/bash

filename=invalid_all_empty.md
if go run validate_readme_spec.go -alsologtostderr testdata/"${filename}"; then
  echo "Validation passed, but failure expected"
  exit 1
fi
filename=invalid_empty_rpcs.md
if go run validate_readme_spec.go -alsologtostderr testdata/"${filename}"; then
  echo "Validation passed, but failure expected"
  exit 1
fi
filename=invalid_heading.md
if go run validate_readme_spec.go -alsologtostderr testdata/"${filename}"; then
  echo "Validation passed, but failure expected"
  exit 1
fi
filename=invalid_path.md
if go run validate_readme_spec.go -alsologtostderr testdata/"${filename}"; then
  echo "Validation passed, but failure expected"
  exit 1
fi
filename=valid_empty_paths.md
if ! go run validate_readme_spec.go -alsologtostderr testdata/"${filename}"; then
  echo "Validation failed, but pass expected"
  exit 1
fi
echo "PASS"
