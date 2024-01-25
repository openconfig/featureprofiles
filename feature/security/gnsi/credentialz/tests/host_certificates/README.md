# Credentialz-3: Host Certificates

## Summary

Test that Credentialz can properly fetch and push SSH host certificates, and that the DUT sends 
this certificate during SSH authentication.


## Procedure

* Fetch the DUT's public key using gnsi.Credentialz
  * If DUT doesnt have one, generate and set the private key using gnsi.Credentialz.
* Sign the DUT's public key with the ca key to create a host certificate.
* Add the newly created certificate to the DUT using gnsi.Credentialz
* Perform the following tests and assert the expected result:
    * Case 1: Success
        * SSH to the device and assert that the host key returned is the host key that was 
          pushed in the test set up
        * Ensure telemetry values for version and created-on match the values set by
            RotateHostParameters for
            `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:active-host-certificate-version`
            and
            `/oc-sys:system/oc-sys:ssh-server/oc-sys:state:active-host-certificate-created-on`


## Config Parameter coverage

* /gnsi/credz


## Telemetry Parameter coverage

N/A


## Protocol/RPC Parameter coverage

N/A


## Minimum DUT platform requirement

N/A