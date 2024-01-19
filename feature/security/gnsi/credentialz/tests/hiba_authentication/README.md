# Credentialz-5: Hiba Authentication

## Summary

Test that Credentialz properly configures (hiba) certificate authentication.


## Procedure

* Prior to writing test the following steps were taken to create appropriate keys/certs.
  * Build hiba
  * `./hiba-ca.sh -c` to create a CA keypair
  * `./hiba-ca.sh -c -u -I testuser` to create a user key pair for the testuser user
  * `./hiba-ca.sh -c -h -I dut` to create a new host key pair for the dut
  * `./hiba-gen -i -f ~/.hiba-ca/policy/identities/prod domain example.com` to create a "prod" identity
  * `./hiba-gen -f ~/.hiba-ca/policy/grants/shell domain example.com` to create a grant called "shell"
  * `./hiba-ca.sh -p -I testuser -H shell` to give the "shell" grant to the testuser
  * `./hiba-ca.sh -s -h -I dut -H prod -V +520w` to create a host certificate valid for a very long time (so we dont have to worry about it expiring and breaking the test)
  * `./hiba-ca.sh -s -u -I testuser -H shell` to create a user certificate for testuser

* Set DUT allowed authentication types to only public key using gnsi.Credentialz
* Create a user `testuser` (with no certificate at this point)
* Set the AuthorizedPrincipalsCommand by setting the tool to `TOOL_HIBA_DEFAULT`

* Perform the following tests and assert the expected result:
    * Case 1: Failure
        * Authenticate with the `testuser` username the previously created public key via SSH
        * Assert that authentication has failed (because the DUT doesnt have the Hiba host certificate at this point)
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


## Config Parameter coverage

* /gnsi/credz


## Telemetry Parameter coverage

N/A


## Protocol/RPC Parameter coverage

N/A


## Minimum DUT platform requirement

N/A