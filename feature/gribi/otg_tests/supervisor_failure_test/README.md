# TE-8.2: Supervisor Failure

## Summary

Ensure that gRIBI entries are persisted over supervisor failure.

## Procedure

*   Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2. Assign IPv4
    addresses to all ports.

*   Connect gRIBI client to DUT specifying persistence mode PRESERVE,
    `SINGLE_PRIMARY` client redundancy in the SessionParameters request, and
    make it become leader. Ensure that no error is reported from the gRIBI
    server.

*   Add an `IPv4Entry` for prefix `203.0.113.0/24` pointing to ATE port-2 via
    `gRIBI-A`. Ensure that the entry is active through AFT telemetry and correct
    ACK is received.

*   Send traffic from ATE port-1 to prefix `203.0.113.0/24`, and ensure traffic
    flows 100% and reaches ATE port-2.

*   Validate: Traffic continues to be forwarded between ATE port-1 and ATE
    port-2 during supervisor switchover triggered using gNOI
    `SwitchControlProcessor`.

    Following reconnection of a gRIBI client to new master supervisor , ensure
    the prefix `203.0.113.0/24` pointing to ATE port-2 is present and traffic
    flows 100% from ATE port-1 to ATE port-2.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
    ## Config Parameter coverage

    ## Telemetry Parameter coverage

    /components/component/state/last-reboot-time:
     platform_type: ["CHASSIS", "CONTROLLER_CARD"]
    /components/component/state/last-reboot-reason:
     platform_type: ["CHASSIS", "CONTROLLER_CARD" ]
    /components/component/state/redundant-role:
     platform_type: [ "CONTROLLER_CARD" ]
    /components/component/state/last-switchover-time:
     platform_type: [ "CONTROLLER_CARD" ]
    /components/component/state/last-switchover-reason/trigger:
     platform_type: [ "CONTROLLER_CARD" ]
    /components/component/state/last-switchover-reason/details:
     platform_type: [ "CONTROLLER_CARD" ]

rpcs:
    gnmi:
        gNMI.Set:
        gNMI.Get:
        gNMI.Subscribe:
    gnoi:
        system.System.SwitchControlProcessor:
```

## Minimum DUT Required

vRX - Virtual Router Device
