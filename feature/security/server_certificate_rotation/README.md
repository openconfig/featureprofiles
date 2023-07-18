# X.X: Server Certificate Rotation

## Summary

Certificates on network devices (servers) must be rotated over time for various
operational reasons. The ability to perform a rotation is a key component of
safe operation practices.

## Baseline Setup

### Input Args

   * the set of certificate testdata generated with the mk_cas.sh script
   in featureprofiles/feature/certificate-authorities/

### DIT Service Setup

Configure the DUT to enable the following services (that are using gRPC) are up
and required user mTLS for authentication:

   * gNMI
   * gNOI
   * gNSI
   * gRIBI

Be prepared to load the relevant trust_bundle.pem file for each test Certificate
Authority(CA) under test on the DUT. Each CA has an RSA and ECDSA form, both
must be tested.

## Tests

### XXX-1

Perform these positive tests:



### XXX-2

Perform these negative tests:

## Config Parameter Coverage

## Telemetry Parameter Coverage

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX
