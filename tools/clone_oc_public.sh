#!/bin/bash
# Usage: clone_oc_public.sh local_dir_name
set -e
git clone https://github.com/openconfig/public.git "$1"
cd "$1"
# presence of "-" indicates prelease https://semver.org/#spec-item-9
branch="$(git tag -l | grep -v "-" | sort -V | tail -1)"
git checkout "$branch"
echo "Using github.com/openconfig/public branch: $branch"
