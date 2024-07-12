# TE-20: basic gue scaling tests

## Summary

Ensure that gRIBI policy-forwarding rules for GUE can be scaled by batching.The frequency of this updates may fall within a range of O(30s), should affect within 10 seconds.

## Procedure

### TE-20.1: Match ingress logical interface, dest IP

* Configure DUT `port-1` to be connected to ATE `port-1`, and DUT `port-2` to
  be connected to ATE `port-2`. ATE `port-2` is configured to have an assigned
  address of `10.200.200.5`, and the interface is enabled.
* Program 20,000 policy forwarding entry's matching logical interface and dest ip prefix pointing to a NHG containing a single NH, pushing MPLS label
encapsulated in a GUE header. 
* All of which are updated in a batch of every 30 seconds. The update should ideally take with in 10 seconds.
* Verify update should take affect with in 10 seconds.

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
    *   `ModifyResponse`
    *   `AFTResult`
        *   `id`
        *   `status`

## Config parameter coverage


## Telemetry parameter coverage
