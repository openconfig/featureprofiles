# Credentialz-4: SSH Public Key Authentication

## Summary

Test that Credentialz properly configures authorized SSH public keys for a given user, and that 
the DUT properly allows or disallows authentication based on the configured settings.


## Procedure

* Set DUT TrustedUserCAKeys using gnsi.Credentialz to the previously created CA
* Set a username of `testuser` with a password of `i$V5^6IhD*tZ#eg1G@v3xdVZrQwj` using gnsi.Credentialz
* Perform the following tests and assert the expected result:
    * Case 1: Failure
        * Authenticate with the `testuser` username the previously created public key via SSH
        * Assert that authentication has failed (because the key is not authorized)
    * Case 2: Success
        * Configure the previously created ssh public key as an authorized key for the 
          `testuser` using gnsi.Credentialz/AuthorizedKeysRequest
        * Authenticate with the `testuser` username the previously created public key via SSH
        * Assert that authentication has been successful
        * Ensure telemetry values for version and created-on match the values set by
          RotateHostParameters for
          `/oc-sys:system/oc-sys:aaa/oc-sys:authentication/oc-sys:users/oc-sys:user/oc-sys:state:authorized-keys-list-version`
          and
          `/oc-sys:system/oc-sys:aaa/oc-sys:authentication/oc-sys:users/oc-sys:user/oc-sys:state:authorized-keys-list-created-on`
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