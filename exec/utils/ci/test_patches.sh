#!/bin/bash
git reset --hard HEAD
git checkout main

failed=0
for f in $1/*
do
 echo "Processing $f"
 git reset --hard HEAD
 git apply $f --ignore-space-change --ignore-whitespace --verbose || failed=1
 echo "************************************************"
done

git reset --hard HEAD
if [ "$failed" -ne 0 ] ; then
    exit 1
fi
