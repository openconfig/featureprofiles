# TE-1.25: gNSI Operations During gRIBI/P4RT Programming and gNMI Streaming

## Summary

This test validates control-plane security lifecycle resilience and data-plane
isolation when `gNSI` credential and authorization updates
(`gnsi.certz.v1.Certz.Rotate` and `gnsi.authz.v1.Authz.Rotate`) execute
concurrently with high-rate `gRIBI` Abstract Forwarding Table (AFT) route
programming, `P4Runtime` (`P4RT`) table modifications, and continuous `gNMI`
streaming telemetry. It verifies that active mTLS gRPC sessions remain
uninterrupted without RPC drops or unbounded latency spikes, transit forwarding
and `PacketIn` punting sustain zero packet loss, newly established connections
immediately enforce the rotated certificates and authorization policies, and
unfinalized rotations roll back cleanly under load.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

```text
                             +---------+
  [ ATE port1 ] ------------ |         | ------------ [ ATE port2 ] (Protected Transit)
  (Transit & Punt Src)       |   DUT   |
                             | (mTLS   | ------------ [ ATE port3 ] (Churn Egress 1)
                             |  gNxI)  |
                             |         | ------------ [ ATE port4 ] (Churn Egress 2)
                             +---------+
```

* Connect 4 interfaces between the DUT and ATE and configure dual-stack IPv4 and
  IPv6 addresses in the `DEFAULT` network instance:
  * **Link 1 (Transit & Packet-In Ingress)**: DUT `port1` (`192.0.2.1/30`,
    `2001:db8:1::1/126`, P4RT `id = 1`) <-> ATE `port1` (`192.0.2.2/30`,
    `2001:db8:1::2/126`).
  * **Link 2 (Protected Transit Egress)**: DUT `port2` (`192.0.2.5/30`,
    `2001:db8:2::1/126`, P4RT `id = 2`) <-> ATE `port2` (`192.0.2.6/30`,
    `2001:db8:2::2/126`).
  * **Link 3 (Churn ECMP Egress 1)**: DUT `port3` (`192.0.2.9/30`,
    `2001:db8:3::1/126`, P4RT `id = 3`) <-> ATE `port3` (`192.0.2.10/30`,
    `2001:db8:3::2/126`).
  * **Link 4 (Churn ECMP Egress 2)**: DUT `port4` (`192.0.2.13/30`,
    `2001:db8:4::1/126`, P4RT `id = 4`) <-> ATE `port4` (`192.0.2.14/30`,
    `2001:db8:4::2/126`).
* Configure `/components/component/integrated-circuit/config/node-id` (`1`) on
  the forwarding integrated circuit (`IntegratedCircuit0`) and interface
  `/interfaces/interface/config/id` (`1`–`4`) via `gNMI.Set`.
* Enable mTLS transport security (`transport-security = true`) on the DUT
  `DEFAULT` gRPC server covering `gNMI`, `gNOI`, `gNSI`, `gRIBI`, and `P4RT`:
  * Bootstrap SSL profile `system_default_profile` (`certificate-id =
    "system_default_profile"`) with CA trust bundle `ca-v1`
    (`ca-trust-bundle-version = "ca-v1"`) and server credential `server-cert-v1`
    (`certificate-version = "cert-v1"`), preparing both `RSA` and `ECDSA` key
    variants.
  * Push and finalize baseline gRPC authorization policy `policy-v1` (`version =
    "authz-v1"`, `created_on = 1700000000`) via `gnsi.authz.v1.Authz.Rotate`
    containing:
    * `allow-test-infra`: permits the test infrastructure SPIFFE ID on `/*`.
    * `allow-sdn-controller`: permits
      `spiffe://example.org/ns/sdn/sa/controller` on `/gnmi.gNMI/*`,
      `/gribi.gRIBI/*`, and `/p4.v1.P4Runtime/*`.
    * `allow-sec-auditor`: permits `spiffe://example.org/ns/sec/sa/auditor` on
      `/gnmi.gNMI/Get`, `/gribi.gRIBI/Get`, and `/gnsi.authz.v1.Authz/Get`.
