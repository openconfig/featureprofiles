# DP-1.19: Egress traffic DSCP rewrite

## Summary

This test validates egress traffic scheduling and packet remarking (rewrite) on a DUT. IP payload encapsulated as IPoGRE,
IPoMPLSoGRE,IPoGUE or IPoMPLSoGUE would be decapsulated by DUT as per tunnel termination configuration and forwarded as an 
IP payload via the egress interface. IP payload should be scheduled via egress interface according to DSCP markings prior to rewrite.
DSCP values of IP payload encapsulated as IPoGRE, IPoMPLSoGRE, IPoGUE or IPoMPLSoGUE should have payload DSCP values re-written 
per egress QOS re-write policy applied to the egress interface. Egress QOS policy should support match conditions on payload 
IP header fields, specifically DSCP values. Based on the match condition, DSCP must be set to a new value. DUT configuration 
will be evaluated against OpenConfig QOS model, and traffic flows analyzed to ensure proper scheduling, re-marking, and forwarding.

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

* DUT has an ingress port and 1 egress port.

| | [ ATE Port 1 ] ----- | DUT | ----- [ ATE Port 2 ] | |


* Configure DUT's ingress and egress interfaces.


#### Configuration

It is assumed that DUT supports 8 QOS queues and use the following class mapping to code points:
| FWD   | Code-point | DSCP    | TOS   | Queue  |
| class | Bits       | value   | value |        |
| ————--| ---------- | ——————— | ————— | ———--- |
| NC1   |   1110xx   | 56 - 59 | E0-EF |   NC1  |
| ————--| ---------- | ——————— | ————— | ———--- |
| NC1   |   1100xx   | 48 - 51 | C0-CF |   NC1  |
| ————--| ---------- | ——————— | ————— | ———--- |
| AF4   |   1000xx   | 32 - 35 | 80-8F |   AF4  |
| ————--| ---------- | ——————— | ————— | ———--- |
| AF3   |   0110xx   | 24 - 27 | 60-6F |   AF3  |
| ————--| ---------- | ——————— | ————— | ———--- |
| AF2   |  0100xx    | 16 - 19 | 40-4F |   AF2  |
| ————--| ---------- | ——————— | ————— | ———--- |
| AF1   |  0010xx    | 8 - 11  | 20-2F |   AF1  |
| ————--| ---------- | ——————— | ————— | ———--- |
| BE0   |  0001xx    | 4 - 7   | 10-1F |   BE1  |
| ————--| ---------- | ——————— | ————— | ———--- |
| BE1   |  0000xx    | 0 - 3   | 00-0F |   BE1  |

1. DUT:Port1 is a Singleton IP interface towards ATE:Port1.
2. DUT:Port2 is a Singleton IP interface towards ATE:Port2.
3. DUT has egress forwarding  entry in it's FIB via ATE:Port1  towards  ATE:Port2: BGP routing could be used to populate FIB as follows:
   3a. DUT forms one IPv4 and one IPV6 eBGP session with ATE:Port1 using connected Singleton interface IPs.
   3b. DUT forms one IPv4 and one IPV6 eBGP session with ATE:Port2 using connected Singleton interface IPs.
   3c. DUT has IPv4-DST-DECAP/32 and IPv6-DST-DECAP/128 advertised to ATE:Port1 via IPv4 BGP. This IP is used for decapsulation.
7.    ATE:Port2 advertises destination networks IPv4-DST-NET/32 and IPv6-DST-NET/128 to DUT.
8. DUT decapsulates IPoGRE, IPoMPLSoGRE,IPoGUE or IPoMPLSoGUE payload with destination of IPv4-DST-DECAP/32 and IPv6-DST-DECAP/128
9. DUT has MPLS static forwarding rule (aka static LSP) for label 100020 pointing to a NHG resolved via ATE:Port2.
10. DUT matches packets on DSCP/TC values and sets new DSCP values based on remarking rules.


* Re-marking rules:

