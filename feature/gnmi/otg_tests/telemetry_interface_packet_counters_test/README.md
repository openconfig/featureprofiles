# gNMI-1.11: Telemetry: Interface Packet Counters

## Summary

Validate interfaces counters including both IPv4 and IPv6 counters.

## Procedure

In the automated ondatra test, verify the presence of the telemetry paths of the
following features:

*   Configure IPv4 and IPv6 addresses under subinterface:

    *   /interfaces/interface/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled

    Validate that IPv4 and IPv6 addresses are enabled:

    *   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/enabled

*   For the parent interface counters in-pkts and out-pkts:

    Check the presence of packet counter paths and monitor counters every 30 seconds:

    *   /interfaces/interface[name=<port>]/state/counters/in-pkts
    *   /interfaces/interface[name=<port>]/state/counters/out-pkts

*   Subinterfaces counters:

    Check the presence of packet counter paths:

    *   TODO:
        /interfaces/interface[name=<port>]/subinterfaces/subinterface[index=<index>]/ipv4/state/counters/in-pkts
    *   TODO:
        /interfaces/interface[name=<port>]/subinterfaces/subinterface[index=<index>]/ipv4/state/counters/out-pkts
    *   TODO:
        /interfaces/interface[name=<port>]/subinterfaces/subinterface[index=<index>]/ipv6/state/counters/in-discarded-pkts
    *   TODO:
        /interfaces/interface[name=<port>]/subinterfaces/subinterface[index=<index>]/ipv6/state/counters/out-discarded-pkts

*   Ethernet interface counters

    Check the presence of counter path including in-maxsize-exceeded:

    *   TODO: /interfaces/interface/ethernet/state/counters/in-maxsize-exceeded
    *   /interfaces/interface/ethernet/state/counters/in-mac-pause-frames
    *   /interfaces/interface/ethernet/state/counters/out-mac-pause-frames
    *   /interfaces/interface/ethernet/state/counters/in-crc-errors
    *   /interfaces/interface/ethernet/state/counters/in-fragment-frames
    *   /interfaces/interface/ethernet/state/counters/in-jabber-frames

*   Interface CPU and management

    Check the presence of CPU and management paths:

    *   TODO: /interfaces/interface/state/cpu
    *   TODO: /interfaces/interface/state/management

## Config Parameter coverage

*   /interfaces/interface/config/enabled
*   /interfaces/interface/subinterfaces/subinterface/config/enabled
*   /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled
*   /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled

## Telemetry Parameter coverage

*   /interfaces/interface/state/counters/in-pkts
*   /interfaces/interface/state/counters/out-pkts

*   /interfaces/interface/subinterfaces/subinterface]/ipv4/state/counters/in-pkts

*   /interfaces/interface/subinterfaces/subinterface]/ipv4/state/counters/out-pkts

*   /interfaces/interface/subinterfaces/subinterface]/ipv6/state/counters/in-pkts

*   /interfaces/interface/subinterfaces/subinterface]/ipv6/state/counters/out-pkts

*   /interfaces/interface/subinterfaces/subinterface]/ipv6/state/counters/in-discarded-pkts

*   /interfaces/interface/subinterfaces/subinterface]/ipv6/state/counters/out-discarded-pkts

*   /interfaces/interface/ethernet/state/counters/in-maxsize-exceeded

*   /interfaces/interface/ethernet/state/counters/in-mac-pause-frames

*   /interfaces/interface/ethernet/state/counters/out-mac-pause-frames

*   /interfaces/interface/ethernet/state/counters/in-crc-errors

*   /interfaces/interface/ethernet/state/counters/in-fragment-frames

*   /interfaces/interface/ethernet/state/counters/in-jabber-frames

*   /interfaces/interface/state/cpu

*   /interfaces/interface/state/management

## Protocol/RPC Parameter coverage

No coverage

## Minimum DUT platform requirement

N/A
