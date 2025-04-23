# Credentialz-4: SSH Public Key Authentication

## Summary

Test that Credentialz properly configures authorized SSH public keys for a given user, and that
the DUT properly allows or disallows authentication based on the configured settings.


## Procedure

* Create a user ssh keypair with `ssh-keygen`
* Set a username of `testuser`
* Perform the following tests and assert the expected result:
    * Case 1: Failure
        * Attempt to ssh into the server with the `testuser` username, presenting the ssh key.
        * Assert that authentication has failed (because the key is not authorized)
    * Case 2: Success
        * Configure the previously created ssh public key as an authorized key for the
          `testuser` using gnsi.Credentialz/AuthorizedKeysRequest
        * Authenticate with the `testuser` username and the previously created public key via SSH
        * Assert that authentication has been successful
        * Ensure telemetry values for version and created-on match the values set by
          RotateHostParameters for
          `/oc-sys:system/oc-sys:aaa/oc-sys:authentication/oc-sys:users/oc-sys:user/oc-sys:state:authorized-keys-list-version`
          and
          `/oc-sys:system/oc-sys:aaa/oc-sys:authentication/oc-sys:users/oc-sys:user/oc-sys:state:authorized-keys-list-created-on`
        * Ensure that access accept telemetry counters are incremented
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:access-accepts`
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:last-access-accept`


## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##
  /system/aaa/authentication/users/user/state/authorized-keys-list-version:
  /system/aaa/authentication/users/user/state/authorized-keys-list-created-on:
  /system/ssh-server/state/counters/access-accepts:
  /system/ssh-server/state/counters/last-access-accept:

rpcs:
  gnsi:
    credentialz.v1.Credentialz.RotateAccountCredentials:
```


## Minimum DUT platform requirement

N/A