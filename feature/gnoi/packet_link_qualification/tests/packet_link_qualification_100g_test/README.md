# gNOI-2.1: Packet-based Link Qualification on 100G links

## Summary

Validate gNOI Packet Link Qualification (`gNOI.LinkQualification`) on a 100G
link pair with dynamic port-speed and packet-size calculation, vendor-aware
capabilities verification, Max-PPS qualification, and single-pair Singleton and
static Member-Link LAG qualification. Multi-link scale testing is not performed;
the new test addition specifically covers Max-PPS qualification alongside
single-pair Singleton and Member-Link LAG coverage.

## Testbed type

*   [`featureprofiles/topologies/dut_1_loop_100g.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut_1_loop_100g.testbed)

## Procedure

### gNOI-2.1.1 - Capabilities, Error Handling, and List/Delete Validation

1.  Issue `gnoi.packet_link_qualification.LinkQualification.Capabilities` on
    the DUT and validate vendor-aware `MaxMtu`, `MaxBps`, `MaxPps`,
    setup/teardown durations, `MinSampleInterval`,
    `MaxHistoricalResultsPerInterface`, and reflector timing capabilities.
2.  Issue `Get` and `Delete` with non-existent session IDs and verify
    `NOT_FOUND` (status code 5).
3.  Create qualification sessions on the 100G pair, confirm presence via `List`,
    delete via `Delete`, and verify removal from `List` and `Get`.

### gNOI-2.1.2 - Max-PPS Link Qualification

1.  Configure dual-stack IPv4/IPv6 addresses on the 100G link pair and wait for
    `OperStatus == UP`.
2.  Compute packet size dynamically from device `MaxMtu`, `MaxBps`, and link
    line rate so testing at the device `MaxPps` capability does not exceed link
    or generator throughput limits.
3.  Qualify the link at `MaxPps`, poll `List` until completion, verify interface
    `OperStatus == TESTING` during `RUNNING` (unless skipped by deviation), and
    assert zero drops/errors and expected packet counts on both generator and
    reflector (when reflector stats are supported).

### gNOI-2.1.3 - Singleton Link Qualification

1.  Configure dual-stack IPv4/IPv6 addresses on the 100G link pair.
2.  Compute per-link traffic targeting 10% of 100G line rate (using standard
    packet sizing clamped to device capability bounds).
3.  Run `Create` on the singleton pair, poll `List` to completion, validate
    packet delivery and zero drops/errors, and delete sessions.

### gNOI-2.1.4 - Member-Link LAG Qualification

1.  Configure static LAGs across the 100G port pair (using two distinct
    single-member aggregate interfaces on a single DUT) with dual-stack
    IPv4/IPv6 addressing and wait for member and aggregate interfaces to come
    `UP`.
2.  Qualify the 100G member interfaces via `LinkQualification`, verify
    completion and traffic counters, and delete sessions.

### Cleanup

All qualification sessions created during each subtest are deleted via
`gnoi.packet_link_qualification.LinkQualification.Delete`, and all test-specific
LAG, IPv4/IPv6 subinterface, and network-instance configurations are reverted so
the DUT returns to its baseline operational state.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "config": {
          "enabled": true,
          "name": "Port-Channel1",
          "type": "iana-if-type:ieee8023adLag"
        },
        "name": "Port-Channel1"
      },
      {
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "config": {
          "enabled": true,
          "name": "Port-Channel2",
          "type": "iana-if-type:ieee8023adLag"
        },
        "name": "Port-Channel2"
      },
      {
        "config": {
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Port-Channel1"
          }
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 31
                      },
                      "ip": "192.0.2.1"
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::1",
                        "prefix-length": 126
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
          "enabled": true,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Port-Channel2"
          }
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.2",
                        "prefix-length": 31
                      },
                      "ip": "192.0.2.2"
                    }
                  ]
                }
              },
              "ipv6": {
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
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/ethernet/state/negotiated-port-speed:
  /interfaces/interface/ethernet/state/port-speed:
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/state/oper-status:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    packet_link_qualification.LinkQualification.Capabilities:
    packet_link_qualification.LinkQualification.Create:
    packet_link_qualification.LinkQualification.Delete:
    packet_link_qualification.LinkQualification.Get:
    packet_link_qualification.LinkQualification.List:
```
