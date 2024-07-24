# PF-1.2: Policy-based traffic GRE Encapsulation to IPv4 GRE tunnel

## Summary

The test verifies policy forwarding(PF) encapsulation action to IPv4 GRE tunnel when matching on source/destination.

## Testbed type

*  [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

*   DUT has an ingress port and 2 egress ports.

    ```
                             |         | --eBGP-- | ATE Port 2 |
        [ ATE Port 1 ] ----  |   DUT   |          |            |
                             |         | --eBGP-- | ATE Port 3 |
    ```

*   Traffic is generated from ATE Port 1.
*   ATE Port 2 is used as the destination port for encapsulated 
    traffic.
*   ATE Port 3 is used as the fallback destination for
    pass-through traffic.

#### Configuration

1.  All DUT Ports are configured as a singleton IP interfaces. Configure MTU of 9216 (L2) on ATE Port1, MTU 2000 on ATE Port 2, 3
 
2.  IPv4 and IPv6 static routes to test destination networks IPV4-DST/IPV6-DST are configured on DUT towards ATE Port 3.

3.  Another set of IPv4 static routes to 32x IPv4 GRE encap destinations towards ATE Port 2.

4.  2 IPv4 and 2 IPv6 source prefixes will be used to generate tests traffic 
(SRC1-SRC2). Apply policy-forwarding with 4 rules to DUT Port 1:
    - Match IPV4-SRC1 and accept/foward.
    - Match IPV6-SRC1 and accept/foward.
    - Match IPV4-SRC2 and encapsulate to 32 IPv4 GRE destinations.
    - Match IPV6-SRC2 and encapsulate to 32 IPv4 GRE destinations.

5.   Set GRE encap source to device's loopback interface.
6.   Either `identifying-prefix` or `targets/target/config/destination` can be used to configure GRE destinations based on vendor implementation.
7.   Configure QoS classifier for incoming traffic on ATE Port1 for IPv4 and IPv6 traffic. 
     QoS classifier should remark egress packet to the matching ingress DSCP value (eg. match DSCP 32, set egress DSCP 32).
     Match and remark all values for 3 leftmost DSCP bits [0, 8, 16, 24, 32, 40, 48, 56].
    

### PF-1.1.1: Verify PF GRE encapsulate action for IPv4 traffic
Generate traffic on ATE Port 1 from IPV4-SRC2 from a random combination of 1000 source addresses to IPV4-DST at linerate.
Use 512 bytes frame size.

Verify:

*  All traffic received on ATE Port 2 GRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 32 GRE destinations.
*  Verify PF packet counters matching traffic generated.

### PF-1.1.2: Verify PF GRE encapsulate action for IPv6 traffic
Generate traffic on ATE Port 1 from IPV6-SRC2 from a random combination of 1000 source addresses to IPV6-DST.
Use 512 bytes frame size.

Verify:

*  All traffic received on ATE Port 2 GRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 32 GRE destinations.
*  Verify PF packet counters matching traffic generated.

### PF-1.1.3: Verify PF IPV4 forward action
Generate traffic on ATE Port 1 from sources IPV4-SRC1 to IPV4-DST.

Verify:

*  All traffic received on ATE Port 3.
*  No packet loss when forwarding.

### PF-1.1.4: Verify PF IPV6 forward action
Generate traffic on ATE Port 1 from sources IPV6-SRC1 to IPV6-DST.

Verify:

*  All traffic received on ATE Port 3.
*  No packet loss when forwarding.

### PF-1.1.5: Verify PF GRE DSCP copy to outer header for IPv4 traffic
Generate traffic on ATE Port 1 from IPV4-SRC1 source for every DSCP value in [0, 8, 16, 24, 32, 40, 48, 56]

Verify:

*  All traffic received on ATE Port 2 GRE-encapsulated.
*  Outer GRE IPv4 header has same marking as ingress non-encapsulated IPv4 packet.

### PF-1.1.6: Verify PF GRE DSCP copy to outer header for IPv6 traffic
Generate traffic on ATE Port 1 from IPV6-SRC1 for every IPv6 TC 8-bit value [0, 32, 64, 96, 128, 160, 192, 224]

Verify:

*  All traffic received on ATE Port 2 GRE-encapsulated.
*  Outer GRE IPv4 header has DSCP match to ingress IPv6 TC packet.

### PF-1.1.7: Verify MTU handling during GRE encap
* Generate traffic on ATE Port 1 from IPV4-SRC1 with frame size of 4000 with DF-bit set.
* Generate traffic on ATE Port 1 from IPV6-SRC1 with frame size of 4000 with DF-bit set.

Verify:

*  DUT generates "Fragmentation Needed" message back to ATE source.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # match condition
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address:
    # encap action
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encapsulate-gre/targets/target/config/id:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encapsulate-gre/targets/target/config/source:
    # either destination or identifying-prefix can be specified based on specific vendor implementation.
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encapsulate-gre/targets/target/config/destination:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encapsulate-gre/config/identifying-prefix:
    # application to the interface
    /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy:

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

* MFF
* FFF