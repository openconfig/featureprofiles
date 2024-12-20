# DP-2.5: QoS Prioritization of Eggress Traffic

## Summary

The scope of this test is limited to supporting the prioritization of egress traffic only. Traffic coming from ATE-1 to DUT is classified based on the EXP bits of the MPLS label. Classifiers can match on the EXP bits of the inner MPLS header which supports all 8 values (EXP is a 3-bit field). Packets arriving on the DUT are expected to be MPLSoGRE where the EXP value are mapped from the DSCP value of the inner IP header according to this table.

Traffic Class# | Inner DSCP ranges marked by the Customer | MPLS EXP bits 
-------------- | ---------------------------------------- | -------------
    TC1        | 000xxx                                   | 001
    TC2        | 001xxx                                   | 010
    TC3        | 010xxx                                   | 011
    TC4        | 011xxx                                   | 100
    TC5        | 100xxx                                   | 101
    TC6        | 11xxxx                                   | 110

Egress Traffic:
  Traffic coming from ATE-1 to the DUT is encapsulated in MPLS over GRE. The DUT matches the EXP bits and, based on the policy and matched EXP bits, assigns the traffic to a specific traffic class.
Ingress Traffic:
  Traffic coming from ATE-2 to the DUT is plane IPv4. The DUT matches the Destination IP and, based on the policy and matched DSCP value, assigns the traffic to a specific traffic class.

## Topology

```

+------------+            +-------------+                  +--------------+
|    ATE-1   |            |             |                  |     ATE-2    |
|            |            |             |                  |              |
|            |            |             |                  |              |
|.-------.   |            |             |                  |   .-------.  |
(  pfx1   )  |     .      |             |  p2 :      : p2  |  (  pfx2   ) |
|`-------'   | p1 ; : p1  |     DUT     +-------+-+--------+   .-------.  |
|            |----+-+-----|             |------------------|              |
|            |    | |     |             +-------+-+--------+              |
|            |    | |     |             |  p3 : | |  : p3  |              |
|            |    : ;     |             |       | |        |              |
|            |     '      |             |        '         |              |
|            |   LAG_1    |             |      LAG_2       |              |
+------------+            +-------------+                  +--------------+

