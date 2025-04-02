# RT-5.1: Singleton Interface

## Summary

Singleton L3 interface (non-LAG) is supported on DUT.

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

### Sub Test 1

For each port speed and breakout port configuration that need to be tested, add
a new testbed configuration with the desired port types.

* Configure ATE port-1 connected to DUT port-1, and ATE port-2 connected to
    DUT port-2, with the relevant IPv4 and IPv6 addresses.
* Configure static MAC address to be 02:1a:WW:XX:YY:ZZ where WW:XX:YY:ZZ are
    the octets of IPv4.
  * Ensure: ARP discovers static MAC address specified when port is
        configured with static MAC.

### RT-5.1.1 - singleton interface verification:

* Validate that port speed is reported correctly and that port telemetry
    matches expected negotiated speeds for forced, auto-negotiation, and
    auto-negotiation while overriding port speed and duplex.
  * TODO: If the port is a breakout, ensure that all breakout ports are
        correctly reported.
* For IPv4 and IPv6:
  * With traffic flow from ATE port-1 to ATE port-2, ensure:
    * For MTUs [^1] of 1500, 5000, 9236:
      * Packets with size greater than the configured MTU with DF-bit
                set are not transmitted.
      * Packets with size of configured MTU are received.
      * Packets with size less than the configured MTU are received.
### Sub Test 2 [TODO: https://github.com/openconfig/featureprofiles/issues/2148]
Verify that interface packet counters are properly incremented in every streaming telemetry report
* Subscribe to all interface counters of DUT Port1 and DUT Port 2 and with 30s interval.
* Generate IPv4 and IPv6  traffic flow of packet size 9005B and IPv4 Don't Fragment bit set,  from ATE port-1 to ATE port-2 at constant rate, ensure:
    * Set Port2 MTUs [^1] of 9000
    * Verify that  for period of 300seconds difference between 2 consecutive reports of below counters for DUT's Port1 and Port2 is constant and:
      * `/interfaces/interface[name=Port2]/state/counters/out-pkts` = 0
      * `/interfaces/interface[name=Port2]/state/counters/out-octets` = 0
      * `/interfaces/interface[name=Port2]/state/counters/out-discards` > 0
      * `/interfaces/interface[name=Port1]/state/counters/in-pkts` > 0
      * `/interfaces/interface[name=Port1]/state/counters/in-octets`> 0
    * Verify that  for period of 300seconds values returned by below couters for DUT's Port1 and Port2 are constatnt:
      * `/interfaces/interface[name=Port1]/state/out-rate`
      * `/interfaces/interface[name=Port1]/state/in-rate` > 0
      * `/interfaces/interface[name=Port2]/state/out-rate`= 0
      * `/interfaces/interface[name=Port2]/state/in-rate` = 0
### Sub Test 3 [TODO: https://github.com/openconfig/featureprofiles/issues/2148]
Verify that interface packet counters are properly incremented in every streaming telemetry report
* Subscribe to all interface counters of DUT Port1 and DUT Port 2 and with 30s interval.
* Generate IPv4 and IPv6  traffic flow of packet size 4000B and IPv4 Don't Fragment bit set,  from ATE port-1 to ATE port-2 at constant rate, ensure:
    * Set Port2 MTUs [^1] of 9000
    * Verify that  for period of 300seconds difference between 2 consecutive reports of below counters for DUT's Port1 and Port2 is constant and:
      * `/interfaces/interface[name=Port2]/state/counters/out-octets` > 0
      * `/interfaces/interface[name=Port2]/state/counters/out-discards` > 0
      * `/interfaces/interface[name=Port1]/state/counters/in-pkts` > 0
      * `/interfaces/interface[name=Port1]/state/counters/in-octets`> 0
    * Verify that  for period of 300seconds values returned by below couters for DUT's Port1 and Port2 are constatnt, and that Port1 `in-rate` is equal to Port2 `out-rate`
      * `/interfaces/interface[name=Port1]/state/out-rate` = 0
      * `/interfaces/interface[name=Port1]/state/in-rate` > 0
      * `/interfaces/interface[name=Port2]/state/out-rate`> 0
      * `/interfaces/interface[name=Port2]/state/in-rate` = 0
      

[^1]: The MTU specified above refers to the L3 MTU, which is the payload portion
    of an Ethernet frame.

### RT-5.1.2 - link flaps:

* Bring down the physical layer of ATE port-1, and bring it back up.
    Repeat this a few times (minimum 2)
  * Verify that the interface goes down by checking the physical state on DUT/ATE.
  * Verify that the interface is back up by checking the physical state on DUT/ATE.
  * Ensure that the number of interface state changes are accurately
            captured in the OC path.
  * Verify that the traffic flow from ATE port-1 to ATE port-2 is
            now working after the interface is back up.

### RT-5.1.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2320]
####  Subscribe to interface counters with SAMPLE mode:

*   Run the test twice, once with a SAMPLE interval of 10 Seconds and once again
    with a SAMPLE interval of 15 seconds
*   Initiate traffic
*   Counters should be verified using gNMI subscribe with sample mode for inbound port (DUT port-1):
    *   /interfaces/interface/state/counters/in-unicast-pkts
    *   /interfaces/interface/state/counters/in-broadcast-pkts
    *   /interfaces/interface/state/counters/in-multicast-pkts
    *   /interfaces/interface/state/counters/in-octets
    *   /interfaces/interface/state/counters/in-discards
    *   /interfaces/interface/state/counters/in-errors
    *   /interfaces/interface/state/counters/in-fcs-errors
*   Counters should be verified using gNMI subscribe with sample mode for outbound port (DUT port-2):
    *   /interfaces/interface/state/counters/out-unicast-pkts
    *   /interfaces/interface/state/counters/out-broadcast-pkts
    *   /interfaces/interface/state/counters/out-multicast-pkts
    *   /interfaces/interface/state/counters/out-octets
    *   /interfaces/interface/state/counters/out-errors
    *   /interfaces/interface/state/counters/out-discards
*   Ensure inbound and outbound unicast counters are the same
*   Ensure counters increment at the selected SAMPLE interval

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
    *   /interfaces/interface/ethernet/state/
    *   /components/component/port/breakout-mode/groups/group/state

## Config Parameter Coverage

* /interfaces/interface/config/name
* /interfaces/interface/config/description
* /interfaces/interface/config/enabled
* /interfaces/interface/subinterfaces/subinterface/ipv4/config/mtu
* /interfaces/interface/subinterfaces/subinterface/ipv6/config/mtu
* /interfaces/interface/subinterfaces/subinterface/ipv4/unnumbered/config/enabled
* /interfaces/interface/config/id
* /interfaces/interface/ethernet/config/mac-address
* /interfaces/interface/ethernet/config/port-speed
* /interfaces/interface/ethernet/config/duplex-mode

## Telemetry Parameter Coverage

* /interfaces/interface/ethernet/state/counters/in-mac-pause-frames
* /interfaces/interface/ethernet/state/counters/out-mac-pause-frames
* /interfaces/interface/ethernet/state/mac-address
* /interfaces/interface/state/counters/in-broadcast-pkts
* /interfaces/interface/state/counters/in-discards
* /interfaces/interface/state/counters/in-errors
* /interfaces/interface/state/counters/in-multicast-pkts
* /interfaces/interface/state/counters/in-octets
* /interfaces/interface/state/counters/in-unicast-pkts
* /interfaces/interface/state/counters/in-unknown-protos
* /interfaces/interface/state/counters/out-broadcast-pkts
* /interfaces/interface/state/counters/out-discards
* /interfaces/interface/state/counters/out-errors
* /interfaces/interface/state/counters/out-multicast-pkts
* /interfaces/interface/state/counters/out-octets
* /interfaces/interface/state/counters/out-pkts
* /interfaces/interface/state/counters/out-unicast-pkts
* /interfaces/interface/subinterfaces/subinterface/ipv4/state/mtu
* /interfaces/interface/subinterfaces/subinterface/ipv6/state/mtu
* /interfaces/interface/subinterfaces/subinterface/ipv4/unnumbered/state/enabled
* /interfaces/interface/state/oper-status
* /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/ip
* /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts
* /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts
* /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/ip
* /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-discarded-pkts
* /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts
* /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-discarded-pkts
* /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts
* /interfaces/interface/ethernet/state/aggregate-id
* /interfaces/interface/ethernet/state/port-speed
* /interfaces/interface/state/admin-status
* /interfaces/interface/state/counters/out-octets
* /interfaces/interface/state/description
* /interfaces/interface/state/type
* /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-pkts
* /interfaces/interface/state/hardware-port
* /interfaces/interface/state/id
* /interfaces/interface/state/counters/in-fcs-errors
* /interfaces/interface/state/counters/carrier-transitions

## Protocol/RPC Parameter Coverage

* gNMI
  * Get
  * Subscribe

## Minimum DUT Platform Requirement

vRX

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPC intended to be covered by this test.

```yaml
paths:
  /interfaces/interface/ethernet/state/counters/in-mac-pause-frames:
  /interfaces/interface/ethernet/state/counters/out-mac-pause-frames:
  /interfaces/interface/ethernet/state/mac-address:
  /interfaces/interface/state/counters/in-broadcast-pkts:
  /interfaces/interface/state/counters/in-discards: 
  /interfaces/interface/state/counters/in-errors:
  /interfaces/interface/state/counters/in-multicast-pkts:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/in-unknown-protos:
  /interfaces/interface/state/counters/out-broadcast-pkts:
  /interfaces/interface/state/counters/out-discards:
  /interfaces/interface/state/counters/out-errors:
  /interfaces/interface/state/counters/out-multicast-pkts:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/out-unicast-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/mtu:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/mtu:
  /interfaces/interface/subinterfaces/subinterface/ipv4/unnumbered/state/enabled:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-discarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-discarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts: 
  /interfaces/interface/ethernet/state/aggregate-id:
  /interfaces/interface/ethernet/state/port-speed:
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/description:
  /interfaces/interface/state/type:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-pkts:
  /interfaces/interface/state/hardware-port: 
  /interfaces/interface/state/id:
  /interfaces/interface/state/counters/in-fcs-errors:
  /interfaces/interface/state/counters/carrier-transitions:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: false
    gNMI.Subscribe:
      on_change: false
```
