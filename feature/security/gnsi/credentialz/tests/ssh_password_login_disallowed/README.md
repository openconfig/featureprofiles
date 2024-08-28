# Credentialz-2: SSH Password Login Disallowed

## Summary

Test that Credentialz properly disallows password based SSH authentication when configured to do 
so, furthermore, ensure that certificate based SSH authentication is allowed, and properly 
accounted for. 


## Procedure

* Create a ssh CA keypair with `ssh-keygen -f /tmp/ca`
* Create a user keypair with `ssh-keygen -t ed25519`
* Sign the user public key into a certificate using the CA using `ssh-keygen -s
  /tmp/ca -I testuser -n principal_name -V +52w user.pub`. You will
  find your certificate ending in `-cert.pub`
* Set DUT TrustedUserCAKeys using gnsi.Credentialz with the CA public key
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


## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##
  /system/ssh-server/state/counters/access-rejects:
  /system/ssh-server/state/counters/last-access-reject:
  /system/ssh-server/state/counters/access-accepts:
  /system/ssh-server/state/counters/last-access-accept:

rpcs:
  gnsi:
    credentialz.v1.Credentialz.RotateAccountCredentials:
    credentialz.v1.Credentialz.RotateHostParameters:
```


## Minimum DUT platform requirement

N/A