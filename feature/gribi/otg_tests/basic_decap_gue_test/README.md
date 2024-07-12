# TE-19: basic gue decapsulation tests

## Summary

Ensure that gRIBI programmed operations results in the correct GUE decapsulation.

## Procedure

### TE-19.1: Match on dest IP and perform decapsulate IP, GUE header

* Configure DUT `port-1` to be connected to ATE `port-1`, and DUT `port-2` to
  be connected to ATE `port-2`. ATE `port-2` is configured to have an assigned
  address of `2001:db8::1/128`, and the interface is enabled.
* Program policy forwarding entry matching dest ip prefix pointing to a NHG containing a single NH performing decapsulate header OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV6.
* Perform top label pop and chooses egress interface.
* Verify at destination port packet is received.

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
            * `OPENCONFIGAFTTYPESENCAPSULATIONHEADERTYPE_IPV6`
            * `popped_label_stack`
    *   `ModifyResponse`
    *   `AFTResult`
        *   `id`
        *   `status`

## Config parameter coverage

## Telemetry parameter coverage
