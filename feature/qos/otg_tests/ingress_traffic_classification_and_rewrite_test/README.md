# DP-1.16: Ingress traffic classification and rewrite

## Summary

This test aims to validate the functionality of ingress traffic classification and subsequent packet remarking (rewrite) on a Device Under Test (DUT). The DUT's configuration will be evaluated against the OpenConfig QOS model, and traffic flows will be analyzed to ensure proper classification, marking, and forwarding.

## Topology

* ATE:port1 <-> port1:DUT:port2 <-> ATE:port2

## Procedure

* DUT Configuration:
    * Configure the DUT's ingress and egress interfaces.
    * Apply QoS classifiers using the OpenConfig QOS model, matching packets based on DSCP/TC/EXP values as per the classification table.
    * Configure packet remarking rules based on the marking table.
    * Configure Static MPLS LSP with MPLS pop and IPv4/IPv6 forward actions for a specific label (ie. 20-100).
    * Configure Static MPLS LSP MPLS swap/forward actions for a specific label (ie. 101-200).

* Traffic Generation:
    * Use the traffic generator to send:
        * IPv4 packets with various DSCP values
        * IPv6 packets with various TC values
        * MPLS traffic with swap action.
        * IPv4-over-MPLS traffic with pop action.
        * IPv6-over-MPLS traffic with pop action.

* Verification:
    * Monitor telemetry on the DUT to verify that packets are being matched to the correct classifier terms.
    * Capture packets on the ATE's ingress interface to verify packet marking according to the marking table.
    * Analyze traffic flows to confirm that no packets are dropped on the DUT.

*   QoS Classification and Marking table

    *  Classification table

    IPv4 TOS      |       IPv6 TC           |         MPLS EXP        |    Forwarding Group
    ------------- | ----------------------- | ----------------------- | ---------------------
    0             |      0-7                |          0              |         be1
    1             |      8-15               |          1              |         af1
    2             |      16-23              |          2              |         af2
    3             |      24-31              |          3              |         af3
    4,5           |      32-47              |          4,5            |         af4
    6,7           |      48-63              |          6,7            |         nc1

    *   Marking table

        Forwarding Group | IPv4 TOS     |       IPv6 TC           |         MPLS EXP        
     --------------------|------------- | ----------------------- | ----------------------- 
             be1         |   0          |      0                  |          0              
             af1         |   1          |      8                  |          1              
             af2         |   2          |      16                 |          2              
             af3         |   3          |      24                 |          3              
             af4         |   4          |      32                 |          4              
             nc1         |   6          |      48                 |          6              

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/config/type:
  /qos/classifiers/classifier/terms/term/config/id:
  /qos/classifiers/classifier/terms/term/actions/config/target-group:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/mpls/config/traffic-class:
  /qos/classifiers/classifier/terms/term/actions/remark/config/set-dscp:
  /qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc:
  /qos/interfaces/interface/input/classifiers/classifier/config/name:
  /qos/interfaces/interface/input/classifiers/classifier/config/type:

  ## State paths
  /qos/interfaces/interface/input/classifiers/classifier/terms/term/state/matched-packets:
  /qos/interfaces/interface/input/classifiers/classifier/terms/term/state/matched-octets:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
