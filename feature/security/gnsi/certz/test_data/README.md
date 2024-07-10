# Certificate Authority Test Data

Creation of test data for use in TLS tests.

## Content

   * mk_cas.sh - create sets of Certificate Authority(CA) content for tests.
   * cleanup.sh - clean out the CA content for recreation efforts.
   * ca-01 - a single CA set where signatures are RSA or ECDSA.
   * ca-02 - a set of two CAs where signatures are RSA or ECDSA.
   * ca-10 - a set of ten CAs where signatures are RSA or ECDSA.
   * ca-1000 - a set of one thousand CAs where signatures are RSA or ECDSA.
   * server_cert.cnf/server_cert_ext.cnf - server openssl profile configuration
   * client_cert.cnf/client_cert_ext.cnf - client openssl profile configuration

Each CA set includes, for both RSA and ECDSA signature types:
  * CA key
  * CA public certificate
  * client key, certificate request, certificate
  * server key, certificate request, certificate
  * CA trust bundle

NOTE: Creation of bad data has not been completed yet.
