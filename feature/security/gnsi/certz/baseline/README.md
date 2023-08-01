# Security Features to Test

# Certificate Related Tests

Support certificates and certificate trust bundles for identification/authentication
of endpoints in a gN\*I ecosystem. Manipulate these certificates through the Rotate()
rpc.

  1) support a server certificate on gN\*I service endpoint (/server_certificates)
     * provide basic cert signed by a CA - expect success
     * provide a cert signed by a 'not known' CA - expect load fail/service fail
     * provide a cert which is not a cert at all - expect load/service fail

  2) support loading a trust bundle (root ca set) on the gN\*I service endpoint (/trust_bundle)
     * provide single CA cert in a bundle
     * provide a set of 2 CA certs in a bundle - expect success
     * provide a set of 10 CA certs in a bundle - expect success
     * provide a set of 1000 CA certs in a bundle - expect success
     * provide a set of 10 CA certs with bogus content for some CA certs - expect <decide>
       TODO: Define what this final case should provide as an answer, ie:
         "gN\*I services fail to load"
         "gN\*I clients fail to connect"
         "device fails to load certificate bundle"

  3) support connections from clients that provide a client certificate for
       authentication/identification (/client_certificates)
    * provide a cert from a known CA and a client command that exercises that cert
        - expect success of connection and identification
    * provide a cert from an unknown CA and a client command to exercise that cert
        - expect failure of the connection


  4) support rotating the trust bundle contents (/trust_bundle_rotation)
     * make 2x of the items in 1, swap from A to B
       - expect no connection drops
       - expect new connections to use B content

  5) Support altering the TLS Profile content:
     * AddProfile - add new tls artifacts management to a network device.
     * DeleteProfile - delete a tls artifats management configuration on a network device.
     * GetProfile - Report a tls artifacts management configuration for a network device.
