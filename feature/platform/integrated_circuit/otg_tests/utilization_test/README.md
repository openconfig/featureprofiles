# gNMI-1.21: Integrated Circuit Hardware Resource Utilization Test

## Summary

Test `used-threshold-upper` configuration and telemetry for hardware resources.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Establish BGP session between ATE Port1 --- DUT Port1.

*   Get initial utilization percentages (free/(used+free) * 100) for the FIB
    resource in the system.

*   Configure DUT used-threshold-upper to 60% and used-threshold-upper-clear to
    50%.

    *   The configuration must be done at the
        [system level](https://openconfig.net/projects/models/schemadocs/yangdoc/openconfig-system.html#system-utilization-resources-resource-config)
        such that the percentages are reflected in all components using the
        resource.

*   Inject unique BGP routes such that FIB utilization increases by at-least 1%
    (250000 routes should increase utilization by at-least 1% for
    Arista/Cisco/Juniper/Nokia).

*   Get utilization percentages again and validate increase in utilization.

*   Teardown BGP session such that routes are removed from FIB.

*   Get utilization percentages again and validate decrease in utilization.

## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.
```yaml
paths:
  ## Config parameter coverage
  /system/utilization/resources/resource/config/name:
  /system/utilization/resources/resource/config/used-threshold-upper:
  /system/utilization/resources/resource/config/used-threshold-upper-clear:

  ## Telemetry parameter coverage
  /system/utilization/resources/resource/state/name:
  /system/utilization/resources/resource/state/used-threshold-upper:
  /system/utilization/resources/resource/state/used-threshold-upper-clear:
  /components/component/integrated-circuit/utilization/resources/resource/state/name:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/used:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/utilization/resources/resource/state/free:
    platform_type: ["INTEGRATED_CIRCUIT"]
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
          Mode: [ "ON_CHANGE", "SAMPLE" ]
```
