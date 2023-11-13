# PLT-1.1: Interface breakout Test

## Summary

Validate Interface breakout configuration.

## Procedure


*   This test is carried out for different breakout types
*   Connect DUT with ATE to all interfaces in the breakout port
*   Configure each interface with test IP addressing
*   Verify correct interface state and speed reported
*   Verify that DUT responds to ARP/ICMP on all tested interfaces

## Config Parameter coverage

*   /components/component/port/breakout-mode/groups/group/index
*   /components/component/port/breakout-mode/groups/group/config
*   /components/component/port/breakout-mode/groups/group/config/index
*   /components/component/port/breakout-mode/groups/group/config/num-breakouts
*   /components/component/port/breakout-mode/groups/group/config/breakout-speed
*   /components/component/port/breakout-mode/groups/group/config/num-physical-channels


## Telemetry Parameter coverage
    *   interfaces/interface/state
    *   interfaces/interface/ethernet/stateOutput power thresholds:

## Minimum DUT Platform Requirement

*   Breakout types - 4x100G, 2x100G and 4x10G