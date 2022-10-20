# FACTORY RESET 

## Summary
Performs Factory Reset with and without disk-encryption 

## Procedure
*   Create dummy files in the harddisk of the router using bash dd
*   Checks for disk-encryption status and performs reset on both the scenarios
*   Send out Factory reset via GNOI Raw API 
    *  Wait for the box to boot up via Secure ZTP  
        *   The base config is updated on the box via Secure ZTP  
*   Connect to the router and check if the files in the harddisk are removed as a part of verifying Factory reset. 

## Config Parameter coverage

*   No new configuration covered.

