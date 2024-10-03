# RT-1.12: BGP always compare MED 

## Summary

BGP always compare MED 

## Procedure

*   Establish BGP sessions as follows between OTG and DUT.
    *   OTG emulates three eBGP neighbors peering the DUT.
        *   DUT Port1 (AS 65501) ---iBGP 1--- OTG Port1 (AS 65501)
        *   DUT Port2 (AS 65501) ---eBGP 1--- OTG Port2 (AS 65502)
        *   DUT Port3 (AS 65501) ---eBGP 2--- OTG Port3 (AS 65503)
*   Associate eBGP neighbors #1 and #2 with MED values of 100 and 50 on the advertised routes.
*   Enable “always-compare-med” knob on the DUT.
*   Validate traffic flowing to the prefixes received from eBGP neighbor #2 from DUT (OTG Port3).
*   Disable MED settings on DUT and OTG ports. 
*   Validate the change of traffic flow because of the change (OTG Port2).
*   Validate session state and capabilities received on DUT using telemetry.     

## Config Parameter coverage

*   /route-selection-options/config/always-compare-med 
*   /global/afi-safis/afi-safi/route-selection-options/config/always-compare-med 
*   /global/route-selection-options/config/always-compare-med 

## Telemetry Parameter coverage

*   /global/afi-safis/afi-safi/route-selection-options/state/always-compare-med 
*   /global/route-selection-options/state/always-compare-med 

## Protocol/RPC Parameter coverage

N/A

## Minimum DUT platform requirement

N/A