```

## Baseline setup

*   Connect ATE-1 port-1 to DUT ports 1(100G), and ATE-2 ports 2 through 3 to DUT ports 2-3(10G). 
*   Configure ATE-1 and DUT ports 1 to be part of a LAG_1.
*   Configure ATE-2 and DUT ports 2-3 to be part of a LAG_2.
*   Configure policy-map to decap traffic from ATE-1 to ATE-2.
*   Configure policy-map to encap traffic from ATE-2 to ATE-1.
*   Enable QoS on Ingress and Egress of aggregate interface with QoS policy.
*   Configure ATE MPLSoGRE flow for different traffic-class (TC0 to TC7) with mpls exp bits. 


## Procedure

At the start of each of the following scenarios, ensure:

*   All ports are up and baseline is reset as above.
*   Push baseline config to DUT and ATE

Set the inner DSCP value to 26 for all traffic flows.
Set the inner TTL value to 64 for all traffic flows.

On outer header set mpls exp bit for different traffic class as follows:
Traffic-Class: MPLS EXP Bit
 default:0, TC1:1, TC2:2, TC3:3, TC4:4, TC5:5, TC6:6, TC7:7

### Test-1

No changes in DSCP marking and TTL when policy is applied to bundle interface

*   Apply policy number 3.4
*   Generate MPLSoGRE Traffic as below:
    TC1:TC2:TC3:TC4:TC5:TC6:TC7:Default = 2Gbps:2Gbps:2Gbps:2Gbps:2Gbps:2Gbps:2Gbps:2Gbps
*   Capture the traffic on ATE-2 port and Verify the Inner packet doesn't change DSCP and TTL Value.
*   Verify traffic distribution as per egress policy 3.4 applied on dut LAG_2 port as follows:
    TC1: 1Gbps, TC2: 1Gbps, TC3: 2Gbps, TC3: 2Gbps, TC4: 2Gbps, TC5: 2Gbps, TC6: 2Gbps, TC7: 2Gbps, Default: 1Gbps.

### Test-2

No changing in behavior of ingress traffic

*   Apply policy number 3.4
*   Generate Egress MPLSoGRE Traffic and Ingress traffic with plane IPv4 as below:
    Egress Traffic Flows:
      TC1:TC2:TC3:TC4:TC5:TC6:TC7:Default = 2Gbps:2Gbps:2Gbps:2Gbps:2Gbps:2Gbps:2Gbps:2Gbps
    Ingress Traffic:
      Flow: 4Gbps
*   Verify Ingress Traffic has no packet drop.
*   Verify traffic distribution as per egress policy 3.4 applied on dut LAG_2 port as follows:
    TC1: 1Gbps, TC2: 1Gbps, TC3: 2Gbps, TC3: 2Gbps, TC4: 2Gbps, TC5: 2Gbps, TC6: 2Gbps, TC7: 2Gbps, Default: 1Gbps.


### Test-3

Verify shaper can enforce maximum transmit rate

*   Apply policy number 3.4
*   Generate Egress MPLSoGRE Traffic and Ingress traffic with plane IPv4 as below:
    Egress Traffic Flows:
      TC1:TC2:TC3:TC4:TC5:TC6:TC7:Default = 5Gbps:5Gbps:5Gbps:5Gbps:5Gbps:5Gbps:5Gbps:5Gbps
*   Verify traffic distribution as per egress policy 3.4 applied on dut LAG_2 port as follows:
    TC1: 1Gbps, TC2: 1Gbps, TC3: 2Gbps, TC4: 3Gbps, TC5: 3Gbps, TC6: 4Gbps, TC7: 5Gbps, Default: 1Gbps.


### Test-4

LLQ without shaping

*   Apply policy number 5.6
*   Generate Egress MPLSoGRE Traffic:
    Egress Traffic Flows: TC1 sends 80% of bandwidth, default queue, TC2-TC7 each send 20% of bandwidth
      TC1:TC2:TC3:TC4:TC5:TC6:TC7:Default = 16Gbps:4Gbps:4Gbps:4Gbps:4Gbps:4Gbps:4Gbps:4Gbps
*   Verify traffic distribution as per egress policy 5.6 applied on dut LAG_2 port as follows:
    TC1: 16Gbps, TC2: 0.4Gbps, TC3: 0.6Gbps, TC4: 0.6Gbps, TC5: 0.6Gbps, TC6: 0.8Gbps, TC7: 0.8Gbps, Default: 0.2Gbps

### Test-5

LLQ with shaping

*   Apply policy number 5.7
*   Generate Egress MPLSoGRE Traffic:
    Egress Traffic Flows: TC1 sends 80% of bandwidth, default queue, TC2-TC7 each send 20% of bandwidth
      TC1:TC2:TC3:TC4:TC5:TC6:TC7:Default = 16Gbps:4Gbps:4Gbps:4Gbps:4Gbps:4Gbps:4Gbps:4Gbps
*   Verify traffic distribution as per egress policy 5.7 applied on dut LAG_2 port as follows:
    TC1: 2Gbps, TC2: 0.4Gbps, TC3: 0.6Gbps, TC4: 0.6Gbps, TC5: 0.6Gbps, TC6: 0.8Gbps, TC7: 0.8Gbps, Default: 0.2Gbps
### Test-6

Min bandwidth reservation

*   Apply policy number 6.1
*   Generate Egress MPLSoGRE Traffic:
    Egress Traffic Flows: TC1 sends 80% of bandwidth, default queue, TC2-TC7 each send 20% of bandwidth
      TC1:TC2:TC3:TC4:TC5:TC6:TC7:Default = 4Gbps:4Gbps:4Gbps:4Gbps:4Gbps:4Gbps:4Gbps:4Gbps
*   Verify traffic distribution as per egress policy 6.1 applied on dut LAG_2 port as follows:
    TC1: 2Gbps, TC2: 0.4Gbps, TC3: 0.6Gbps, TC4: 0.6Gbps, TC5: 0.6Gbps, TC6: 0.8Gbps, TC7: 0.8Gbps, Default: 0.2Gbps


### Test-7

Priority queue

*   Apply policy number 7.1
*   Generate Egress MPLSoGRE Traffic:
    Egress Traffic Flows: TC1 sends 80% of bandwidth, default queue, TC2-TC7 each send 20% of bandwidth
      TC1:TC2:TC3:TC4:TC5:TC6:TC7:Default = 6Gbps:6Gbps:6Gbps:6Gbps:6Gbps:6Gbps:6Gbps:6Gbps
*   Verify traffic distribution as per egress policy 7.1 applied on dut LAG_2 port as follows:
    TC1-TC3: 0, TC4: 2Gbps, TC5-TC7: 6Gbps, Default: 0

### Test-8

Equal Priority queue

*   Apply policy number 7.1
*   Generate Egress MPLSoGRE Traffic:
    Egress Traffic Flows: TC1 sends 80% of bandwidth, default queue, TC2-TC7 each send 20% of bandwidth
      TC1:TC2:TC3:TC4:TC5:TC6:TC7:Default = 6Gbps:6Gbps:6Gbps:6Gbps:6Gbps:6Gbps:6Gbps:6Gbps
*   Verify traffic distribution as per egress policy 7.1 applied on dut LAG_2 port as follows:
    TC1-TC7: 2.85Gbps, Default: 0

## Telemetry Parameter Coverage



## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
  gribi:
```

## Config parameter coverage
