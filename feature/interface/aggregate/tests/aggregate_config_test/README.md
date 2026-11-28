# RT-5.22: Aggregate interface configuration

## Summary

Create one LACP aggregate, attach the first Ethernet port, and set the LACP
interval to FAST and then SLOW. SpyTest rewrites aggregate-id, interval, and
lag-type on an aggregate that is already configured. Negotiation of the bundle
is [RT-5.11](../../lacp/tests/lacp_interval_test/README.md).

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   Replace interface `PortChannel100` with type `ieee8023adLag` and lag-type
    `LACP`.
*   Replace the first Ethernet port's `aggregate-id` with `PortChannel100`.
    Read lag-type and aggregate-id from config.
*   Replace the LACP interval for `PortChannel100` with FAST, then with SLOW.
    Read each value from config and state.
*   Delete the LACP interface, restore or delete the member aggregate-id, and
    delete `PortChannel100`.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "PortChannel100",
        "config": {"name": "PortChannel100", "type": "iana-if-type:ieee8023adLag"},
        "aggregation": {"config": {"lag-type": "LACP"}}
      },
      {
        "name": "Ethernet0",
        "ethernet": {"config": {"aggregate-id": "PortChannel100"}}
      }
    ]
  },
  "lacp": {
    "interfaces": {
      "interface": [
        {"name": "PortChannel100", "config": {"name": "PortChannel100", "interval": "FAST"}}
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /lacp/interfaces/interface/config/interval:
  /lacp/interfaces/interface/state/interval:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
```
