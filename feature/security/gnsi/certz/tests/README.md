# gNSI TLS Authentication Testing

## Summary
Test gRPC TLS authentication under various conditions created using
combinations of different security credentials and artifacts manageable using
the gNSI protocol, including device certificates, Trust-Bundle (TB),
Certificate Revocation List (CRL) and Authentication Policy.

## Authentication Policy Enforcement

Test device enforcement of
[`gnsi.certz.v1.AuthenticationPolicy`](https://github.com/openconfig/gnsi/blob/main/certz/certz.proto)
against clients connecting with different certificates.  Repeat all test cases
for every TLS secured service.

Testing of `AuthenticationPolicy` rotation over the gNSI protocol is not within
the scope of this section.

### Input Args {#input-args}

* `testdata/leaf_cert.pem`: valid client certificate
* `testdata/leaf_key.pem`: private key for `leaf_cert.pem`
* `testdata/bad_leaf_cert_1.pem`: client certificate issued by unauthorized issuer
* `testdata/bad_leaf_key_1.pem`: private key for `bad_leaf_cert_1.pem`
* `testdata/bad_leaf_cert_2.pem`: client certificate containing inconsistent trust domains
* `testdata/bad_leaf_key_2.pem`: private key for `bad_leaf_cert_2.pem`
* `testdata/bad_leaf_cert_3.pem`: client certificate containing inconsistent realm
* `testdata/bad_leaf_key_3.pem`: private key for `bad_leaf_cert_3.pem`

### DUT service setup

Setup all gRPC services of DUT with the followings:
* `testdata/leaf_cert.pem`: server certificate
* `testdata/leaf_key.pem`: private key for `leaf_cert.pem`
* `testdata/root_cert.pem`: trust bundle
* `testdata/policy_1.pb`: authentication policy that client certificates are tested against
* `loas3_disable_realm_consistency_check`: Server flag set according to [Tests](#tests) below.

The following files are included for debugging purposes only:
* `testdata/policy_1.textproto`: textual format of `policy_1.pb`
* `testdata/root_key.pem`: private key for `root_cert.pem` used to sign all leaf certificates

All DUT services should also be configured with Authorization Policy that will grant
access to the clients' identities as minted in their certificates.

### Tests {#tests}

Setup clients to call every test service with the certificate/private-key pairs
from [Input Args](#input-args) and `root_cert.pem` as trust bundle.

Client Cert/Key | Server Realm Check | Test Expectation
:-------------- | ------------------ | ----------------
leaf       | enabled  | RPC returns Ok
bad leaf 1 | enabled  | !Ok due to signer unauthorized to sign for the certificate's role
bad leaf 2 | enabled  | !Ok due to mismatch of cert's synthetic and SPIFFE ID trust domains
bad leaf 3 | enabled  | !Ok due to inconsistent security realm
leaf       | disabled | RPC returns Ok
bad leaf 1 | disabled | !Ok due to signer unauthorized to sign for the certificate's role
bad leaf 2 | disabled | !Ok due to mismatch of cert's synthetic and SPIFFE ID trust domains
bad leaf 3 | disabled | RPC returns Ok
