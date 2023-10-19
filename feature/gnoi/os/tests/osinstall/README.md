# gNOI-4.1: Software Upgrade

## Summary

*   Validate new software can be copied and activated on single and dual supervisors.
*   Validate successful configiuration push to the DUT post OS upgrade.


## Procedure

* gNOI-4.1.1 : OS Install on the primary and secondary controller cards. 
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
           
      **Please Note:** Some implememtations do configuration verification of the existing configuration in the box before activating the changed OS version. And if the exitsing configuration isn't compatible with the new OS version then the Software activation would fail. This should be considered as a test failure and the test should exit.
           
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

               
* gNOI-4.1.2: Configuration push verification post OS update.
  1. Push a test configuration to the router using gNMI.Set() RPC with "replace operation".
  2. If the configuration push is successful, make a gNMI.Get() RPC call and compare the configuration received with the originally pushed configuration and check if the configuration is a match. Test is a failure if either the gNMI.Get() operation fails or the configuration do not match with the one that was pushed.

     Note: For the test configuration, please include interface and BGP configuration.
  

## Telemetry Parameter Coverage
*   /system/state/boot-time
