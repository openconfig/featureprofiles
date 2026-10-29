# gNOI-2.1: Packet-based Link Qualification on 100G and 400G links

## Summary

Validate gNOI RPC can support packet-based link qualification test for the 400g
and 100g links within 1 DUTs.

## Testbed type

*   [`featureprofiles/topologies/dut_400zr_100g_4links.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut_400zr_100g_4links.testbed)

## Procedure

### Topology

+------------------------+                         +------------------------+
|   NEAR_END Generator   |                         |   FAR_END Reflector    |
|                        |        400G Link        |                        |
|  dut:port1 (lag1)      | <=====================> |  dut:port2 (lag2)      |
|  192.0.2.1/30          |  (PBLQ TX / Loopback)   |  192.0.2.2/30          |
|  198.51.100.1/30 (LAG) |                         |  198.51.100.2/30 (LAG) |
|                        |        100G Link        |                        |
|  dut:port3 (lag3)      | <=====================> |  dut:port4 (lag4)      |
|  192.0.2.5/30          |  (PBLQ TX / Loopback)   |  192.0.2.6/30          |
|  198.51.100.5/30 (LAG) |                         |  198.51.100.6/30 (LAG) |
+------------------------+                         +------------------------+
```

*   `dut1:port 1 <--> port 2:dut1` - 400G ports (port 1 and 2 as singleton and
    memberlink)
*   `dut1:port 3 <--> port 4:dut1` - 100G ports (port 3 and 4 as singleton and
    memberlink)

*   Connect the ports as per topology.
    *   Validate the link qualification Capabilities response.
        *   MaxHistoricalResultsPerInterface is >= 2.
        *   Time exists.
        *   Generator:
            *   MinMtu >= 64,
            *   MaxMtu >= 8184,
            *   MaxBps >= 4e11,
            *   MaxPps >= 5e8,
            *   MinSetupDuration > 0
            *   MinTeardownDuration > 0,
            *   MinSampleInterval > 0,
        *   Reflector:
            *   MinSetupDuration > 0
            *   MinTeardownDuration > 0,
*   Validate the error code is returned for Get and Delete requests with
    non-existing ID.
    *   Error code is 5 NOT_FOUND (HTTP Mapping: 404 Not Found).
*   Validate the link qualification List and Delete.
    *   Issue List qualifications request.
    *   Delete the qualification if qualification is found.
    *   Issue List qualifications request again.
    *   Verify that the qualification has been deleted successfully by checking
        List response.
*   Set a port as the NEAR_END (generator) device for Packet Based Link Qual.
    *   Issue gnoi.LinkQualification Create RPC to the device and provide
        following parameters:
        *   Id: A unique identifier for this run of the test
        *   InterfaceName: interface as the interface to be used as generator
            end.
            *   This interface must be connected to the interface chosen on the
                reflector device using 400G connection.
        *   EndpointType: Qualification_end set as NEAR_END with
            PacketGeneratorConfiguration.
    *   Set the following parameters for link qualification service usage:
        *   PacketRate: Packet per second rate to use for this test.
        *   PacketSize: Size of packets to inject. The value is 8184 bytes.
    *   RPCSyncedTiming:
        *   SetupDuration: The requested setup time for the endpoint.
        *   PreSyncDuration: Minimum_wait_before_preparation_seconds. Within
            this period, the device should:
            *   Initialize the link qualification state machine.
            *   Set port’s state to TESTING. This state is only relevant inside
                the linkQual service.
            *   Set the port in loopback mode.
        *   Duration:The length of the qualification.
        *   PostSyncDuration: The amount time a side should wait before starting
            its teardown.
        *   TeardownDuration: The amount time required to bring the interface
            back to pre-test state.
    *       Verify generator interface oper-state is 'TESTING'
   *   Set another port as the FAR_END (reflector) device for Packet Based Link
    Qual.
    *   Issue gnoi.LinkQualification Create RPC to the device and provide
        following parameters:
        *   Id: A unique identifier for this run of the test
        *   InterfaceName: Interface as the interface to be used as a reflector
            to turn the packet back.
        *   EndpointType: Qualification_end set as FAR_END.
        *   RPCSyncedTiming:
            *   Reflector timers should be same as the ones on the generator.
        *   Verify reflector interface oper-state is 'TESTING'
*   Get the result by issuing gnoi.LinkQualification Get RPC to gather the
    result of link qualification. Provide the following parameter:
    *   Id: The identifier used above on the NEAR_END side.
    *   Validate the response to
        *   Ensure that the current_state is QUALIFICATION_STATE_COMPLETED
        *   Ensure that the num_corrupt_packets and num_packets_dropped_by_mmu
            are 0
        *   Ensure that RPC status code is 0 for success.
        *   Packets sent count matches with packets received.

### Scenario 1: Concurrent Multi-Link Qualification on Singleton Interfaces (Dual-Stack IPv4 and IPv6)

This scenario validates that the gNOI `LinkQualification` service can
concurrently qualify all 4 physical links (both 400G and 100G interfaces, either
hairpinned on a single DUT or connected between two peer DUTs) as singleton
Layer 3 interfaces without encountering `DEADLINE_EXCEEDED` gRPC errors during
session creation (`Create`), state monitoring (`List`), result retrieval
(`Get`), or post-qualification cleanup (`Delete`).

1.  **Interface Configuration and Pre-Test Baseline**:
    *   Configure `DUT 1 port 1`, `DUT 1 port 2` (400G interfaces) and `DUT 1
        port 3`, `DUT 1 port 4` (100G interfaces)—or `DUT 1 port 1` through
        `DUT 1 port 4` connected to `DUT 2 port 1` through `DUT 2 port 4` in a
        two-DUT topology—as singleton Ethernet interfaces using `gNMI.Set`:
        *   Set `/interfaces/interface/config/type` to `ethernetCsmacd`,
            `/interfaces/interface/config/enabled` to `true`, and
            `/interfaces/interface/config/mtu` to `9000`.
        *   Configure dual-stack IPv4 and IPv6 addressing on subinterface `0`
            (`/interfaces/interface/subinterfaces/subinterface/index`):
            *   `port1`: IPv4 `192.0.2.1/30` and IPv6 `2001:db8::1/126`
            *   `port2`: IPv4 `192.0.2.2/30` and IPv6 `2001:db8::2/126`
            *   `port3`: IPv4 `192.0.2.5/30` and IPv6 `2001:db8::5/126`
            *   `port4`: IPv4 `192.0.2.6/30` and IPv6 `2001:db8::6/126`
            *   Enable both address families by setting
                `/interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled`
                to `true` and
                `/interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled`
                to `true`.
    *   Verify via `gNMI.Subscribe` that
        `/interfaces/interface/state/admin-status` is `UP` and
        `/interfaces/interface/state/oper-status` is `UP` on all 4 interfaces
        before starting link qualification.
    *   Issue `gnoi.packet_link_qualification.LinkQualification.List` and delete
        any pre-existing qualification records via
        `gnoi.packet_link_qualification.LinkQualification.Delete` so that the
        service starts from a clean baseline with `0` active or historical
        sessions.

2.  **Concurrent Multi-Link Qualification Start (`Create` RPC)**:
    *   Query `gnoi.packet_link_qualification.LinkQualification.Capabilities` to
        determine the minimum setup and teardown durations supported by the
        generator (`PacketGenerator`) and reflector (`AsicLoopback` or
        `PmdLoopback`).
    *   Issue `gnoi.packet_link_qualification.LinkQualification.Create` with an
        explicit RPC deadline (e.g., 60 seconds) to initiate link qualification
        simultaneously across all connected port pairs (`port1 <--> port2` on
        400G and `port3 <--> port4` on 100G, or all 4 `DUT 1 <--> DUT 2` links
        concurrently):
        *   **NEAR_END (Generator) Interfaces**: Configure
            `PacketGeneratorConfiguration` with `packet_rate` set to `138888`
            PPS and `packet_size` set to `8184` bytes, and `RPCSyncedTiming` set
            to `pre_sync_duration: 10s`, `setup_duration: >= 30s`, `duration:
            180s`, `post_sync_duration: 0s`, and `teardown_duration: >= 30s`.
        *   **FAR_END (Reflector) Interfaces**: Configure
            `AsicLoopbackConfiguration` (or `PmdLoopbackConfiguration` based on
            device capabilities) with `RPCSyncedTiming` set to
            `pre_sync_duration: 0s`, `setup_duration: >= 40s`, `duration: 180s`,
            `post_sync_duration: 30s`, and `teardown_duration: >= 10s`.
    *   **Pass/Fail Criteria (`Create`)**: The `Create` RPC must complete before
        the context deadline expires without returning `DEADLINE_EXCEEDED`, and
        `CreateResponse.status` must contain an entry for every requested
        generator and reflector qualification ID with `status.code == 0` (`OK`).

3.  **Concurrent Active State and Telemetry Verification (`List` RPC)**:
    *   While the qualification is active, query
        `gnoi.packet_link_qualification.LinkQualification.List` with an explicit
        RPC deadline.
    *   Verify that all requested qualification IDs are present in
        `ListResponse.results` and transition to `QUALIFICATION_STATE_RUNNING`.
    *   While the qualification state is `QUALIFICATION_STATE_RUNNING`, verify
        via `gNMI.Get` or `gNMI.Subscribe` that
        `/interfaces/interface/state/oper-status` on all participating generator
        and reflector ports reports `TESTING`.

4.  **Concurrent Result Retrieval and Counter Validation (`Get` RPC)**:
    *   Wait for the qualification duration and teardown window to complete, and
        confirm via `LinkQualification.List` that all qualification IDs reach
        `QUALIFICATION_STATE_COMPLETED`.
    *   Issue a batched `gnoi.packet_link_qualification.LinkQualification.Get`
        RPC requesting all qualification IDs simultaneously with an explicit RPC
        deadline.
    *   **Pass/Fail Criteria (`Get`)**:
        *   The `Get` RPC must complete within the context deadline without
            `DEADLINE_EXCEEDED`.
        *   For every qualification ID: `status.code == 0`, `state ==
            QUALIFICATION_STATE_COMPLETED`, `packets_error == 0`, and
            `packets_dropped == 0`.
        *   `packets_sent > 0` and `packets_received > 0` on all qualified
            interfaces, and the discrepancy between generator
            `packets_sent`/`packets_received` and reflector
            `packets_received`/`packets_sent` must not exceed `0.0001%`.

5.  **Cleanup and Interface State Restoration (`Delete` RPC)**:
    *   Issue `gnoi.packet_link_qualification.LinkQualification.Delete`
        containing all qualification IDs with an explicit RPC deadline to clean
        up all completed sessions.
    *   **Pass/Fail Criteria (`Delete` & Recovery)**:
        *   The `Delete` RPC must complete within the context deadline without
            `DEADLINE_EXCEEDED` and return `status.code == 0` for each ID.
        *   A subsequent `LinkQualification.List` call must return `0`
            qualification entries.
        *   Verify via `gNMI.Subscribe` that
            `/interfaces/interface/state/oper-status` on all 4 interfaces
            returns from `TESTING` to `UP`.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Singleton 400G Generator Port 1",
          "enabled": true,
          "mtu": 9000,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
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
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
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
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "Singleton 400G Reflector Port 2",
          "enabled": true,
          "mtu": 9000,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
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
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.2"
                    }
                  ]
                },
                "config": {
                  "enabled": true
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
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "Singleton 100G Generator Port 3",
          "enabled": true,
          "mtu": 9000,
          "name": "port3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port3",
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
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
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
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "Singleton 100G Reflector Port 4",
          "enabled": true,
          "mtu": 9000,
          "name": "port4",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port4",
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
                        "ip": "192.0.2.6",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.6"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8::6",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8::6"
                    }
                  ]
                },
                "config": {
                  "enabled": true
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

### Scenario 2: Concurrent Multi-Link Qualification on Aggregate Member Links (Dual-Stack IPv4 and IPv6)

This scenario validates that physical interfaces configured as member links of
static IEEE 802.3ad Link Aggregation Group (LAG) bundles with dual-stack IPv4
and IPv6 subinterfaces can undergo concurrent Packet-Based Link Qualification
across multiple links without gNOI RPC timeouts, and that both the physical
member ports and parent LAG interfaces recover to `UP` upon test completion and
cleanup.

1.  **Aggregate and Member-Link Configuration**:
    *   Configure static aggregate interfaces (`lag1`, `lag2` for the 400G ports
        and `lag3`, `lag4` for the 100G ports) using `gNMI.Set`:
        *   Set `/interfaces/interface/config/type` to `ieee8023adLag`,
            `/interfaces/interface/config/enabled` to `true`,
            `/interfaces/interface/config/mtu` to `9000`, and
            `/interfaces/interface/aggregation/config/lag-type` to `STATIC`.
        *   Configure dual-stack IPv4 and IPv6 addresses on subinterface `0` of
            each aggregate interface:
            *   `lag1`: IPv4 `198.51.100.1/30` and IPv6 `2001:db8:1::1/126`
            *   `lag2`: IPv4 `198.51.100.2/30` and IPv6 `2001:db8:1::2/126`
            *   `lag3`: IPv4 `198.51.100.5/30` and IPv6 `2001:db8:1::5/126`
            *   `lag4`: IPv4 `198.51.100.6/30` and IPv6 `2001:db8:1::6/126`
            *   Set
                `/interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled`
                and
                `/interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled`
                to `true`.
    *   Bind physical member ports `port1` -> `lag1`, `port2` -> `lag2`, `port3`
        -> `lag3`, and `port4` -> `lag4` by setting
        `/interfaces/interface/ethernet/config/aggregate-id` and
        `/interfaces/interface/config/id` on each physical interface.
    *   Verify via `gNMI.Subscribe` that
        `/interfaces/interface/state/oper-status` is `UP` on all aggregate
        interfaces (`lag1` through `lag4`) and all physical member interfaces
        (`port1` through `port4`), and that
        `/interfaces/interface/ethernet/state/aggregate-id` reflects the
        assigned LAG name.

2.  **Concurrent Member-Link Qualification Execution (`Create`, `List`, `Get`)**:
    *   Issue `gnoi.packet_link_qualification.LinkQualification.Create`
        concurrently across all 4 physical LAG member interfaces (`port1` and
        `port3` as `NEAR_END` `PacketGenerator` endpoints with `packet_rate:
        138888` and `packet_size: 8184`; `port2` and `port4` as `FAR_END`
        `AsicLoopback` or `PmdLoopback` endpoints) with an explicit RPC
        deadline.
    *   Verify that `Create` succeeds without `DEADLINE_EXCEEDED` and returns
        `status.code == 0` for all 4 member-link qualification IDs.
    *   Query `gnoi.packet_link_qualification.LinkQualification.List` during the
        qualification run and verify all 4 member-link qualification IDs are
        active and that `/interfaces/interface/state/oper-status` on `port1`,
        `port2`, `port3`, and `port4` transitions to `TESTING` while `state ==
        QUALIFICATION_STATE_RUNNING`.
    *   Once all 4 member-link qualifications transition to
        `QUALIFICATION_STATE_COMPLETED`, invoke
        `gnoi.packet_link_qualification.LinkQualification.Get` for all 4 IDs
        simultaneously with an explicit RPC deadline.
    *   **Pass/Fail Criteria**:
        *   `Get` completes within the RPC deadline without `DEADLINE_EXCEEDED`.
        *   Every member link reports `status.code == 0`, `state ==
            QUALIFICATION_STATE_COMPLETED`, `packets_error == 0`,
            `packets_dropped == 0`, and generator/reflector packet counts match
            within `0.0001%`.

3.  **Cleanup and LAG Recovery Verification (`Delete` RPC)**:
    *   Delete all 4 member-link qualification records via
        `gnoi.packet_link_qualification.LinkQualification.Delete` with an
        explicit RPC deadline and verify `status.code == 0` for each ID.
    *   Confirm via `LinkQualification.List` that `0` qualification records
        remain on the device.
    *   Verify via `gNMI.Subscribe` that
        `/interfaces/interface/state/oper-status` returns to `UP` on all 4
        physical member interfaces (`port1` through `port4`) and all 4 parent
        aggregate interfaces (`lag1` through `lag4`), and restore baseline
        interface configuration.

#### Canonical OC

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
          "description": "400G Aggregate Interface 1",
          "enabled": true,
          "mtu": 9000,
          "name": "lag1",
          "type": "iana-if-type:ieee8023adLag"
        },
        "name": "lag1",
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
                        "ip": "198.51.100.1",
                        "prefix-length": 30
                      },
                      "ip": "198.51.100.1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:1::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:1::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "aggregation": {
          "config": {
            "lag-type": "STATIC"
          }
        },
        "config": {
          "description": "400G Aggregate Interface 2",
          "enabled": true,
          "mtu": 9000,
          "name": "lag2",
          "type": "iana-if-type:ieee8023adLag"
        },
        "name": "lag2",
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
                        "ip": "198.51.100.2",
                        "prefix-length": 30
                      },
                      "ip": "198.51.100.2"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:1::2",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:1::2"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "LAG Member Port 1",
          "enabled": true,
          "id": 10,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag1"
          }
        },
        "name": "port1"
      },
      {
        "config": {
          "description": "LAG Member Port 2",
          "enabled": true,
          "id": 11,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "lag2"
          }
        },
        "name": "port2"
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/id:
  /interfaces/interface/config/mtu:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/ethernet/state/aggregate-id:
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
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
