# Credentialz-5: Hiba Authentication

## Summary

Test that Credentialz properly configures (hiba) certificate authentication.


## Procedure

* Follow the instructions for setting up a [HIBA CA](https://github.com/google/hiba/blob/main/CA.md)
* Set DUT allowed authentication types to only public key using gnsi.Credentialz
* Create a user `testuser` (with no certificate at this point)
* Set the AuthorizedPrincipalsCommand by setting the tool to `TOOL_HIBA_DEFAULT`

* Perform the following tests and assert the expected result:
    * Case 1: Failure
        * Authenticate with the `testuser` username and the previously created public key via SSH
        * Assert that authentication has failed (because the DUT doesn't have the Hiba host certificate at this point)
        * Ensure that access rejects telemetry counter is incremented `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:access-rejects`
    * Case 2: Success
        * Configure the dut with the Hiba host certificate.
        * Authenticate with the `testuser` username the previously created public key via SSH
        * Assert that authentication has been successful
        * Ensure telemetry values for version and created-on match the values set by
          RotateHostParameters for
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:active-host-certificate-version`
          and
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:active-host-certificate-created-on`
        * Ensure that access accept telemetry counters are incremented after successful login
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:access-accepts`
          `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:counters:last-access-accept`


## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##
  /system/ssh-server/state/active-host-certificate-version:
  /system/ssh-server/state/active-host-certificate-created-on:
  /system/ssh-server/state/counters/access-accepts:
  /system/ssh-server/state/counters/last-access-accept:
  /system/ssh-server/state/counters/access-rejects:

rpcs:
  gnsi:
    credentialz.v1.Credentialz.RotateHostParameters:
```


## Minimum DUT platform requirement

N/A