# Authz-5: gRPC Server Per-RPC Statistics YANG State Bounding

## Summary

Verify gRPC server per-RPC telemetry state bounding under wildcard and explicit
allowlist `gNSI.authz` policies, cross-API hardware resource contention
(`RESOURCE_EXHAUSTED`) between `P4RT` and `gRIBI`, and gRPC error-path hygiene.

## Testbed type

* [`featureprofiles/topologies/dut_ate_4links.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut_ate_4links.testbed)

## Procedure

### Test environment setup

* Connect DUT `port-1` to ATE `port-1`, DUT `port-2` to ATE `port-2`, DUT
  `port-3` to ATE `port-3`, and DUT `port-4` to ATE `port-4`.
* Configure DUT loopback `lo0` (subinterface `0`: `192.0.2.1/32`,
  `2001:db8::1/128`) and physical interfaces `port-1` through `port-4` in the
  `DEFAULT` network-instance with dual-stack IPv4/IPv6 addresses and P4RT SDN
  port IDs (`/interfaces/interface/config/id`), and resolve ARP/ND across all
  DUT-ATE links:
  * `port-1` (`id: 101`): DUT `192.0.2.5/30`, `2001:db8:0:1::1/126`; ATE
    `192.0.2.6/30`, `2001:db8:0:1::2/126`
  * `port-2` (`id: 102`): DUT `192.0.2.9/30`, `2001:db8:0:2::1/126`; ATE
    `192.0.2.10/30`, `2001:db8:0:2::2/126`
  * `port-3` (`id: 103`): DUT `192.0.2.13/30`, `2001:db8:0:3::1/126`; ATE
    `192.0.2.14/30`, `2001:db8:0:3::2/126`
  * `port-4` (`id: 104`): DUT `192.0.2.17/30`, `2001:db8:0:4::1/126`; ATE
    `192.0.2.18/30`, `2001:db8:0:4::2/126`
* Configure the DUT integrated-circuit component `ic0` with `node-id = 1` using
  `/components/component/integrated-circuit/config/node-id`.
* Prepare three mTLS client certificates with SPIFFE IDs:
  * `cert_test_infra`: `spiffe://example.org/test-infra/admin`
  * `cert_conformance_client`: `spiffe://example.org/client/conformance-probe`
  * `cert_denied_client`: `spiffe://example.org/client/denied-probe`
* Policy evaluation and hardware resource precedence parameters:
  * **gNSI Authorization Policy Precedence**: `deny_rules` are evaluated first,
    followed by `allow_rules`, and finally default implicit deny
    (`PERMISSION_DENIED`). Unvalidated or non-existent HTTP/2 `:path` strings
    must never create persistent per-RPC state entries at any evaluation stage.
  * **Forwarding and Hardware Resource Precedence**: Forwarding lookup follows
    `P4RT` ingress ACL redirect -> `gRIBI` AFT entry. Shared hardware
    lookup/TCAM resources are allocated first-come, first-served across `P4RT`
    and `gRIBI`; at `100%` utilization, existing entries remain active and
    unaffected while new writes from either API fail atomically with
    `RESOURCE_EXHAUSTED`.

### Authz-5.1 - Baseline Policy Rotation, gRIBI/P4RT Programming, and Telemetry Verification

* Step 1 - Generate and push DUT configuration: Configure three mTLS-enabled
  gRPC servers (`DEFAULT` on port `9339`, `gribi-server` on port `9340`, and
  `p4rt-server` on port `9559`) listening on `192.0.2.1` and `2001:db8::1` in
  the `DEFAULT` network-instance using `gNMI.Set` (`REPLACE`).
* Step 2 - Rotate baseline `gNSI.authz` policy: Record the initial authorization
  policy via `gNSI.authz.v1.Authz.Get`, then push and finalize
  `policy-grpc-rpc-statistics-bounding` (`version =
  "policy-grpc-rpc-statistics-bounding-v1"`, `created_on = 1700000000000000000`)
  using `gNSI.authz.v1.Authz.Rotate`:

```json
{
  "name": "policy-grpc-rpc-statistics-bounding",
  "allow_rules": [
    {
      "name": "allow-test-infra",
      "source": {
        "principals": [
          "spiffe://example.org/test-infra/admin"
        ]
      },
      "request": {
        "paths": [
          "/*"
        ]
      }
    },
    {
      "name": "allow-gribi-wildcard",
      "source": {
        "principals": [
          "spiffe://example.org/client/conformance-probe"
        ]
      },
      "request": {
        "paths": [
          "/gribi.gRIBI/*"
        ]
      }
    },
    {
      "name": "allow-p4rt-explicit-methods",
      "source": {
        "principals": [
          "spiffe://example.org/client/conformance-probe"
        ]
      },
      "request": {
        "paths": [
          "/p4.v1.P4Runtime/Write",
          "/p4.v1.P4Runtime/Read",
          "/p4.v1.P4Runtime/SetForwardingPipelineConfig",
          "/p4.v1.P4Runtime/GetForwardingPipelineConfig",
          "/p4.v1.P4Runtime/StreamChannel",
          "/p4.v1.P4Runtime/Capabilities"
        ]
      }
    }
  ],
  "deny_rules": [
    {
      "name": "deny-explicit-prober",
      "source": {
        "principals": [
          "spiffe://example.org/client/denied-probe"
        ]
      },
      "request": {
        "paths": [
          "/*"
        ]
      }
    }
  ]
}
```

* Step 3 - Program baseline `gRIBI` and `P4RT` entries:
  * Using `cert_conformance_client`, program `64` IPv4 prefixes
    (`198.51.100.1/32` to `198.51.100.64/32`) and `64` IPv6 prefixes
    (`2001:db8:1000::/64` to `2001:db8:103f::/64`) via `gribi.gRIBI.Modify` with
    ECMP next-hop groups resolving across ATE `port-2`, `port-3`, and `port-4`.
  * Using `cert_conformance_client`, establish `P4Runtime.StreamChannel`
    mastership (`device_id = 1`), push the pipeline via
    `P4Runtime.SetForwardingPipelineConfig`, and program `64` ACL entries via
    `P4Runtime.Write` matching ingress `port-1` (`id = 101`), DSCP `46`, and
    destinations `198.51.100.129/32` to `198.51.100.192/32` with redirect action
    to egress `port-2` (`id = 102`).
* Step 4 - Validation with pass/fail criteria: Validate policy activation, AFT
  programming, and baseline telemetry using `gNMI.Subscribe` (`ON_CHANGE`) and
  `gNMI.Get`:
  * Verify `/system/aaa/authorization/state/grpc-authz-policy-version` ==
    `"policy-grpc-rpc-statistics-bounding-v1"`
  * Verify `/system/aaa/authorization/state/grpc-authz-policy-created-on` ==
    `1700000000000000000`
  * Verify
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry/state/origin-protocol`
    == `GRIBI` for all `64` IPv4 prefixes
  * Verify
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry/state/origin-protocol`
    == `GRIBI` for all `64` IPv6 prefixes
  * Verify
    `/components/component[name=ic0]/integrated-circuit/pipeline-counters/packet/lookup-block/state/acl-memory-used-entries`
    >= `64`
  * Record the initial registered RPC list from
    `/system/grpc-servers/grpc-server[name=gribi-server]/authz-policy-counters/rpcs/rpc/state/name`
    and
    `/system/grpc-servers/grpc-server[name=p4rt-server]/authz-policy-counters/rpcs/rpc/state/name`.
* Step 5 - Validate baseline data-plane forwarding: Start continuous dual-stack
  traffic from ATE `port-1` matching the `gRIBI` prefixes (DSCP `10`) and `P4RT`
  ACL entries (DSCP `46`). Verify 0% packet loss, ECMP distribution (`33.3% ±
  5%`) across ATE `port-2`, `port-3`, and `port-4` for `gRIBI` flows, and 100%
  delivery to ATE `port-2` for the `P4RT` flow.

### Authz-5.2 - Per-RPC State Bounding on Wildcard Policy (gRIBI UNIMPLEMENTED Path)

* Step 1 - Invoke unimplemented RPC paths on `gribi-server`: Using
  `cert_conformance_client`, send `500` unary gRPC calls over IPv4
  (`192.0.2.1:9340`) and `500` unary gRPC calls over IPv6
  (`[2001:db8::1]:9340`) with distinct unimplemented HTTP/2 `:path` values
  (`/gribi.gRIBI/NonExistentMethod_0001` through
  `/gribi.gRIBI/NonExistentMethod_1000`).
* Step 2 - Validation with pass/fail criteria: Validate client status codes and
  `gribi-server` per-RPC telemetry state via `gNMI.Get`:
  * Verify all `1,000` calls fail with gRPC status code `UNIMPLEMENTED` (`12`).
  * Verify
    `/system/grpc-servers/grpc-server[name=gribi-server]/authz-policy-counters/rpcs/rpc/state/name`
    contains no `/gribi.gRIBI/NonExistentMethod_*` entries and remains strictly
    bounded to registered `gRIBI` methods (`Get`, `Modify`, `Flush`, or a single
    fixed aggregate unknown-path bucket).
  * Verify
    `/system/grpc-servers/grpc-server[name=gribi-server]/authz-policy-counters/rpcs/rpc/state/access-accepts`
    and
    `/system/grpc-servers/grpc-server[name=gribi-server]/authz-policy-counters/rpcs/rpc/state/last-access-accept`
    update only on registered or aggregate entries.

### Authz-5.3 - Per-RPC State Bounding on Explicit Allowlist Policy (P4RT PERMISSION_DENIED Path)

* Step 1 - Invoke unregistered methods and unknown services on `p4rt-server`:
  Using `cert_conformance_client`, send `500` unary gRPC calls over IPv4 and
  IPv6 (`192.0.2.1:9559` and `[2001:db8::1]:9559`) for unregistered methods on a
  known service (`/p4.v1.P4Runtime/NonExistentMethod_0001` to `0500`) and `500`
  calls for unknown services (`/p4.v1.P4RuntimeConformanceProbe_0001/Read` to
  `0500/Read`).
* Step 2 - Validation with pass/fail criteria: Validate client status codes and
  `p4rt-server` per-RPC telemetry state via `gNMI.Get`:
  * Verify all `1,000` calls fail with gRPC status code `PERMISSION_DENIED`
    (`7`).
  * Verify
    `/system/grpc-servers/grpc-server[name=p4rt-server]/authz-policy-counters/rpcs/rpc/state/name`
    contains no `/p4.v1.P4Runtime/NonExistentMethod_*` or
    `/p4.v1.P4RuntimeConformanceProbe_*/Read` entries.
  * Verify
    `/system/grpc-servers/grpc-server[name=p4rt-server]/authz-policy-counters/rpcs/rpc/state/access-rejects`
    and
    `/system/grpc-servers/grpc-server[name=p4rt-server]/authz-policy-counters/rpcs/rpc/state/last-access-reject`
    update only on registered or aggregate entries, and the active
    `P4Runtime.StreamChannel` session remains uninterrupted.

### Authz-5.4 - P4RT Hardware Resource Exhaustion and gRIBI RESOURCE_EXHAUSTED Handling

* Step 1 - Exhaust shared hardware table via `P4RT`: Program scale `P4RT` ACL
  entries (`10.64.0.1/32` upward and `2001:db8:4000::1/128` upward) in batches
  via `P4Runtime.Write` (`INSERT`) until the integrated-circuit lookup/ACL table
  reaches capacity.
* Step 2 - Attempt `gRIBI` route installation under exhaustion: Send a
  `gribi.gRIBI.Modify` request programming `10` new IPv4 (`198.18.255.241/32` to
  `198.18.255.250/32`) and `10` new IPv6 (`2001:db8:ffff:ffff::1/128` to
  `2001:db8:ffff:ffff::a/128`) entries.
* Step 3 - Validation with pass/fail criteria: Validate hardware telemetry,
  error reporting, and forwarding isolation:
  * Verify via `gNMI.Subscribe` that
    `/components/component[name=ic0]/integrated-circuit/pipeline-counters/packet/lookup-block/state/lookup-utilization`
    == `100` (or
    `/components/component[name=ic0]/integrated-circuit/pipeline-counters/packet/lookup-block/state/acl-memory-used-entries`
    ==
    `/components/component[name=ic0]/integrated-circuit/pipeline-counters/packet/lookup-block/state/acl-memory-total-entries`).
  * Verify `gribi.gRIBI.Modify` fails with `RESOURCE_EXHAUSTED` and
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry/state/prefix`
    and
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry/state/prefix`
    do not contain the rejected prefixes.
  * Send probe traffic from ATE `port-1` to `198.18.255.241` and
    `2001:db8:ffff:ffff::1`: verify probe packets are dropped and increment
    `/components/component[name=ic0]/integrated-circuit/pipeline-counters/drop/lookup-block/state/no-route`,
    while existing baseline flows continue without packet loss.

### Authz-5.5 - gRIBI Hardware Resource Exhaustion and P4RT RESOURCE_EXHAUSTED Handling

* Step 1 - Transition exhaustion from `P4RT` to `gRIBI`: Delete the scale `P4RT`
  entries from `Authz-5.4` via `P4Runtime.Write` (`DELETE`), confirm lookup
  utilization returns to baseline via `gNMI.Subscribe`, and program scale
  `gRIBI` IPv4 (`203.0.113.0/32` upward) and IPv6 (`2001:db8:2000::/64` upward)
  entries via `gribi.gRIBI.Modify` until shared lookup memory saturates.
* Step 2 - Attempt `P4RT` entry installation under exhaustion: Send a
  `P4Runtime.Write` (`INSERT`) request for `10` new ACL entries matching
  `198.18.255.251/32` to `198.18.255.260/32`.
* Step 3 - Validation with pass/fail criteria: Validate hardware telemetry,
  error reporting, and forwarding isolation:
  * Verify via `gNMI.Subscribe` that
    `/components/component[name=ic0]/integrated-circuit/pipeline-counters/packet/lookup-block/state/lookup-memory-used`
    reflects table saturation.
  * Verify `P4Runtime.Write` fails with `RESOURCE_EXHAUSTED` (`8`) and
    `P4Runtime.Read` returns none of the `10` rejected entries.
  * Send probe traffic from ATE `port-1` to `198.18.255.251` (DSCP `46`): verify
    packets are not redirected to ATE `port-2`, while existing baseline flows
    continue without packet loss.

### Authz-5.6 - Negative Test Cases

Ensure that explicit deny rules, unsupported encodings, malformed payloads, and
unfinalized policy rotations are properly rejected by the DUT.

* **Negative Test 1 (Explicit Deny Rule Precedence):** Using
  `cert_denied_client` (`spiffe://example.org/client/denied-probe`), invoke
  registered RPCs `/gribi.gRIBI/Get` and `/p4.v1.P4Runtime/Read`. Verify both
  calls fail with `PERMISSION_DENIED` (`7`) and increment
  `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-rejects`
  on their respective registered RPC entries.
* **Negative Test 2 (Unsupported gRPC Compression Encoding):** Using
  `cert_conformance_client`, send `256` sequential gRPC requests across new
  connections with header `grpc-encoding: invalid-codec` to `gribi-server` and
  `p4rt-server`. Verify each request fails with `UNIMPLEMENTED` (`12`) and that
  a valid `gribi.gRIBI.Get` and `P4Runtime.Capabilities` request on the 257th
  connection succeeds with `OK` (`0`) without `RESOURCE_EXHAUSTED` session
  leaks.
* **Negative Test 3 (Malformed P4RT StreamChannel and Invalid Read Entity):**
  Send a `P4Runtime.StreamChannel` `StreamMessageRequest` with an empty `update`
  field and a `P4Runtime.Read` request referencing non-existent `table_id =
  0xFFFFFFFF`. Verify both requests are rejected with `INVALID_ARGUMENT` (`3`)
  (or `NOT_FOUND` (`5`) for the unknown table ID) rather than `OK`.
* **Negative Test 4 (Unfinalized gNSI Authz Rotate Rollback):** Open a
  `gNSI.authz.v1.Authz.Rotate` stream using `cert_test_infra`, upload a policy
  with `version = "policy-unfinalized-rollback-v2"`, and close the stream
  without sending `FinalizeRequest`. Verify via `gNMI.Subscribe` that
  `/system/aaa/authorization/state/grpc-authz-policy-version` remains
  `"policy-grpc-rpc-statistics-bounding-v1"`.

### Authz-5.7 - Teardown and Cleanup Verification

* Step 1 - Delete test entries: Delete the scale `gRIBI` entries from
  `Authz-5.5` via `gribi.gRIBI.Modify` (`DELETE`), flush remaining `gRIBI`
  entries via `gribi.gRIBI.Flush`, and delete all `P4RT` entries via
  `P4Runtime.Write` (`DELETE`).
* Step 2 - Restore initial authorization policy: Push and finalize the initial
  `gNSI.authz` policy recorded during setup using `gNSI.authz.v1.Authz.Rotate`.
* Step 3 - Validation with pass/fail criteria:
  * Verify via `gNMI.Subscribe` that
    `/components/component[name=ic0]/integrated-circuit/pipeline-counters/packet/lookup-block/state/lookup-utilization`
    returns to its initial pre-test value.
  * Verify via `gNMI.Get` that
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv4-unicast/ipv4-entry/state/prefix`
    and
    `/network-instances/network-instance[name=DEFAULT]/afts/ipv6-unicast/ipv6-entry/state/prefix`
    contain no remaining `GRIBI` test prefixes.

## Canonical OC

```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "ic0"
        },
        "integrated-circuit": {
          "config": {
            "node-id": "1"
          }
        },
        "name": "ic0"
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "enabled": true,
          "name": "lo0",
          "type": "iana-if-type:softwareLoopback"
        },
        "name": "lo0",
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
                        "prefix-length": 32
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
                        "prefix-length": 128
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
          "id": 101,
          "name": "port-1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port-1"
      },
      {
        "config": {
          "enabled": true,
          "id": 102,
          "name": "port-2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port-2"
      },
      {
        "config": {
          "enabled": true,
          "id": 103,
          "name": "port-3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port-3"
      },
      {
        "config": {
          "enabled": true,
          "id": 104,
          "name": "port-4",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port-4"
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
            "certificate-id": "server-mtls-cert",
            "enable": true,
            "listen-addresses": [
              "192.0.2.1",
              "2001:db8::1"
            ],
            "name": "DEFAULT",
            "network-instance": "DEFAULT",
            "port": 9339,
            "services": [
              "openconfig-system-grpc:GNMI",
              "openconfig-gnsi:GNSI"
            ],
            "transport-security": true
          },
          "name": "DEFAULT"
        },
        {
          "config": {
            "certificate-id": "server-mtls-cert",
            "enable": true,
            "listen-addresses": [
              "192.0.2.1",
              "2001:db8::1"
            ],
            "name": "gribi-server",
            "network-instance": "DEFAULT",
            "port": 9340,
            "services": [
              "openconfig-gribi:GRIBI"
            ],
            "transport-security": true
          },
          "name": "gribi-server"
        },
        {
          "config": {
            "certificate-id": "server-mtls-cert",
            "enable": true,
            "listen-addresses": [
              "192.0.2.1",
              "2001:db8::1"
            ],
            "name": "p4rt-server",
            "network-instance": "DEFAULT",
            "port": 9559,
            "services": [
              "openconfig-p4rt:P4RT"
            ],
            "transport-security": true
          },
          "name": "p4rt-server"
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/id:
  /system/grpc-servers/grpc-server/config/enable:
  /system/grpc-servers/grpc-server/config/port:
  /system/grpc-servers/grpc-server/config/services:
  /system/grpc-servers/grpc-server/config/listen-addresses:
  /system/grpc-servers/grpc-server/config/network-instance:
  /system/grpc-servers/grpc-server/config/transport-security:
  /system/grpc-servers/grpc-server/config/certificate-id:

  ## State paths
  /system/aaa/authorization/state/grpc-authz-policy-version:
  /system/aaa/authorization/state/grpc-authz-policy-created-on:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/name:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-accepts:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-rejects:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-accept:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-reject:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-protocol:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/prefix:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-protocol:
  /components/component/integrated-circuit/pipeline-counters/packet/lookup-block/state/lookup-utilization:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/pipeline-counters/packet/lookup-block/state/lookup-memory-used:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/pipeline-counters/packet/lookup-block/state/acl-memory-total-entries:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/pipeline-counters/packet/lookup-block/state/acl-memory-used-entries:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/no-route:
    platform_type: ["INTEGRATED_CIRCUIT"]

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      replace: true
      union_replace: true
    gNMI.Subscribe:
      on_change: true
  gnsi:
    authz.v1.Authz.Rotate:
    authz.v1.Authz.Get:
    authz.v1.Authz.Probe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
  p4rt:
    P4Runtime.Write:
    P4Runtime.Read:
    P4Runtime.SetForwardingPipelineConfig:
    P4Runtime.GetForwardingPipelineConfig:
    P4Runtime.StreamChannel:
    P4Runtime.Capabilities:
```

## Required DUT platform

* FFF