| Queue | TOS | TOS |
|       | v4  | v6  |  
| ----- | --- | --- |
| BE1   |  0  |  0  |
| ----- | --- | --- |
| AF1   | 0   |  0  |
| ----- | --- | --- |
| AF2   | 0   |  0  |
| ----- | --- | --- |
| AF3   | 0   |  0  |
| ----- | --- | --- |
| AF4   | 0   |  0  |
| ----- | --- | --- |
| NC1   | 6   |  48 |


### DP-1.17.1 Egress Classification and rewrite of IPv4 packets with various DSCP values

* Traffic:
    * Generate IPv4 traffic from ATE Port 1 with various DSCP values per table listed in the "Test environment setup" section.
* Verification:
    * Monitor telemetry on DUT ATE Port 1 to verify packet scheduling into correct egress QOS queues by observing queue counters per configued QOS classes.
    * Capture packets on ATE Port 2 ingress to verify packet re-marking per table above.
    * Analyze traffic flows to confirm no packet drops on DUT.

### DP-1.17.2 Egress Classification and rewrite of IPv6 packets with various TC values

* Traffic:
    * Generate IPv6 traffic from ATE Port 1 with various DSCP values per table listed in the "Test environment setup" section.
* Verification:
    * Monitor telemetry on  DUT ATE Port 1 to verify packet scheduling into correct egress QOS queues based on the DSCP markings of the payload prir to re-write rules by observing queue counters per configued QOS classes.
    * Capture packets on ATE Port 2 ingress to verify packet marking per table above.
    * Analyze traffic flows, confirming no drops on DUT.

### DP-1.17.3 Egress Classification and rewrite of IPoMPLSoGUE traffic with pop action

*   Configuration:
    *   Configure Static MPLS LSP with MPLS pop and IPv4/IPv6 forward actions for a specific label 100020 via ATE Port 1.
    *   Configure decapsulation rules for IPv4-DST-DECAP/32
    *   Configufe egress QOS re-write rules based on DSCP/TC values and set new DSCP values based on remarking rules.
*   Traffic:
    *   Generate IPoMPLSoGUE traffic from ATE Port 1 with label 100020 
*   Verfication:
    *   Monitor telemetry on  DUT ATE Port 1 to verify packet scheduling into correct egress QOS queues based on the DSCP markings of the payload prir to re-write rules by observing queue counters per configued QOS classes.. 
    *   Capture packets on the DUT ATE Port 2 ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.

### DP-1.17.4 Egress Classification and rewrite of IPv6oMPLSoGUE traffic with pop action
*   Configuration:
    *   Configure Static MPLS LSP with MPLS pop and IPv6 forward actions for a specific label 10020.
    *   Configure decapsulation rules for IPv6-DST-DECAP/12
    *   Configufe egress QOS re-write rules based on DSCP/TC values and set new DSCP values based on remarking rules.

*   Traffic:
    *   Generate IPv6oMPLSoGUE traffic from ATE Port 1 with label 100020
*   Verfication:
    *   Monitor telemetry on  DUT ATE Port 1 to verify packet scheduling into correct egress QOS queues based on the DSCP markings of the payload prir to re-write rules by observing queue counters per configued QOS classes.
    *   Capture packets on the DUT ATE Port 2 ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.

### DP-1.17.5 Egress Classification and rewrite of IPoMPLSoGRE traffic with pop action
*   Configuration:
    *   Configure Static MPLS LSP with MPLS pop and IPv4 forward actions for a specific label 100020
    *   Configure decapsulation rules for IPv4-DST-DECAP/32
    *   Configufe egress QOS re-write rules based on DSCP/TC values and set new DSCP values based on remarking rules.

*   Traffic:
    *   Generate IPoMPLSoGRE traffic from ATE Port 1 with label 100020
    *   Verfication:
    *   Monitor telemetry on the DUT to verify that packets re being scehduled for transmission into correct forwarding groups based on the DSCP markings of the payload prir to re-write rules
    *   Capture packets on the DUT ATE Port 2 ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.

