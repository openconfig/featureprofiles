# Credentialz-2: SSH Password Login Disallowed

## Summary

Test that Credentialz properly disallows password based SSH authentication when configured to do 
so, furthermore, ensure that certificate based SSH authentication is allowed, and properly 
accounted for. 


## Procedure

* Prior to writing test the following steps were takin to create a CA key pair, a user key pair, 
  and to create a signed public key for the user key from the CA. Note that the lifetime of the 
  certificate was set to "forever" and there are no passphrases on the keys. The principal on 
  the certificate has been set to `my_principal`.
    * `cd` to this test package
    * `ssh-keygen -t ed25519 -f ca -C ca`
    * `ssh-keygen -t ed25519 -f id_ed25519 -C featureprofile@openconfig`
    * `ssh-keygen -s ca -I testuser -n my_principal -V -1m:forever id_ed25519.pub`

* Set DUT TrustedUserCAKeys using gnsi.Credentialz to the previously created CA
* Set a username of `testuser` with a password of `i$V5^6IhD*tZ#eg1G@v3xdVZrQwj` using gnsi.Credentialz
* Set DUT authentication types to permit only public key (PUBKEY) using gnsi.Credentialz
* Set DUT authorized_users for `testuser` with a principal of `my_principal` (configured above 
  when signing public key)
* Perform the following tests and assert the expected result:
    * Case 1: Failure
        * Authenticate with the `testuser` username and password `i$V5^6IhD*tZ#eg1G@v3xdVZrQwj` 
          via SSH
        * Assert that authentication has failed
        * Ensure that access failure telemetry counters are incremented
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:access-rejects`
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:last-access-reject` 
    * Case 2: Success
        * Authenticate with the `testuser` username and password of `i$V5^6IhD*tZ#eg1G@v3xdVZrQwj` 
          via console
        * Assert that authentication has been successful (password authentication was only 
          disallowed for SSH)
        * Ensure that access accept telemetry counters are incremented
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:access-accepts`
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:last-access-accept`
    * Case 3: Success
        * Authenticate with the `testuser` and certificate created above
        * Assert that authentication has been successful
        * Assert that gnsi accounting recorded the principal (`my_principal`) from the 
          certificate rather than the SSH username (`testuser`)
        * Ensure that access accept telemetry counters are incremented
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:access-accepts`
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:last-access-accept`


## Config Parameter coverage

* /gnsi/credz


## Telemetry Parameter coverage

N/A


## Protocol/RPC Parameter coverage

N/A


## Minimum DUT platform requirement

N/A