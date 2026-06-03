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
 - in-rate, out-rate

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
