# gNOI-5.3: Copying Debug Files

## Summary

Validate that the debug files can be copied out of the DUT.

## Procedure

*   Test #1 - Software Process Health Check
    *   Issue gnoi System.KillProcessRequest to the DUT to crash a software process. 
    *   Issue gnoi System.Healthz Get RPC to chassis.
    *   Verify that the DUT responds without any errors.

*   Test #2 - Chassis Component Health Check
    *   Issue Healthz Check RPC to the DUT for Chassis component to trigger the generation of Artifact ID(s) equivalent to 'show tech support'.
    *   Verify that the DUT returns the artifact IDs in the Check RPC's response.
    *   Invoke ArtifactRequest to transfer the requested Artifact ID(s).
    *   Verify that the DUT returns the artifacts requested.

## Config Parameter Coverage

N/A

## Telemetry Parameter Coverage

## Protocol/RPC Parameter Coverage

*   gNOI
    *   System
        *   KillProcess
    *   Healthz
        *   Get
        *   Check
        *   Artifact
