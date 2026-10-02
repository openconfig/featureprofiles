#!/bin/bash

set -eu

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
#
# Create test certificate authority content for feature
# profile test cases.
#
# The list of directories of CA contents, also the count of CAs built
# in each directory.
DEFAULT_DIRS=(01 02 10 1000 20000)
# Accept either:
#   ./mk_cas.sh "01,02,10,1000" /tmp/outdir
#   ./mk_cas.sh /tmp/outdir
#   ./mk_cas.sh
first_arg="${1:-}"
if [ -n "${first_arg}" ] && [[ "${first_arg}" =~ ^[0-9,]+$ ]]; then
  echo "Using provided DIRS: ${first_arg}"
  IFS=',' read -r -a DIRS <<< "${first_arg}"
  OUTDIR="${2:-.}"
elif [ -n "${first_arg}" ]; then
  OUTDIR="${first_arg}"
  echo "Using default DIRS: ${DEFAULT_DIRS[*]}"
  DIRS=("${DEFAULT_DIRS[@]}")
else
  echo "Using default DIRS: ${DEFAULT_DIRS[*]}"
  DIRS=("${DEFAULT_DIRS[@]}")
  OUTDIR="."
fi

mkdir -p "${OUTDIR}"
OUTDIR="$(cd -- "${OUTDIR}" && pwd)"

CLIENT_CNF="${SCRIPT_DIR}/client_cert.cnf"
CLIENT_EXT="${SCRIPT_DIR}/client_cert_ext.cnf"
SERVER_CNF="${SCRIPT_DIR}/server_cert.cnf"
SERVER_EXT="${SCRIPT_DIR}/server_cert_ext.cnf"

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
  if [ ! -d ${OUTDIR}/ca-${d} ] ; then
    mkdir -p ${OUTDIR}/ca-${d}
  fi
  # Create a CA key and certificate for each of the DIRS count of
  # keys / certs. Do this for each of the TYPES key types.
  for k in $(seq 1 ${d}); do
    OFFSET=$(printf  "%05i" ${k})
    for t in ${TYPES[@]}; do
      # Generate the appropriate key type keys.
      case ${t} in
        rsa)
          openssl genrsa -out ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-key.pem ${RSAKEYLEN}
          ;;
        ecdsa)
          openssl ecparam -name ${CURVE} \
            -out ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-key.pem -genkey
          ;;
      esac
      # Create a CA certificate cleanly without relying on brittle system openssl.cnf defaults or duplicating extensions.
      openssl req -new -nodes \
        -key ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-key.pem \
        -out ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-req.pem \
        -subj "/CN=CA ${OFFSET}/C=AQ/ST=NZ/L=NZ/O=OpenConfigFeatureProfiles"

      openssl x509 -req -days ${LIFETIME} \
        -in ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-req.pem \
        -signkey ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-key.pem \
        -out ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-cert.pem \
        -sha256 \
        -extfile <(printf "basicConstraints=critical,CA:TRUE\nkeyUsage=critical,keyCertSign,cRLSign")

      rm -f ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-req.pem
    done
  done
done

# Make the trust bundles.
for d in ${DIRS[@]}; do
  for t in ${TYPES[@]}; do
    cat ${OUTDIR}/ca-${d}/ca-*-${t}-cert.pem > ${OUTDIR}/ca-${d}/trust_bundle_${d}_${t}.pem
    CERTS=""
    for cf in ${OUTDIR}/ca-${d}/ca-*-${t}-cert.pem; do
      CERTS="${CERTS} -certfile ${cf}"
    done
    openssl crl2pkcs7 -nocrl ${CERTS} -out ${OUTDIR}/ca-${d}/trust_bundle_${d}_${t}.p7b
  done
done

# Create client / server certificates for each CA set.
# Two client and Two server certificates are all that are required per type.
#   * Create keys per type for each client/server certificate to create.
#   * Create CSRs per type for each client/server certificate to create.
#   * Use the CA + extensions config to create the client/server certificates.
#
for  d in ${DIRS[@]}; do
  if [ ! -d ${OUTDIR}/ca-${d} ] ; then
    mkdir -p ${OUTDIR}/ca-${d}
  fi
  for t in ${TYPES[@]}; do
    OFFSET=$(printf "%05i" ${d})
    # Create both client and server cert keys for each type.
    # use a/b here to signal the required 2 client or server certs/keys.
    for g in a b; do
      for cs in client server; do
        case ${t} in
          rsa)
            openssl genrsa -out ${OUTDIR}/ca-${d}/${cs}-${t}-${g}-key.pem ${RSAKEYLEN}
            ;;
          ecdsa)
            openssl ecparam -name ${CURVE} \
              -out ${OUTDIR}/ca-${d}/${cs}-${t}-${g}-key.pem -genkey
            ;;
        esac
      done
    done

    # Create the client and server requests, for both A and B (the 2 required certs)
    for cs in client server; do
      for g in a b ; do
        if [ "${cs}" = "client" ]; then
          openssl req -new -key ${OUTDIR}/ca-${d}/${cs}-${t}-${g}-key.pem \
            -out ${OUTDIR}/ca-${d}/${cs}-${t}-${g}-req.pem \
            -config "${CLIENT_CNF}"
        else
          openssl req -new -key ${OUTDIR}/ca-${d}/${cs}-${t}-${g}-key.pem \
            -out ${OUTDIR}/ca-${d}/${cs}-${t}-${g}-req.pem \
            -config "${SERVER_CNF}"
        fi
        # Create the client and server complete certificates.
        openssl x509 -req -in ${OUTDIR}/ca-${d}/${cs}-${t}-${g}-req.pem \
          -CA ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-cert.pem \
          -CAkey ${OUTDIR}/ca-${d}/ca-${OFFSET}-${t}-key.pem \
          -out ${OUTDIR}/ca-${d}/${cs}-${t}-${g}-cert.pem \
          -CAcreateserial \
          -days ${LIFETIME} \
          -sha256 \
          -extfile "$( [ "${cs}" = "client" ] && printf '%s' "${CLIENT_EXT}" || printf '%s' "${SERVER_EXT}" )"
       done
    done
  done
done