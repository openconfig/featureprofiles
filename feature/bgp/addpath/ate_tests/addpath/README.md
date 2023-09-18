## Summary
This test checks for DUT functionality of BGP addpath capability. It also confirms accuracy of addpath capabilities negotiation, following checks
1. DUT sets value for the Send/Receive field as "3" during capabilities negotiation to signal support for both Send and Receive ability for multiple paths.
2. DUT sets value for the Send/Receive field as "1" during capabilities negotiation to signal support for Receive ability for multiple paths.
3. DUT sets value for the Send/Receive field as "2" during capabilities negotiation to signal support for Send ability for multiple paths.
4. Tests are conducted at scale of 1M v4 and 600k v6 routes with 64 different NHs each. Also, routes are withdrawn at scale to ensure addpath functionality is maintained at scale.

## Testbed type

* TESTBED_DUT_ATE_9LINKS

## Topology:


ATE1:Port1 <-EBGP-> DUT:Port1

ATE2:port2 <-IBGP-> DUT:port2

ATE3:port3 <-IBGP-> DUT:port3

ATE4:port4 <-IBGP-> DUT:port4


### Test procedure
All tests below are expected to be run at the global, peer-group and neighbor levels.  


**Test-1**: Verify ADDPATH Send/Receive capability of "3"  
* DUT:Port1 has EBGP peering with ATE1:Port1.
* DUT:port2 and DUT:port3 has IBGP peering with directly connected ATE2:port2 and ATE3:port3 respectively. In this case, DUT is the RR server and ATE2 and ATE3 are RR clients
* DUT:port4 has IBGP peering with directly connected ATE4:port4. In this case DUT is the RR client and ATE4:port4 is the RR server
* Ensure that the EBGP and IBGP peering are enabled for both address famailies.
* DUT should be configured with "Both" Send and Receive ability for addpath on all IBGP and EBGP peering. Same for ATE1:Port1, ATE2:port2, ATE3:port3 and ATE4:port4
  * During BGP capabilities negotiation in Open message, verify that the DUT negotiated addpath cability with Send/Receive field set to "3"

* Configure ATE1:Port1 to advertise same prefix "prefix-1" with different Protocol next-hops to DUT:Port1
  * Verify that the DUT is advertising multiple paths to prefix-1 to RRCs ATE2 and ATE3 as well as to the RRS ATE4 with different path-ids
* Configure ATE1:Port1 to advertise 4 different paths for prefix-2 with different Protocol next-hops to DUT:Port1. Then configure DUT to send maximum 3 paths over ADD-Path to ATE2 and ATE3
  * Verify that ATE2 and ATE3 receive only 3 different paths (out of 4) w/ unique path-ids from DUT for prefix-2.
* Configure RRCs ATE2:port2 and ATE3:port3 to advertise "prefix-3" to DUT with different Protocol next-hops.
  * Verify that the DUT advertises multiple paths for prefix-3 to ATE4 with different path-ids
* Configure RRCs ATE2:port2 and ATE3:port3 each to advertise 2 different paths for "prefix-4" to DUT with different Protocol next-hops each. Then configure DUT to send maximum 3 paths over ADD-Path to ATE4
  * Verify that ATE4 receives only 3 different paths (out of 4) w/ unique path-ids from DUT for prefix-4.
 
**Test-2**: Verify ADDPATH Send/Receive capability of "1" and Send/Receive capability of "2"
* DUT:Port1 has EBGP peering with ATE1:Port1.
* DUT:port2 and DUT:port3 has IBGP peering with directly connected ATE2:port2 and ATE3:port3 respectively. In this case, DUT is the RR server and ATE2 and ATE3 are RR clients
* DUT:port4 has IBGP peering with directly connected ATE4:port4. In this case DUT is the RR client and ATE4:port4 is the RR server
* Ensure that the EBGP and IBGP peering are enabled for both address famailies.
* DUT should be configured with Send/Receive field set to "1" on the EBGP peering and Send/Receive field set to "2" on all the IBGP peering. ATE1:port1, ATE2:port2, ATE3:port3 and ATE4:port4 can continue to funtion w/ Send/Receive ability of "Both"
  * During BGP capabilities negotiation in Open message, verify that the DUT negotiated addpath w/ the right Send/Receive field on each of the peering.
