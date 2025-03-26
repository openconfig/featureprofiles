# PLT-1.1: Interface breakout Test

## Summary

Validate Interface breakout configuration.

## Procedure


*   This test is carried out for different breakout types
*   Connect DUT with ATE to all interfaces in the breakout port
*   Configure each interface with test IP addressing
*   Verify correct interface state and speed reported
*   Verify that DUT responds to ARP/ICMP on all tested interfaces

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
## Config paths
  /components/component/port/breakout-mode/groups/group/index:
    platform_type: [ "PORT" ]
  /components/component/port/breakout-mode/groups/group/config/index:
    platform_type: [ "PORT" ]
  /components/component/port/breakout-mode/groups/group/config/num-breakouts:
    platform_type: [ "PORT" ]
  /components/component/port/breakout-mode/groups/group/config/breakout-speed:
    platform_type: [ "PORT" ]
  /components/component/port/breakout-mode/groups/group/config/num-physical-channels:
    platform_type: [ "PORT" ]
rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Minimum DUT Platform Requirement

*   Breakout types - 4x100G, 2x100G and 4x10G
