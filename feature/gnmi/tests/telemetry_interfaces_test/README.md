# gNMI-1.28: Telemetry: Interface openconfig validation.

## Summary

Telemetry: Interface oc path validation

## Testbed type

*  [`featureprofiles/topologies/dut_2links.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut_2links.testbed)

## Procedure

### Test environment setup

    ```
      |         |
      |   DUT1  |----------PORT1
      |         |
      |         |----------PORT2
    ```

#### Prerequisites

* We assume all counter values to be populated with non zero values.

### gNMI-1.28.1: Validate Interfaces_State

* Validate interface state leaves are streamed and holds expected value
 - last-change
 - oper-status
 - admin-status
 - description

### gNMI-1.28.2: Validate Interfaces_State_Counters

* Validate interface state counter leaves are streamed and holds expected value
 - in-octets, out-octets
 - in-unicast-pkts, out-unicast-pkts
 - in-broadcast-pkts, out-broadcast-pkts
 - in-multicast-pkts, out-multicast-pkts
 - in-discards, out-discards
 - in-errors, out-errors

### gNMI-1.28.3: Validate Interfaces_State_Rates

* Validate interface state rate leaves are streamed and holds expected value
 - `/interfaces/interface[name=<port>]/state/in-rate`
 - `/interfaces/interface[name=<port>]/state/out-rate`

The test constructs these paths with `gnmi.OC().Interface(port).InRate().State()` and `OutRate().State()` in `testTelemetryInterfacesStateRate`. The OpenConfig `openconfig-interfaces` schema defines both as read-only interface state leaves ([schema](https://www.openconfig.net/projects/models/schemadocs/yangdoc/openconfig-interfaces.html)). This records the generic test expectation; it does not imply that every vendor's OCIF implementation exposes the leaves. For IOS XR `26.4.1.26I` and `26.4.1.28I` on 8808, the platform exception marks the rate paths unsupported.

### gNMI-1.28.4: Validate Interfaces_State_Subinterface

* Validate interface state subinterface leaves are streamed with expected value
 - subinterface/state/oper-status
 - subinterface/config/description
 - subinterface/config/index

### gNMI-1.28.5: Validate Interfaces_Config

* Validate interface config leaves are streamed with expected value
  - /interfaces/interface/config/enabled
  - /interfaces/interface/config/type
  - /interfaces/interface/ethernet/config/mac-address

### gNMI-1.28.6: Validate Interfaces_Aggregation

* Validate interface aggregation leaves are streamed with expected value
  - /interfaces/interface/aggregation/config/lag-type
  - /interfaces/interface/aggregation/state/member

These leaves belong to the logical aggregate interface. TEST6 creates an `ieee8023adLag` interface, sets its `aggregation/config/lag-type`, assigns the physical test ports to it using `ethernet/config/aggregate-id`, and reads `aggregation/state/member` on the aggregate interface.

Platforms with `aggregate_atomic_update`, including Nokia, receive the aggregate and all physical members in one root-level gNMI Update. This prevents separate transactions from dropping `lag-type` or rejecting member attachment.

The member leaf-list is compared without relying on its returned order. XR can report the two configured members in reverse order; both names must be present, but that order is not a test requirement.

TEST5 restores the original MAC address configuration, including whether an override was configured. When no override existed, cleanup deletes the temporary `ethernet/config/mac-address` leaf. Writing the original operational MAC back as configuration would leave an explicit override and cause XR to reject later bundle membership with `Conflicting: /ord_m/macaddr`.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "ethernet-1/1",
        "config": {
          "type": "ethernetCsmacd",
          "enabled": true
        },
        "state": {
          "admin-status": "UP",
          "oper-status": "DOWN",
          "last-change": "1775518814873000000",
          "counters": {
            "in-octets": "2252",
            "in-pkts": "19",
            "in-unicast-pkts": "0",
            "in-broadcast-pkts": "0",
            "in-multicast-pkts": "19",
            "in-errors": "0",
            "in-discards": "0",
            "out-octets": "5460",
            "out-pkts": "21",
            "out-unicast-pkts": "0",
            "out-broadcast-pkts": "0",
            "out-multicast-pkts": "21",
            "out-discards": "0",
            "out-errors": "0"
          }
        },
        "ethernet": {
          "config": {
            "mac-address": "AA:83:82:94:78:00"
          },
          "state": {
            "mac-address": "AA:83:82:94:78:00"
          }
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
    /interfaces/interface/state/last-change:
    /interfaces/interface/state/oper-status:
    /interfaces/interface/state/admin-status:
    /interfaces/interface/state/description:
    /interfaces/interface/state/counters/in-octets:
    /interfaces/interface/state/counters/out-octets:
    /interfaces/interface/state/counters/in-unicast-pkts:
    /interfaces/interface/state/counters/out-unicast-pkts:
    /interfaces/interface/state/counters/in-broadcast-pkts:
    /interfaces/interface/state/counters/out-broadcast-pkts:
    /interfaces/interface/state/counters/in-multicast-pkts:
    /interfaces/interface/state/counters/out-multicast-pkts:
    /interfaces/interface/state/counters/in-discards:
    /interfaces/interface/state/counters/out-discards:
    /interfaces/interface/state/counters/in-errors:
    /interfaces/interface/state/counters/out-errors:
    /interfaces/interface/state/in-rate:
    /interfaces/interface/state/out-rate:
    /interfaces/interface/subinterfaces/subinterface/state/oper-status:
    /interfaces/interface/subinterfaces/subinterface/config/description:
    /interfaces/interface/subinterfaces/subinterface/config/index:
    /interfaces/interface/config/enabled:
    /interfaces/interface/config/type:
    /interfaces/interface/ethernet/config/mac-address:
    /interfaces/interface/aggregation/config/lag-type:
    /interfaces/interface/aggregation/state/member:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Get:
```
