# gNSI Client Certificate Tests

## Summary

Clients must be able to validate a remote server's TLS certificate
and present a valid client certificate to that server in order to provide
identification information. The client certificate should have a
SPIFFE Idenitifier embedded in it to be used as the identifier of
the client to the server.

## Baseline Setup

### Input Args

* the set of certificate testdata generated with the mk_cas.sh
   script in featureprofiles/feature/security/gnsi/certz/test_data

### DUT service setup

Configure the DUT to enable the following services (that are using gRPC) are
up and using mTLS for authentication:

* gNMI
* gNOI
* gNSI
* gRIBI
* P4RT

Be prepared to load the relevant trust_bundle.pem file for each test
Certificate Authority(CA) under test on the DUT. Each CA has a RSA and ECDSA
form, both must be tested.

## Tests

### Certz-1.1

Perform these positive tests:

Test that client certificates from a set of one CA are able to be validated and
used for authentication to a device when used by a client connecting to each
gRPC service.

Perform this for both RSA and ECDSA signed CA bundles and
certificates.
Perform this for the permutations of 1, 2, 10, 1000 CA
trust_bundle configurations: (## indicates the 1, 2, 10, 1000 CA testdata)

   1) Load the correct key-type trust bundle onto the device and client system:
        ca-##/trust_bundle_##_rsa.pem
        ca-##/trust_bundle_##_ecdsa.pem

   2) Load the correct key-type client certificate into the test gRPC client:
        ca-##/client-rsa-key.pem
        ca-##/client-rsa-cert.pem
        ca-##/client-ecdsa-key.pem
        ca-##/client-ecdsa-cert.pem

   3) Load the correct key-type server certificate into the services on the DUT:
        ca-##/server-rsa-key.pem
        ca-##/server-rsa-cert.pem
        ca-##/server-ecdsa-key.pem
        ca-##/server-ecdsa-cert.pem

   4) Validate that the certificate is loaded and useful for outbound
      client connections.
      
   5) Connect to the service on the DUT.

   6) Validate that the connection is established and that the client's
      provided certificate is validated by the service on the DUT.

   7) Validate that the connection's provided certificate is used for
      authentication of the connection to the service on the DUT.

### Certz-1.2

Perform these negative tests:

This test should show that the trust-bundle/CA-set being mis-matched
between client and server results in failed connections.

Perform these tests with both RSA and ECDSA trust_bundles and
certificates:

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
   
## Config Parameter Coverage

## Telemetry Parameter Coverage

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX
