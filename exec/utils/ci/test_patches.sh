#!/bin/bash
git reset --hard HEAD
git checkout origin/main

for f in ../firex/plugins/fp_patch/*.patch
 echo "Processing $f"
 git reset --hard HEAD
 git apply $f --ignore-space-change --ignore-whitespace --verbose
 echo "************************************************"
done