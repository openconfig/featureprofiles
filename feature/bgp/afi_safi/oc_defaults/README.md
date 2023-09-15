Summary:
  - When operating in "openconfig mode", NOS (network operating system) defaults should match what OC defines as the defaults i.e,
    - For BGP, there are no defaults for AFI-SAFI at the neighbor and peer-group levels. However at the global level the default is "false"
  - This test currently only verifies the defaults for ipv4-unicast and ipv6-unicast families. However, this test can be extended further to cover for other
    AFI-SAFIs as well in future.
  - The test will check for default implementations under the neighbor and peer-group hierarchies and also test for inheritance rules as was specified in [pull/774](https://github.com/openconfig/public/pull/774) and [pull/815](https://github.com/openconfig/public/pull/815).


Topology:
ATE (Port1) <-EBGP-> (Port1) DUT (Port2) <-IBGP-> (Port2) ATE
  - Connect ATE Port1 to DUT port1 (EBGP peering)
  - Connect ATE Port2 to DUT port2 (IBGP peering)

Procedure:
  - [Test case-1] AFI-SAFI configurations at "neighbor level":
    - Push EBGP and IBGP OC configuration to the DUT 
      - Configuration should include corresponding IPv4 and IPv6 neighbor configurations.
      - Ensure that only IPv4-Unicast enabled boolean is made "true" for IPv4 neighbor. "IPv6-unicast enabled" boolean is left to OC default for the IPv4 peer".
      - Ensure that only IPv6-Unicast enabled boolean is made "true" for IPv6 neighbor. "IPv4-unicast enabled" boolean is left to OC default for the IPv6 peer".
      - Ensure that there are no AFI-SAFI configurations at the global and peer-group levels. 
      - On the ATE side ensure that IPv4-unicast and IPv6-unicast AFI-SAFI are enabled==true for IPv4 and IPv6 neighbors.
  - verification:
      - For IPv4 neighbor, ensure that the IPv4 neighborship is up and IPv6-unicast capability is not negotiated.
      - For IPv6 neighbor ensure that the IPv6 neighborship is up and IPv4-unicast capability is not negotiated.    
  - [Test case-2] IPv4-unicast and IPv6-Unicast AFI-SAFIs enabled at peer-group level:
    - Configuration at the neighbor level is same as in [Test case-1] except for IPv4-unicast and IPv6-unicast being enabled at the peer-group level
    - No configuration should be made at the global AFI-SAFI level
    - verification:
      - For IPv4 neighbor, ensure that the IPv4 neighborship is up and both IPv4-unicast and IPv6-unicast capabilities are negotiated.
      - For IPv6 neighbor ensure that the IPv6 neighborship is up and both IPv4-unicast and IPv6-unicast capabilities are negotiated.
  - [Test case-4] IPv4-unicast and IPv6-Unicast AFI-SAFIs enabled at Global level:
    - Configuration at the neighbor level is same as in [Test case-1] except for IPv4-unicast and IPv6-unicast being enabled at the global level
    - No configuration should be made at the peer-group AFI-SAFI level
    - verification:
      - For IPv4 neighbor, ensure that the IPv4 neighborship is up and both IPv4-unicast and IPv6-unicast capabilities are negotiated.
      - For IPv6 neighbor ensure that the IPv6 neighborship is up and both IPv4-unicast and IPv6-unicast capabilities are negotiated.

Config Parameter coverage:

  /network-instances/network-instance/protocols/protocol/bgp/global/config/as
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/auth-password
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/neighbor-address
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled


  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/auth-password
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/neighbor-address
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group/peer-group-name
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enabled

  /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled

Telemetry Parameter coverage:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-type
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-type
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-as
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/local-as
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-group

