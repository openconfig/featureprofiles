# FPGA-1.1: FPGA Status Test

## Summary

This test verifies the status of FPGA on the DUT. 

## Testbed type

* [dut.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Test environment setup

* No special setup is required. Ensure the DUT is operational.

### FPGA-1.1.1: FPGA Status Test

1. Get a list of all FPGA components on the DUT.

2. For each FPGA component, query the FPGA status using the OpenConfig path `/components/component/properties/property/state/value`.

3. Verify that the FPGA status is valid and is one of the following:
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
              "name": "fpga-status",
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
  /components/component/properties/property/state/value:
    platform_type: [FPGA]
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```

## Required DUT platform

* FFF - fixed form factor
