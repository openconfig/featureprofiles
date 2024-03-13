# gNOI-5.3: Copying Debug Files

## Summary

* Validate that the debug files can be copied out of the DUT.

## Procedure

* gNOI-5.3.1 - Software Process Health Check
   * Issue gnoi System.KillProcessRequest to the DUT to crash a software process.
   * Issue gnoi System.Healthz Get RPC to chassis.
   * Verify that the DUT responds without any errors.

* gNOI-5.3.2 - Configuration daemon kill process and Health check
   * Execute gNMI subscribe ON_CHANGE to the config process in the background
   * While subscribe is waiting for updates, Kill the process that manages device configuration using the gNOI.KillProcessRequest_SIGNAL_ABRT operation and restart flag set to true. This will terminate the process and will also dump a Core file.
   *  Read the subscribe updates and verify if the leaf **/components/component[process that handles configuration of the DUT]/healthz/state/status** transitioned to **UNHEALTHY**. If not, then fail the test.
   * Since the software module had a status of UNHEALTHY, issue healthZ.Get() to collect more details on the event. Also, use HealthZ.Artifact() to collect artifacts like core dump, logs etc. The test should fail if any of these RPCs fail.
   * Push test configuration to the router using gNMI.Set() RPC with "replace operation" and reverify the status of the leaf /components/component[process that handles configuration of the DUT]/healthz/state/status/.
   * If the configuration push is successful, make a gNMI.Get() RPC call and compare the configuration received with the originally pushed configuration for a match. Test is a failure if either the gNMI.Get() operation fails or the configuration do not match with the one that was pushed
  
* gNOI-5.3.3 - Chassis Component Health Check
   * Issue Healthz Check RPC to the DUT for Chassis component to trigger the generation of Artifact ID(s). Artifacts returned should be sufficient for vendor tech support teams to determine if any of the field replaceable components are faulty and must be replaced for that device.
   * Verify that the DUT returns the artifact IDs in the Check RPC's response.
   * Invoke ArtifactRequest to transfer the requested Artifact ID(s).
   * Verify that the DUT returns the artifacts requested.

## Process names by vendor
* BGP Process
   * ARISTA:  "IpRib"
   * CISCO: " "
   * JUNIPER: "rpd"
   * NOKIA:   "sr_bgp_mgr"
* Configuration process
   * Arista: ConfigAgent
   * Cisco:
   * Juniper: mgd
   * Nokia:
  
* NOS implementations will need to model their agent that handles device configuration as a [" component of the type SOFTWARE_MODULE"](https://github.com/openconfig/public/blob/master/release/models/platform/openconfig-platform-types.yang#L394) and represent it under the componenets/component tree


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
