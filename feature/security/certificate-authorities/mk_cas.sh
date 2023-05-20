#!/bin/sh
#
# Create test certificate authority content for feature
# profile test cases.

# Create CA keys.
# for d in 01 10 1000; do 
for d in 01 10 ; do 
  for k in $(seq 1 ${d}); do
    OFFSET=$(printf  "%04i" ${k})
    openssl genrsa 2048 > ca-${d}/ca-${OFFSET}-key.pem
  done
done

# Create CA certificates.
