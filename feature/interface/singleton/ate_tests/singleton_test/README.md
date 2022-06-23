# RT-5.1: Singleton Interface

## Summary

Singleton L3 interface (non-LAG) is supported on DUT.

## Procedure

*   Connect ATE port-1 to DUT port-1.
*   Configure ATE port-1 and DUT port-1 with specified address family. Configure
    destination subnet (192.0.2.0/24, 2001:db8::/48) to be received at ATE
    port-1.
*   Configure static MAC address to be 02:1a:WW:XX:YY:ZZ where WW:XX:YY:ZZ are
    the octets of IPv4.
    *   Ensure: ARP discovers static MAC address specified when port is
        configured with static MAC.
*   For IPv4 and IPv6:
    *   Ensure:
        *   For MTUs of 1500, 5000, 9212:
            *   Packets with size greater than the configured MTU with DF-bit
                set are not transmitted.
            *   Packets with size of configured MTU are received.
            *   Packets with size less than the configured MTU are received.
        *   For each port speed required to be supported:
            *   Validate port speed is reported correctly.
            *   Validate that port telemetry matches expected values
                (particularly, effective speeds are reported).
            *   For each breakout port configuration, ensure that ports are
                correctly reported and packets are forwarded as expected.

## Config Parameter coverage

*   /interfaces/interface/config/name
*   /interfaces/interface/config/description
*   /interfaces/interface/config/enabled
*   /interfaces/interface/subinterfaces/subinterface/ipv4/config/mtu
*   /interfaces/interface/subinterfaces/subinterface/ipv6/config/mtu
*   /interfaces/interface/config/id
*   /interfaces/interface/ethernet/config/mac-address
*   /interfaces/interface/ethernet/config/port-speed
*   /interfaces/interface/ethernet/config/duplex-mode

## Telemetry Parameter coverage

*   /interfaces/interface/ethernet/state/counters/in-mac-pause-frames
*   /interfaces/interface/ethernet/state/counters/out-mac-pause-frames
*   /interfaces/interface/ethernet/state/mac-address
*   /interfaces/interface/state/counters/in-broadcast-pkts
*   /interfaces/interface/state/counters/in-discards
*   /interfaces/interface/state/counters/in-errors
*   /interfaces/interface/state/counters/in-multicast-pkts
*   /interfaces/interface/state/counters/in-octets
*   /interfaces/interface/state/counters/in-unicast-pkts
*   /interfaces/interface/state/counters/in-unknown-protos
*   /interfaces/interface/state/counters/out-broadcast-pkts
*   /interfaces/interface/state/counters/out-discards
*   /interfaces/interface/state/counters/out-errors
*   /interfaces/interface/state/counters/out-multicast-pkts
*   /interfaces/interface/state/counters/out-octets
*   /interfaces/interface/state/counters/out-pkts
*   /interfaces/interface/state/counters/out-unicast-pkts
*   /interfaces/interface/subinterfaces/subinterface/ipv4/state/mtu
*   /interfaces/interface/subinterfaces/subinterface/ipv6/state/mtu
*   /interfaces/interface/state/oper-status
*   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/ip
*   /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts
*   /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts
*   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/ip
*   /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-discarded-pkts
*   /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts
*   /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-discarded-pkts
*   /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts
*   /interfaces/interface/ethernet/state/aggregate-id
*   /interfaces/interface/ethernet/state/port-speed
*   /interfaces/interface/state/admin-status
*   /interfaces/interface/state/counters/out-octets
*   /interfaces/interface/state/description
*   /interfaces/interface/state/type
*   /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-multicast-pkts
*   /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-forwarded-pkts
*   /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-multicast-pkts
*   /interfaces/interface/state/hardware-port
*   /interfaces/interface/state/id
*   /interfaces/interface/state/counters/in-fcs-errors

## Protocol/RPC Parameter coverage

None

## Minimum DUT platform requirement

vRX
