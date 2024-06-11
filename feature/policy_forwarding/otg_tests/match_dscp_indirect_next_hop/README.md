# PF-1.1: IPv4/IPv6 policy-forwarding to indirect NH matching DSCP/TC.

## Summary

The test verifies policy-forwarding(PF) when matching specific DSCP values in IPv4/IPv6 header and redirecting traffic to an indirect BGP next-hop.

2 right-most bits are used for this test with all the possibles combinations of 3 left-most DSCP bits: `...011`.

## Testbed type

*  [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

*   DUT has an ingress port (Port 1) and 2 egress ports combined.

    ```
                             |         | --eBGP-- | ATE Port 2 |
        [ ATE Port 1 ] ----  |   DUT   |          |            |
                             |         | -------- | ATE Port 3 |
    ```

*   Traffic is generated from ATE Port 1.
*   ATE Port 2 is used as the destination port for PF. eBGP peerings
        on this port announces BG4IE NH.
*   ATE Port 3 is used as the fallback destination when PF NH routes
        are withdrawn.

#### Configuration

1.  All DUT Ports are configured as a singleton IP interfaces.

2.  Static routes (ST1) to test IPv4 and IPv6 destination networks (IPV4-DST1/IPV6-DST1) are configured on DUT towards ATE Port 3.

3.  eBGP session is configured on DUT port 2. Indirect /32 (IPV-NH-V4) and /128 (IPV-NH-V6) prefixes are announced via eBGP from ATE Port 2.

4.  PF is configured on DUT port 1 to match the traffic marked with rightmost 2 bits set in DSCP to 11. PF action is to redirect to BGP-announced next-hops (IPV-NH-V4/IPV-NH-V6): 
  *  List of DSCP values (6-bit) to be matched  [3, 11, 19, 27, 35, 43, 51, 59]
  *  Matching rules for IPv6 should map the above 6-bit DSCP values to the leftmost 6-bits of IPv6 traffic-class.
  *  PF should permit the rest of the traffic.

### PF-1.1.1: Verify PF next-hop action
Generate traffic on ATE Port 1 to test IPv4 and IPv6 destination networks (IPV4-DST1/IPV6-DST1) with DSCP/TC rightmost 2 bits set to `11`. Generate flows for every DSCP value in the set [3, 11, 19, 27, 35, 43, 51, 59].
IPv6 flows should use TC 8-bit values [12, 44, 76, 108, 172, 163, 204, 236]

Verify:

*  All traffic received on ATE Port 2.
*  No packet loss when forwarding.
*  Verify PF packet counters matching traffic generated.

### PF-1.1.2: Verify PF no-match action
Generate traffic on ATE Port 1 to test IPv4 and IPv6 destination networks (IPV4-DST1/IPV6-DST1) with DSCP/TC rightmost 2 bits set to `00`. Generate flows for every DSCP/TC values in the set [0, 8, 16, 24, 32, 40, 48, 56]. IPv6 flows should use TC 8-bit values [0, 32, 64, 96, 128, 160, 192, 224]

Verify:

*  All traffic received on ATE Port 3.
*  No packet loss when forwarding.

### PF-1.1.3: Verify PF without NH present
Withdraw next-hop prefixes (IPV-NH-V4/IPV-NH-V6) from BGP announcement. Generate traffic on ATE Port 1 to test IPv4 and IPv6 destination (IPV4-DST1/IPV6-DST1) networks with DSCP/TC rightmost 2 bits set to `11`. Generate flows for every IPv4 DSCP value in the set [3, 11, 19, 27, 35, 43, 51, 59] and IPv6 TC [0, 32, 64, 96, 128, 160, 192, 224].

Verify:

*  All traffic received on ATE Port 3.
*  Traffic follows fallbacks static routes (ST1).
*  No packet loss when forwarding.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # PF configuration 
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/dscp-set:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/dscp-set:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop:
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