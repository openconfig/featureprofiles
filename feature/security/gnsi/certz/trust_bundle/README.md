# Trust Bundle

## Summary

Server and client TLS endpoints use x.509 certificates for
identification of the calling or called endpoint. Systems
could use self-signed certificates and not validate, but
this is an insecure practice.

Servers and clients should require that the certificates
used are validated and are signed by a known Certificate
Authority(CA).

The known CAs which can be used are contained in a
'trust bundle', which is a list of public keys of the CAs.
The list of CA public keys must be kept up to date, as
CAs will rotate their key material on a regular cadence.

CA keys may be one of two valid (at this time) key algorithms:

   * RSA
   * ECDSA

(Note: Security of key algorithms is subject to change, the
system must be able to support more than one key type at any
point in time in order to support key algorithm change events.)

A trust bundle may have one or more certificates contained in
it, systems should be able to support at least one thousand
CA keys in such a bundle.


## Baseline Setup

### Input Args

  * the set of certificate testdata generated with the mk_cas.sh
    script in the featureprofiles/feature/security/gnsi/certz/test_data
    directory.

### DUT service setup

Configure the DUT to enable the following sevices (that are using gRPC) are
up and require using mTLS for authentication:

  * gNMI
  * gNOI
  * gNSI
  * gRIBI
  * P4RT

For each trust_bundle created by mk_cas.sh, configure the
services to load the correct key-type certificate, key and
trust_bundle.

## Tests

### Test Certz-4.1

Load the server certificate and key from each of the following CA sets:
   * ca-01
   * ca-02
   * ca-10
   * ca-1000

Each service must be configured to use the appropriate certificate and validate
that certificate using the included trust_bundle.

Perform this test with both RSA dn ECDSA key-types.

## Config Parameter Coverage

## Telemetry Parameter Coverage

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX
