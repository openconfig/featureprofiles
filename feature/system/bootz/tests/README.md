# bootz: General bootz bootstrap tests

## Summary

Ensures the device can booted via bootz with various initial configurations

## Procedure

Each test should send the different configuration options required in a bootz request.
The device should always start with a empty configuration and start the bootstrap process.

The results should validate the expected state of the device for each configuration option set.
For negative tests the device should exit with clear message and immediately go back into the
bootz mode. At the end of the negative test cycle the test must provide a valid initial configuration
to allow the device to be restored into a valid state.

### Test Setup

1. Start bootserver test instance
2. Store bootserver IP:port to be used by DHCP server
3. Get the DUT MAC address for mgmt ports
4. Configure DHCP service with those mgmt ports to return a DHCP offer with the URI `bootz://<ip>:<port>`
    * OPTION_V4_SZTP_REDIRECT(136)
    * OPTION_V6_SZTP_REDIRECT(143)
5. Store the required device image on the bootserver
6. Store the base valid device configuration on the bootserver

### bootz-1: Validate minimum necessary bootz configuration

This test validates that the device can start in bootz mode and upon getting a bootz response from
bootserver can initialize the devices configuration into the provided configuration.

| ID        | Case  | Result |
| --------- | ------------- | --- |
| bootz-1.1 | Missing configuration  | Device fails with status invalid parameter  |
| bootz-1.2 |Invalid configuration  | Device fails with status invalid parameter  |
| bootz-1.3 |Valid configuration  | Device succeded with status ok  |

1. Provide bootstrap reponse configured as prescribed.
2. Initiate bootz boot on device via gnoi.FactoryReset()
3. Validate device sends bootz request to bootserver
4. Validate device telemetry

    * `/system/bootz/state/last-boot-attempt` is in expected state
    * `/system/bootz/state/error-count` is in incremented if failure case
    * `/system/bootz/state/status` is in expected state
    * `/system/bootz/state/checksum` matches sent proto

5. Validate device state

    * OS version is the same
    * System configuration is as expected.

### bootz-2: Validate Software image in bootz configuration

This test validates the bootz behavior based changes to software version.

| ID        | Case  | Result |
| --------- | ------------- | --- |
| bootz-2.1 | Software version is different  | Device is upgraded to the new version  |
| bootz-2.2 | Invalid software image  | Device fails with status invalid parameter  |

1. Validate the device is on a different version from the expected new version.
2. Provide bootstrap reponse configured as prescribed.
3. Initiate bootz boot on device via gnoi.FactoryReset()
4. Validate device sends bootz request to bootserver
5. Validate the progress periodically by polling `/system/bootz/state/status`
    * The status should transition from:
        * BOOTZ_UNSPECIFIED
        * BOOTZ_SENT
        * BOOTZ_RECEIVED
        * BOOTZ_OS_UPGRADE_IN_PROGRESS
        * BOOTZ_OS_UPGRADE_COMPLETE
        * BOOTZ_CONFIGURATION_APPLIED
        * BOOTZ_OK
    * For error case device should report
        * BOOTZ_UNSPECIFIED
        * BOOTZ_SENT
        * BOOTZ_RECEIVED
        * BOOTZ_OS_UPGRADE_IN_PROGRESS
        * BOOTZ_OS_INVALID_IMAGE
6. Validate device telemetry
    * `/system/bootz/state/last-boot-attempt` is in expected state
    * `/system/bootz/state/error-count` is in incremented if failure case
    * `/system/bootz/state/status` is in expected state
    * `/system/bootz/state/checksum` matches sent proto
7. Validate device state
    * OS version is the same
    * System configuration is as expected.

### bootz-3: Validate Ownership Voucher in bootz configuration

The purpose of this test is to validate that the ownership voucher can
be sent to the device and properly handled.

| ID        |Case  | Result |
| --------- | ------------- | --- |
| bootz-3.1 | No ownership voucher  | Device boots without OV present  |
| bootz-3.2 | Invalid OV  | Device fails with status invalid parameter  |
| bootz-3.3 | OV fails | Device fails with status invalid parameter |
| bootz-3.4 | OV valid | Device boots with OV installed |

1. Provide bootstrap reponse configured as prescribed.
2. Initiate bootz boot on device via gnoi.FactoryReset()
3. Validate device sends bootz request to bootserver
4. Validate the progress periodically by polling `/system/bootz/state/status`
    * The status should transition from:
        * BOOTZ_UNSPECIFIED
        * BOOTZ_SENT
        * BOOTZ_RECEIVED
        * BOOTZ_CONFIGURATION_APPLIED
        * BOOTZ_OK
    * For error case device should report
        * BOOTZ_UNSPECIFIED
        * BOOTZ_SENT
        * BOOTZ_RECEIVED
        * BOOTZ_OV_INVALID
5. Validate device telemetry
    * `/system/bootz/state/last-boot-attempt` is in expected state
    * `/system/bootz/state/error-count` is in incremented if failure case
    * `/system/bootz/state/status` is in expected state
    * `/system/bootz/state/checksum` matches sent proto
6. Validate device state
    * System configuration is as expected.

### bootz-4: Validate device properly resets if provided invalid image

The purpose of this test is to validate that when providing an invalid or
non bootable image the device properly handles this and resets itself into
bootz mode.

| ID        |Case  | Result |
| --------- | ------------- | --- |
| bootz-4.1 | no OS provided  | Device boots with existing image  |
| bootz-4.2 | Invalid OS image provided  | Device fails with status invalid parameter  |
| bootz-4.3 | failed to fetch image from remote URL | Device fails with status invalid parameter |
| bootz-4.4 | OS checksum doesn't match | Device fails with invalid parameter |

1. Provide bootstrap reponse configured as prescribed.
2. Initiate bootz boot on device via gnoi.FactoryReset()
3. Validate device sends bootz request to bootserver
4. Validate the progress periodically by polling `/system/bootz/state/status`
    * The status should transition from:
        * BOOTZ_UNSPECIFIED
        * BOOTZ_SENT
        * BOOTZ_RECEIVED
        * BOOTZ_CONFIGURATION_APPLIED
        * BOOTZ_OK
    * For error case device should report
        * BOOTZ_UNSPECIFIED
        * BOOTZ_SENT
        * BOOTZ_RECEIVED
        * BOOTZ_OS_INVALID_IMAGE
5. Validate device telemetry
    * `/system/bootz/state/last-boot-attempt` is in expected state
    * `/system/bootz/state/error-count` is in incremented if failure case
    * `/system/bootz/state/status` is in expected state
    * `/system/bootz/state/checksum` matches sent proto
6. Validate device state
    * System configuration is as expected.

### bootz-5: Validate gNSI components in bootz configuration
