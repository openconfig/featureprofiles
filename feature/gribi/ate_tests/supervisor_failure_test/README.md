# TE-8.2: Supervisor Failure

## Summary

Ensure that gRIBI entries are persisted over supervisor failure.

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Establish a gRIBI connection (SINGLE_PRIMARY and PRESERVE mode) to the DUT,
    and inject entries for 203.0.113.0/24 into default VRF.

*   Ensure that the entries are installed through telemetry and forwarding
    traffic between ATE port-1 and port-2.

*   Validate:

    *   Traffic continues to be forwarded between ATE port-1 and port-2 during
        supervisor switchover triggered using gNOI SwitchControlProcessor.

    *   Following reconnection of a gRIBI client, Get returns 203.0.113.0/24 as
        an installed entry.

## Config Parameter coverage

No configuration parameters.

## Telemetry Parameter coverage

*   (type=CONTROLLER_CARD)
    /components/component[name=<supervisor>]/state/redundant-role

*   (type=CONTROLLER_CARD)
    /components/component[name=<supervisor>]/state/last-switchover-time

*   (type=CONTROLLER_CARD)
    /components/component[name=<supervisor>]/state/last-switchover-reason/trigger

*   (type=CONTROLLER_CARD)
    /components/component[name=<supervisor>]/state/last-switchover-reason/details

## Protocol/RPC Parameter coverage

*   gNOI
    *   System
        *   SwitchControlProcessor

## Minimum DUT platform requirement

MFF