### DP-1.17.7 Egress Classification and rewrite of IPv6oMPLSoGRE traffic with pop action
*   Configuration:
    *  Configure Static MPLS LSP with MPLS pop and IPv6 forward actions for a specific label 100020.
    *  Configure decapsulation rules for IPv6-DST-DECAP/12
    *  Configufe egress QOS re-write rules based on DSCP/TC values and set new DSCP values based on remarking rules.

*   Traffic:
    *   Generate IPv6oMPLSoGRE traffic from ATE Port 1 with label 100020
*   Verfication:
    *   Monitor telemetry on  DUT ATE Port 1 to verify packet scheduling into correct egress QOS queues based on the DSCP markings of the payload prir to re-write rules by observing queue counters per configued QOS classes.
    *   Capture packets on the DUT ATE Port 2 ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.

### DP-1.17.8 Egress Classification and rewrite of IPoGRE traffic with decapsulate action
*   Configuration:
    *    Configure decapsulation rules for IPv4-DST-DECAP/32
    *    Configufe egress QOS re-write rules based on DSCP/TC values and set new DSCP values based on remarking rules.

*   Traffic:
    *   Generate IPoGRE traffic from ATE Port 1 with IP payload dest reachable via ATE:Port2
*   Verfication:
    *   Monitor telemetry on  DUT ATE Port 1 to verify packet scheduling into correct egress QOS queues based on the DSCP markings of the payload prir to re-write rules by observing queue counters per configued QOS classes.. 
    *   Capture packets on the DUT ATE Port 2 ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.

### DP-1.17.9 Egress Classification and rewrite of IPv6oGRE traffic with decapsulate action
*   Configuration:
    *   Configure decapsulation rules for  IPv6-DST-DECAP/12
    *   Configufe egress QOS re-write rules based on DSCP/TC values and set new DSCP values based on remarking rules.
*   Traffic:
    *   Generate IPv6oGRE traffic from ATE Port 1 with payload reachable via ATE:Port2
*   Verfication:
    *   Monitor telemetry on  DUT ATE Port 1 to verify packet scheduling into correct egress QOS queues based on the DSCP markings of the payload prir to re-write rules by observing queue counters per configued QOS classes.. 
    *   Capture packets on the DUT ATE Port 2 ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.

### DP-1.17.10 Egress Classification and rewrite of IPoGUE traffic with pop action
*   Configuration:ƒ
    *    Configure decapsulation rules for IPv4-DST-DECAP/32
    *    Configufe egress QOS re-write rules based on DSCP/TC values and set new DSCP values based on remarking rules.

*   Traffic:
    *   Generate IPoGUE traffic from ATE Port 1 with payload reachable via ATE:Port2
*   Verfication:
    *   Monitor telemetry on  DUT ATE Port 1 to verify packet scheduling into correct egress QOS queues based on the DSCP markings of the payload prir to re-write rules by observing queue counters per configued QOS classes.. ƒ
    *   Capture packets on the DUT ATE Port 2 ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.

### DP-1.17.11 Egress Classification and rewrite of IPv6oGUE traffic with pop action
*   Configuration:
    *   Configure decapsulation rules for Id IPv6-DST-DECAP/12
    *   Configufe egress QOS re-write rules based on DSCP/TC values and set new DSCP values based on remarking rules.

*   Traffic:
    *   Generate IPv6oGUE traffic from ATE Port 1 with IPv6 payload reachable via ATE:Port2
*   Verfication:
    *   Monitor telemetry on  DUT ATE Port 1 to verify packet scheduling into correct egress QOS queues based on the DSCP markings of the payload prir to re-write rules by observing queue counters per configued QOS classes. 
    *   Capture packets on the DUT ATE Port 2 ingress interface to verify packet marking according to the marking table.
    *   Analyze traffic flows to confirm that no packets are dropped on the DUT.

## Canonical OC
### TODO: Fix Canonical OC
```json
{}
```

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
