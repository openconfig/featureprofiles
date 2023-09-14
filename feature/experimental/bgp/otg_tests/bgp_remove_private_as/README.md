# RT-1.11: BGP remove private AS 

## Summary

BGP remove private AS

## Procedure

*   Establish BGP sessions as follows between OTG and DUT.
    *   OTG emulates two eBGP neighbors peering with the DUT using public AS numbers.
        *   DUT Port1 (AS 500) ---eBGP--- OTG Port1 (AS 100)
        *   DUT Port2 (AS 500) ---eBGP--- OTG Port2 (AS 200)
    *   Inject routes with AS_PATH modified to have private AS number 65501 from eBGP neighbor #1 
        (OTG Port1).
    *   Validate received routes on OTG Port2 should have AS Path "500 100 65501".
    *   Configure "remove private AS" with type PRIVATE_AS_REMOVE_ALL  on DUT.    
    *   Validate that private AS numbers are stripped before advertisement to the eBGP peer OTG 
        Port2.
    *   AS path for received routes on OTG Port2 should be "500 100".   
    *   Tested AS-Path Patterns:
        *   PRIV_AS1
        *   PRIV_AS1 PRIV_AS2
        *   AS1 PRIV_AS1 PRIV_AS2
        *   PRIV_AS1 AS1
        *   AS1 PRIV_AS1 AS2

## Config Parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/remove-private-as

## Telemetry Parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as4-path/as4-segment/state

## Protocol/RPC Parameter coverage

N/A

## Minimum DUT platform requirement

N/A