# PF-1.3: Policy-based Static GUE Encapsulation to IPv4 tunnel

## Summary

This test verifies Policy Forwarding (PF) action to encapsulate packets in an
IPv4 GUE tunnel. The encapsulation is based on a statically configured GUE
tunnel and static route.

## Testbed Type

*  [`featureprofiles/topologies/ate2_dut5_dum5.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/ate2_dut5_dum5.testbed)

## Procedure

### Test environment setup

*   DUT and DUM have 5 ports each.

    ```
                              -------                  -------
                             |       | ==== LAG1 ==== |       |
          [ ATE:Port1 ] ---- |  DUT  |                |  DUM  | ---- [ ATE:Port2 ]
                             |       | ==== LAG2 ==== |       |
                              -------                  -------
    ```

*   Routes are advertised from ATE:Port2.
*   Traffic is generated from ATE:Port1.
*   ATE:Port2 is used as the destination port for GUE encapsulated traffic.

#### Configuration

1.  DUT:Port1 is configured as Singleton IP interface towards ATE:Port1.

2.  DUT:Port2 and DUT:Port3 are configured as LAG1 IP interface towards
    DUM:Port2 and DUM:Port3 respectively.

3.  DUT:Port4 and DUT:Port5 are configured as LAG2 IP interface towards
    DUM:Port4 and DUM:Port5 respectively.

4.  DUM:Port1 is configured as Singleton IP interface towards ATE:Port2.

5.  DUT is configured to form two eBGP sessions with DUM using the directly
    connected LAG interface IPs.

6.  DUM is configured to form an eBGP sessions with ATE:Port2 using the directly
    connected Singleton interface IP.

7.  ATE:Port2 is configured to advertise destination networks (IPv4-DST-NET,
    IPv6-DST-NET) and tunnel destination (IPv4-DST-GUE) to DUM. When DUM
    advertised these prefixes to the DUT over the two eBGP sessions, the
    protocol next hops for the destination networks should be re-configured as
    below:

    -   Destination network IPv4-DST-NET with protocol next hop PNH-IPv4.
    -   Destination network IPv6-DST-NET with protocol next hop PNH-IPv6.

8.  DUT is configured with an IPv4 GUE tunnel with destination IPv4-DST-GUE
    without any explicit tunnel Type of Service (ToS) or Time to Live (TTL)
    values.

9.  DUT is configured with the following static routes:

    -   To PNH-IPv4, next hop is the statically configured IPv4 GUE tunnel.
    -   To PNH-IPv6, next hop is the statically configured IPv4 GUE tunnel.

### PF-1.3.1: IPv4 traffic GUE encapsulation without explicit ToS/TTL configuration on tunnel

ATE action:

*   Generate **IPv4 traffic** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET using a random combination source addresses at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of the packets to *10*.

Verify:

*   Policy forwarding packet counters matches the packet count of traffic
    generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets egressing DUT ports 2 to 5.
*   All packets egressing DUT ports 2 to 5 are GUE encapsulated.
*   ECMP hashing works (equal traffic) over the two LAG interfaces.
*   LAG hashing works (equal traffic) over the two Singleton ports.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x80**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **9**.
    *   Inner header TTL is **9**.

### PF-1.3.2: IPv6 traffic GUE encapsulation without explicit ToS/TTL configuration on tunnel

ATE action:

*   Generate **IPv6 traffic** from ATE:Port1 to random IP addresses in
    IPv6-DST-NET using a random combination source addresses at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of the packets to *10*.

Verify:

*   Policy forwarding packet counters matches the packet count of traffic
    generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets egressing DUT ports 2 to 5.
*   All packets egressing DUT ports 2 to 5 are GUE encapsulated.
*   ECMP hashing works (equal traffic) over the two LAG interfaces.
*   LAG hashing works (equal traffic) over the two Singleton ports.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x80**.
    *   Inner header Traffic Class is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **9**.
    *   Inner header Hop Limit is **9**.

### PF-1.3.3: IPv4 traffic GUE encapsulation with explicit ToS configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT with ToS value *0x60*.
*   Generate **IPv4 traffic** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET using a random combination source addresses at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of the packets to *10*.

Verify:

*   Policy forwarding packet counters matches the packet count of traffic
    generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets egressing DUT ports 2 to 5.
*   All packets egressing DUT ports 2 to 5 are GUE encapsulated.
*   ECMP hashing works (equal traffic) over the two LAG interfaces.
*   LAG hashing works (equal traffic) over the two Singleton ports.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x60**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **9**.
    *   Inner header TTL is **9**.

### PF-1.3.4: IPv6 traffic GUE encapsulation with explicit ToS configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT with ToS value *0x60*.
*   Generate **IPv6 traffic** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET using a random combination source addresses at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of the packets to *10*.

Verify:

*   Policy forwarding packet counters matches the packet count of traffic
    generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets egressing DUT ports 2 to 5.
*   All packets egressing DUT ports 2 to 5 are GUE encapsulated.
*   ECMP hashing works (equal traffic) over the two LAG interfaces.
*   LAG hashing works (equal traffic) over the two Singleton ports.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x60**.
    *   Inner header Traffic Class is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **9**.
    *   Inner header Hop Limit is **9**.

### PF-1.3.5: IPv4 traffic GUE encapsulation with explicit TTL configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT without an explicit ToS value.
*   Re-configure the IPv4 GUE tunnel on the DUT with TTL value of *20*.
*   Generate **IPv4 traffic** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET using a random combination source addresses at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of the packets to *10*.

Verify:

*   Policy forwarding packet counters matches the packet count of traffic
    generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets egressing DUT ports 2 to 5.
*   All packets egressing DUT ports 2 to 5 are GUE encapsulated.
*   ECMP hashing works (equal traffic) over the two LAG interfaces.
*   LAG hashing works (equal traffic) over the two Singleton ports.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x80**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **20**.
    *   Inner header TTL is **9**.

### PF-1.3.6: IPv6 traffic GUE encapsulation with explicit TTL configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT without an explicit ToS value.
*   Re-configure the IPv4 GUE tunnel on the DUT with TTL value of *20*.
*   Generate **IPv6 traffic** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET using a random combination source addresses at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of the packets to *10*.

Verify:

*   Policy forwarding packet counters matches the packet count of traffic
    generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets egressing DUT ports 2 to 5.
*   All packets egressing DUT ports 2 to 5 are GUE encapsulated.
*   ECMP hashing works (equal traffic) over the two LAG interfaces.
*   LAG hashing works (equal traffic) over the two Singleton ports.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x80**.
    *   Inner header Traffic Class is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **20**.
    *   Inner header Hop Limit is **9**.

### PF-1.3.7: IPv4 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT with ToS value *0x60*.
*   Re-configure the IPv4 GUE tunnel on the DUT with TTL value of *20*.
*   Generate **IPv4 traffic** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET using a random combination source addresses at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of the packets to *10*.

Verify:

*   Policy forwarding packet counters matches the packet count of traffic
    generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets egressing DUT ports 2 to 5.
*   All packets egressing DUT ports 2 to 5 are GUE encapsulated.
*   ECMP hashing works (equal traffic) over the two LAG interfaces.
*   LAG hashing works (equal traffic) over the two Singleton ports.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x60**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **20**.
    *   Inner header TTL is **9**.

### PF-1.3.8: IPv6 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv4 GUE tunnel on the DUT with ToS value *0x60*.
*   Re-configure the IPv4 GUE tunnel on the DUT with TTL value of *20*.
*   Generate **IPv6 traffic** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET using a random combination source addresses at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of the packets to *10*.

Verify:

*   Policy forwarding packet counters matches the packet count of traffic
    generated from ATE:Port1.
*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets egressing DUT ports 2 to 5.
*   All packets egressing DUT ports 2 to 5 are GUE encapsulated.
*   ECMP hashing works (equal traffic) over the two LAG interfaces.
*   LAG hashing works (equal traffic) over the two Singleton ports.
*   ToS for all GUE encapsulated packets:
    *   GUE header ToS is **0x60**.
    *   Inner header Traffic Class is **0x80**.
*   TTL for all GUE encapsulated packets:
    *   GUE header TTL is **20**.
    *   Inner header Hop Limit is **9**.

### PF-1.3.9: IPv4 traffic that should be GUE encapsulation but TTL=1

ATE action:

*   Generate **IPv4 traffic** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET using source address of ATE:Port1 at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of the packets to *1*.

Verify:

*   All packets should have TTL decremented to 0, dropped and ICMP Time
    Exceeded (Type 11) / Time to Live exceeded in Transit (Code 0) sent back to
    source address (ATE:Port1).

### PF-1.3.10: IPv6 traffic that should be GUE encapsulation but Hop Limit=1

ATE action:

*   Generate **IPv6 traffic** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET using source address of ATE:Port1 at linerate.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set Hop Limit of the packets to *1*.

Verify:

*   All packets should have Hop Limit decremented to 0, dropped and
    ICMPv6 Time Exceeded (Type 3) / hop limit exceeded in transit (Code 0) sent
    back to source address (ATE:Port1).

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
