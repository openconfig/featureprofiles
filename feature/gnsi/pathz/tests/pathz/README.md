# Pathz: Path-level Authorization (1-4) tests

## Summary

Test gNSI Pathz API behaviors and path-level authorization enforcement.

## Testbed type

* [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Baseline Setup

#### DUT service setup

Configure the DUT to enable the following services (that are using gRPC) and
use mTLS for authentication:

* gNMI
* gNSI

The support of SPIFFE-ID should NOT require explicitly pre-configured local
users in the DUT config.

#### Client certs

Prepare the following client certs with the specified SPIFFE ID:

* `gnmi_admin` with `spiffe://test-realm.foo.bar/role/admin`
* `gnmi_reader` with `spiffe://test-realm.foo.bar/role/reader`
* `gnmi_unauthorized` with `spiffe://test-realm.foo.bar/role/unauthorized`

#### Pathz Authorization Policy

The policy used for enforcement tests is defined below.

```json
{
  "rules": [
    {
      "id": "allow-reader-read-system",
      "user": "spiffe://test-realm.foo.bar/role/reader",
      "path": {
        "elem": [
          {"name": "system"}
        ]
      },
      "action": "ACTION_PERMIT",
      "mode": "MODE_READ"
    },
    {
      "id": "deny-reader-write-system",
      "user": "spiffe://test-realm.foo.bar/role/reader",
      "path": {
        "elem": [
          {"name": "system"}
        ]
      },
      "action": "ACTION_DENY",
      "mode": "MODE_WRITE"
    },
    {
      "id": "allow-admin-write-interfaces",
      "group": "admin-group",
      "path": {
        "elem": [
          {"name": "interfaces"},
          {"name": "interface"}
        ]
      },
      "action": "ACTION_PERMIT",
      "mode": "MODE_WRITE"
    },
    {
      "id": "deny-admin-write-port1",
      "user": "spiffe://test-realm.foo.bar/role/admin",
      "path": {
        "elem": [
          {"name": "interfaces"},
          {"name": "interface", "key": {"name": "<DUT_PORT1>"}}
        ]
      },
      "action": "ACTION_DENY",
      "mode": "MODE_WRITE"
    }
  ],
  "groups": [
    {
      "name": "admin-group",
      "users": [
        {"name": "spiffe://test-realm.foo.bar/role/admin"}
      ]
    }
  ]
}
```

### Pathz-1: Policy Rotation and Freshness Verification

This test verifies the rotation mechanism of the Pathz policy and the
correctness of the telemetry reporting the policy status.

#### Procedure:

1.  **Initial Push**:
    *   Use `gNSI.Pathz.Rotate` to push the baseline Pathz policy.
    *   Set `version` to `v1` and `created_on` to a valid timestamp (e.g., `100`).
    *   Do **not** finalize yet.
2.  **Telemetry Verification (Sandbox)**:
    *   Verify that the sandbox policy version and creation time are updated in telemetry:
        *   `/system/gnmi-pathz-policies/policies/policy[instance=SANDBOX]/state/version` should be `v1`.
        *   `/system/gnmi-pathz-policies/policies/policy[instance=SANDBOX]/state/created-on` should match the pushed timestamp.
3.  **Rollback on Disconnect**:
    *   Close the gRPC session without sending `Finalize`.
    *   Verify that the sandbox policy is cleared or rolled back.
4.  **Finalize Rotation**:
    *   Start a new rotation, push the policy (`v1`), and send `Finalize`.
    *   Verify that the active policy is now updated:
        *   `/system/gnmi-pathz-policies/policies/policy[instance=ACTIVE]/state/version` should be `v1`.
        *   `/system/gnmi-pathz-policies/policies/policy[instance=ACTIVE]/state/created-on` should match the timestamp.
5.  **Get Verification**:
    *   Call `gNSI.Pathz.Get` and verify that the returned policy matches the pushed `v1` policy.
6.  **Force Overwrite**:
    *   Attempt to push the same policy version `v1` without changing the version string. The rotation should fail with an error.
    *   Push the policy again with the same version string `v1` but set `force_overwrite` to `true`. The rotation should succeed and can be finalized.

### Pathz-2: Path-level Authorization Enforcement

This test verifies that the DUT enforces the Pathz policy correctly using the
"Best Match" algorithm.

#### Procedure:

1.  **Push Policy**:
    *   Ensure the baseline Pathz policy is active (from Pathz-1).
2.  **Verify Reader Access (Read Permitted, Write Denied)**:
    *   Use `gnmi_reader` to perform a `gNMI.Get` on `/system/config/hostname`.
        *   Expect: **Success** (allowed by `allow-reader-read-system`).
        *   Telemetry check: `/system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path[name=/system/config/hostname]/state/reads/access-accepts` increments.
    *   Use `gnmi_reader` to perform a `gNMI.Set` (update) on `/system/config/hostname`.
        *   Expect: **Permission Denied** (blocked by `deny-reader-write-system`).
        *   Telemetry check: `/system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path[name=/system/config/hostname]/state/writes/access-rejects` increments.
3.  **Verify Admin Group Access (Group Permit, Specific User Deny)**:
    *   Use `gnmi_admin` (member of `admin-group`) to perform a `gNMI.Set` (update) on `/interfaces/interface[name=<DUT_PORT2>]/config/description`.
        *   Expect: **Success** (allowed by `allow-admin-write-interfaces` via group membership).
    *   Use `gnmi_admin` to perform a `gNMI.Set` (update) on `/interfaces/interface[name=<DUT_PORT1>]/config/description`.
        *   Expect: **Permission Denied** (blocked by `deny-admin-write-port1` which overrides the group permit because user-specific rule is more specific).
4.  **Verify Default Deny**:
    *   Use `gnmi_unauthorized` to perform any `gNMI.Get` or `gNMI.Set`.
        *   Expect: **Permission Denied** (implicit deny).

### Pathz-3: Pathz Policy Verification via Probe RPC

This test verifies the `gNSI.Pathz.Probe` RPC functionality.

#### Procedure:

1.  **Probe Active Policy**:
    *   Use `gNSI.Pathz.Probe` to check access for `spiffe://test-realm.foo.bar/role/reader` to `/system` with `MODE_READ`.
        *   Expect: `action: ACTION_PERMIT` in the response.
    *   Use `gNSI.Pathz.Probe` to check access for `spiffe://test-realm.foo.bar/role/reader` to `/system` with `MODE_WRITE`.
        *   Expect: `action: ACTION_DENY` in the response.
2.  **Probe Sandbox Policy (During Rotation)**:
    *   Start a rotation and push a new policy that denies all access to `/system` for `spiffe://test-realm.foo.bar/role/reader`.
    *   Before finalization, call `gNSI.Pathz.Probe` specifying `policy_instance: POLICY_INSTANCE_SANDBOX`.
        *   Expect: `action: ACTION_DENY` for `MODE_READ` on `/system`.
    *   Call `gNSI.Pathz.Probe` specifying `polcy_instance: POLICY_INSTANCE_ACTIVE`.
        *   Expect: `action: ACTION_PERMIT` (still using active `v1` policy).
    *   Abort the rotation.

### Pathz-4: Policy Removal via CLI

This test verifies that a Pathz policy can be removed from the device using vendor-native CLI commands, and that this removal is correctly reflected in the gNSI Pathz service.

#### Procedure:

1.  **Push Policy**:
    *   Use `gNSI.Pathz.Rotate` to push a test Pathz policy and finalize it.
2.  **Verify Policy Active**:
    *   Call `gNSI.Pathz.Get` and verify that the returned policy is active.
3.  **Removal via CLI**:
    *   Access the DUT via vendor-native CLI (using console or SSH).
    *   Execute the vendor-specific command to remove the Pathz policy.
4.  **Verify Policy Cleared**:
    *   Call `gNSI.Pathz.Get` and verify that no policy is returned (or an empty policy is returned, or the RPC returns an error indicating no policy is configured).

## Canonical OC

```json
{
  "system": {
    "config": {
      "hostname": "dut"
    }
  }
}
```

<!-- disableFinding(LINE_OVER_80) -->
## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPCs intended to be covered by this test.

```yaml
paths:
  /system/gnmi-pathz-policies/policies/policy/state/version:
  /system/gnmi-pathz-policies/policies/policy/state/created-on:
  /system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path/state/reads/access-accepts:
  /system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path/state/writes/access-rejects:

rpcs:
  gnsi:
    pathz.v1.Pathz.Rotate:
    pathz.v1.Pathz.Probe:
    pathz.v1.Pathz.Get:
  gnmi:
    gNMI.Get:
    gNMI.Set:
```

## Required DUT platform

* KNE or physical DUT supporting gNSI Pathz.