* Configure ATE1:Port1 to advertise "prefix-1" with different Protocol next-hops to DUT:Port1
  * Verify that the DUT is advertising multiple paths to prefix-1 to RRCs ATE2 and ATE3 as well as to the RRS ATE4 with different path-ids
* Configure ATE1:Port1 to advertise 4 different paths for prefix-2 with different Protocol next-hops to DUT:Port1. Then configure DUT to send maximum 3 paths over ADD-Path to ATE2, ATE3 and ATE4
  * Verify that ATE2, ATE3 and ATE4 each receive only 3 different paths (out of 4) w/ unique path-ids from DUT for prefix-2.
  

**Test-3**: Verify ADDPATH scaling with Multipath on EBGP peering
* DUT:Port1 and DUT:Port2 has EBGP peering with ATE1:Port1 and ATE:Port2 respectively
  * ATE1:Port1 belongs to AS100 and ATE2:Port2 belongs to AS200. DUT is in AS300
* DUT:port3 and DUT:port4 has IBGP peering with directly connected ATE3:port3 and ATE4:port4 peers respectively. In this case, DUT is the RR server and ATE3 and ATE4 are RR clients
* Ensure that the EBGP and IBGP peering are enabled for both address famailies.
* Enable multipath on the DUT for both EBGP and IBGP learnt paths
  * Validate that multipath is enabled on each peering
* DUT is configued with addpath Send capability only on its IBGP peering with ATE3:port3 and ATE4:port4
  * During BGP capabilities negotiation in Open message, verify that the DUT negotiated addpath cability with Send/Receive field set to "2" signaling Send capability.
* ATE3 and ATE4 should be configured for addpath "Receive" capability.
* Configiure ATE1:Port1 and ATE2:Port2 each with 1M v4 prefixes with 32 different NHs for each prefix. Follow the same process for IPv6 with 600k prefixes and 32 distinct NHs for each prefix from each ATE. Given that ADD-Path is not enabled in this peering, The ATEs (ATE1 and ATE2) will advertise only one path per v4 and v6 prefix **Hence the DUT will recieve in total 1M v4 and 600k IPv6 routes from each, ATE1 and ATE2. Make sure that the boolean leaf "allow-multiple-as" is enabled on the DUT. The DUT should be able to program all these v4 and v6 routes to the FIB**.
  * Validate that allow-multiple-as is configured on the EBGP peering
* Configure the DUT to advertise all these **ECMP routes in the LOC-RIB** over the IBGP peering to the RRCs ATE3 and ATE4.
  * Valiidate receipt of these routes at ATE3 and ATE4
*  Withdraw advertisement of 500k v4 and 300k v6 prefixes from ATE2 and verify the following for the withdrawn prefixes
  * For the withdrawn prefixes there are only single routes in the FIB pointing at ATE1.
  * ATE3 and ATE4 now has only one path for the withdrawn prefixes.
    
 
## Config Parameter coverage
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/receive
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/send-max
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/config/eligible-prefix-policy
* /network-instances/network-instance/protocols/protocol/bgp/global/neighbor/afi-safis/afi-safi/use-multiple-paths/config/enabled
* /network-instances/network-instance/protocols/protocol/bgp/global/neighbor/afi-safis/afi-safi/use-multiple-paths/ebgp
* /network-instances/network-instance/protocols/protocol/bgp/global/neighbor/afi-safis/afi-safi/use-multiple-paths/ebgp/config/allow-multiple-as
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/config/maximum-paths
  
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/receive
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/send-max
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/config/eligible-prefix-policy
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/config/enabled
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/config/allow-multiple-as
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/config/maximum-paths
  
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/receive
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/send
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/send-max
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/config/eligible-prefix-policy
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/use-multiple-paths/config/enabled
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/use-multiple-paths/ebgp
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/use-multiple-paths/ebgp/config/allow-multiple-as

## Telemetry Parameter coverage
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/receive
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/send-max
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/add-paths/state/eligible-prefix-policy
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/state/
* work-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/state/enabled
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/state/allow-multiple-as
* /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/ebgp/state/maximum-paths
  
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/receive
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/send-max
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/add-paths/state/eligible-prefix-policy
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/state/enabled
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/state/allow-multiple-as
* /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/use-multiple-paths/ebgp/state/maximum-paths

* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/receive
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/send-max
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/add-paths/state/eligible-prefix-policy
  
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/use-multiple-paths/state/enabled
* /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/use-multiple-paths/ebgp/state/allow-multiple-as



