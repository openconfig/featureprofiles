# gNMI-1.12: Telemetry: Aggregate Interface Counters

## Summary

Validate aggregate interfaces counters including both IPv4 and IPv6 counters.
Also enable the rea

## Procedure

In the automated ondatra test, verify the presence of the telemetry paths of the
following features:

*   Configure IPv4 and IPv6 addresses under aggregate subinterface:

    *   /interfaces/interface/config/enabled
    *   /interfaces/interface/config/enabled
    *   /interfaces/interface/ipv4/config/enabled
    *   /interfaces/interface/ipv6/config/enabled

    Validate that IPv4 and IPv6 addresses are enabled:

    *   /interfaces/interface/ipv4/addresses/address/state/enabled
    *   /interfaces/interface/ipv6/addresses/address/state/enabled

*   For the parent interface counters in-pkts and out-pkts:

    Check presence of packet counter paths and monitor counters every 30 sec:
    /interfaces/interface[name=aggregate port]/state/counters/in-pkts:
    /interfaces/interface[name=aggregate port]/state/counters/out-pkts:

*   Configure aggregate interfaces and assign 4 members to the aggregate interface:
    * For both static LAG
        Ensure that LAG is successfully negotiated, verifying port status for each of DUT ports 2-9 reflects expected LAG state via ATE and DUT telemetry.
        Ensure that below status with minimum links configuration on LAG:
            Down when min-1 links are up
            Up when min links are up
            Up when >min links are up
        Poll the counters of the bundle interface and make sure all the counters
        are populated successfully

    * Now delete the bundle interface and reconfigure the bundle interface
        Ensure counters of the bundle interface are now populated successfully.
        /interfaces/interface/config/enabled

*   Interface counters:
    Check the presence of packet counter paths:
    /interfaces/interface[name=aggregate port name]/state/counters/in-pkts:
    /interfaces/interface[name=aggregate_port_name]/state/counters/out-pkts:
    /interfaces/interface[name=aggregate_port_name]/state/counters/in-octets:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/out-octets:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/in-unicast-pkts:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/in-broadcast-pkts:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/in-multicast-pkts:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/in-errors:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/in-discards:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/out-unicast-pkts:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/out-broadcast-pkts:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/out-multicast-pkts:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/out-errors:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/out-discards:
    /interfaces/interface[name=aggregate_port_name]/state/counters/state/counters/last-clear:

*   Interface CPU and management
    Check the presence of CPU and management paths:

    *   TODO: /interfaces/interface/state/cpu
    *   TODO: /interfaces/interface/state/management

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /interfaces/interface/config/enabled:
  /interfaces/interface/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/ethernet/config/duplex-mode:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/aggregation/config/min-links:

  ## State Paths ##
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/in-broadcast-pkts:
  /interfaces/interface/state/counters/in-multicast-pkts:
  /interfaces/interface/state/counters/in-errors:
  /interfaces/interface/state/counters/in-discards:
  /interfaces/interface/state/counters/out-unicast-pkts:
  /interfaces/interface/state/counters/out-broadcast-pkts:
  /interfaces/interface/state/counters/out-multicast-pkts:
  /interfaces/interface/state/counters/out-errors:
  /interfaces/interface/state/counters/out-discards:
  /interfaces/interface/state/counters/last-clear:
  /interfaces/interface/state/cpu:
  /interfaces/interface/state/management:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-discarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-discarded-pkts:
rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```
## Minimum DUT platform requirement

N/A

