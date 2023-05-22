# gNOI-4.1: Software Upgrade

## Summary

Validate new software can be copied and activated on single and dual
supervisors.

## Procedure

*   Install and activate a new software version on primary supervisor:

    *   Issue gnoi.os.Install rpc to the chassis with
        InstallRequest.TransferRequest message. The message should set the
        version to the desired new version image, and standby_supervisor to
        FALSE.

        *   Wait for the switch to respond with InstallResponse. Expect it to
            return TransferReady.

    *   Transfer the content by issuing gnoi.os.Install rpc with
        InstallRequest.transfer_content message.

        *   Expect it to return InstallResponse with a TransferProgress status
            asynchronously at certain intervals.
        *   TODO: When the expected amount of bytes_received is reported by the
            switch, move to the activation step next.

    *   End the transfer of software by issuing gnoi.os.Install rpc with
        InstallRequest.TransferEnd message.

        *   Expect the switch to return InstallResponse with a Validated
            message. The version in the message should be set to the one which
            was transferred above.

    *   Activate the software by issuing gnoi.os.Activate rpc.

        *   Set the version field of the ActivateRequest message to be the same
            as the version specified in the TransferRequest message above.
        *   Set the no_reboot flag to true.
        *   Set the standby_supervisor to FALSE.

*   Install and activate the same new software version on standby supervisor:

    *   Repeat the above process of TransferRequest. This time set the
        standby_supervisor to TRUE.
        *   Expect the switch to return a InstallResponse with a SyncProgress
            message. The switch should sync the software image from primary SUP
            to standby.
        *   Expect the sync to return a value of 100 for percentage_transferred
            field.
        *   At the end, expect the switch to return InstallResponse with a
            Validated message. The version in the message should be set to the
            one which was transferred above.

*   Activate the software by issuing gnoi.os.Activate rpc as in the case of
    primary supervisor.

    *   Set the version field of the ActivateRequest message to be the same as
        the version specified in the TransferRequest message above.
    *   Set the no_reboot flag to true.
    *   Set the standby_supervisor to TRUE this time.

*   Reboot the switch:

    *   Issue gnoi.system.Reboot as specified in
        [gNOI-3.1: Complete Chassis Reboot](feature/gnoi/tests/complete_chassis_reboot/complete_chassis_reboot_test.md).

*   Verify that the supervisor image has moved to the new image:

    *   Verify that the supervisor has a valid image by issuing gnoi.os.Verify
        rpc.
        *   Expect a VerifyResponse with the version field set to the version
            specified in messages above eventually.
    *   Verify the standby supervisor version.
        *   Expect that the VerifyResponse.verify_standby has the same version
            in messages above.

## Config Parameter Coverage

## Telemetry Parameter Coverage

*   /system/state/boot-time
