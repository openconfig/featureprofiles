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

1.  All DUT Ports are configured as a singleton IP interfaces.

2.  IPv4 and IPv6 static routes to test destination networks IPV4-DST/IPV6-DST are configured on DUT towards ATE Port 3.

3.  Another set of IPv4 static routes to 32x IPv4 GRE encap destinations towards ATE Port 2.

4.  2 IPv4 and 2 IPv6 source prefixes will be used to generate tests traffic 
(SRC1-SRC2). Apply policy-forwarding with 4 rules to DUT Port 1:
    - Match IPV4-SRC1 and accept/foward.
    - Match IPV6-SRC1 and accept/foward.
    - Match IPV4-SRC2 and encapsulate to 32 IPv4 GRE destinations.
    - Match IPV6-SRC2 and encapsulate to 32 IPv4 GRE destinations.

5.   Set GRE encap source to device's loopback interface.
6.   Either `identifying-prefix` or `targets/target/config/destination` can be used to configure GRE destinations.

### PF-1.1.1: Verify PF GRE encapsulate action for IPv4 traffic
Generate traffic on ATE Port 1 from IPV4-SRC2 from a random combination of 1000 source addresses to IPV4-DST.

Verify:

*  All traffic received on ATE Port 2 GRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 32 GRE destinations.
*  Verify PF packet counters matching traffic generated.

### PF-1.1.2: Verify PF GRE encapsulate action for IPv6 traffic
Generate traffic on ATE Port 1 from IPV6-SRC2 from a random combination of 1000 source addresses to IPV6-DST.

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

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # match condition
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address:
    # encap action
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encapsulate-gre/targets/target/config/id:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/encapsulate-gre/targets/target/config/source:
    # either destination or identifying-prefix can be specified
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