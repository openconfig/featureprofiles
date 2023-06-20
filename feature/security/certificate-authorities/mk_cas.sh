#!/bin/bash
#
# Create test certificate authority content for feature
# profile test cases.

# The list of directories of CA contents, also the count of CAs built in each directory.
DIRS=(01 02 10 1000)

# The types of signatures to support for the CA Certs.
TYPES=(rsa ecdsa)

# The ECDSA curve to use in creating ECDSA keys/certificates.
CURVE=prime256v1

# Create RSA and ECDSA CA keys, and associated certificates.
for d in ${DIRS[@]} ; do 
  if [ ! -d ca-${d} ] ; then
    mkdir ca-${d}
  fi
  for k in $(seq 1 ${d}); do
    OFFSET=$(printf  "%04i" ${k})
    for t in ${TYPES[@]}; do
      case ${t} in
        rsa)
          openssl genrsa -out ca-${d}/ca-${OFFSET}-${t}-key.pem 2048
          ;;
        ecdsa)
          openssl ecparam -name ${CURVE} -out ca-${d}/ca-${OFFSET}-${t}-key.pem -genkey
          ;;
      esac
      # Create a cert with the fresh key.
      openssl req -new -x509 -nodes -days 3650 \
        -key ca-${d}/ca-${OFFSET}-${t}-key.pem \
        -out ca-${d}/ca-${OFFSET}-${t}-cert.pem \
        -subj "/CN=CA ${OFFSET}/C=AQ/ST=NZ/L=NZ/O=OpenConfigFeatureProfiles"
    done
  done
done

# Make the trust bundles.
for d in ${DIRS[@]}; do
  for t in ${TYPES[@]}; do
    cat ca-${d}/ca-*-${t}-cert.pem > ca-${d}/trust_bundle_${d}_${t}.pem
  done
done
