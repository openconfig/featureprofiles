# Aggregate Not Viable All

## Summary

Ensure that when **all LAG member** become set with forwarding-viable == FALSE:

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
|  ATE/OTG   |            |             +-------;-:--------+   ATE/OTG    |
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

- Connect ATE/OTG port-1 to DUT port-1, and ATE/OTG ports 2 through 7 to DUT ports 2-7,
  and ATE/OTG ports 8, 9 to DUT ports 8, 9
- Configure ATE/OTG and DUT ports 1 to be LAG_1 w/ LACP running.
- Configure ATE/OTG and DUT ports 2-7 to be LAG_2 w/ LACP running.
- Configure ATE/OTG and DUT ports 8-9 to be LAG_3 w/ LACP running.
- Establish ISIS adjacencies on LAG_1, LAG_2, LAG_3.
  1. Advertise one network prefix (pfx1) from ATE/OTG LAG_1
  1. Advertise one network prefix (pfx2) from ATE/OTG LAG_2 and ATE/OTG LAG_3.
- Establish iBGP session between ATE/OTG LAG_1 and advertise prefix pfx3 with BGP NH
  attribute of address form pfx1 range.
- Programm via gRIBI route for prefix pfx4 with single NHG pointing LAG_2 (al
  portsl are forwarding-viable at this point).
- For both (ISIS cost of LAG_2 equal to ISIS cost of LAG_3) AND (ISIS cost of
  LAG_2 lower than ISIS cost of LAG_3):
  - Run traffic:
    - From prefix pfx1 to all three: pfx2, pfx3, pfx4
    - From prefix pfx2 to all three: pfx1
  - Make the forwarding-viable transitions from TRUE --> FALSE on a ports 3-7
    within the LAG_2 on the DUT
    - ensure that only DUT port 2 of LAG ports has bidirectional traffic.
    - Ensure there is no traffic transmitted form DUT ports 3-7
    - ensure that traffic is received on all port2-7 and delivered to ATE/OTG port1
    - ensure there are no packet losses in steady stATE/OTG (no congestion).
    - Ensure there is no traffic received on DUT LAG_3
    - Ensure there is no traffic transmitted on DUT LAG_3
  - Disable/deactivATE/OTG laser on ATE/OTG port2; All LAG members are either down or
    set with forwarding-viable=FALSE
    - Ensure ISIS adjacency is UP on DUT LAG_2 and ATE/OTG LAG_2
    - Ensure there is no traffic transmitted form DUT ports 2-7
    - ensure that traffic is received on all port2-7 and delivered to ATE/OTG LAG_1
    - ensure there are no packet losses in steady stATE/OTG (no congestion) for
      traffic to ATE/OTG LAG_1.
    - ensure there are no packet losses in steady stATE/OTG (no congestion) for
      traffic from ATE/OTG LAG_1 to pfx_2, pfx3.
    - Ensure there is no traffic received on DUT LAG_3
    - Ensure that traffic from ATE/OTG port1 to pfx2, pfx3 are transmitted via DUT
      LAG3
    - Ensure that traffic from ATE/OTG port1 to pfx4 are discarded on DUT
  - Make the forwarding-viable transitions from FALSE --> TRUE on a ports 7
    within the LAG on the DUT
    - ensure that only DUT port 7 of LAG ports has bidirectional traffic.
    - Ensure there is no traffic transmitted form DUT ports 2-6
    - ensure that traffic is received on all port3-7 and delivered to ATE/OTG port1
    - ensure there are no packet losses in steady stATE/OTG (no congestion).
    - Ensure there is no traffic received on DUT LAG_3
    - Ensure there is no traffic transmitted on DUT LAG_3
  - Enable/activATE/OTG laser on ATE/OTG port2; Make the forwarding-viable transitions
    from FALSE --> TRUEon a ports 3-7

### Deviation option

It is foreseen that implementation may drop ISIS adjacency after some members of
primary egress LAG are not viable while all others down. This scenario may be
handled via the yet to be defined deviation `logicalInterfaceUPonNonViableAll`.

## Config Parameter coverage

- /interfaces/interface/ethernet/config/aggregate-id
- /interfaces/interface/ethernet/config/forwarding-viable (from hercules) [ph1]
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


