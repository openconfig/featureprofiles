# PF-1.1: IPv4/IPv6 policy forwarding to indirect NH matching DSCP/TC.

## Summary

The test verifies policy forwarding(PF) when matching specific DSCP/traffic-class(TC) in IPv4/IPv6 header and pointing to indirect BGP next-hop.
2 rightmost DSCP/TC bits are used for this test with all the possibles combinations of 3 left-most DSCP/TC bits.

## Setup


*   DUT has an ingress port (Port 1) and 2 egress ports combined.

    ```
                             |         | --eBGP-- | ATE Port 2 |
        [ ATE Port 1 ] ----  |   DUT   |          |            |
                             |         | --eBGP-- | ATE Port 3 |
    ```

*   Traffic is generated from ATE Port 1.
*   ATE Port 2 is used as the destination port for PF. eBGP peerings
        on this port announces BG4IE NH.
*   ATE Port 3 is used as the fallback destination when PBR NH routes
        are withdrawn.

### Configuration

1.  All DUT Ports are configured as a singleton IP interfaces.

2.  Static routes to test IPv4 and IPv6 destination networks are configured on DUT towards ATE Port 3.

3.  eBGP session is configured on DUT port 2. Indirect /32 and /128 prefixes (PF next-hops) are announced via eBGP.

4.  PF matching traffic marked with rightmost DSCP/TC 2 bits set to `11` is configured on DUT port 1. PF
action is to redirect to next-hops announced in the previous step. List of
DSCP/TC values to be matched  [3, 7, 11, 19, 27, 35, 39, 51, 55, 59]


## Test cases

### PF-1.1.1: Verify PF next-hop action
Generate traffic on ATE Port 1 to test IPv4 and IPv6 destination networks with DSCP/TC rightmost 2 bits set to `11`. Generate flows for every DSCP/TC values in the set [3, 7, 11, 19, 27, 35, 39, 51, 55, 59].

Verify:

*  All traffic received on ATE Port 2.
*  No packet loss when forwarding.
*  Verify PF packet counters matching traffic generated.

### PF-1.1.2: Verify PF no-match action
Generate traffic on ATE Port 1 to test IPv4 and IPv6 destination networks with DSCP/TC rightmost 2 bits set to `00`. Generate flows for every DSCP/TC values in the set [0, 4, 8, 16, 24, 32, 36, 48, 52, 56]. 

Verify:

*  All traffic received on ATE Port 3.
*  No packet loss when forwarding.

### PF-1.1.3: Verify PF without NH present
Withdraw next-hop prefixes from BGP announcement. Generate traffic on ATE Port 1 to test IPv4 and IPv6 destination networks with DSCP/TC rightmost 2 bits set to `11`. Generate flows for every DSCP/TC value in the set [3, 7, 11, 19, 27, 35, 39, 51, 55, 59]. 

Verify:

*  All traffic received on ATE Port 3 (traffic follow default route).
*  No packet loss when forwarding.

## Config Parameter Coverage

*   `/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/dscp-set`
*   `/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/dscp-set`
*   `/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop`


## Telemetry Parameter Coverage

*   `/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts`
*   `/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets`