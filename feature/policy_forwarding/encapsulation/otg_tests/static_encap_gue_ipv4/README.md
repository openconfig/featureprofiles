# PF-1.3: Policy-based Static GUE Encapsulation to IPv4 tunnel

## Summary

This test verifies Policy Forwarding (PF) action to encapsulate packets in an
IPv4 GUE tunnel. The encapsulation is based on a statically configured GUE
tunnel and static route.

The GUE header ((UDPoIP)) used in this test refers to GUE Variant 1 as specified
in
[draft-ietf-intarea-gue-09](https://datatracker.ietf.org/doc/html/draft-ietf-intarea-gue-09).

The following behavioral properties are called out for awareness:

*   When the DSCP for the tunnel is not explicitly set, it is copied from the
    inner to the outer encapsulating header.
*   When the TTL for the tunnel is not explicitly set, it is decremented by 1
    and copied from inner to the outer encapsulating header. Except TTL=1, where
    Time Exceeded is sent back to the inner source IP.

## Testbed Type

*  [`featureprofiles/topologies/atedut_5.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_5.testbed)

## Procedure

### Test environment setup

*   DUT has 5 ports connected to ATE.

    ```
                              -------
                             |       | ==== LAG1 ==== [ ATE:Port2, ATE:Port3 ]
          [ ATE:Port1 ] ---- |  DUT  |
                             |       | ==== LAG2 ==== [ ATE:Port4, ATE:Port5 ]
                              -------
    ```

*   Routes are advertised from ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5.
*   Traffic is generated from ATE:Port1.
*   ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5 are used as the destination port for GUE encapsulated traffic.

#### Configuration

1.  DUT:Port1 is configured as Singleton IP interface towards ATE:Port1.

2.  DUT:Port2 and DUT:Port3 are configured as LAG1 IP interface towards
    ATE:Port2 and ATE:Port3 respectively.

3.  DUT:Port4 and DUT:Port5 are configured as LAG2 IP interface towards
    ATE:Port4 and ATE:Port5 respectively.

4.  DUT is configured to form two eBGP sessions with ATE using the directly
    connected LAG interface IPs.

5.  ATE:LAG1 and ATE:LAG2 are configured to advertise destination networks
    (IPv4-DST-NET, IPv6-DST-NET) and tunnel destination (IPv4-DST-GUE) to DUT.
    The protocol next hops for the destination networks should be re-configured
    as below:

    -   Destination network IPv4-DST-NET with protocol next hop PNH-IPv4.
    -   Destination network IPv6-DST-NET with protocol next hop PNH-IPv6.

6.  DUT is configured with an IPv4 GUE tunnel with destination IPv4-DST-GUE
    without any explicit tunnel Type of Service (ToS) or Time to Live (TTL)
    values.

7.  DUT is configured with the following static routes:

    -   To PNH-IPv4, next hop is the statically configured IPv4 GUE tunnel.
    -   To PNH-IPv6, next hop is the statically configured IPv4 GUE tunnel.

### PF-1.3.1: IPv4 traffic GUE encapsulation without explicit ToS/TTL configuration on tunnel

ATE action:

*   Generate 12000000 **IPv4 packets** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET ensuring that there are at least 65000 different 5-tuple flows.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of all packets to *10*.

Verify:

*   Policy forwarding rule `matched-packets` counters equals the number of
    packets generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets:
    *   Egressing DUT:Port2, DUT:Port3, DUT:Port4 and DUT:Port5.
    *   Received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5.
*   All packets received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5 are
    GUE encapsulated.
*   ECMP hashing works over the two LAG interfaces with a tolerance of 6%.
*   LAG hashing works over the two Singleton ports within LAG1 and LAG2 with a
    tolerance of 6%.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x80**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **9**.
    *   Inner header TTL is **9**.

### PF-1.3.2: IPv6 traffic GUE encapsulation without explicit ToS/TTL configuration on tunnel

*   Modify the flows in `PF-1.3.1` to use IPv6 and repeat the traffic generation
    and validation.

### PF-1.3.3: IPv4 traffic GUE encapsulation with explicit ToS configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT with ToS value *0x60*.
*   Generate 12000000 **IPv4 packets** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET ensuring that there are at least 65000 different 5-tuple flows.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of all packets to *10*.

Verify:

*   Policy forwarding rule `matched-packets` counters equals the number of
    packets generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets:
    *   Egressing DUT:Port2, DUT:Port3, DUT:Port4 and DUT:Port5.
    *   Received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5.
*   All packets received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5 are
    GUE encapsulated.
*   ECMP hashing works over the two LAG interfaces with a tolerance of 6%.
*   LAG hashing works over the two Singleton ports within LAG1 and LAG2 with a
    tolerance of 6%.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x60**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **9**.
    *   Inner header TTL is **9**.

### PF-1.3.4: IPv6 traffic GUE encapsulation with explicit ToS configuration on tunnel

*   Modify the flows in `PF-1.3.3` to use IPv6 and repeat the traffic generation
    and validation.

### PF-1.3.5: IPv4 traffic GUE encapsulation with explicit TTL configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT without an explicit ToS value.
*   Re-configure the IPv4 GUE tunnel on the DUT with TTL value of *20*.
*   Generate 12000000 **IPv4 packets** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET ensuring that there are at least 65000 different 5-tuple flows.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of all packets to *10*.

Verify:

*   Policy forwarding rule `matched-packets` counters equals the number of
    packets generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets:
    *   Egressing DUT:Port2, DUT:Port3, DUT:Port4 and DUT:Port5.
    *   Received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5.
*   All packets received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5 are
    GUE encapsulated.
*   ECMP hashing works over the two LAG interfaces with a tolerance of 6%.
*   LAG hashing works over the two Singleton ports within LAG1 and LAG2 with a
    tolerance of 6%.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x80**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **20**.
    *   Inner header TTL is **9**.

### PF-1.3.6: IPv6 traffic GUE encapsulation with explicit TTL configuration on tunnel

*   Modify the flows in `PF-1.3.5` to use IPv6 and repeat the traffic generation
    and validation.

### PF-1.3.7: IPv4 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT with ToS value *0x60*.
*   Re-configure the IPv4 GUE tunnel on the DUT with TTL value of *20*.
*   Generate 12000000 **IPv4 packets** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET ensuring that there are at least 65000 different 5-tuple flows.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of all packets to *10*.

Verify:

*   Policy forwarding rule `matched-packets` counters equals the number of
    packets generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets:
    *   Egressing DUT:Port2, DUT:Port3, DUT:Port4 and DUT:Port5.
    *   Received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5.
*   All packets received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5 are
    GUE encapsulated.
*   ECMP hashing works over the two LAG interfaces with a tolerance of 6%.
*   LAG hashing works over the two Singleton ports within LAG1 and LAG2 with a
    tolerance of 6%.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x60**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **20**.
    *   Inner header TTL is **9**.

### PF-1.3.8: IPv6 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel

*   Modify the flows in `PF-1.3.7` to use IPv6 and repeat the traffic generation
    and validation.

### PF-1.3.9: IPv4 traffic GUE encapsulation to a single 5-tuple tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT with a fixed source and
    destination IPs as well as fixed source and destination UDP ports.
*   Generate 12000000 **IPv4 packets** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET ensuring that there are at least 65000 different 5-tuple flows.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of all packets to *10*.

Verify:

*   Policy forwarding rule `matched-packets` counters equals the number of
    packets generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets:
    *   Egressing DUT:Port2, DUT:Port3, DUT:Port4 and DUT:Port5.
    *   Received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5.
*   All packets received on only a single ATE port, either ATE:Port2, ATE:Port3,
    ATE:Port4 or ATE:Port5 and are GUE encapsulated.
*   All traffic is hashed to a only one LAG and only one Singleton port in the
    LAG.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x60**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **20**.
    *   Inner header TTL is **9**.

### PF-1.3.10: IPv4 traffic GUE encapsulation to a single tunnel

*   Modify the flows in `PF-1.3.9` to use IPv6 and repeat the traffic generation
    and validation.

### PF-1.3.11: IPv4 traffic that should be GUE encapsulated but TTL=1

ATE action:

*   Generate 12000000 **IPv4 packets** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET ensuring that there are at least 65000 different 5-tuple flows.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of all packets to *1*.

Verify:

*   All packets should have TTL decremented to 0, dropped and ICMP Time
    Exceeded (Type 11) / Time to Live exceeded in Transit (Code 0) sent back to
    source address (ATE:Port1).

### PF-1.3.12: IPv6 traffic that should be GUE encapsulated but Hop Limit=1

*   Modify the flows in `PF-1.3.11` to use IPv6 and repeat the traffic
    generation and validation.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # TODO propose new OC paths for GUE encap based on the protocol next hop of a route

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
