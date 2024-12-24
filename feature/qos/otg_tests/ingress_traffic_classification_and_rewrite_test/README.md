# DP-1.16: Ingress traffic classification and rewrite

## Summary

This test aims to validate the functionality of ingress traffic classification and subsequent packet remarking (rewrite) on a Device Under Test (DUT). The DUT's configuration will be evaluated against the OpenConfig QOS model, and traffic flows will be analyzed to ensure proper classification, marking, and forwarding.

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

*   DUT has an ingress port and 1 egress port.

    ```
                             |         |
        [ ATE Port 1 ] ----  |   DUT   | ---- [ ATE Port 2 ]
                             |         |
    ```

* Configure the DUT's ingress and egress interfaces.

### Configuration

*   Apply QoS classifiers using the OpenConfig QOS model, matching packets based on DSCP/TC/EXP values as per the classification table.
*   Configure packet remarking rules based on the marking table.
*   QoS Classification and Marking table

    *   Classification table

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

### DP-1.16.1 Ingress Classification and rewrite of IPv4 packets with various DSCP values

*   Traffic:
    *   Generate IPv4 traffic from ATE Port 1 with various DSCP values 
*   Verfication:
    *   Monitor telemetry on the DUT to verify that packets are being matched to the correct classifier terms.
    *   Capture packets on the ATE's ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.
### DP-1.16.2 Ingress Classification and rewrite of IPv6 packets with various TC values

*   Traffic:
    *   Generate IPv6 traffic from ATE Port 1 with various TC values
*   Verfication:
    *   Monitor telemetry on the DUT to verify that packets are being matched to the correct classifier terms.
    *   Capture packets on the ATE's ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.
### DP-1.16.3 Ingress Classification and rewrite of MPLS traffic with swap action

*   Configuration:
    *   Configure Static MPLS LSP MPLS swap/forward actions for a specific labels range (100101-100200).
*   Traffic:
    *   Generate MPLS traffic from ATE Port 1 with labels between 1000101 and 1000200
*   Verfication:
    *   Monitor telemetry on the DUT to verify that packets are being matched to the correct classifier terms.
    *   Capture packets on the ATE's ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.
### DP-1.16.4 Ingress Classification and rewrite of IPv4-over-MPLS traffic with pop action

*   Configuration:
    *   Configure Static MPLS LSP with MPLS pop and IPv4/IPv6 forward actions for a specific labels range (100020-100100).
*   Traffic:
    *   Generate MPLS traffic from ATE Port 1 with labels between 100020 and 1000100
*   Verfication:
    *   Monitor telemetry on the DUT to verify that packets are being matched to the correct classifier terms.
    *   Capture packets on the ATE's ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.
### DP-1.16.5 Ingress Classification and rewrite of IPv6-over-MPLS traffic with pop action

*   Configuration:
    *   Configure Static MPLS LSP with MPLS pop and IPv4/IPv6 forward actions for a specific labels range (100020-100100).
*   Traffic:
    *   Generate MPLS traffic from ATE Port 1 with labels between 100020 and 1000100
*   Verfication:
    *   Monitor telemetry on the DUT to verify that packets are being matched to the correct classifier terms.
    *   Capture packets on the ATE's ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.
### DP-1.16.6 Ingress Classification and rewrite of IPv4 packets traffic with label push action

*   Configuration:
    *   Configure Static MPLS LSP with MPLS label (=100201) push action to a IPv4 subnet destination DST1.
*   Traffic:
    *   Generate IPv4 traffic from ATE Port 1 with destinations matching DST1. 
*   Verfication:
    *   Monitor telemetry on the DUT to verify that packets are being matched to the correct classifier terms.
    *   Capture packets on the ATE's ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.
### DP-1.16.7 Ingress Classification and rewrite of IPv6 packets traffic with label push action

*   Configuration:
    *   Configure Static MPLS LSP with MPLS label (=100202) push action to a IPv6 subnet destination DST2.
*   Traffic:
    *   Generate IPv6 traffic from ATE Port 1 with destinations matching DST2. 
*   Verfication:
    *   Monitor telemetry on the DUT to verify that packets are being matched to the correct classifier terms.
    *   Capture packets on the ATE's ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.

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

* FFF - fixed form factor
