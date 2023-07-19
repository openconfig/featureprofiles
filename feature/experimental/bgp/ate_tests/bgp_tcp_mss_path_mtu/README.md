# RT-1.21: BGP TCP MSS and PMTUD 

## Summary

BGP TCP MSS and PMTUD

## Procedure

*   Establish BGP sessions between:
    *   ATE port-1 ---  eBGP-IPv4/IPv6 ---- DUT1 
    *   ATE port-1 ---- iBGP IPv4      ---- DUT2.
*   DUT-2 is directly connected to DUT-1.  
*   TODO : Verify that the default TCP MSS value is set below interface MTU value.
*   Change the Interface MTU to the ATE port as 5040.
*   Configure IP TCP MSS value of 4096 bytes on the interface to the ATE port.
*   Re-establish the BGP sessions by tcp reset.
*   Verify that the TCP MSS value is set to 4096 bytes for IPv4 and IPv6.
*   Establish iBGP session with MD5 enabled from ATE port-1 to DUT-2. 
*   Change the MTU on the DUT-1 to DUT-2 link to 512 bytes and enable PMTUD on the DUT. 
*   TODO : Validate that the min MSS value has been adjusted to be below 512 bytes on the tcp session.

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
