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

* Add 50 `IPv4Entry`s (starting at `203.0.113.1/32`) and 50 `IPv6Entry`s
(starting at `2001:db8:203:0:113::1/128`) pointing to ATE port-2 via
`gRIBI-A`. Ensure that the entries are active through AFT telemetry and correct ACK is received.

* Send traffic from ATE port-1 to the 100 prefixes (50 IPv4 and 50 IPv6), and ensure traffic
flows 100% and reaches ATE port-2.

* Validate: Supervisor switchover is triggered using gNOI `SwitchControlProcessor`.
Ensure gRIBI entries persist over switchover and traffic contniues to be forwarded.

* Following reconnection of a gRIBI client to the new master supervisor, ensure
the 100 prefixes pointing to ATE port-2 are present and traffic
flows 100% from ATE port-1 to ATE port-2.

### TE-8.2.2 - Post Switchover FIB Programming Validation

* Add another 50 `IPv4Entry`s (starting at `203.0.114.1/32`) and 50
`IPv6Entry`s (starting at `2001:db8:203:0:114::1/128`) pointing to ATE
port-2. Ensure that these new entries are active through AFT telemetry and correct ACKs are received.

* Send traffic to all 200 prefixes (100 initial + 100 post-switchover) and
ensure traffic flows 100% from ATE port-1 to ATE port-2.

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
