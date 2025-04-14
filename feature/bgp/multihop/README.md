# RT-1.63: BGP Multihop

## Summary

This test case validates the multihop eBGP feature, where two BGP speakers establish a peering session even when they are not directly connected.

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure 

### Configuration

1)  Create the topology below:

```mermaid
graph LR; 
A[ATE:Port1] -- EBGP --> B[Port1:DUT:Port2];
B ----> C[Port2:ATE] --> eBGP peer;
```

2)  Configure the eBGP peering session: Establish an eBGP peering relationship between ATE1 and Port 1 on the DUT.
3)  Assign IP addresses: Configure IPv4 and IPv6 addresses on all relevant interfaces on the DUT, ATE1, and ATE2.
4)  Create a loopback interface: Configure a loopback interface on the DUT and assign it an IP address. This will serve as the DUT's identifier for the BGP session.
5)  Establish static routing on the DUT: Configure a static route on the DUT that directs traffic destined for the eBGP peer's IP address (connected to ATE2) through ATE2. This ensures the DUT can reach the peer.
6)  Establish static routing on ATE2: Configure a static route on ATE2 that directs traffic destined for the DUT's loopback IP address towards the DUT. This allows the eBGP peer on ATE1 to reach and identify the DUT via its loopback address.

### Tests

### RT-1.63.1: Establish eBGP session over multihop

1)  Configure the eBGP peering session: Establish an eBGP peering relationship between the DUT's loopback interface and the eBGP peer connected to ATE2. This means the DUT will use its loopback address as its BGP identifier.
2)  Specify the neighbor address: Configure the DUT to use the eBGP peer's IP address (connected to ATE2) as the neighbor address for this BGP session.
3)  Enable multihop: Since the eBGP peer is not directly connected to the DUT, enable the multihop option for this BGP neighbor on the DUT. Set the Time-to-Live (TTL) value to 2 to allow the BGP packets to traverse the intermediate link between the DUT and ATE2.
4)  Validate session establishment: Confirm that the eBGP session is successfully established between the DUT's loopback interface and the eBGP peer connected to ATE2. This can be verified by checking the BGP neighbor status, following this path
    *  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state
5)  Verify BGP updates: Ensure that the DUT and the eBGP peer are exchanging BGP updates correctly, indicating that the peering is functional and routing information is being shared.
    * Check the counters in the followong paths are incrementing:
      * /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/sent/UPDATE
      * /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/received/UPDATE

### RT-1.63.2: Advertise prefixes over multihop eBGP

1)  Advertise prefixes: Configure the eBGP peer to advertise the following IPv4 and IPv6 prefixes to the DUT:
    *   BGP-V4 = 203.0.200.0/24
    *   BGP-V6 = 2001:db8:128:200::/64
2)  Verify prefix reception: Confirm that the DUT successfully receives the advertised prefixes from the eBGP peer.
3)  Check routing table: Inspect the DUT's routing table to ensure that the received prefixes have been installed correctly.
4)  Validate next hop: Verify that the next hop associated with the installed prefixes in the DUT's routing table is the IP address of ATE Port 2. This confirms that the DUT knows to forward traffic for these prefixes towards ATE2, following paths:
  * /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address
  * /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix

### RT-1.63.3: Traffic forwarding over multihop eBGP

1)  Generate traffic: Initiate traffic from ATE port-1 towards the DUT. This traffic should be destined for the prefixes that were advertised by the eBGP peer connected to ATE2.
    * Send 1000 packets on 10 flows per address family
2)  Verify forwarding: Confirm that the DUT correctly forwards the traffic to the eBGP peer via ATE port-2. This demonstrates that the multihop eBGP session is functioning as expected and that the DUT is using the learned routes to direct traffic appropriately.
3)  Monitor traffic counters: Examine the traffic counters on the relevant DUT and ATE interfaces to verify that traffic is flowing as expected and that there are no drops or errors.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/ebgp-multihop/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/ebgp-multihop/config/multihop-ttl:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/ebgp-multihop/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/ebgp-multihop/config/multihop-ttl:

  ## state paths
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/ebgp-multihop/state/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/ebgp-multihop/state/multihop-ttl:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/ebgp-multihop/state/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/ebgp-multihop/state/multihop-ttl:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Minimum DUT platform requirement

*   FFF - Fixed Form Factor