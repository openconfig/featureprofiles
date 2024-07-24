# Trust Bundle Rotation

## Summary

Trust bundles on both clients and servers will be rotated as new CA information
is added to the trust_bundles over time.

## Baseline Setup

### Input Args

   * the set of certificate testdata generated with the mk_cas.sh script in
     featureprofiles/feature/security/gnsi/certz/test_data

### DUT Service Setup

Configure the DUT to enable the following services (that are using gRPC) are
up and require using mTLS for authentication:

   * gNMI
   * gNOI
   * gNSI
   * gRIBI
   * P4RT

Be prepared to load the relevant trust_bundle.pem file for each test
Certificate Authority(CA) under test on the DUT. Each CA has an RSA and ECDSA
form, both must be tested.

## Tests

### Certz-5.1

Perform these positive tests:

Test that a server trust_bundle can be rotated by using the gNSI certz api.

Perform this test with both the RSA and ECDSA types.

   0) Build the test data, configure the DUT to use the ca-0001 form
      key/certificate/trust_bundle, use the server-${TYPE}-a key/certificate.

   1) With the server running, connect and note that the ceritficate loaded
      is the appropriate one.

   2) Concatentate the ca-01/trust_bundle_${TYPE}.pem file and
      ca-02/trust_bundle_${TYPE}.pem file together, to create a new trust_bundle
      file to be used in the next step.

   3) Use the gNSI Rotate RPC to load the newly created trust_bundle file
      on the server.

   4) Test that the bundle is properly loaded, using the Probe RPC.
      Note that the same certificate is properly served by the server.

   5) Send the Finalize RPC to the server.

   6) Verify that the server is still serving the certifcate properly.

### Certz-5.2

Perform these negative tests:

Test that a server trust_bundle can be rotated by using the gNSI certz api.

Perform this test with both the RSA and ECDSA types.

   0) Build the test data, configure the DUT to use the ca-0001 form
      key/certificate/trust_bundle, use the server-${TYPE}-a key/certificate.

   1) With the server running, connect and note that the ceritficate loaded
      is the appropriate one.

   2) Use the gNSI Rotate RPC to load the ca-02/trust_bundle_${TYPE}.pem
      trust_bundle file on the server.

   3) See that the bundle load fails because the server certificate is no
      longer signed by a trusted CA.

   4) Tear down the Rotate RPC.

   5) Verify that the server is still serving the certifcate properly.


## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

TODO(OCRPC): Record may not be complete

```yaml
rpcs:
  gnsi:
    certz.v1.Certz.Rotate:
```


## Minimum DUT Platform Requirement

vRX
