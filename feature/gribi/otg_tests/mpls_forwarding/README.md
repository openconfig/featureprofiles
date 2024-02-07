# TE-10: gRIBI MPLS Forwarding

## Summary

Ensure that gRIBI programmed operations result in the correct traffic forwarding
behaviour on the DUT.

## Procedure

### TE-10.1: Push MPLS Labels to MPLS payload

* Configure DUT `port-1` to be connected to ATE `port-1`, and DUT `port-2` to
  be connected to ATE `port-2`. ATE `port-2` is configured to have an assigned
  address of `198.18.2.2`, and the interface is enabled.
* For label stack depths beginning at `baseLabel`, with `numLabels` addition
  labels:
   - Program a `LabelEntry` matching outer label 100 pointing to a NHG
     containing a single NH.
   - Program a `NextHopEntry` which points to `192.18.2.2` pushing `[baseLabel,
     ..., baseLabel+numLabels]` onto the MPLS label stack.
* Run an MPLS flow matching label 100's forwarding entry and validate that is
  received at the destination port.

### TE-10.2: Push MPLS Labels to IP payload

* Configure DUT `port-1` to be connected to ATE `port-1`, and DUT `port-2` to
  be connected to ATE `port-2`. ATE `port-2` is configured to have an assigned
  address of `198.18.2.2`, and the interface is enabled.
* For label stack depth beginning at `baseLabel`, with `numLabels` addition
  labels:
   - Program a IPv4Entry matcing IP Prefix `192.168.0.0/24` pointing to a NHG
     containing a single NH.
   - Program a `NextHopEntry` which points to `192.18.2.2` pushing `[baseLabel,
     ..., baseLabel+numLabels]` onto the MPLS label stack.
* Run an IP flow matching IP Prefix `192.168.0.0/24` and validate that it is
  received at the destination port.

## Protocol/RPC Parameter coverage

*   gRIBI:
    *  `Modify()`
      * `ModifyRequest`
        *   `AFTOperation`
          *   `id`
          *   `network_instance`
          *   `op`: `ADD`
          *  `ip`
            * `next_hop_group`
          *  `mpls`
            *   `next_hop_group`
          *   `next_hop_group`
            *  `id`
            *  `next_hop`
          *   `next_hop`
            * `id`
            * `ip_address`
            * `pushed_label_stack`
    *   `ModifyResponse`
    *   `AFTResult`
        *   `id`
        *   `status`

## Config parameter coverage

## Telemetry parameter coverage