* Establish persistent mTLS client streams authenticated with
  `spiffe://example.org/ns/sdn/sa/controller` (`ca-v1`):
  * **gNMI Streaming Client**: Open two persistent `gNMI.Subscribe` (`STREAM`)
    sessions:
    1. `ON_CHANGE` subscription on `/interfaces/interface/state/oper-status`,
       `/network-instances/network-instance/afts/`,
       `/system/aaa/authorization/state/`, and
       `/system/grpc-servers/grpc-server/state/`.
    2. `SAMPLE` subscription (`1s` sample interval) on
       `/interfaces/interface/state/counters/` and
       `/system/grpc-servers/grpc-server/authz-policy-counters/`.
  * **gRIBI Client**: Connect in `SINGLE_PRIMARY` mode with `PRESERVE`
    persistence and `FIB_ACK` requested (`election_id = {high: 0, low: 1}`), and
    clear initial AFT state via `gRIBI.Flush` on `DEFAULT`.
  * **P4RT Client**: Open `P4Runtime.StreamChannel` (`device_id = 1`,
    `election_id = {high: 0, low: 1}`), complete primary controller arbitration,
    and push the forwarding pipeline via
    `P4Runtime.SetForwardingPipelineConfig` (`VERIFY_AND_COMMIT`).
* Verify all 4 DUT ports report `oper-status = UP` via the `ON_CHANGE`
  `gNMI.Subscribe` stream, start OTG protocols, and explicitly resolve IPv4 ARP
  and IPv6 Neighbor Discovery (ND) across all 4 links before starting traffic.

### TE-1.25.1 - Baseline Protected Forwarding, Telemetry, and Control-Plane Load

* **Step 1 - Program Protected Baseline Entries**:
  * Via `gRIBI.Modify` (`ADD`, `FIB_ACK`), program protected routes in `DEFAULT`
    for `198.51.100.0/24` and `2001:db8:100::/64` pointing to `NextHopGroup 10`
    (`NextHop 1` = `192.0.2.6` and `NextHop 2` = `2001:db8:2::2` on ATE
    `port2`). Confirm installation via `gRIBI.Get` and `gNMI.Subscribe`
    (`ON_CHANGE`).
  * Via `P4Runtime.Write` (`INSERT`), install a punt entry matching LLDP frames
    (`ether_type == 0x88cc`, action `PuntToController`).
* **Step 2 - Start Protected Data-Plane and Packet-In Traffic**:
  * From ATE `port1`, start continuous dual-stack OTG transit flows (`512`-byte
    frames, `5,000 fps` IPv4 to `198.51.100.1` and `5,000 fps` IPv6 to
    `2001:db8:100::1`, `10,000 fps` aggregate) and continuous LLDP frames (`100
    fps`).
  * Verify `100%` of transit packets egress ATE `port2` (`0%` loss) and `100%`
    of LLDP frames arrive as `PacketIn` messages on `P4Runtime.StreamChannel`.
* **Step 3 - Start Continuous High-Rate Control-Plane Churn**:
  * Over the persistent `sdn/sa/controller` mTLS sessions, launch continuous
    background programming loops (kept active across `TE-1.25.2`–`TE-1.25.4`):
    * `gRIBI.Modify` (`ADD` -> `REPLACE` -> `DELETE` cycles with `FIB_ACK`)
      across `10,000` IPv4 `/32` routes (`100.64.0.1/32`–`100.64.39.16/32`) and
      `10,000` IPv6 `/128` routes
      (`2001:db8:1000::1/128`–`2001:db8:1000::2710/128`) referencing ECMP
      `NextHopGroups` across ATE `port3` and ATE `port4`.
    * `P4Runtime.Write` (`INSERT` -> `MODIFY` -> `DELETE` batches of `100`
      entries) across `1,000` ingress ACL entries matching non-transit prefixes
      (`198.18.0.0/16`).
  * Record baseline median and 99th-percentile batch completion latencies for
    `gRIBI.Modify` (`FIB_PROGRAMMED`) and `P4Runtime.Write`.

### TE-1.25.2 - Concurrent mTLS Certificate Rotation (gNSI.certz)

