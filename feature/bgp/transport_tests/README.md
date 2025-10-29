# RT-1.9: BGP Transport Parameters test

## Summary
 - Validate the ability to configure a different TCP port than the default 179 for a remote peer. This should be tested for a EBGP as well as IBGP neighborship
 - Ensure that state information on configured transport parameters can be accurately derived.

## Topology
ATE (Port1) <-IBGP-> (Port1) DUT (Port2) <-EBGP-> (Port2) ATE
  - Connect ATE Port1 to DUT port1
  - Connect ATE Port2 to DUT port2

## Procedure
  - Sub test 1
    - Establish IS-IS adjacency between ATE Port1 and DUT Port1. 
    - Establish BGP sessions as follows between ATE and DUT . 
      - ATE port 1 is used for IBGP connection between the Loopback address of the DUT and the IS-IS learnt address behind ATE:Port1. Please ensure that the ATE has BGP listening on a different TCP port than 179 (example: 1800) AND BGP session on ATE is configured as *passive*
      - The DUT Port2 IP has eBGP peering with ATE port 2 IP and is receiving IPv4/6 routes. Here too for EBGP, ensure that ATE is listening on a different TCP port than default 179 (example: 1800).
    - Validate session and transport states on ATE and DUT ports using telemetry.
      - As part of the validation, we should ensure that the values for the leaves "neighbors/neighbor/state/neighbor-port" and "neighbors/neighbor/transport/state/remote-port" are as configured above and none are "179"
      - Ensure that the remote address derived from the leaves, "neighbors/neighbor/state/neighbor-address" and "neighbor/transport/state/remote-address" are as configured for the neighbors.
    - Validate session state and capabilities received on DUT using telemetry.
    - Validate the BGP route/path and corresponding attributes for v4 and v6 prefixes
      - NH
      - Local Pref
      - Metric
      - Communities
  - Sub test 2 [Negative test case-1 for BGP peer flapping]
    - Initiate EBGP and IBGP session reset at the DUT end few times and ensure following are collected accurately each time on the DUT. Session reset can be initiated by toggling neighbors/neighbor/config/enabled
      - last-notification-time, last-notification-error-code, last-notification-error-subcode in the `neighbour/state/messages/sent` container
      - neighbors/neighbor/transport/state/remote-port, neighbors/neighbor/transport/state/remote-address, neighbors/neighbor/state/neighbor-address, neighbors/neighbor/state/neighbor-port
    - Initiate EBGP and IBGP session reset at both the ATE ends few times and ensure following are collected accurately each time on the DUT. Session reset can be initiated by toggling neighbors/neighbor/config/enabled
      - last-notification-time, last-notification-error-code, last-notification-error-subcode in the received container
      - neighbors/neighbor/transport/state/remote-port, neighbors/neighbor/transport/state/remote-address, neighbors/neighbor/state/neighbor-address, neighbors/neighbor/state/neighbor-port
     
  - Sub test 3 [Negative test case-2 for non-matching BGP TCP port configuration]
    - Configure DUT with neighbor-port (say 1801) that mismatches ATE listen port (say 1800). Configure ATE in "Passive" mode. Conduct this test for both IBGP as well as EBGP peering. Verify that,
      - BGP is not established, which is a test pass
      - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-port returns 1801
      - Whereas, /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/remote-port returns null.

## Config Parameter Coverage
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-port
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled

## Telemetry Parameter Coverage
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-port
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/remote-port
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/remote-address
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-type
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/sent/last-notification-time
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/sent/last-notification-error-code
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/sent/last-notification-error-subcode
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/last-notification-time
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/last-notification-error-code
  - /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/last-notification-error-subcode

