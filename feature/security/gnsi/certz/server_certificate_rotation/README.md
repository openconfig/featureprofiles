# Server Certificate Rotation

## Summary

Certificates on network devices (servers) must be rotated over time for various
operational reasons. The ability to perform a rotation is a key component of
safe operation practices.

## Baseline Setup

### Input Args

   * the set of certificate testdata generated with the mk_cas.sh script
   in featureprofiles/feature/security/gnsi/certz/test_data

### DUT Service Setup

Configure the DUT to enable the following services (that are using gRPC) are up
and require using mTLS for authentication:

   * gNMI
   * gNOI
   * gNSI
   * gRIBI
   * P4RT

Be prepared to load the relevant trust_bundle.pem file for each test Certificate
Authority(CA) under test on the DUT. Each CA has an RSA and ECDSA form, both
must be tested.

## Tests

### Certz-3.1

Perform these positive tests:

Test that a server certificate can be rotated by using the gNSI certz Rotate()
api if the certificate is requested without the device generated CSR.

Perform this test with both the RSA and ECDSA types.

   0) Build the test data, configure the DUT to use the ca-0001 form
      key/certificate/trust_bundle, use the server-${TYPE}-a key/certificate.

   1) Connect a client to the service on the DUT. The client should maintain
      it's connection to the service throughout the rotation process being
      undertaken.

   2) With the server running, connect and note that the certificate loaded
      is the appropriate one, that it is the 'a' certificate in the ca-0001
      set of certificates, validate the SN/SAN are correct.

   3) Use the gNSI Rotate RPC to load a server-${TYPE}-b key and certificate
      on to the server.

   4) Test that the certificate is properly loaded, using the Probe RPC.
      Note that the new certificate is properly served by the server. Note
      that the certificate's SN/SAN has changed to the 'b' certificate.

   5) Send the Finalize RPC to the server.

   6) Verify that the server is now serving the certifcate properly, that
      the certificate is the 'b' certificate.

   7) Verify that at no time during the rotation process were existing
      connections to the service impaired / restarted / delayed due to
      the rotation event.


### Certz-3.2

Perform these negative tests:

Test that a server certificate can be rotated by using the gNSI certz Rotate()
api if the certificate is requested without the device generated CSR, expect a
failure because the certificate loaded is not signed by a trusted CA.

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



## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

TODO(OCRPC): Record may not be correct or complete

```yaml
rpcs:
  gnsi:
    certz.v1.Certz.Rotate:
```

## Minimum DUT Platform Requirement

vRX