* **Step 1 - Rotate Server Certificate and Trust Bundle Under Churn**:
  * Execute this procedure for both `RSA` and `ECDSA` credential profiles while
    the `gRIBI.Modify` churn loop, `P4Runtime.Write` churn loop,
    `gNMI.Subscribe` streams, and OTG traffic streams remain continuously
    active:
    1. Open a `gnsi.certz.v1.Certz.Rotate` stream on `system_default_profile`
       and send `UploadRequest` containing rotated server key/certificate
       `server-cert-v2` (`certificate-version = "cert-v2"`) and updated CA trust
       bundle `ca-v1-v2` (`ca-trust-bundle-version = "ca-v2"`, adding `ca-v2`
       and subsequently rotating to `ca-v2`-only trust).
    2. After receiving `UploadResponse` and prior to `FinalizeRequest`, open a
       new short-lived mTLS `gNMI.Get` connection using a client certificate
       signed by `ca-v2` (`spiffe://example.org/ns/sec/sa/auditor`):
       * Confirm the TLS handshake succeeds and the DUT presents
         `server-cert-v2` (matching expected Serial Number and SAN).
       * Attempt a separate mTLS connection using a client certificate signed by
         an untrusted CA (`ca-untrusted`) and verify the TLS handshake is
         rejected.
    3. Send `FinalizeRequest` on the `Certz.Rotate` stream to commit the new
       credentials.
* **Step 2 - Pass/Fail Criteria**:
  * Existing long-lived `gNMI.Subscribe`, `gRIBI.Modify`, and
    `P4Runtime.StreamChannel` / `Write` sessions experience `0` gRPC stream
    disconnects, `0` failed operations, `0` missing telemetry sample intervals,
    and no unbounded batch latency spikes or `DEADLINE_EXCEEDED` timeouts
    relative to the `TE-1.25.1` baseline.
  * Verify via the active `gNMI.Subscribe` (`ON_CHANGE`) stream and `gNMI.Get`
    that `/system/grpc-servers/grpc-server/state/certificate-version` updates to
    `"cert-v2"`, that
    `/system/grpc-servers/grpc-server/state/ca-trust-bundle-version` updates to
    `"ca-v2"`, and that `certificate-created-on`, `certificate-expires-on`, and
    `ca-trust-bundle-created-on` reflect the new bundle timestamps.
  * Protected transit traffic (`port1 -> port2`) and LLDP `PacketIn` punting
    sustain `0%` packet loss.

### TE-1.25.3 - Concurrent Authorization Policy Rotation (gNSI.authz)

* **Step 1 - Push Updated gRPC Authorization Policy Under Churn**:
  * While `gRIBI`, `P4RT`, `gNMI.Subscribe`, and OTG traffic remain active:
    1. Open a `gnsi.authz.v1.Authz.Rotate` stream and send `UploadRequest` with
       `policy-v2` (`version = "authz-v2"`, `created_on = 1700000100`) that:
       * Retains `allow-test-infra` and `allow-sdn-controller`.
       * Adds an explicit `deny_rules` entry blocking
         `spiffe://example.org/ns/sec/sa/auditor` from `/gribi.gRIBI/Get`.
       * Adds an `allow_rules` entry permitting new principal
         `spiffe://example.org/ns/ops/sa/viewer` on `/gnmi.gNMI/Get`.
    2. Invoke `gnsi.authz.v1.Authz.Probe` both before and after sending
       `FinalizeRequest` to verify `sec/sa/auditor` evaluates to `ACTION_DENY`
       on `/gribi.gRIBI/Get` and `ops/sa/viewer` evaluates to `ACTION_PERMIT` on
       `/gnmi.gNMI/Get`.
    3. Send `FinalizeRequest` to commit `policy-v2` and verify via
       `gnsi.authz.v1.Authz.Get` that the active policy version is `"authz-v2"`.
    4. Issue live RPCs over newly opened mTLS connections:
       * `sec/sa/auditor` calling `gRIBI.Get` -> expect gRPC
         `PERMISSION_DENIED`.
       * `ops/sa/viewer` calling `gNMI.Get` on `/system/state/current-datetime`
         -> expect gRPC `OK`.
* **Step 2 - Pass/Fail Criteria**:
  * Verify via `gNMI.Subscribe` (`ON_CHANGE`) that
    `/system/aaa/authorization/state/grpc-authz-policy-version` equals
    `"authz-v2"` and
    `/system/aaa/authorization/state/grpc-authz-policy-created-on` equals
    `1700000100`.
  * Verify via `gNMI.Subscribe` (`SAMPLE`) that
    `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-accepts`,
    `access-rejects`, `last-access-accept`, and `last-access-reject` increment
    for `/gribi.gRIBI/Get` and `/gnmi.gNMI/Get`.
  * Active `sdn/sa/controller` `gRIBI.Modify`, `P4Runtime.Write`, and
    `gNMI.Subscribe` streams sustain `0` errors, `0` disconnects, and no
    unbounded batch latency spikes relative to the `TE-1.25.1` baseline, and
    transit / `PacketIn` flows sustain `0%` packet loss.

### TE-1.25.4 - Unfinalized gNSI Rotation Rollback Under Churn

* **Step 1 - Abort Certz and Authz Rotations Without Finalizing**:
  * With `gRIBI`, `P4RT`, `gNMI.Subscribe`, and OTG traffic still running:
    1. Open `gnsi.certz.v1.Certz.Rotate`, upload temporary server certificate
       `server-cert-v3-temp` (`certificate-version = "cert-v3-temp"`), receive
       `UploadResponse`, and immediately close/cancel the stream without sending
       `FinalizeRequest`.
    2. Open `gnsi.authz.v1.Authz.Rotate`, upload temporary policy
       `policy-v3-temp` (`version = "authz-v3-temp"`, `created_on =
       1700000200`), verify temporary evaluation via `Authz.Probe`, and close
       the stream without sending `FinalizeRequest`.
* **Step 2 - Pass/Fail Criteria**:
  * Verify new TLS handshakes present rolled-back `"cert-v2"` and
    `/system/grpc-servers/grpc-server/state/certificate-version` reports
    `"cert-v2"`.
  * Verify `gnsi.authz.v1.Authz.Get` and
    `/system/aaa/authorization/state/grpc-authz-policy-version` report
    `"authz-v2"`.
  * Verify `0` errors on concurrent `gRIBI.Modify`, `P4Runtime.Write`, and
    `gNMI.Subscribe` streams and `0%` packet loss on transit and `PacketIn`
    traffic.

### Cleanup

* Stop background `gRIBI` and `P4RT` churn loops, delete all P4RT entries via
  `P4Runtime.Write` (`DELETE`), and execute `gRIBI.Flush` on `DEFAULT`.
* Stop OTG traffic streams and protocol sessions.
* Rotate server certificates, CA trust bundle, and `gNSI.authz` policy back to
  the baseline pre-test credentials and restore baseline DUT OpenConfig state.

## Canonical OC

```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "IntegratedCircuit0"
        },
        "integrated-circuit": {
          "config": {
            "node-id": "1"
          }
        },
        "name": "IntegratedCircuit0"
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "Transit Ingress Port 1",
          "enabled": true,
          "id": 1,
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
        "config": {
          "description": "Protected Transit Egress Port 2",
          "enabled": true,
          "id": 2,
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
                        "ip": "2001:db8:2::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:2::1"
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
          "description": "Churn ECMP Egress 1 Port 3",
          "enabled": true,
          "id": 3,
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
                        "ip": "192.0.2.9",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.9"
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
                        "ip": "2001:db8:3::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:3::1"
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
          "description": "Churn ECMP Egress 2 Port 4",
          "enabled": true,
          "id": 4,
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
                        "ip": "192.0.2.13",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.13"
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
                        "ip": "2001:db8:4::1",
                        "prefix-length": 126
                      },
                      "ip": "2001:db8:4::1"
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
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "name": "DEFAULT"
      }
    ]
  },
  "system": {
    "grpc-servers": {
      "grpc-server": [
        {
          "config": {
            "certificate-id": "system_default_profile",
            "enable": true,
            "name": "DEFAULT",
            "transport-security": true
          },
          "name": "DEFAULT"
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/id:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address:
  /system/aaa/authorization/state/grpc-authz-policy-created-on:
  /system/aaa/authorization/state/grpc-authz-policy-version:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-accepts:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-rejects:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-accept:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-reject:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/name:
  /system/grpc-servers/grpc-server/config/certificate-id:
  /system/grpc-servers/grpc-server/config/enable:
  /system/grpc-servers/grpc-server/config/name:
  /system/grpc-servers/grpc-server/config/transport-security:
  /system/grpc-servers/grpc-server/state/ca-trust-bundle-created-on:
  /system/grpc-servers/grpc-server/state/ca-trust-bundle-version:
  /system/grpc-servers/grpc-server/state/certificate-created-on:
  /system/grpc-servers/grpc-server/state/certificate-expires-on:
  /system/grpc-servers/grpc-server/state/certificate-version:
  /system/state/current-datetime:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
  gnsi:
    authz.v1.Authz.Get:
    authz.v1.Authz.Probe:
    authz.v1.Authz.Rotate:
    certz.v1.Certz.Rotate:
  gribi:
    gRIBI.Flush:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
