# PF-1.3: Policy-based IPv4 GRE Decapsulation

## Summary

This test verifies the functionality of policy-based forwarding (PF) to decapsulate GRE-encapsulated traffic.
The test verified IPv4, IPv6 and MPLS encapsulated traffic.
The test also confirms the correct forwarding of traffic not matching the decapsulation policy.

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

*   DUT has an ingress port and an egress port.

    ```
                             |         |
        [ ATE Port 1 ] ----  |   DUT   | ---- [ ATE Port 2 ]
                             |         |
    ```

*  ATE Port 1: Generates GRE-encapsulated traffic with various inner (original) destinations.
*  ATE Port 2: Receives decapsulated traffic whose inner destination matches the policy.

### DUT Configuration

1.  Interfaces: Configure all DUT ports as singleton IP interfaces.
 
2.  Static Routes/LSPs:
    *  Configure an IPv4 static route to GRE decapsulation destination (DECAP-DST) to Null0.
    *  Configure static routes for encapsulated traffic destinations IPV4-DST1 and IPV6-DST1 towards ATE Port 2.
    *  Configure static MPLS label binding (LBL1) towards ATE Port 2. Next hop of ATE Port 1 should be indicated for MPLS pop action.
    *  Configure static routes for destination IPV4-DST2 and IPV6-DST2 towards ATE Port 2.

3.  Policy-Based Forwarding: 
    *  Rule 1: Match GRE traffic with destination DECAP-DST using destination-address-prefix-set and decapsulate.
    *  Rule 2: Match all other traffic and forward (no decapsulation).
    *  Apply the defined policy with to the ingress ATE Port 1 interface. 
    
    **TODO:** OC model does not have a provision to apply decap policy at the network-instance level for traffic destined to device loopback interface (see Cisco CLI config exepmt below). Needs clarification and/or augmentation by vendors if required. [PR #1150](https://github.com/openconfig/public/pull/1150)

    ```
    vrf-policy
      vrf default address-family ipv4 policy type pbr input DECAP-POLICY
    ```

    
### PF-1.3.1: GRE Decapsulation of IPv4 traffic
-  Push DUT configuration.

Traffic: 
-  Generate GRE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST. 
-  Inner IPv4 destination should match IPV4-DST1.
-  Inner-packet DSCP value should be set to 32. 

Verification: 
-  Decapsulated IPv4 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet DSCP should be preserved.
-  PF counters reflect decapsulated packets.

### PF-1.3.2: GRE Decapsulation of IPv6 traffic
-  Push DUT configuration.

Traffic: 
-  Generate IPv6 GRE-encapsulated traffic from ATE Port 1 with destinations matching DECAP-DST. 
-  Inner IPv6 destination should match IPV6-DST1.
-  Inner-packet traffic-class should be set to 128. 

Verification:
-  Decapsulated IPv6 traffic is received on ATE Port 2.
-  No packet loss.
-  Inner-packet traffic-class should be preserved.
-  PF counters reflect decapsulated packets.


### PF-1.3.3: GRE Decapsulation of IPv4-over-MPLS traffic
-  Push DUT configuration.

Traffic: 
-  Generate GRE-encapsulated IPv4-over-MPLS traffic from ATE Port 1 with destinations matching DECAP-DST. 
-  Encapsulated MPLS top label should match LBL1.
-  Inner IPv4 packet DSCP should be set to 32. 

Verification:
-  Decapsulated IPv4 traffic is received on ATE Port 2.
-  No packet loss.
-  TTL should be taken from the outer GRE header, decremented by 1 and copied to egress IP packet header.
-  Inner-packet DSCP should be preserved.
-  PF counters reflect decapsulated packets.


### PF-1.3.4: GRE Decapsulation of IPv6-over-MPLS traffic
-  Push DUT configuration.

Traffic: 
-  Generate GRE-encapsulated IPv4-over-MPLS traffic from ATE Port 1 with destinations matching DECAP-DST. 
-  Encapsulated MPLS top label should match LBL1.
-  Inner IPv6 packet traffic-class should be set to 128. 

Verification:
-  Decapsulated IPv6 traffic is received on ATE Port 2.
-  No packet loss.
-  TTL should be taken from the outer GRE header, decremented by 1 and copied to egress IP packet header.
-  Inner-packet traffic-class should be preserved.
-  PF counters reflect decapsulated packets.



### PF-1.3.5: GRE Decapsulation of multi-label MPLS traffic
-  Push DUT configuration.

Traffic: 
-  Generate GRE-encapsulated MPLS traffic from ATE Port 1 with destinations matching DECAP-DST. 
-  MPLS packets will have 2 labels. 
-  Top label should match LBL1. 
-  MPLS second label can be any. 
-  MPLS EXP bit on both labels should be set to 4.

Verification:
-  Decapsulated MPLS traffic is received on ATE Port 2.
-  TTL should be taken from the outer GRE header, decremented by 1 and copied to egress MPLS packet header.
-  No packet loss.
-  PF counters reflect decapsulated packets.
-  EXP should set to original value.


### PF-1.3.6: GRE Pass-through (Negative)
-  Push DUT configuration.

Traffic: 
-  Generate GRE-encapsulated traffic from ATE Port 1 with destinations that match IPV4-DST1/IPV6-DST2.

Verification:
-  Traffic is forwarded to ATE Port 2 unchanged.


## OpenConfig Path and RPC Coverage

```yaml
paths:
    # match condition
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/destination-address-prefix-set:
    # decap action
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gre:
    # application to the interface
    /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy:
    # TODO: provision apply decap to network-instance level does not exist. Needs clarification and/or augmentation by vendors.

    # telemetry
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF
