#!/bin/sh
for d in 01 02 10 1000 20000; do
  find ca-${d}/ -type f -exec /usr/bin/rm -f {} \;
  /usr/bin/rm -rf ca-${d}/
done
