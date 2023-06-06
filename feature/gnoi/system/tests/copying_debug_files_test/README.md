# gNOI-5.3: Copying Debug Files

## Summary

Validate that the debug files can be copied out of the DUT.

## Procedure

*   Issue gnoi System.KillProcessRequest to the DUT to crash a software process. 
*   Issue gnoi System.Healthz Get RPC to chassis.
*   TODO: Validate that the device returns the vendor relevant information for
    debugging via gnoi Healthz.Artifact

## Config Parameter Coverage

N/A

## Telemetry Parameter Coverage

## Protocol/RPC Parameter Coverage

*   gNOI
    *   System
        *   KillProcess
    *   Healthz
        *   Get
        *   Artifact
