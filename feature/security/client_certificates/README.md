# X.X: Client Certificate

## Summary

Clients must be able to validate a remote server's TLS certificate
and present a valid client certificate to that server in order to provide
identification information. The client certificate should have a
SPIFFE Idenitifier embedded in it to be used as the identifier of
the client to the server.

## Procedure

Perform these positive tests:

Test that client certificaates from a set of one CA is validatable and usable
for authentication by a device when used by a client connecting to a gRPC service.

Perform this for both RSA and ECDSA signed CA bundles and
certificates, perform this for the permutations of 1, 2, 10, 1000 CA
trust_bundle configurations:

   1) Load the correct key-type trust bundle onto the device:
        ca-##/trust_bundle_##_rsa.pem
        ca-##/trust_bundle_##_ecdsa.pem

   2) Load the correct key-type client certificate into the test gRPC client:
        ca-##/client-rsa-key.pem
        ca-##/client-rsa-cert.pem
        ca-##/client-ecdsa-key.pem
        ca-##/client-ecdsa-cert.pem

   3) Validate that the certificate is loaded and useful for outbound
      client connections. Connect to the remote device.

   4) Validate that the connection is established and that the client's
      provided certificate is validated by the remote device.

   5) Validate that the connection's provided certificate is used for
      authentication of the connection to the remote device.

Perform these negative tests, perform these tests with both RSA and ECDSA
trust_bundles and certificates:

   1) Load the correct key type trust_bundle from ca-02 on to the device:
       ca-02/trust_bundle_02_rsa.pem
       ca-02/trust_bundle_02_ecsda.pem

   2) Load the correct key type client certificate from the ca-01 set into
      the test gRPC client:
        ca-01/client-rsa-key.pem
        ca-01/client-rsa-cert.pem
        ca-01/client-ecdsa-key.pem
        ca-01/client-ecdsa-cert.pem

   3) Validate that the certificate is loaded and useful for outbound
      client connections. Connect to the remote device.

   4) Validate that the connection to the remote device is established,
      validate the client certificate can not be used (is untrusted) by
      the device.

   5) Validate that the connection is properly torn down by the device.
   
## Config Parameter Coverage

## Telemetry Parameter Coverage

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX
