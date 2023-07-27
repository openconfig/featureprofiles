# RT-1.21: BGP TCP MSS and PMTUD 

## Summary

BGP TCP MSS and PMTUD

## Topology

"Virtual host on ATE" <--> ATE (port1) <--> DUT

## Procedure

*   Establish BGP sessions between:
    *   ATE (port-1) ---  eBGP-IPv4/IPv6 ---- DUT 
    *   Virtual host on ATE ---- iBGP IPv4 ---- DUT.
*   TODO : Verify that the default TCP MSS value is set below interface MTU value.
*   Change the Interface MTU on the DUT port as well as ATE(port1) to 5040B
*   Configure IP TCP MSS value of 4096 bytes on the DUT interface to the ATE port1.
*   Re-establish the BGP sessions by tcp reset.
*   Verify that the TCP MSS value is set to 4096 bytes for IPv4 and IPv6.
*   Establish iBGP session with MD5 enabled between the "virtual host on ATE" and the DUT.  
*   Ensure that the MTU on the ATE port towards the virtual host is set at defualt while the virtual-host interface towards ATE is set at 5040B. 
*   Enable PMTUD on the DUT. 
*   TODO : Validate that the min MSS value has been adjusted to be below 1500 bytes on the tcp session.

## Config Parameter coverage

*   /neighbors/neighbor/transport/config/tcp-mss 
*   /neighbors/neighbor/transport/config/mtu-discovery 

## Telemetry Parameter coverage

*   /neighbors/neighbor/transport/state/tcp-mss 
*   /neighbors/neighbor/transport/state/mtu-discovery 

## Protocol/RPC Parameter coverage

N/A

## Minimum DUT platform requirement

N/A
