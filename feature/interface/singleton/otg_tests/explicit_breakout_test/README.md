# RT-5.1: Singleton Interface

## Summary

Singleton L3 interface (non-LAG) is supported on DUT.

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/dut_400.testbed

## Procedure

The port should a 400G interface

### RT-5.1.4 [TODO: https://github.com/openconfig/featureprofiles/issues/2338]
#### Breakout must be explicitly configured by gNMI client

*   On DUT Port-1 with a QSFP-DD 400GBASE-DR4 transceiver inserted
*   Ensure no breakout is configured
*   Set Port-1 port-speed to 100G
    *   /interfaces/interface/ethernet/config/port-speed
*   Validate that the DUT does not create breakouts implicitly and does not set the breakout speed
    *   /components/component/port/breakout-mode/groups/group/config
    *   /components/component/port/breakout-mode/groups/group/config/index
    *   /components/component/port/breakout-mode/groups/group/config/breakout-speed
    *   /components/component/port/breakout-mode/groups/group/state
    *   /components/component/port/breakout-mode/groups/group/state/index
    *   /components/component/port/breakout-mode/groups/group/state/breakout-speed
*   Validate the port state changes to "DOWN"
    *   /interfaces/interface/state/oper-status

### RT-5.1.5 [TODO: https://github.com/openconfig/featureprofiles/issues/2338]
#### Setting port-speed on interface that have breakout configured should not be allowed

*   Configure a breakout on Port-1 to 4x100 Gig
    *   /components/component/port/breakout-mode/groups/group/config
*   Try to set port speed of Port-1 to 100G
    *   /interfaces/interface/ethernet/config/port-speed
*   Validate the port-speed is rejected
    *   Since a breakout port is not expected to support port-speed, verify the gNMI Set operation is rejected
    *   /interfaces/interface/ethernet/state/port-speed

### RT-5.1.6 [TODO: https://github.com/openconfig/featureprofiles/issues/2338]
#### Remove breakout and interface config to delete the interface config

*   Using a single gNMI Replace, remove the DUT port-1 and its breakout config
*   Ensure the gNMI Replace is successful and configuration for DUT port-1 including its breakout is removed
    *   /interfaces/interface/ethernet/state/ - holds default value
    *   /components/component/port/breakout-mode/groups/group/state
