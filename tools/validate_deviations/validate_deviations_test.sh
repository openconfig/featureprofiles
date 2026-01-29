#!/bin/bash

# A valid deviation file should pass when we skip the completeness check.
filename=valid_deviation.textproto

if ! ./validate_deviations --skip_completeness_check --deviations_file testdata/"${filename}"; then
  echo "Validation failed for ${filename}, but pass expected"
  exit 1
fi

# Invalid files should fail validation.
filenames=(
  "invalid_path_dev_missing_paths.textproto"
  "invalid_value_dev_missing_values.textproto"
  "invalid_missing_issue_url.textproto"
)

for filename in "${filenames[@]}"; do
  if ./validate_deviations --skip_completeness_check --deviations_file testdata/"${filename}"; then
    echo "Validation passed for ${filename}, but failure expected"
    exit 1
  fi
done

echo "PASS"