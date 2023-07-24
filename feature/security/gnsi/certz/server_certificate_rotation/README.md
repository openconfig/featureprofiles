# X.X: Server Certificate Rotation

## Summary

Certificates on network devices (servers) must be rotated over time for various
operational reasons. The ability to perform a rotation is a key component of
safe operation practices.

## Baseline Setup

### Input Args

   * the set of certificate testdata generated with the mk_cas.sh script
   in featureprofiles/feature/security/gnsi/certz/test_data

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

### Certz-3.0

Perform these positive tests:

Test that a server certificate can be rotated by using the gNSI certz api if
the certificate is requested without the device generated CSR.

Perform this test with both the RSA and ECDSA types.

   0) Build the test data, configure the DUT to use the ca-0001 form
      key/certificate/trust_bundle, use the server-${TYPE}-a key/certificate.

   1) With the server running, connect and note that the ceritficate loaded
      is the appropriate one.

   2) Use the gNSI Rotate RPC to load a server-${TYPE}-b key and certificate
      on to the server.

   3) Test that the certificate is properly loaded, using the Probe RPC.
      Note that the new certificate is properly served by the server.

   4) Send the Finalize RPC to the server.

   5) Verify that the server is now serving the certifcate properly.


### Certz-3.1

Perform these negative tests:

Test that a server certificate can be rotated by using the gNSI certz api if
the certificate is requested without the device generated CSR, expect a failure
because the certificate loaded is not signed by a trusted CA.

Perform this test with both the RSA and ECDSA types.

   0) Build the test data, configure the DUT to use the ca-0001 form
      key/certificate/trust_bundle, use the server-${TYPE}-a key/certificate.

   1) With the server running, connect and note that the ceritficate loaded
      is the appropriate one.

   2) Use the gNSI Rotate RPC to load a ca-02/server-${TYPE}-b key and
      certificate on to the server.

   3) Test that the certificate load fails, because the certificate is not
      trusted by a known CA.

   4) Tear down the Rotate RPC, forcing the device to return to the
      previously used certificate/key material.

   5) Verify that the server is now serving the previous certifcate properly.



## Config Parameter Coverage

## Telemetry Parameter Coverage

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX
