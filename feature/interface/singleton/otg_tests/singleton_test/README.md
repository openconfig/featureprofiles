# RT-5.1: Singleton Interface

## Summary

Singleton L3 interface (non-LAG) is supported on DUT.

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

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

## Config Parameter Coverage

* /interfaces/interface/config/name
* /interfaces/interface/config/description
* /interfaces/interface/config/enabled
* /interfaces/interface/subinterfaces/subinterface/ipv4/config/mtu
* /interfaces/interface/subinterfaces/subinterface/ipv6/config/mtu
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
