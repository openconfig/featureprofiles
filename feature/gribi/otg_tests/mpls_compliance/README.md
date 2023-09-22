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


## Protocol/RPC Parameter coverage

*   gRIBI:
    *  `Modify()`
      * `ModifyRequest`
        *   `AFTOperation`:
          *   `id`
          *   `network_instance`
          *   `op`: `ADD`
          *  `mpls`:
            *   `next_hop_group`
          *   `next_hop_group`
            *  `id`
            *  `next_hop`
          *   `next_hop`
            * `id`
            * `ip_address`
            * `pushed_label_stack`
    *   `ModifyResponse`:
    *   `AFTResult`:
        *   `id`
        *   `status`

## Config parameter coverage

## Telemetry parameter coverage


