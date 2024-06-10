# TUN-1.1: Interface-based traffic IPv4 GRE Encapsulation using next-hop redirection

## Summary

The test verifies traffic encapsulation using a IPv4 tunnel GRE interface using next-hop redirection via policy forwarding.

## Testbed type

*  [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

*   DUT has an ingress port and 2 egress ports.

    ```
                             |         | ---- | ATE Port 2 |
        [ ATE Port 1 ] ----  |   DUT   |      |            |
                             |         | ---- | ATE Port 3 |
    ```

*   Traffic is generated from ATE Port 1.
*   ATE Port 2 is used as the destination port for encapsulated 
    traffic.
*   ATE Port 3 is used as the fallback destination for
    pass-through traffic.


#### Configuration

1.  All DUT Ports are configured as a singleton IP interfaces. Configure MTU of 9216 (L2) on ATE Port1, Port2
 
2.  IPv4 and IPv6 static routes to test destination networks IPV4-DST/IPV6-DST are configured on DUT towards ATE Port 3.

3.  A set of IPv4 static routes to 32x IPv4 GRE tunnel endpoints towards ATE Port 2.

4.  32 tunnel interfaces are configured wtih IPv4 tunnel endpoints as destination and device loopback as source. Configure MTU on tunnel interfaces to 2000.
    All tunnel interfaces should be unnumbered referencing loopback interface.

5.  IPv4 encapsulation next-hop (IPV4-ENCAP-NH) is allocated. A set of IPv4 static routes to IPV4-ENCAP-NH is configured toward 32 tunnel interfaces.

6.  2 IPv4 and 2 IPv6 source prefixes will be used to generate tests traffic 
(SRC1-SRC2). Apply policy-forwarding with 4 rules to DUT Port 1:
    - Define prefix-sets for IPV4-SRC1,IPV6-SRC1,IPV4-SRC2,IPV6-SRC2
    - Match /22 IPV4-SRC1 and accept/foward.
    - Match /48 IPV6-SRC1 and accept/foward.
    - Match IPV4-SRC2 and set next-hop to IPV4-ENCAP-NH.
    - Match IPV6-SRC2 and set next-hop to IPV4-ENCAP-NH.
    

### TUN-1.1.1: Verify PF GRE encapsulate action for IPv4 traffic
Generate traffic on ATE Port 1 from IPV4-SRC2 from a random combination of 1000 source addresses to IPV4-DST.
Use 512 bytes frame size.

Verify:

*  All traffic received on ATE Port 2 GRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 32 GRE tunnels.
*  Verify PF packet counters matching traffic generated.

### TUN-1.1.2: Verify PF GRE encapsulate action for IPv6 traffic
Generate traffic on ATE Port 1 from IPV6-SRC2 from a random combination of 1000 source addresses to IPV6-DST.
Use 512 bytes frame size.

Verify:

*  All traffic received on ATE Port 2 GRE-encapsulated.
*  No packet loss when forwarding.
*  Traffic equally load-balanced across 32 GRE tunnels.
*  Verify PF packet counters matching traffic generated.

### TUN-1.1.3: Verify PF IPV4 forward action
Generate traffic on ATE Port 1 from sources IPV4-SRC1 to IPV4-DST.

Verify:

*  All traffic received on ATE Port 3.
*  No packet loss when forwarding.

### TUN-1.1.4: Verify PF IPV6 forward action
Generate traffic on ATE Port 1 from sources IPV6-SRC1 to IPV6-DST.

Verify:

*  All traffic received on ATE Port 3.
*  No packet loss when forwarding.

### TUN-1.1.5: Verify PF GRE DSCP copy to outer header for IPv4 traffic
Generate traffic on ATE Port 1 from IPV4-SRC1 source for every DSCP value in [0, 8, 16, 24, 32, 40, 48, 56]

Verify:

*  All traffic received on ATE Port 2 GRE-encapsulated.
*  Outer GRE IPv4 header has same marking as ingress non-encapsulated IPv4 packet.

### TUN-1.1.6: Verify MTU handling during GRE encap
* Generate traffic on ATE Port 1 from IPV4-SRC1 with frame size of 4000 with DF-bit set.
* Generate traffic on ATE Port 1 from IPV6-SRC1 with frame size of 4000 with DF-bit set.

Verify:

*  DUT generates "Fragmentation Needed" message back to ATE source.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # tunnel interfaces
    /interfaces/interface/tunnel/config/src:
    /interfaces/interface/tunnel/config/dst:
    /interfaces/interface/tunnel/ipv4/config/mtu:
    /interfaces/interface/tunnel/ipv6/config/mtu:
    /access-points/access-point/interfaces/interface/tunnel/ipv4/unnumbered/config/enabled:
    /access-points/access-point/interfaces/interface/tunnel/ipv4/unnumbered/interface-ref/config/interface:
    # define prefix-set
    /defined-sets/ipv4-prefix-sets/ipv4-prefix-set:
    /defined-sets/ipv4-prefix-sets/ipv6-prefix-set:
    # PF match condition
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address-prefix-set:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address-prefix-set:
    # PF nexthop action
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop:
    # PF application to the interface
    /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy:
    # static route
    /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
    /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/interface-ref/config/interface:
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