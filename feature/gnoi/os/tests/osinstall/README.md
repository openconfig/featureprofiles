# gNOI-4.1: Software Upgrade

## Summary

*   Validate new software can be copied and activated on single and dual supervisors.
*   Validate successful configiuration push to the DUT and also confirm the health of the software-module that allows configuration of the router.


## Procedure

* Subtest-1 : OS Install on the primary and secondary controller cards. 
   1. Install and activate a new software version on primary supervisor:
      
      a. Issue gnoi.os.Install rpc to the chassis with InstallRequest.TransferRequest message. The message should set the version to the desired new version image, and standby_supervisor to FALSE.
         * Wait for the switch to respond with InstallResponse. Expect it to
         return TransferReady.

      b. Transfer the content by issuing gnoi.os.Install rpc with InstallRequest.transfer_content message.
         * Expect it to return InstallResponse with a TransferProgress status
         asynchronously at certain intervals.
         * TODO: When the expected amount of bytes_received is reported by the
         switch, move to the activation step next.

      c. End the transfer of software by issuing gnoi.os.Install rpc with InstallRequest.TransferEnd message.
         * Expect the switch to return InstallResponse with a Validated message. The version in the message should be set to the one which was transferred above.
      
      d. Activate the software by issuing gnoi.os.Activate rpc.
         * Set the version field of the ActivateRequest message to be the same as the version specified in the TransferRequest message above.
         * Set the no_reboot flag to true.
         * Set the standby_supervisor to FALSE.
           
   2. Install and activate the same new software version on standby supervisor:
      
      a. Repeat the above process of TransferRequest. This time set the standby_supervisor to TRUE.
         * Expect the switch to return a InstallResponse with a SyncProgress message. The switch should sync the software image from primary SUP to standby.
         * Expect the sync to return a value of 100 for percentage_transferred field.
         * At the end, expect the switch to return InstallResponse with a Validated message. The version in the message should be set to the one which was transferred above.
           
   3. Activate the software by issuing gnoi.os.Activate rpc as in the case of primary supervisor.
      
      a. Set the version field of the ActivateRequest message to be the same as the version specified in the TransferRequest message above.
      
      b. Set the no_reboot flag to true.
      
      c. Set the standby_supervisor to TRUE this time.
      
   4. Reboot the switch:
      
      a. Issue gnoi.system.Reboot as specified in [gNOI-3.1: Complete Chassis Reboot](feature/gnoi/tests/complete_chassis_reboot/complete_chassis_reboot_test.md).
      
   5. Verify that the supervisor image has moved to the new image:
       
      a. Verify that the supervisor has a valid image by issuing gnoi.os.Verify rpc.
         * Expect a VerifyResponse with the version field set to the version specified in messages above eventually.
         * Verify the standby supervisor version.
         * Expect that the VerifyResponse.verify_standby has the same version in messages above.

               
* Subtest-2 : Configuration push verification post OS change and check health of the software-module that allows configuration.
  1. Check the health of the software-module component that allows configuration of the router and verify if it is **HEALTHY** using the leaf /components/component[**process that handles configuration of the DUT**]/healthz/state/status/
     
     a. If unhealthy, run HealthZ.Get() and HealthZ.Artifact() RPCs on the subject component to fetch artifacts corresponding to the event.
        * Rollback to the previous OS version by following the steps in 3, 4 and 5 above from Sub-test-1 to recover the DUT from the faulty state.
        * Mark the test as a failure due to issues with the OS upgrade and exit the test.
          
  2. Do a gNMI.GET() RPC to extract the current configuration on the DUT as backup.
     
  3. Push test configuration to the router using gNMI.Set() RPC with "replace operation" and reverify the status of the leaf /components/component[process that handles configuration of the DUT]/healthz/state/status/.
     
     a. If **UNHEALTHY**, run HealthZ.Get() and HealthZ.Artifact() RPCs on the subject component to fetch artifacts corresponding to the event.
        [TODO: Below step needs to be discussed to inderstand how to recover the DUT. May just need to depend on the Test infrastructure]
        * Push the backup configuration fetched in Subtest-2 bullet#2 above to the DUT to recover the DUT.
        * Mark the test as a failure due to issues with the OS upgrade and exit the test.
 

## Process that controls configuration of a router by vendor
   * Different processes by vendors
      * Juniper: mgd
      * Cisco:
      * Arista: ConfigAgent
   * NOS implementations will need to model their agent that handles device configuration as a [" component of the type SOFTWARE_MODULE"](https://github.com/openconfig/public/blob/master/release/models/platform/openconfig-platform-types.yang#L394) and represent it under the componenets/component tree
     
  
## Config Parameter Coverage
*   HealthZ.Get()
*   HealthZ.Artifact()

## Telemetry Parameter Coverage
*   /system/state/boot-time
*   /components/component[**process that handles configuration of the DUT**]/healthz/state/status/
