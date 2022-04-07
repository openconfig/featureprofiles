# gNOI-3.1: Complete Chassis Reboot

## Summary

Validate gNOI RPC can reboot entire chassis

## Procedure


* Configure ATE port-1 connected to DUT port-1 with the relevant IPv4 and IPv6 addresses.
* Send gNOI reboot request using the method COLD with the delay of N seconds.
     - method: Only the COLD method is required to be supported by all targets.
     - Delay: In nanoseconds before issuing reboot.
     - message: Informational reason for the reboot.
     - force: Force reboot if basic checks fail. (ex. uncommitted configuration).
   * Verify the following items.
     - DUT remains reachable for N seconds by checking DUT current time is updated.
     - DUT boot time is updated after reboot.
     - DUT software version is the same after the reboot.
* Send gNOI reboot request using the method COLD without delay.
     - method: Only the COLD method is required to be supported by all targets.
     - Delay: 0 - no delay.
     - message: Informational reason for the reboot.
     - force: Force reboot if basic checks fail. (ex. uncommitted configuration).
   * Verify the following items.
     - DUT boot time is updated after reboot.
     - DUT software version is the same after the reboot.

## Test notes:
  * A RebootRequest requests the specified target be rebooted using the specified
    method after the specified delay.  Only the DEFAULT method with a delay of 0
    is guaranteed to be accepted for all target types.
  * A RebootMethod determines what should be done with a target when a Reboot is
    requested.  Only the COLD method is required to be supported by all
    targets.  Methods the target does not support should result in failure.
  * gnoi operation commands can be sent and tested using CLI command grpcurl.
    https://github.com/fullstorydev/grpcurl

## Telemetry Parameter Coverage

*  /system/state/boot-time 
