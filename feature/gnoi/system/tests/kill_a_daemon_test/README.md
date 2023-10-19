# gNOI-3.6: Kill a Daemon Test

## Summary

This is a negative test to kill some Software daemons and verify if the implementation is able to accurately stream the Health and all related artifacts for the subject daemon.
  
## Procedure

* Subtest-1 : Kill configuration daemon.

  Kill the process that manages device configuration using the **gNOI.KillProcessRequest_SIGNAL_ABRT** and restart flag set to False. This will terminate the process and will also dump a Core file, while maintaing the process in a down state. Verify if the leaf /components/component/healthz/state/status transitioned to **UNHEALTHY**.
   1. If the software module has a status of **UNHEALTHY**, issue healthZ.Get() to collect more details on the event. Also, use HealthZ.Artifact() to collect artifacts like core dump, logs etc.
   2. Initiate a **gNOI.KillProcessRequest_SIGNAL_HUP** operation to restart and recover the killed process.
   3. Push test configuration to the router using gNMI.Set() RPC with "replace operation" and reverify the status of the leaf /components/component[process that handles configuration of the DUT]/healthz/state/status/.
   4. Ensure that the configuuration push is successful and artifacts can be collected. If yes, mark the test as success.


## Process that controls configuration of a router by vendor
   * Different processes by vendors
      * Juniper: mgd
      * Cisco:
      * Arista: ConfigAgent
      * Nokia: 
   * NOS implementations will need to model their agent that handles device configuration as a [" component of the type SOFTWARE_MODULE"](https://github.com/openconfig/public/blob/master/release/models/platform/openconfig-platform-types.yang#L394) and represent it under the componenets/component tree


## Config Parameter Coverage
*   HealthZ.Get()
*   HealthZ.Artifact()

## Telemetry Parameter Coverage
*   /components/component[**process that handles configuration of the DUT**]/healthz/state/status/
