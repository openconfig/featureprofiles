# Authz-5: gRPC Server Per-RPC Statistics YANG State Bounding

## Summary

Verify that DUT gRPC servers (`gRIBI` and `P4RT`) strictly bound per-RPC
authorization telemetry state entries under wildcard and explicit allowlist
`gNSI.authz` policies without creating persistent YANG state entries for
unvalidated or non-existent HTTP/2 `:path` strings, and validate gRPC/P4RT
negative error-path hygiene and AAA session cleanup.

## Testbed type

* [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Test environment setup

* Input arguments:
  * `test_infra_id`: the SPIFFE ID used by test infrastructure clients.
* Configure the DUT so that `gNMI`, `gNOI`, `gNSI`, `gRIBI`, and `P4RT` gRPC
  services are enabled and use mTLS (`transport-security = true`,
  `certificate-id = "server-mtls-cert"`, `listen-addresses = ["ANY"]`) under
  `/system/grpc-servers/grpc-server`:
  * `gribi-server`: `services = ["GRIBI"]`, `port = 9340`, `enable = true`
  * `p4rt-server`: `services = ["P4RT"]`, `port = 9559`, `enable = true`
* Configure P4RT on an integrated-circuit component (`node-id = 100`) using
  `/components/component/integrated-circuit/config/node-id` and configure a
  P4RT port ID (`id = 20`) on a DUT interface using
  `/interfaces/interface/config/id`.
* Generate two mTLS client certificates (SVIDs) signed by the test CA bundle:
  * `cert_conformance_client`: `spiffe://test-abc.foo.bar/xyz/conformance-probe`
  * `cert_denied_client`: `spiffe://test-abc.foo.bar/xyz/deny-all`
* Policy evaluation precedence parameters:
  * `deny_rules` are evaluated first, followed by `allow_rules`, and finally
    default implicit deny (`PERMISSION_DENIED`).
  * At no evaluation stage may an unvalidated, unimplemented, or unauthorized
    HTTP/2 `:path` string create a persistent per-RPC state entry under
    `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc`.

### Authz-5.1 - Baseline Policy Rotation and Registered RPC Telemetry Verification

* Step 1 - Record initial policy and rotate baseline `gNSI.authz` policy: Record
  the initial authorization policy on the DUT using `gNSI.authz.v1.Authz.Get`
  (if present), then push and finalize `policy-grpc-rpc-statistics-bounding`
  (`version = "policy-grpc-rpc-statistics-bounding-v1"`, `created_on = 100`)
  using `gNSI.authz.v1.Authz.Rotate`:

```json
{
  "name": "policy-grpc-rpc-statistics-bounding",
  "allow_rules": [
    {
      "name": "allow-test-infra",
      "source": {
        "principals": [
          "<test_infra_id>"
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
          "spiffe://test-abc.foo.bar/xyz/conformance-probe"
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
          "spiffe://test-abc.foo.bar/xyz/conformance-probe"
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
          "spiffe://test-abc.foo.bar/xyz/deny-all"
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

* Step 2 - Establish baseline `gRIBI` and `P4RT` sessions: Using
  `cert_conformance_client`, invoke `gribi.gRIBI.Get` on `gribi-server` (port
  `9340`), establish `P4Runtime.StreamChannel` mastership (`device_id = 100`,
  `election_id = 100`) on `p4rt-server` (port `9559`), push the WBB `P4Info`
  pipeline via `P4Runtime.SetForwardingPipelineConfig`, and invoke
  `P4Runtime.Capabilities` and `P4Runtime.Read`.
* Step 3 - Validation with pass/fail criteria: Validate policy activation and
  baseline per-RPC counters using `gNMI.Subscribe` (`ON_CHANGE` / `SAMPLE`) and
  `gNMI.Get`:
  * Verify `/system/aaa/authorization/state/grpc-authz-policy-version` ==
    `"policy-grpc-rpc-statistics-bounding-v1"`
  * Verify `/system/aaa/authorization/state/grpc-authz-policy-created-on` ==
    `100`
  * Verify `/system/grpc-servers/grpc-server[name=gribi-server]/state/enable` ==
    `true` and `/system/grpc-servers/grpc-server[name=gribi-server]/state/port`
    == `9340`
  * Verify `/system/grpc-servers/grpc-server[name=p4rt-server]/state/enable` ==
    `true` and `/system/grpc-servers/grpc-server[name=p4rt-server]/state/port`
    == `9559`
  * Verify `/system/grpc-servers/grpc-server[name=gribi-server]/authz-policy-counters/rpcs/rpc[name=/gribi.gRIBI/Get]/state/access-accepts` >= `1`
  * Verify `/system/grpc-servers/grpc-server[name=p4rt-server]/authz-policy-counters/rpcs/rpc[name=/p4.v1.P4Runtime/Read]/state/access-accepts` >= `1`
  * Record the baseline set of registered RPC names under
    `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/name`.

### Authz-5.2 - Per-RPC State Bounding on Wildcard Policy (gRIBI UNIMPLEMENTED Path)

* Step 1 - Invoke unimplemented RPC paths matching wildcard allow rule: Using
  `cert_conformance_client`, send `10` unary gRPC calls to `gribi-server` (port
  `9340`) using distinct unimplemented HTTP/2 `:path` strings
  (`/gribi.gRIBI/NonExistentMethod_01` through
  `/gribi.gRIBI/NonExistentMethod_10`).
* Step 2 - Validation with pass/fail criteria: Validate client status codes and
  streamed per-RPC telemetry state via `gNMI.Subscribe` and `gNMI.Get`:
  * Verify all `10` calls fail with gRPC status code `UNIMPLEMENTED` (`12`).
  * Verify
    `/system/grpc-servers/grpc-server[name=gribi-server]/authz-policy-counters/rpcs/rpc/state/name`
    contains no `/gribi.gRIBI/NonExistentMethod_*` entries and remains strictly
    bounded to registered `gRIBI` RPC methods (or a single fixed aggregate
    unknown-path bucket).
  * Verify
    `/system/grpc-servers/grpc-server[name=gribi-server]/authz-policy-counters/rpcs/rpc/state/access-accepts`
    and
    `/system/grpc-servers/grpc-server[name=gribi-server]/authz-policy-counters/rpcs/rpc/state/last-access-accept`
    update only on registered or aggregate entries.

### Authz-5.3 - Per-RPC State Bounding on Explicit Allowlist Policy (P4RT PERMISSION_DENIED Path)

* Step 1 - Invoke unregistered methods and unknown services on explicit
  allowlist policy: Using `cert_conformance_client`, send `10` unary gRPC calls
  to `p4rt-server` (port `9559`) for unregistered methods on the known service
  (`/p4.v1.P4Runtime/NonExistentMethod_01` through
  `/p4.v1.P4Runtime/NonExistentMethod_10`) and `10` unary gRPC calls for unknown
  services (`/p4.v1.P4RuntimeConformanceProbe_01/Read` through
  `/p4.v1.P4RuntimeConformanceProbe_10/Read`).
* Step 2 - Validation with pass/fail criteria: Validate client status codes and
  streamed per-RPC telemetry state via `gNMI.Subscribe` and `gNMI.Get`:
  * Verify all `20` calls fail with gRPC status code `PERMISSION_DENIED` (`7`).
  * Verify
    `/system/grpc-servers/grpc-server[name=p4rt-server]/authz-policy-counters/rpcs/rpc/state/name`
    contains no `/p4.v1.P4Runtime/NonExistentMethod_*` or
    `/p4.v1.P4RuntimeConformanceProbe_*/Read` entries.
  * Verify
    `/system/grpc-servers/grpc-server[name=p4rt-server]/authz-policy-counters/rpcs/rpc/state/access-rejects`
    and
    `/system/grpc-servers/grpc-server[name=p4rt-server]/authz-policy-counters/rpcs/rpc/state/last-access-reject`
    update only on registered or aggregate entries.

### Authz-5.4 - Negative Test Cases

Ensure that explicit deny rules, unsupported compression encodings, malformed
`P4RT` requests, and unfinalized policy rotations are properly rejected without
leaking AAA sessions or corrupting DUT state.

* **Negative Test 1 (Explicit Deny Rule Precedence):** Using
  `cert_denied_client` (`spiffe://test-abc.foo.bar/xyz/deny-all`), invoke
  registered RPCs `/gribi.gRIBI/Get` and `/p4.v1.P4Runtime/Read`. Verify both
  calls fail with `PERMISSION_DENIED` (`7`) and increment
  `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-rejects`
  on their respective registered RPC entries.
* **Negative Test 2 (Unsupported gRPC Encoding and Error-Path AAA Session
  Cleanup):** Using `cert_conformance_client`, send `150` sequential
  `gribi.gRIBI.Get` requests across new gRPC connections with an unsupported
  compression header (`grpc-encoding: invalid-codec`) to exceed the `128`
  concurrent AAA session threshold. Verify each request is rejected with
  `UNIMPLEMENTED` (`12`) rather than `OK`, and verify that a subsequent valid
  `gribi.gRIBI.Get` and `P4Runtime.Capabilities` request on the 151st connection
  succeeds with `OK` (`0`) rather than failing with `RESOURCE_EXHAUSTED` (`8`)
  due to leaked AAA sessions.
* **Negative Test 3 (Malformed P4RT StreamChannel and Invalid Read Entity):**
  Using `cert_conformance_client`, send a `P4Runtime.StreamChannel`
  `StreamMessageRequest` with an empty `update` oneof field and a
  `P4Runtime.Read` request referencing a non-existent `table_id = 0xFFFFFFFF`.
  Verify both requests are rejected with `INVALID_ARGUMENT` (`3`) (or
  `NOT_FOUND` (`5`) for the unknown table ID) rather than `OK`.
* **Negative Test 4 (Unfinalized gNSI Authz Rotate Rollback):** Open a
  `gNSI.authz.v1.Authz.Rotate` stream, send an `UploadRequest` with `version =
  "policy-unfinalized-rollback-v2"`, and close the stream without sending
  `FinalizeRequest`. Verify via `gNSI.authz.v1.Authz.Get` and `gNMI.Get` on
  `/system/aaa/authorization/state/grpc-authz-policy-version` that the active
  policy rolls back to `"policy-grpc-rpc-statistics-bounding-v1"`.

### Authz-5.5 - Teardown and Cleanup Verification

* Step 1 - Close test streams, restore initial authorization policy, and revert
  configurations: Close the `P4Runtime.StreamChannel` session, restore the
  initial `gNSI.authz` policy recorded during setup using
  `gNSI.authz.v1.Authz.Rotate`, and revert the P4RT `node-id`, interface `id`,
  and `grpc-servers` configurations to their pre-test states.
* Step 2 - Validation with pass/fail criteria:
  * Verify via `gNSI.authz.v1.Authz.Get` and `gNMI.Get` on
    `/system/aaa/authorization/state/grpc-authz-policy-version` that the
    pre-test authorization policy version and content are restored, and that the
    DUT is restored to its baseline operational state.

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
            "node-id": "100"
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
          "id": 20,
          "name": "port-1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port-1"
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
              "ANY"
            ],
            "name": "gribi-server",
            "port": 9340,
            "services": [
              "openconfig-system-grpc:GRIBI"
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
              "ANY"
            ],
            "name": "p4rt-server",
            "port": 9559,
            "services": [
              "openconfig-system-grpc:P4RT"
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
  /system/grpc-servers/grpc-server/config/name:
  /system/grpc-servers/grpc-server/config/enable:
  /system/grpc-servers/grpc-server/config/port:
  /system/grpc-servers/grpc-server/config/services:
  /system/grpc-servers/grpc-server/config/transport-security:
  /system/grpc-servers/grpc-server/config/certificate-id:
  /system/grpc-servers/grpc-server/config/listen-addresses:

  ## State paths
  /system/aaa/authorization/state/grpc-authz-policy-version:
  /system/aaa/authorization/state/grpc-authz-policy-created-on:
  /system/grpc-servers/grpc-server/state/name:
  /system/grpc-servers/grpc-server/state/enable:
  /system/grpc-servers/grpc-server/state/port:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/name:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-accepts:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-rejects:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-accept:
  /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-reject:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      replace: true
    gNMI.Subscribe:
      on_change: true
  gnsi:
    authz.v1.Authz.Rotate:
    authz.v1.Authz.Get:
    authz.v1.Authz.Probe:
  gribi:
    gRIBI.Get:
  # P4Runtime RPCs (github.com/p4lang/p4runtime; commented out as validate_readme_spec only clones github.com/openconfig/<api>.git):
  # p4runtime:
  #   p4.v1.P4Runtime.StreamChannel:
  #   p4.v1.P4Runtime.SetForwardingPipelineConfig:
  #   p4.v1.P4Runtime.GetForwardingPipelineConfig:
  #   p4.v1.P4Runtime.Capabilities:
  #   p4.v1.P4Runtime.Write:
  #   p4.v1.P4Runtime.Read:
```

## Required DUT platform

* FFF
