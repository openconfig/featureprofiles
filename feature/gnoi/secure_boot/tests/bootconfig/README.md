# gNOI-7.1: BootConfig

## Summary

* Validate SetBootConfig and GetBootConfig rpcs for setting and getting persistent system configuration

## Procedure

* gNOI-7.1.1 : Validate ability to update the system boot configuration.

  1. Call gnoi.bootconfig.BootConfig.GetBootConfig.
  2. Validate that the returned information matches expected initial configuration
  3. Store bootconfig to be able to reset it to initial state at end of test
  4. Update the hostname of the device to `test-device`
  5. Call gnoi.bootconfig.BootConfig.SetBootConfig with a new bootconfig
  6. Call gnmi.Subscribe to `system/state/hostname`
  7. Call gnoi.bootconfig.BootConfig.GetBootConfig
  8. Validate that hostname matches the gnmi subscribe as well as the GetBootConfig
  9. Reset bootconfig to orignal state

* gNOI-7.1.2 : Validate gNSI artifacts - Credentialz

  1. Call gnoi.bootconfig.BootConfig.GetBootConfig.
  2. Validate that the returned information matches expected initial configuration
  3. Store bootconfig to be able to reset it to initial state at end of test
  4. Create new user `bootconfig-test-user` and add to oc configuration
  5. Create proto message for adding password credential for user `bootconfig-test-user`

```text proto
passwords {
  accounts {
    account: "bootconfig-test-user"
    password {
      plaintext: "test-password"
    }
  }
}
```

  6. Call gnoi.bootconfig.BootConfig.SetBootConfig with a new bootconfig
  7. Call gnoi.bootconfig.BootConfig.GetBootConfig
  8. Validate the user has a password credential set
  9. Validate that user can ssh into device.
  10. Reset bootconfig to orginal configuration

* gNOI-7.1.3 : Validate gNSI artifacts - Certz

  1. Call gnoi.bootconfig.BootConfig.GetBootConfig.
  2. Validate that the returned information matches expected initial configuration
  3. Store bootconfig to be able to reset it to initial state at end of test
  4. Create a new certificate for the tls profile in the GetBootConfig that is setup for base
      services.
  5. Create proto message for the certificate

```text proto
certz {
  profiles {
    ssl_profile_id: <profile>
    certz: {
      entities {
        certificate_chain: {
          certificate: {
            type: CERTIFICATE_TYPE_X509
            encoding: CERTIFICATE_ENCODING_PEM
            raw_certificate: <bytes>
            raw_private_key: <bytes>
          }
        }
      }
    }
  }
}
```

  6. Call gnoi.bootconfig.BootConfig.SetBootConfig with a new bootconfig
  7. Call gnoi.bootconfig.BootConfig.GetBootConfig
  8. Validate that new cert is loaded by making grpc call to device using the new cert.
  9. Reset bootconfig to orginal configuration

* gNOI-7.1.4: Validate that password set in VC is properly namespaced and cannot be set
  via gnsi.Credentialz.Rotate.

  1. Call gnoi.bootconfig.BootConfig.GetBootConfig.
  2. Validate that the returned information matches expected initial configuration
  3. Store bootconfig to be able to reset it to initial state at end of test
  4. Create a new user in the VC portion of the bootconfig.
  5. Update vc portion of bootconfig with new user `bootconfig-test-user` and test password `test-password`
  6. Call gnoi.bootconfig.BootConfig.SetBootConfig with a new bootconfig
  7. Call gnoi.bootconfig.BootConfig.GetBootConfig
  8. Build gnsi.Credentialz.Rotate for the new user with a password `temp-password`

```text proto
  #proto: RotateAccountCredentialsRequest
  password {
  accounts {
    account: "bootconfig-test-user"
    password {
      plaintext: "temp-password"
    }
  }
}
```

  9. Make call to gnsi.Credentialz.Rotate - this should fail since the vc namespace should take precedence.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  ## State Paths ##
  /system/state/hostname:
  /system/config/hostname:

rpcs:
  gnmi:
    gNMI.Subscribe:
  gnoi:
    bootconfig.BootConfig.SetBootConfig:
    bootconfig.BootConfig.GetBootConfig:
    system.System.Reboot:
```
