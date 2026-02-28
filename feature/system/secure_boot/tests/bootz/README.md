# bootz: General bootz bootstrap tests

## Summary

Ensures the device can be booted via bootz with various initial configurations.

## Prerequisite

The test assumes a DHCP server is running and configured to redirect the DUT to the bootz server URL specified via the `-bootz_addr` flag. The test launches a bootz server listening on the same URL.

## General Procedure

1. The test initializes the bootz server with a bootstrap response based on the test scenario.
2. The test initiates bootz via a gNOI factory reset request (zero fill?).
3. The test validates that the device has sent a BootStrapRequest to the server.
4. The test validates that the server has received ReportStatusRequest(s) with BOOTSTRAP_STATUS_INITIATED and CONTROL_CARD_STATUS_UNINITIALIZED for each controller card.
5. The test validates that the server has received ReportStatusRequest(s) with BOOTSTRAP_STATUS_FAILURE for a negative test, or BOOTSTRAP_STATUS_SUCCESS for a positive test. In addition, a ReportStatusRequest with CONTROL_CARD_STATUS_INITIALIZED for each controller card is expected for a positive test.
6. The test validates the telemetry:
    * `/system/bootz/state/last-boot-attempt` is in the expected state
    * `/system/bootz/state/error-count` is incremented in the failure case
    * `/system/bootz/state/status` is in the expected state
    * `/system/bootz/state/checksum` matches the sent proto
7. The test validates the software image version against the expected version.

For negative tests, the device should exit with a clear message and immediately go back into
bootz mode. At the end of the negative test cycle, the test must provide a valid initial configuration
to allow the device to be restored to a valid state.

### Test Setup

### bootz-1: Validate minimum necessary bootz configuration

This test validates that the device can bootstrap using a minimal vendor configuration.

| ID        | Case  | Result |
| --------- | ------------- | --- |
| bootz-1.1 | Missing configuration  | BOOTZ_CONFIGURATION_INVALID  |
| bootz-1.2 | Invalid configuration  | BOOTZ_CONFIGURATION_INVALID  |
| bootz-1.3 | Valid configuration  | BOOTZ_OK  |

For each case, validate whether the configuration is loaded/rejected as expected.

### bootz-2: Validate OC configuration

This test validates that the device properly handles OpenConfig configuration in addition
to vendor config.

| ID        | Case  | Result |
| --------- | ------------- | --- |
| bootz-2.1 | Invalid OC config  | BOOTZ_CONFIGURATION_INVALID  |
| bootz-2.2 | Valid OC config  | BOOTZ_OK  |

For each case, validate whether the configuration is loaded or rejected as expected. Verify
that the OC config is present.

### bootz-3: Validate basic gNSI artifacts
This test validates that the device properly handles basic gNSI artifacts.

| ID        | Case  | Result |
| --------- | ------------- | --- |
| bootz-3.1 | Invalid AuthZ config  | BOOTZ_CONFIGURATION_INVALID  |
| bootz-3.2 | Valid AuthZ config  | BOOTZ_OK  |
| bootz-3.3 | Invalid PathZ config  | BOOTZ_CONFIGURATION_INVALID  |
| bootz-3.4 | Valid PathZ config  | BOOTZ_OK  |
| bootz-3.5 | Invalid CredZ config  | BOOTZ_CONFIGURATION_INVALID  |
| bootz-3.6 | Valid CredZ config  | BOOTZ_OK  |
| bootz-3.7 | Invalid CertZ config  | BOOTZ_CONFIGURATION_INVALID  |
| bootz-3.8 | Valid CertZ config  | BOOTZ_OK  |


For each case, validate whether the configuration is loaded or rejected as expected using the appropriate probe request.

### bootz-4: Validate Ownership Voucher in bootz configuration
The purpose of this test is to validate that the device properly handles ownership voucher
validation. Note that for negative test cases, we do not expect a ReportStatusRequest.

