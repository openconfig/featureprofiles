# TE-1.1: Static ARP

## Summary

Ensure static ARP entries installed on the DUT are honoured.

## Procedure

*   Configure OTG port-1 connected to DUT port-1, and OTG port-2 connected to
    DUT port-2, with the relevant IPv4 and IPv6 addresses.
*   Without static ARP entry:
    *   Configure OTG traffic flow to enable custom egress filter on the last
        15-bits of the destination MAC (starting at bit offset 33 of the
        ethernet packet).
    *   Ensure that traffic can be forwarded between OTG port-1 and OTG port-2
        normally.
    *   Check that the egress filter picks up the last 15-bit of OTG default MAC
        address.
*   Add static entry to DUT interfaces to override the OTG MAC address.
*   With static ARP entry:
    *   Configure OTG traffic flow with custom egress filter as before, and
        ensure that traffic can be forwarded between OTG port-1 and OTG port-2.
    *   Check that the egress filter picks up the last 15-bit of the MAC address
        set by static ARP.
    *   Verify the IPv4 and IPv6 neighbor telemetry state (`ip` and `link-layer-address`):
        *   `/interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address`
        *   `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/ip`
        *   `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address`

Note that OTG ports are promiscuous, i.e. they will receive all packets
regardless of the destination MAC. The custom egress filter is used to tell what
are the destination MAC addresses of the packets seen by the OTG.

## Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT to ATE source",
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port1",
        "openconfig-if-ethernet:ethernet": {
          "config": {
            "mac-address": "02:1a:c0:00:02:02"
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "openconfig-if-ip:ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.2",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.2"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "ip": "192.0.2.1",
                        "link-layer-address": "12:34:56:78:7a:69"
                      },
                      "ip": "192.0.2.1"
                    }
                  ]
                }
              },
              "openconfig-if-ip:ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::2",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::2"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "ip": "2001:db8::1",
                        "link-layer-address": "12:34:56:78:7a:69"
                      },
                      "ip": "2001:db8::1"
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "DUT to ATE destination",
          "enabled": true,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port2",
        "openconfig-if-ethernet:ethernet": {
          "config": {
            "mac-address": "02:1a:c0:00:02:05"
          }
        },
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "openconfig-if-ip:ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "ip": "192.0.2.6",
                        "link-layer-address": "12:34:56:78:7a:69"
                      },
                      "ip": "192.0.2.6"
                    }
                  ]
                }
              },
              "openconfig-if-ip:ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::5",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "ip": "2001:db8::6",
                        "link-layer-address": "12:34:56:78:7a:69"
                      },
                      "ip": "2001:db8::6"
                    }
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
  ## Config Parameter Coverage
   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
   /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/ip:
   /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/config/link-layer-address:
   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
   /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/config/ip:
   /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/config/link-layer-address:
  ## telemetry Parameter Coverage
   /interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address:
   /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/ip:
   /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```
