# TE-18: basic gue encapsulation tests

## Summary

Ensure that gRIBI programmed operations results in the correct GUE encapsulation(a.k.a MPLS over UDP).

## Procedure

### TE-18.1: Match ingress logical interface, dest IP and perform encaps with set outer packet src, dst IP address and src, dst UDP port.

* Configure DUT `port-1` to be connected to ATE `port-1`, and DUT `port-2` to
  be connected to ATE `port-2`. ATE `port-2` is configured to have an assigned
  address of `10.200.200.5`, and the interface is enabled.
* Program policy forwarding entry matching logical interface and dest ip prefix pointing to a NHG containing a single NH, pushing MPLS label
encapsulated in a GUE header.
* Set outer packet source IP, destination IP.
* Set source, destination port.
* Verify at destination port that packet is received with mpls label encapsulated with GUE header.

### TE-18.2: Match ingress logical interface, dest IP and perform encaps with set DSCP value

* Configure DUT `port-1` to be connected to ATE `port-1`, and DUT `port-2` to
  be connected to ATE `port-2`. ATE `port-2` is configured to have an assigned
  address of `10.200.200.5`, and the interface is enabled.
* Program policy forwarding entry matching logical interface and dest ip prefix pointing to a NHG containing a single NH, pushing MPLS label
encapsulated in a GUE header with set DSCP value.
* Verify at destination port that packet is received with mpls label encapsulated with GUE header and correct DSCP value.

### TE-18.3: Match on dest IP and perform IPSec encryption

* Configure DUT `port-1` to be connected to ATE `port-1`, and DUT `port-2` to
  be connected to ATE `port-2`. ATE `port-2` is configured to have an assigned
  address of `2001:db8::1/128`, and the interface is enabled.
* Program policy forwarding entry matching dest ip prefix pointing to a NHG containing a single NH performing IPSec encryption on packet, insert ESP header denoting SPI.
* Verify at destination port that packet is received with IPSec encryption.

## Protocol/RPC Parameter coverage

*   gRIBI:
    *  `Modify()`
      * `ModifyRequest`
        *   `AFTOperation`
          *   `id`
          *   `policy-forwarding`
          *   `op`: `ADD`
          *  `ip`
            * `next_hop_group`
          *  `next-hop`
            *   `next_hop_group`
          *   `next_hop_group`
            *  `id`
            *  `next_hop`
          *   `next_hop`
            * `id`
            * `OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_UDP`
            * `pushed_label_stack`
    *   `ModifyResponse`
    *   `AFTResult`
        *   `id`
        *   `status`

## Config parameter coverage


## Telemetry parameter coverage


