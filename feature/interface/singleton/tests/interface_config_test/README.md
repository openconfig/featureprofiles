# RT-5.21: Interface configuration leaves

## Summary

Round-trip admin state, description, MTU, and type on one Ethernet port, then
create an IPv6 address and two IPv4 addresses on subinterface 0. SpyTest covers
admin state, description, MTU, type, and one IPv6 address. Two IPv4 addresses
are the primary and secondary address case. Traffic for these leaves is
[RT-5.1](../../otg_tests/singleton_test/README.md).

## Testbed type

*   [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

*   On the first Ethernet interface, replace `enabled` with the opposite of
    the current value, `description` with `featureprofiles interface config`,
    and `mtu` with 1500 or 9216. Write `type` back as its current value.
    Read config and state, then restore the port.
*   Replace `2001:db8:8:1::1/64` and `192.0.2.10/24` plus `192.0.2.11/24` on
    subinterface 0. Read the addresses from config. Delete the IPv6 address
    and check it is gone.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Ethernet0",
        "config": {
          "name": "Ethernet0",
          "description": "featureprofiles interface config",
          "mtu": 9216
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {"ip": "192.0.2.10", "config": {"ip": "192.0.2.10", "prefix-length": 24}},
                    {"ip": "192.0.2.11", "config": {"ip": "192.0.2.11", "prefix-length": 24}}
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {"ip": "2001:db8:8:1::1", "config": {"ip": "2001:db8:8:1::1", "prefix-length": 64}}
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/description:
  /interfaces/interface/config/mtu:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/state/enabled:
  /interfaces/interface/state/mtu:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
```
