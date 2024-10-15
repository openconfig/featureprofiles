# PF-1.4: Interface based GUE Decapsulation to IPv4 tunnel

## Summary

This is to test the functionality of policy-based forwarding (PF) to decapsulate Generic UDP Encapsulation variant 1 ([GUE Variant 1](https://datatracker.ietf.org/doc/html/draft-ietf-intarea-gue-09#section-4)) traffic. These tests verify the use case of IPv4 and IPv6 encapsulated traffic in IPv4 GUE tuennel. The tests validate that the DUT performs the following action.

 - Decapsulate the outer (transport) layer 3 and GUE headers of GUE packets destined to locally configured decap IPv4 address/addresses and matching UDP port. Post decapsulation, the dut will perform a lookup of the exposed inner (payload) L3 header and forward.
 - GUE Inner protocol type must be derived from a unique DST port. If not specifically configured, then the following default DST UDP port will be used.
    - For inner IPv4 - GUE UDP port 6080
    - For inner IPv6 - GUE UDP port 6615
 - Post decapsulation the DUT should copy outer TTL(and decrement) to inner header and maintain the inner DSCP vaule as is.
    - If explicit configration is present to not copy the TTL, then it will be honored. 
 - Decapsulate the packet only if it matches the locally configured decap IPv4 address/addresses and matching UDP port port/port-range.
    - Traffic not subject to match criteria will be forwared using traditional IP forwarding. 

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

* Create the following connections:
* DUT has ingress and egress port connected to the ATE.
  
```mermaid
graph LR; 
A[ATE:Port1] --Ingress--> B[Port1:DUT:Port2];B --Egress--> C[Port2:ATE];
```

*  ATE Port 1: Generates GUE-encapsulated traffic with various inner (original) destinations.
*  ATE Port 2: Receives decapsulated traffic whose inner destination matches the policy TEST-GUE.
  
### DUT Configuration

1.  Interfaces: Configure all DUT ports as singleton IP interfaces.
 
2. GUE Decapsulation related:
    *  Configure an IPv4 static route to GUE decapsulation destination (DECAP-DST) to Null0.
    *  Have policy TEST-GUE configured that match GUE decapsulation destination and default/non-default GUE UDP port/port-range for the decapsulation.
       *  If udp port is not configured then the default GUE UDP port will be used (UDP port 6080 for Inner IPv4 and UDP port 6615 for Inner IPv6 traffic).
    *  Apply the defined policy on the DUT.
    *  Configure static routes for encapsulated traffic destinations IPV4-DST1 and IPV6-DST1 towards ATE Port 2.
    *  Configure static routes for destination IPV4-DST2 and IPV6-DST2 towards ATE Port 2.

3.  Policy-Based Forwarding Related: 
    *  Rule 1: Match GUE traffic with destination DECAP-DST using destination-address-prefix-set and default/non-default GUE UDP port/port-range for decapsulation.
      * If udp port is not configured then the default GUE UDP port will be used (UDP port 6080 for Inner IPv4 and UDP port 6615 for Inner IPv6 traffic).   
    *  Rule 2: Match all other traffic and forward (no decapsulation).
    *  Apply the defined policy on the DUT.
    
### PF-1.4.1: GUE Decapsulation of inner IPv4 traffic using default GUE UDP port 6080
-  Push DUT configuration.

Traffic: 
-  Generate GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and default GUE UDP port 6080.
-  Inner IPv4 destination should match IPV4-DST1.
-  Inner-packet DSCP value should be set to 32.
-  Inner-packet TTL value should be set to 64.
  
Verification: 
-  Decapsulated IPv4 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet DSCP should be preserved.
-  Inner-packet TTL value should be decremented by 1 to 63.
-  PF counters reflect decapsulated packets.

### PF-1.4.2: GUE Decapsulation of inner IPv4 traffic using non-default GUE UDP port or port-range
-  Push DUT configuration.

Traffic: 
-  Generate GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and configured non-default GUE UDP port or port-range. 
-  Inner IPv4 destination should match IPV4-DST1.
-  Inner-packet DSCP value should be set to 32.
-  Inner-packet TTL value should be set to 64.

Verification: 
-  Decapsulated IPv4 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet DSCP should be preserved.
-  Inner-packet TTL value should be decremented by 1 to 63.
-  PF counters reflect decapsulated packets.

### PF-1.4.3: GUE Decapsulation of inner IPv6 traffic using default GUE UDP port 6615
-  Push DUT configuration.

Traffic: 
-  Generate IPv6 GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and GUE UDP port 6615. 
-  Inner IPv6 destination should match IPV6-DST1.
-  Inner-packet traffic-class should be set to 128.
-  Inner-packet TTL value should be set to 64.

Verification:
-  Decapsulated IPv6 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet traffic-class should be preserved.
-  Inner-packet TTL value should be decremented by 1 to 63.
-  PF counters reflect decapsulated packets.

### PF-1.4.4: GUE Decapsulation of inner IPv6 traffic using non-default GUE UDP port or port-range
-  Push DUT configuration.

Traffic: 
-  Generate IPv6 GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and configured non-default GUE UDP port or port-range. 
-  Inner IPv6 destination should match IPV6-DST1.
-  Inner-packet traffic-class should be set to 128.
-  Inner-packet TTL value should be set to 64.

Verification:
-  Decapsulated IPv6 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet traffic-class should be preserved.
-  Inner-packet TTL value should be decremented by 1 to 63.
-  PF counters reflect decapsulated packets.

### PF-1.4.5: GUE Decapsulation of inner IPv4 traffic using default GUE UDP port 6080 and NO-TTL propogation
-  Push DUT configuration with a knob to disable TTL propogation.

Traffic: 
-  Generate GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and default GUE UDP port 6080.
-  Inner IPv4 destination should match IPV4-DST1.
-  Inner-packet DSCP value should be set to 32.
-  Inner-packet TTL value should be set to 64.

Verification: 
-  Decapsulated IPv4 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet DSCP should be preserved.
-  Inner-packet TTL value will remain 64.
-  PF counters reflect decapsulated packets.

### PF-1.4.6: GUE Decapsulation of inner IPv4 traffic using non-default GUE UDP port or port-range and NO-TTL propogation
-  Push DUT configuration with a knob to disable TTL propogation.

Traffic: 
-  Generate GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and configured non-default GUE UDP port or port-range.
-  Inner IPv4 destination should match IPV4-DST1.
-  Inner-packet DSCP value should be set to 32.
-  Inner-packet TTL value should be set to 64.

Verification: 
-  Decapsulated IPv4 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet DSCP should be preserved.
-  Inner-packet TTL value will remain 64.
-  PF counters reflect decapsulated packets.

### PF-1.4.7: GUE Decapsulation of inner IPv6 traffic using default GUE UDP port 6615 and NO-TTL propogation
-  Push DUT configuration with a knob to disable TTL propogation.

Traffic: 
-  Generate IPv6 GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and GUE UDP port 6615.
-  Inner IPv6 destination should match IPV6-DST1.
-  Inner-packet traffic-class should be set to 128.
-  Inner-packet TTL value should be set to 64.

Verification:
-  Decapsulated IPv6 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet traffic-class should be preserved.
-  Inner-packet TTL value will remain 64.
-  PF counters reflect decapsulated packets.

### PF-1.4.8: GUE Decapsulation of inner IPv6 traffic using non-default GUE UDP port or port-range and NO-TTL propogation
-  Push DUT configuration with a knob to disable TTL propogation.

Traffic: 
-  Generate IPv6 GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and configured non-default GUE UDP port or port-range.
-  Inner IPv6 destination should match IPV6-DST1.
-  Inner-packet traffic-class should be set to 128.
-  Inner-packet TTL value should be set to 64.

Verification:
-  Decapsulated IPv6 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet traffic-class should be preserved.
-  Inner-packet TTL value will remain 64.
-  PF counters reflect decapsulated packets.
  
### PF-1.4.9: GUE Decapsulation of inner IPv6 traffic using default IPv4 GUE UDP port 6080 (Negative).
-  Push DUT configuration.

Traffic: 
-  Generate GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and default GUE UDP port 6080 meant for inner IPv4.
-  Inner IPv6 destination should match IPV6-DST1.
-  Inner-packet traffic-class should be set to 128.
-  Inner-packet TTL value should be set to 64.
  
Verification: 
-  Traffic will be dropped on DUT.
-  No negative impact on CPU in case of high traffic rate.
-  PF counters for dropped packets ( invalid inner protocol ) reflect ingress packet count.
-  100% packet loss.

### PF-1.4.10: GUE Decapsulation of inner IPv6 traffic using non-default GUE UDP port or port-range meant for IPv4 inner traffic (Negative).
-  Push DUT configuration.

Traffic: 
-  Generate GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and configured non-default GUE UDP port or port-range that is meant for inner IPv4 traffic. 
-  Inner IPv6 destination should match IPV6-DST1.
-  Inner-packet traffic-class should be set to 128.
-  Inner-packet TTL value should be set to 64.

Verification: 
-  Traffic will be dropped on DUT.
-  No negative impact on CPU in case of high traffic rate.
-  PF counters for dropped packets ( invalid inner protocol ) reflect ingress packet count.
-  100% packet loss.


### PF-1.4.11: GUE Decapsulation of inner IPv4 traffic using default IPv6 GUE UDP port 6615 (Negative).
-  Push DUT configuration.

Traffic: 
-  Generate GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and default GUE UDP port 6615 meant for inner IPv6.
-  Inner IPv4 destination should match IPV4-DST1.
-  Inner-packet DSCP value should be set to 32.
-  Inner-packet TTL value should be set to 64.
  
Verification: 
-  Traffic will be dropped on DUT.
-  No negative impact on CPU in case of high traffic rate.
-  PF counters for dropped packets ( invalid inner protocol ) reflect ingress packet count.
-  100% packet loss.

### PF-1.4.12: GUE Decapsulation of inner IPv4 traffic using non-default GUE UDP port or port-range meant for IPv6 inner traffic (Negative).
-  Push DUT configuration.

Traffic: 
-  Generate GUE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST and configured non-default GUE UDP port or port-range that is meant for inner IPv6 traffic. 
-  Inner IPv4 destination should match IPV4-DST1.
-  Inner-packet DSCP value should be set to 32.
-  Inner-packet TTL value should be set to 64.

Verification: 
-  Traffic will be dropped on DUT.
-  No negative impact on CPU in case of high traffic rate.
-  PF counters for dropped packets ( invalid inner protocol ) reflect ingress packet count.
-  100% packet loss.

### PF-1.4.17: GUE Pass-through (Negative)
-  Push DUT configuration.

Traffic: 
-  Generate GUE-encapsulated traffic from ATE Port 1 with destinations that match IPV4-DST2/IPV6-DST2.

Verification:
-  Traffic will not match the policy and forwarded to ATE Port 2 unchanged.

## Config Parameter Coverage

## Telemetry Parameter Coverage

## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

```
```

## Required DUT platform

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
