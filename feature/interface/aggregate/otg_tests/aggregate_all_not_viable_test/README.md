# RT-5.7 Aggregate Not Viable All
[TODO: test automation/coding; issue https://github.com/openconfig/featureprofiles/issues/1655]
## Summary

Test forwarding-viable with LAG and routing

Ensure that when **all LAG member** become set with forwarding-viable == FALSE.
- forwarding-viable=false impact **only** transmit traffic on all member port.
- All member ports set with forwarding-viable=false can receive all type of
  traffic and forward it normally (same as with forwarding-viable=true)
- ISIS adjacency is established on LAG port w/ all member set to
  forwarding-viable == FALSE
- Traffic that normally egress LAG with all members set to forwarding-viable ==
  FALSE is forwarded by the next best egress interface/LAG.

## Procedure

```
                                               LAG_2
+------------+            +-------------+ p2     .    p2   +--------------+
|  ATE       |            |             +-------;-:--------+   ATE        |
|            |            |             + - - - + + - - - -|              |
|            |            |             + - - - + + - - - -|     .-------.|
|.-------.   |            |             +-------+-+--------+    (  pfx2   )
(  pfx1   )  |     .      |             | p7    : ;   p7   |     `-------'|
|`-------'   | p1 ; : p1  |     DUT     |        '         |     .-------.|
|            |----+-+-----|             |                  |    (  pfx3   )
|            |    | |     |             | p8     .    p8   |     `-------'|
|            |    | |     |             +-------;-:--------+     .-------.|
|            |    : ;     |             |       | |        |    (  pfx4   )
|            |     '      |             |       | |        |     `-------'|
|            |  LAG_1     |             +-------+-+--------+              |
+------------+            +-------------+ p9    : ;   p9   +--------------+
                                                 '
                                                LAG_3

```

- Connect ATE port-1 to DUT port-1, and ATE ports 2 through 7 to DUT ports 2-7,
  and ATE ports 8, 9 to DUT ports 8, 9
- Configure ATE and DUT ports 1 to be LAG_1 w/ LACP running.
- Configure ATE and DUT ports 2-7 to be LAG_2 w/ LACP running.
- Configure ATE and DUT ports 8-9 to be LAG_3 w/ LACP running.
- Establish ISIS adjacencies on LAG_1, LAG_2, LAG_3.
  1. Advertise one network prefix (pfx1) from ATE LAG_1
  1. Advertise one network prefix (pfx2) from ATE LAG_2 and ATE LAG_3.
- Establish iBGP between ATE and DUT over LGA_1 using LAG_1 interface IPs and advertise prefix pfx3 with BGP NH from pfx2 range.
- Programm via gRIBI route for prefix pfx4 with single NHG pointing LAG_2 (al
  ports are forwarding-viable at this point).
  
- For ISIS cost of LAG_2 lower then ISIS cost of LAG_3:
  - Run traffic:
    - From prefix pfx1 to all three: pfx2, pfx3, pfx4
    - From prefix pfx2 to: pfx1
  - Make the forwarding-viable transitions from TRUE --> FALSE on ports 3-7
    within the LAG_2 on the DUT
    - ensure that only DUT port 2 of LAG ports has bidirectional traffic.
    - Ensure there is no traffic transmitted out of DUT ports 3-7
    - ensure that traffic is received on all port2-7 and delivered to ATE port1
    - ensure there are no packet losses in steady state (no congestion).
    - Ensure there is no traffic received on DUT LAG_3
    - Ensure there is no traffic transmitted on DUT LAG_3
  - Disable/deactive laser on ATE port2; All LAG_2 members are either down (port2) or
    set with forwarding-viable=FALSE
    - Ensure ISIS adjacency is UP on DUT LAG_2 and ATE LAG_2
    - Ensure there is no traffic transmitted out of  DUT ports 2-7 (LAG_2)
    - ensure that traffic is received on all port3-7 and delivered to ATE LAG_1
    - ensure there are no packet losses in steady state (no congestion) for
      traffic from ATE LAG_2 to ATE LAG_1 (pfx_1).
    - ensure there are no packet losses in steady state (no congestion) for
      traffic from ATE LAG_1 to ATE LAG_3 (pfx_2, pfx3).
    - Ensure there is no traffic received on DUT LAG_3
    - Ensure that traffic from ATE port1 to pfx2, pfx3 are transmitted via DUT
      LAG3
    - Ensure that traffic from ATE port1 to pfx4 are discarded on DUT
  - Make the forwarding-viable transitions from FALSE --> TRUE on a ports 7
    within the LAG_2 on the DUT
    - ensure that only DUT port 7 of LAG ports has bidirectional traffic.
    - Ensure there is no traffic transmitted out of  DUT ports 2-6
    - ensure that traffic is received on all port3-7 and delivered to ATE port1
    - ensure there are no packet losses in steady state (no congestion).
    - Ensure there is no traffic received on DUT LAG_3
    - Ensure there is no traffic transmitted on DUT LAG_3
  - Enable/activate laser on ATE port2; Make the forwarding-viable transitions
    from FALSE --> TRUE on a ports 3-7
    
- For ISIS cost of LAG_2 equall to ISIS cost of LAG_3
  - Run traffic:
    - From prefix pfx1 to all three: pfx2, pfx3, pfx4
    - From prefix pfx2 to: pfx1
  - Make the forwarding-viable transitions from TRUE --> FALSE on ports 3-7
    within the LAG_2 on the DUT
    - ensure that only DUT port 2 of LAG_2 and all ports of LAG_3 ports has bidirectional
    traffic. The traffic split between LAG_2 and LAG_3 should be 50:50.
    - Ensure there is no traffic transmitted out of DUT ports 3-7
    - ensure that traffic is received on all port2-7 and ports8-9 and delivered to ATE port1
    - ensure there are no packet losses in steady state (no congestion).
  - Disable/deactive laser on ATE port2; All LAG_2 members are either down (port2) or
    set with forwarding-viable=FALSE.
    - Ensure ISIS adjacency is UP on DUT LAG_2 and ATE LAG_2
    - Ensure there is no traffic transmitted out of  DUT ports 2-7 (LAG_2)
    - ensure that traffic received on all port3-7 and ports8-9 is delivered to ATE LAG_1
    - ensure there are no packet losses in steady state (no congestion) for
      traffic from ATE LAG_2, LAG_3 to ATE LAG_1 (pfx_1).
    - ensure there are no packet losses in steady state (no congestion) for
      traffic from ATE LAG_1 to ATE LAG_3 (pfx_2, pfx3).
    - Ensure that traffic from ATE port1 to pfx2, pfx3 are transmitted via DUT
      LAG3
    - Ensure that traffic from ATE port1 to pfx4 are discarded on DUT
  - Make the forwarding-viable transitions from FALSE --> TRUE on a ports 7
    within the LAG_2 on the DUT
    - ensure that only DUT port 7 of LAG_2 and all ports of LAG_3 ports has bidirectional traffic.
    - Ensure there is no traffic transmitted out of  DUT ports 2-6
    - ensure that traffic received on all port3-7 and ports8-9 is delivered to ATE port1
    - ensure there are no packet losses in steady state (no congestion).
  - Enable/activate laser on ATE port2; Make the forwarding-viable transitions
    from FALSE --> TRUE on a ports 3-6 

### Deviation option

It is foreseen that implementation may drop ISIS adjacency if all members of LAG
are set with forwarding-viable = FALSE. This scenario may be
handled via the yet to be defined deviation `logicalInterfaceUPonNonViableAll`.

## Config Parameter coverage

- /interfaces/interface/ethernet/config/aggregate-id
- /interfaces/interface/ethernet/config/forwarding-viable
- /interfaces/interface/aggregation/config/lag-type
- /lacp/config/system-priority
- /lacp/interfaces/interface/config/name
- /lacp/interfaces/interface/config/interval
- /lacp/interfaces/interface/config/lacp-mode
- /lacp/interfaces/interface/config/system-id-mac
- /lacp/interfaces/interface/config/system-priority

## Telemetry Parameter coverage

None

## Protocol/RPC Parameter coverage

None

## Minimum DUT platform requirement

vRX


