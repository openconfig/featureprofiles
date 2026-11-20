# RT-6.2: LLDP configuration

## Summary

Configure global LLDP and one interface. SpyTest sets enabled, system-name,
hello-timer, and per-interface enabled. This test also sets the management
interface that sources the Management Address TLV, and it checks state.

TLV population with traffic is [RT-6.1](../../otg_tests/core_lldp_tlv_population_test/README.md).

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   Replace `/lldp/config/enabled` with true, `system-name` with `fp-lldp`,
    `hello-timer` with 30, and `management-interface` with the first Ethernet
    port. Read each leaf from config and state.
*   Replace `/lldp/interfaces/interface/config/enabled` with true on that port.
    Read config and state.
*   Restore every leaf that was changed.

#### Canonical OC

```json
{
  "lldp": {
    "config": {
      "enabled": true,
      "system-name": "fp-lldp",
      "hello-timer": 30,
      "management-interface": "Ethernet0"
    },
    "interfaces": {
      "interface": [
        {
          "name": "Ethernet0",
          "config": {
            "name": "Ethernet0",
            "enabled": true
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /lldp/config/enabled:
  /lldp/config/system-name:
  /lldp/config/hello-timer:
  /lldp/config/management-interface:
  /lldp/interfaces/interface/config/enabled:
  /lldp/state/enabled:
  /lldp/state/system-name:
  /lldp/state/hello-timer:
  /lldp/state/management-interface:
  /lldp/interfaces/interface/state/enabled:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
```
