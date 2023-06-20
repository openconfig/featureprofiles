# X.X: Trust Bundle

## Summary

Server and client TLS endpoints use x.509 certificates for
identification of the calling or called endpoint. Systems
could use self-signed certificates and not validate, but
this is and insecure practice.

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
point in time.)

A trust bundle may have one or more certificates contained in
it, systems should be able to support at least one thousand
CA keys in such a bundle.


## Procedure

Load the example trust-bundles on to a system, test that
certificates issued by the constituent CAs are validated by
the system. Test that the example trust bundles, which contain
one, ten, one thousand CA keys all operate nominally on the
system.

Trust bundles, server and client certificates are provided
to be used in testing this feature. 

## Config Parameter Coverage

## Telemetry Parameter Coverage

## Protocol/RPC Parameter Coverage

None

## Minimum DUT Platform Requirement

vRX
