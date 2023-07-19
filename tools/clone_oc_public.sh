#!/bin/bash
# Usage: clone_oc_public.sh local_dir_name
set -e
git clone https://github.com/openconfig/public.git "$1"
cd "$1"
# Use latest commit of OpenConfig public repo.
echo "Using github.com/openconfig/public branch: $branch"
