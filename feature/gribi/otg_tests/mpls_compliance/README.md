# TE-9: gRIBI MPLS Compliance

## Summary

Ensure that the gRIBI server implements a base set of MPLS functionality without
traffic validation.


## Procedure

### TE-9.1: Push MPLS Labels to MPLS payload

* Configure ATE `port-1` connected to DUT `port-1`, and ATE `port-2` connected
  to DUT `port-2`.
* ATE `port-2` is configured to have an assigned address of `192.0.2.2`, and the
  interface to the DUT is enabled.
* For label stack depths beginning at `baseLabel`, with `numLabels` addition
  labels:
   - Program a `LabelEntry` matching outer label 100 pointing to a NHG
     containing a single NH.
   - Program a `NextHopEntry` which points to `192.0.2.2` pushing `[baseLabel,
     ..., baseLabel+numLabels]` onto the MPLS label stack.
* Validate that gRIBI transactions are successfully processed by the server.

### TE-9.2: Push MPLS Labels to IP Packet

* Configure DUT with a destination interface connected to an ATE. The ATE is
  configured to have an assigned address of `192.0.2.2`, and the interface to
  the DUT is enabled.
* For label stack depths from `N=1...numLabels` program:
     * an IPv4 entry for `10.0.0.0/24` with a next-hop of `192.0.2.2` pushing N
       additional labels onto the packet.
* Validate that gRIBI transactions are successfully processed by the server.

### TE-9.3: Pop Top MPLS Label

* Configure DUT with a destination interface connected to an ATE. The ATE is
  configured to have assigned address 192.0.2.2.
* Program DUT with a label forwarding entry matching label 100 and specifying to
  pop the top label.
* Validate that gRIBI transactions are successfully processed by the server.

## TE-9.4: Pop N Labels from Stack

* Configure DUT with destination interface connected to an ATE. The ATE is
  configured to have assigned address `192.0.2.2`.
* Program DUT with a label forwarding entry matching label 100 and specifying to
  pop:
    * Label `100`
    * Label stack `[100, 42]`
    * Label stack `[100, 42, 43, 44, 45]`

## TE-9.5: Pop 1 Push N Labels

* Configure DUT with destination interface connected to an ATE. The ATE is
  configured to have assigned address `192.0.2.2`.
* Program DUT with a label forwarding entry matching label 100 and label 200,
  pointing to a next-hop that is programmed to pop the top label, and:
   - push label 100 - resulting in a swap for incoming label 100, and a push of
     100 for incoming label 200.
   - push stack `[100, 200, 300, 400]`
   - push stack `[100, 200, 300, 400, 500, 600]`

## Protocol/RPC Parameter coverage

*   gRIBI:
    *  `Modify()`
      * `ModifyRequest`
        *   `AFTOperation`:
          *   `id`
          *   `network_instance`
          *   `op`: `ADD`
          *  `ipv4`:
            *  `prefix`
          *  `mpls`:
            *   `next_hop_group`
          *   `next_hop_group`
            *  `id`
            *  `next_hop`
          *   `next_hop`
            * `id`
            * `ip_address`
            * `pushed_label_stack`
            * `pop_top_label`
            * `popped_label_stack`
    *   `ModifyResponse`:
    *   `AFTResult`:
        *   `id`
        *   `status`

## Config parameter coverage

## Telemetry parameter coverage


