#!/bin/sh
OUTDIR="${1:-.}"
for d in 01 02 10 1000 20000; do
  if [ -d ${OUTDIR}/ca-${d} ]; then
    find ${OUTDIR}/ca-${d}/ -type f -exec /usr/bin/rm -f {} \;
    /usr/bin/rm -rf ${OUTDIR}/ca-${d}/
  fi
done
