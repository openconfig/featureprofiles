#!/bin/bash
#
# Create test certificate authority content for feature
# profile test cases.

# The list of directories of CA contents, also the count of CAs built
# in each directory.
DIRS=(01 02 10 1000)

# The types of signatures to support for the CA Certs.
TYPES=(rsa ecdsa)

# The ECDSA curve to use in creating ECDSA keys/certificates.
CURVE=prime256v1

# The length of an RSA key to generate/use in openssl comamnds.
RSAKEYLEN=2048

# Lifetime of certificates.
LIFETIME=3650

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
          openssl genrsa -out ca-${d}/ca-${OFFSET}-${t}-key.pem ${RSAKEYLEN}
          ;;
        ecdsa)
          openssl ecparam -name ${CURVE} \
            -out ca-${d}/ca-${OFFSET}-${t}-key.pem -genkey
          ;;
      esac
      # Create a cert with the fresh key.
      openssl req -new -x509 -nodes -days ${LIFETIME} \
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

# Create client / server certificates for each CA set.
# Two client and Two server certificates are all that are required per type.
#   * Create keys per type for each client/server certificate to create.
#   * Create CSRs per type for each client/server certificate to create.
#   * Use the CA + extensions config to create the client certificates.
#
# Repeat for the server certificate creation.
for  d in ${DIRS[@]}; do
  if [ ! -d ca-${d} ] ; then
    mkdir ca-${d}
  fi
  for t in ${TYPES[@]}; do
    OFFSET=$(printf "%04i" ${d})
    # Create both client and server cert keys for each type.
    for g in a b; do
      case ${t} in
        rsa)
          openssl genrsa -out ca-${d}/client-${t}-${g}-key.pem ${RSAKEYLEN}
          openssl genrsa -out ca-${d}/server-${t}-${g}-key.pem ${RSAKEYLEN}
          ;;
        ecdsa)
          openssl ecparam -name ${CURVE} \
            -out ca-${d}/client-${t}-${g}-key.pem -genkey
          openssl ecparam -name ${CURVE} \
            -out ca-${d}/server-${t}-${g}-key.pem -genkey
          ;;
      esac

      # Create the client and server requests.
      openssl req -new -key ca-${d}/client-${t}-${g}-key.pem \
        -out ca-${d}/client-${t}-${g}-req.pem \
        -config client_cert.cnf
      openssl req -new -key ca-${d}/server-${t}-${g}-key.pem \
        -out ca-${d}/server-${t}-${g}-req.pem \
        -config server_cert.cnf

      # Create the client and server complete certificates.
      openssl x509 -req -in ca-${d}/client-${t}-${g}-req.pem \
        -CA ca-${d}/ca-${OFFSET}-${t}-${g}-cert.pem \
        -CAkey ca-${d}/ca-${OFFSET}-${t}-${g}-key.pem \
        -out ca-${d}/client-${t}-${g}-cert.pem \
        -CAcreateserial \
        -days ${LIFETIME} \
        -sha256 \
        -extfile client_cert_ext.cnf
      openssl x509 -req -in ca-${d}/server-${t}-${g}-req.pem \
        -CA ca-${d}/ca-${OFFSET}-${t}-${g}-cert.pem \
        -CAkey ca-${d}/ca-${OFFSET}-${t}-${g}-key.pem \
        -out ca-${d}/server-${t}-${g}-cert.pem \
        -CAcreateserial \
        -days ${LIFETIME} \
        -sha256 \
        -extfile server_cert_ext.cnf
    done
  done
done
