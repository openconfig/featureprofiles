# Security Features to Test

# Certificate Related Tests

Support certificates and certificate trust bundles for identification/authentication
of endpoints in a gN\*I ecosystem.

  1) support a server certificate on gN\*I service endpoint
     provide basic cert signed by a CA - expect success
     provide a cert signed by a 'not known' CA - expect load fail/service fail
     provide a cert which is not a cert at all - expect load/service fail

  2) support loading a trust bundle (root ca set) on the gN\*I service endpoint
     provide basic CA cert in a bundle
     provide a set of 20 CA certs in a bundle - expect success
     provide a set of 1000 CA certs in a bundle - expect success
     provide a set of 20 CA certs with bogus content for some CA certs - expect <decide>

  3) support connections from clients that provide a client certificate for authen/identification
    provide a cert from a known CA and a client command that exercises that cert
        - expect success of connection and identification (hopefully)
    provide a cert from an UNknown CA and a client command to exercise that cert
        - expect failure of the connection


  4) support swapping/rotating the trust bundle contents
     make 2x of the items in 1, swap from A to B, expect no connection drops
                                                  expect new connections use B content