| ID        |Case  | Result |
| --------- | ------------- | --- |
| bootz-4.1 | No OV  | BOOTZ_OV_INVALID  |
| bootz-4.2 | No Standby OV  | BOOTZ_OV_INVALID  |
| bootz-4.3 | Invalid OV | BOOTZ_OV_INVALID |
| bootz-4.4 | Invalid Standby OV | BOOTZ_OV_INVALID |
| bootz-4.5 | Wrong OV | BOOTZ_OV_INVALID |
| bootz-4.6 | Wrong Standby OV | BOOTZ_OV_INVALID |
| bootz-4.7 | Invalid OV Bundle | BOOTZ_OV_INVALID |
| bootz-4.8 | Valid OV | BOOTZ_OK |
| bootz-4.9 | Valid OV Bundle | BOOTZ_OK |


### bootz-5: Validate Software image in bootz configuration

This test validates the bootz behavior based on changes to the software version.

| ID        | Case  | Result |
| --------- | ------------- | --- |
| bootz-5.1 | Invalid image url  | BOOTZ_OS_INVALID_IMAGE  |
| bootz-5.2 | Corrupt image  | BOOTZ_OS_INVALID_IMAGE  |
| bootz-5.3 | Wrong image hash  | BOOTZ_OS_INVALID_IMAGE  |
| bootz-5.4 | Missing image hash  | BOOTZ_OS_INVALID_IMAGE  |
| bootz-5.5 | Missing image version  | BOOTZ_OS_INVALID_IMAGE  |
| bootz-5.6 | Image version matching installed  | BOOTZ_OK  |
| bootz-5.7 | Image version differs from installed  | BOOTZ_OK  |

### bootz-6: Validate credz credentials

This test validates that login is possible with configured CredZ credentials.

| ID        | Case  | Result |
| --------- | ------------- | --- |
| bootz-6.1 | Credz password for invalid user  | BOOTZ_CONFIGURATION_INVALID  |
| bootz-6.2 | Credz key for invalid user  | BOOTZ_CONFIGURATION_INVALID  |
| bootz-6.3 | Credz plain text password   | BOOTZ_CONFIGURATION_INVALID  |
| bootz-6.4 | Credz with md5 password | BOOTZ_OK  |
| bootz-6.5 | Credz with sha2 password | BOOTZ_OK  |
| bootz-6.6 | Credz key | BOOTZ_OK  |

Validate that the user can login with specified password/key.

### bootz-7: Validate gNSI artifacts persistence

This test validates that gNSI artifacts loaded during bootz persist after common
triggers.

1. Prepare a bootstrap response with AuthZ, PathZ, CredZ, and CertZ policies.
2. Initiate bootz and ensure success.
3. Validate policies are loaded using the corresponding probe request.
4. Initiate a switchover.
5. Validate the policies still exist.
6. Initiate a switchover.
7. Validate the policies still exist.
8. Initiate a reload.
9. Validate the policies still exist.

After each trigger, make sure the CredZ passwords/keys can be used to authenticate.

### bootz-9: Validate Config immutability
This test ensures that vendor config loaded during bootz cannot be overridden except through credz.

1. Prepare a bootstrap response with:
    * Vendor config with a user "user1" and password "pass1"
2. Initiate bootz and ensure successful completion.
3. Verify "user1" can login with "pass1".
4. Using gNMI, set the password for "user1" to "pass2".
5. Verify "user1" can still login with "pass1" and cannot login with "pass2".
6. Using gNSI, load a CredZ policy that sets the "user1" password to "pass3".
7. Verify "user1" can login with "pass3" and cannot login with "pass1".
8. Perform a switchover.
9. Verify "user1" can login with "pass3" and cannot login with "pass1".
10. Perform a reload.
11. Verify "user1" can login with "pass3" and cannot login with "pass1".

## OpenConfig Path and RPC Coverage


```yaml
paths:
  /system/bootz/state/last-boot-attempt:
  /system/bootz/state/error-count:
  /system/bootz/state/status:
  /system/bootz/state/checksum:
rpcs:
  gnmi:
    gNMI.Subscribe:
      on_change: true
  gnoi:
    bootconfig.BootConfig.GetBootConfig:
    bootconfig.BootConfig.SetBootConfig:
  bootz:
    Bootstrap.GetBootstrapData:
    Bootstrap.ReportStatus:
```