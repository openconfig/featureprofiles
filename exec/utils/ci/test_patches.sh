#!/bin/bash
git reset --hard HEAD
git checkout main

for f in $1/*
do
 echo "Processing $f"
 git reset --hard HEAD
 git apply $f --ignore-space-change --ignore-whitespace --verbose || exit 1
 echo "************************************************"
done

git reset --hard HEAD
exit 0
