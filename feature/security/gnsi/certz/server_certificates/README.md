# Server Certificate

## Summary

Servers must be able to validate a remote client's TLS certificate
and present a valid TLS certificate to the calling clients.

## Baseline Setup

### Input Args

* the set of certificate testdata generated with the mk_cas.sh
  script in featureprofiles/feature/security/gnsi/certz/test_data

### DUT Service setup

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

### Certz-2.1

Perform these positive tests:

Test that the server certificates from a set of one CA are able to be validated
and used for authentication to a device when used by a client conneting to each
gRPC service.

Perform this for both RSA and ECDSA signed CA bundles and certificates.
Perform this for the permutation of 1, 2, 10, 1000 CA
trust_bundle configurations (## indicates the 1, 2, 10, 1000 CA testdata)

  1) Load the correct key-type trust bundle onto the device and client system:
    ca-##/trust_bundle_##_rsa.pem
    ca-##/trust_bundle_##_ecdsa.pem

  2) Load the correct key-type server certificate into the DUT services:
    ca-##/server-rsa-key.pem
    ca-##/server-rsa-cert.pem
    ca-##/server-ecdsa-key.pem
    ca-##/server-ecdsa-cert.pem

  3) Load the correct key-type client certificate into the gRPC client:
    ca-##/client-rsa-key.pem
    ca-##/client-rsa-cert.pem
    ca-##/client-ecdsa-key.pem
    ca-##/client-ecdsa-cert.pem

  4) Validate that the certificate is loaded and useful for inbound connections
     to the server by clients.

  5) Have the client connect to the services on the DUT.

  6) Validate that the connection is established and the server's certificate
     is trusted by the client.

  7) Validate that the connection is established and the client's certificate is
    validated and trusted by the server.


### Certz-2.2

Perform these negative tests, perform these tests with both the RSA and ECDSA
trust_bundles and certificates.

  1) Load the correct key type trust_bundle from ca-02 on to the DUT:
       ca-02/trust_bundle_02_rsa.pem
       ca-02/trust_bundle_02_ecsda.pem

   2) Load the correct key type client certificate from the ca-01 set into
      the test gRPC client:
        ca-01/client-rsa-key.pem
        ca-01/client-rsa-cert.pem
        ca-01/client-ecdsa-key.pem
        ca-01/client-ecdsa-cert.pem

   3) Validate that the certificate is loaded and useful for outbound
      client connections. Connect to the service on the DUT.

   4) Validate that the connection to the remote device is established,
      validate the client certificate can not be used (is untrusted) by
      service on the DUT.

   5) Validate that the connection is properly torn down by the DUT.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

TODO(OCRPC): Record may not be correct or complete

```yaml
rpcs:
  gnsi:
    certz.v1.Certz.GetProfileList:
```

## Minimum DUT Platform Requirement

vRX
