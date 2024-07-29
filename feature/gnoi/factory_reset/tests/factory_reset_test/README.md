# gNOI-6.1: Factory Reset

## Summary

Performs Factory Reset

## Procedure

### Scenario 1

*   Create a sample file in the harddisk of the router using gNOI PUT RPC
*   Secure ZTP server should be up and running in the background for the router
    to boot up with the base config once factory reset command is sent on the
    box.
*   Send out Factory reset via GNOI Raw API
    *   Wait for the box to boot up via Secure ZTP
        *   The base config is updated on the box via Secure ZTP
*   Send a gNOI file STAT RPC to check if the file in the harddisk are removed
    as a part of verifying Factory reset.

### Scenario 2

*   Check startup-config file exists in mount path.
*   Perform the same steps are `Scenario 1` for startup-config file.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
rpcs:
  gnoi:
    factory_reset.FactoryReset.Start:
    file.File.Put:
    file.File.Stat:
```
