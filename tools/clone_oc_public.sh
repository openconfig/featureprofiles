#!/bin/bash
# Usage: clone_oc_public.sh local_dir version_prefix
set -e
git clone https://github.com/openconfig/public.git "$1"
cd "$1"
branch="$(git tag -l "$2.*" | sort -V | tail -1)"
git checkout "$branch"
