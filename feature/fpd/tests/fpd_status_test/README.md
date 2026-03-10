# FPD-1.1: FPD Status Test

## Summary

This test verifies the status of FPD on the DUT. 

> **NOTE**: There is no OpenConfig component for FPD. The test assumes that FPD Statuses are available at `/openconfig/components/component/properties/property/state/value`.

## Testbed type

* [dut.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Test environment setup

* No special setup is required. Ensure the DUT is operational.

### FPD-1.1.1: FPD Status Test

1. Get a list of all FPD components on the DUT.

> **NOTE**: As FPD is not a standard OpenConfig component, the test queries vendor-specific paths to retrieve FPD information. Component names are formatted as `{location}_{fpd-name}`.

2. For each FPD component, query the FPD status using the OpenConfig path `/openconfig/components/component/properties/property/state/value`.

3. Verify that the FPD status is valid and is one of the following:
   * `CURRENT`
   * `NEED UPGD`
   * `RLOAD REQ`
   * `NOT READY`
   * `UPGD DONE`
   * `UPGD FAIL`
   * `BACK IMG`
   * `N/A`

#### Canonical OC

```json
{
  "components": {
    "component": [
      {
        "name": "0/RP0/CPU0_Bios",
        "properties": {
          "property": [
            {
              "name": "fpd-status",
              "state": {
                "value": "CURRENT"
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  # Telemetry Parameter coverage
  /components/component/properties/property/state/value:
    platform_type: ["FPD"]
```

## Required DUT platform

* FFF - fixed form factor