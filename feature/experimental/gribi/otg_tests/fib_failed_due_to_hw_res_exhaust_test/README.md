# TE-9.1: FIB FAILURE DUE TO HARDWARE RESOURCE EXHAUST

## Summary

Validate gRIBI FIB_FAILED functionality.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Establish a gRIBI connection (SINGLE_PRIMARY and PRESERVE mode) to the DUT.

*   Establish BGP session between ATE Port1 --- DUT Port1. Inject unique BGP routes to exhaust FIB on DUT.

*   Continuously injecting the following gRIB structure until FIB FAILED is received. 
    Each DstIP and VIP should be unique and of /32. All the NHG and NH should be unique (of unique ID).
    DstIP/32 -> NHG -> NH {next-hop:} -> VIP/32 -> NHG -> NH {next-hop: AtePort2Ip}
    
*   Expect FIB_PROGRAMMED message until the first FIB_FAILED message received.

*   Validate that traffic for the FIB_FAILED route will not get forwarded. 

*   Pick any route that received FIB_PROGRAMMED. Validate that traffic hitting the route should be forwarded to port2 


## Protocol/RPC Parameter coverage

*   gRIBI
    *   Flush

## Config parameter coverage

## Telemery parameter coverage
