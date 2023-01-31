#!/bin/bash
cp -r ../firex/plugins/fp_patch _fp_patch
git reset --hard HEAD
git checkout origin/main
for f in _fp_patch/*
 echo "Processing $f"
 git reset --hard HEAD
 git apply $f --ignore-space-change --ignore-whitespace --verbose
 echo "************************************************"
done