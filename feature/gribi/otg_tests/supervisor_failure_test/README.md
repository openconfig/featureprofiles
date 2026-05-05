# TE-8.2: Supervisor Failure

## Summary

Ensure that gRIBI entries are persisted over supervisor failure.

## Testbed type

* https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

### Test environment setup:

* Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2.
* Assign IPv4 addresses to all ports.

### TE-8.2.1 - FIB Programming and Switchover Validation

* Connect gRIBI client to DUT specifying persistence mode PRESERVE,
`SINGLE_PRIMARY` client redundancy in the SessionParameters request, and
make it become leader. Ensure that no error is reported from the gRIBI
server.

* Add 100 `IPv4Entry`s for prefixes (e.g., `203.0.113.0/24` through `203.0.113.99/24`) pointing to ATE port-2 via
`gRIBI-A`. Ensure that the entries are active through AFT telemetry and correct
ACKs are received.

* Send traffic from ATE port-1 to the 100 prefixes, and ensure traffic
flows 100% and reaches ATE port-2.

* Validate: Traffic continues to be forwarded between ATE port-1 and ATE
port-2 during supervisor switchover triggered using gNOI
`SwitchControlProcessor`.

* Following reconnection of a gRIBI client to the new master supervisor, ensure
the 100 prefixes pointing to ATE port-2 are present and traffic
flows 100% from ATE port-1 to ATE port-2.

### TE-8.2.2 - Post Switchover FIB Programming Validation

* Add another 100 `IPv4Entry`s for new prefixes (e.g., `203.0.114.0/24` through `203.0.114.99/24`) pointing to ATE port-2. Ensure that these new entries are active through AFT telemetry and correct ACKs are received.

* Send traffic from ATE port-1 to the additional 100 prefixes, and ensure traffic flows 100% and reaches ATE port-2 to verify that the FIB is programmed correctly after the switchover.

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

* MFF


## Canonical OC
```json
{}
```   
