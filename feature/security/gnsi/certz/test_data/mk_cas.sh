#!/bin/bash
#
# Create test certificate authority content for feature
# profile test cases.
#
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
  # Create a CA key and certificate for each of the DIRS count of
  # keys / certs. Do this for each of the TYPES key types.
  for k in $(seq 1 ${d}); do
    OFFSET=$(printf  "%04i" ${k})
    for t in ${TYPES[@]}; do
      # Generate the appropriate key type keys.
      case ${t} in
        rsa)
          openssl genrsa -out ca-${d}/ca-${OFFSET}-${t}-key.pem ${RSAKEYLEN}
          ;;
        ecdsa)
          openssl ecparam -name ${CURVE} \
            -out ca-${d}/ca-${OFFSET}-${t}-key.pem -genkey
          ;;
      esac
      # Create a cert with the fresh key, require it to be a CA certificate.
      openssl req -new -x509 -nodes -days ${LIFETIME} \
        -addext basicConstraints=critical,CA:TRUE \
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
#   * Use the CA + extensions config to create the client/server certificates.
#
for  d in ${DIRS[@]}; do
  if [ ! -d ca-${d} ] ; then
    mkdir ca-${d}
  fi
  for t in ${TYPES[@]}; do
    OFFSET=$(printf "%04i" ${d})
    # Create both client and server cert keys for each type.
    # use a/b here to signal the required 2 client or server certs/keys.
    for g in a b; do
      for cs in client server; do
        case ${t} in
          rsa)
            openssl genrsa -out ca-${d}/${cs}-${t}-${g}-key.pem ${RSAKEYLEN}
            ;;
          ecdsa)
            openssl ecparam -name ${CURVE} \
              -out ca-${d}/${cs}-${t}-${g}-key.pem -genkey
            ;;
        esac
      done
    done

    # Create the client and server requests, for both A and B (the 2 required certs)
    for cs in client server; do
      for g in a b ; do
        openssl req -new -key ca-${d}/${cs}-${t}-${g}-key.pem \
          -out ca-${d}/${cs}-${t}-${g}-req.pem \
          -config ${cs}_cert.cnf
        # Create the client and server complete certificates.
        openssl x509 -req -in ca-${d}/${cs}-${t}-${g}-req.pem \
          -CA ca-${d}/ca-${OFFSET}-${t}-cert.pem \
          -CAkey ca-${d}/ca-${OFFSET}-${t}-key.pem \
          -out ca-${d}/${cs}-${t}-${g}-cert.pem \
          -CAcreateserial \
          -days ${LIFETIME} \
          -sha256 \
          -extfile ${cs}_cert_ext.cnf
       done
    done
  done
done
