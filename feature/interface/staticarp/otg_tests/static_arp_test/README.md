# TE-1.1: Static ARP

## Summary

Ensure static ARP entries installed on the DUT are honoured.

## Procedure

*   Configure OTG port-1 connected to DUT port-1, and OTG port-2 connected to
    DUT port-2, with the relevant IPv4 and IPv6 addresses.
*   Without static ARP entry:
    *   Configure OTG traffic flow to enable custom egress filter on the last
        15-bits of the destination MAC (starting at bit offset 33 of the
        ethernet packet).
    *   Ensure that traffic can be forwarded between OTG port-1 and OTG port-2
        normally.
    *   Check that the egress filter picks up the last 15-bit of OTG default MAC
        address.
*   Add static entry to DUT interfaces to override the OTG MAC address.
*   With static ARP entry:
    *   Configure OTG traffic flow with custom egress filter as before, and
        ensure that traffic can be forwarded between OTG port-1 and OTG port-2.
    *   Check that the egress filter picks up the last 15-bit of the MAC address
        set by static ARP.

Note that OTG ports are promiscuous, i.e. they will receive all packets
regardless of the destination MAC. The custom egress filter is used to tell what
are the destination MAC addresses of the packets seen by the OTG.

## Config Parameter Coverage

*   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip
*   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length
*   /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/ip
*   /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/link-layer-address
*   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
*   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
*   /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/config/ip
*   /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/config/link-layer-address

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:

```