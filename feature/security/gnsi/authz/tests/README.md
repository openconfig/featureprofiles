# gNSI Authz Tests

## Summary
Test gNSI API behaviors and gRPC authorization policy behaviors.

## Baseline Setup

### Input Args

* `test_infra_id`: the SPIFFI ID that used by test infra clients.

### DUT service setup

Configure the DUT to enable the following services (that are using gRPC) are up, and use mTLS for authentication:
* gNMI
* gNOI
* gNSI
* gRIBI

### Client certs

Prepare the following certs with the specified SPIFFE ID. Cert format details can be found in https://github.com/openconfig/featureprofiles/pull/1563/files

* `cert_user_admin` with `spiffe://test-abc.foo.bar/xyz/admin`
* `cert_user_fake` with `spiffe://test-abc.foo.bar/xyz/fake`
* `cert_gribi_modify` with `spiffe://test-abc.foo.bar/xyz/gribi-modify`
* `cert_gnmi_set` with `spiffe://test-abc.foo.bar/xyz/gnmi-set`
* `cert_gnoi_time` with `spiffe://test-abc.foo.bar/xyz/gnoi-time`
* `cert_gnoi_ping` with `spiffe://test-abc.foo.bar/xyz/gnoi-ping`
* `cert_gnsi_probe` with `spiffe://test-abc.foo.bar/xyz/gnsi-probe`
* `cert_read_only` with `spiffe://test-abc.foo.bar/xyz/read-only`

### gRPC authorization policies

NOTE: unless specifically mentioned, the rule `allow-test-infra` MUST be attached to all the policies, so that the test or the test infra is not blocked from the device.

```json
{
  "name": "allow-test-infra",
  "source": {
    "principals": [
      "<test_infra_id>",
    ]
  },
  "request": {}
},
```
Prepare the following gRPC authorization policies.

```json
{
  "name": "policy-everyone-can-gnmi-not-gribi",
  "allow_rules": [
    {
      "name": "everyone-can-gnmi-get",
      "source": {},
      "request": {
        "paths": [
          "gnmi.GNMI/Get",
        ]
      },
    }
  ],
  "deny_rules": [
    {
      "name": "no-one-can-gribi-get",
      "request": {
        "paths": [
          "gribi.GRIBI/Get",
        ]
      }
    }
  ]
}
```

```json
{
  "name": "policy-everyone-can-gribi-not-gnmi",
  "allow_rules": [
    {
      "name": "admin-can-do-everything",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/admin",
        ]
      },
      "request": {}
    },
  ],
  "deny_rules": [
    {
      "name": "fake-user-can-do-nothing",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/fake",
        ]
      },
    }
  ]
}
```

```json
{
  "name": "policy-invalid-no-allow-rules",
  "deny_rules": [
    {
      "name": "no-one-can-gribi",
      "request": {
        "paths": [
          "gribi.GRIBI/Modify"
        ]
      }
    }
  ]
}
```

```json
{
  "name": "policy-gribi-get",
  "allow_rules": [
    {
      "name": "gribi-get",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/read-only",
        ]
      },
      "request": {
        "paths": ["/gribi.GRIBI/Get"]
      }
    },
  ],
}
```

```json
{
  "name": "policy-gnmi-get",
  "allow_rules": [
    {
      "name": "gnmi-get",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/read-only",
        ]
      },
      "request": {
        "paths": ["/gnmi.GNMI/Get"]
      }
    },
  ],
}
```

The following table describes policy `policy-normal-1`:

Cert | gRIBI.Modify | gRIBI.Get | gNMI.Set | gRIBI.Get | gNOI.Time | gNOI.Ping | gNSI.Rotate | gNSI.Get
:--- | :---  | :--- | :---  | :---  | :---  | :--- | :--- | :-----
cert_user_admin | allow | allow |allow |allow |allow |allow |allow |allow
cert_user_fake | deny |deny |deny |deny |deny |deny |deny |deny
cert_gribi_modify | allow |allow |deny |deny |deny |deny |deny |deny
cert_gnmi_set | deny |deny |deny |deny |deny |deny |allow |allow
cert_gnoi_time |deny |deny |allow |allow |deny |deny |deny |deny
cert_gnoi_ping |deny |deny |deny |deny |allow |deny |deny |deny
cert_gnsi_probe |deny |deny |deny |deny |deny |allow |deny |deny
cert_read_only |deny |deny |allow |allow |deny |deny |deny |allow

```json
{
  "name": "policy-normal-1",
  "allow_rules": [
    {
      "name": "gribi-modify",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/admin",
          "spiffe://test-abc.foo.bar/xyz/gribi-modify",
        ]
      },
      "request": {
        "paths": ["/gribi.GRIBI/*"],
      }
    },
    {
      "name": "gnmi-set",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/admin",
          "spiffe://test-abc.foo.bar/xyz/gnmi-set",
        ]
      },
      "request": {
        "paths": ["/gnmi.GNMI/*"]
      }
    },
    {
      "name": "gnoi-time",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/admin",
          "spiffe://test-abc.foo.bar/xyz/gnoi-time",
        ]
      },
      "request": {
        "paths": ["/gnoi.system.System/Time"]
      }
    },
    {
      "name": "gnoi-ping",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/admin",
          "spiffe://test-abc.foo.bar/xyz/gnoi-ping",
        ]
      },
      "request": {
        "paths": ["/gnoi.system.System/Ping"]
      }
    },
    {
      "name": "gnsi-probe",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/admin",
          "spiffe://test-abc.foo.bar/xyz/gnsi-probe",
        ]
      },
      "request": {
        "paths": ["/gnsi.authz.v1.Authz/Probe"]
      }
    },
    {
      "name": "read-only",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/read-only",
        ]
      },
      "request": {
        "paths": [
          "/gnmi.GNMI/Get",
          "/gribi.GRIBI/Get",
          "/gnsi.authz.v1.Authz/Get",
        ]
      }
    },
  ],
  "deny_rules": [
    {
      "name": "fake-user-can-do-nothing",
      "source": {
        "principals": [
          "spiffe://test-abc.foo.bar/xyz/fake",
        ]
      },
      "request": {
        "paths": ["/*"]
      }
    }
  ]
}
```


## Tests

NOTE: regarding gNMI OC validation:
  * Everytime a gRPC call (including gNSI calls themselves) is allowed or denied, the following OC leaves should be validated:
    * `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc[name]/state/name` is the matched request path, e.g. "/gribi.GRIBI/Get"
    * `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/rpc[name]/state/access-accepts` increments if the rpc call is allowed.
    * `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/rpc[name]/state/access-rejects` increments if the rpc call is denied.
    * `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/rpc[name]/state/last-access-accept` reflects the timestamp of the method call.
    * `/system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/rpc[name]/state/last-access-reject` reflects the timestamp of the method call.
  * Everytime a valid policy is pushed (even it's not finalized), the following OC leaves should be validated:
    * `/system/grpc-servers/grpc-server/state/authz-policy-version` = `UploadRequest.version` in the API proto.
    * `/system/grpc-servers/grpc-server/state/authz-policy-created-on` = `UploadRequest.created_on` (in terms of represented time).
  * Everytime a valid policy is automatically rolled back, the following OC leaves should be validated:
    * `/system/grpc-servers/grpc-server/state/authz-policy-version` = `UploadRequest.version` of the previous request (the one rollback to).
    * `/system/grpc-servers/grpc-server/state/authz-policy-created-on` = `UploadRequest.created_on` of the previous request (the one rollback to).
  * An invalid policy should not trigger the following OC leaf updates:
    * `/system/grpc-servers/grpc-server/state/authz-policy-version`
    * `/system/grpc-servers/grpc-server/state/authz-policy-created-on`

### Authz-1, test policy behaviors, and probe results matches actual client results.

For each of the scenarios in this section, we need to exercise the following 3 actions to get the authorization results:
  * `gNSI.Probe` after `UploadResponse` message but before the `FinalizeRequest` message.
  * `gNSI.Probe` after the `RotateAuthzRequest` call finished.
  * The actual corresponding service client calls, after the `RotateAuthzRequest` call finished.

* Authz-1.1, "Test empty source"
  1. Use `gNSI.Rotate` method to push policy `policy-everyone-can-gnmi-not-gribi`, with `create_on` = `100` and `version` = `policy-everyone-can-gnmi-not-gribi_v1`.
  2. Ensure all results match per the following:
    * `cert_user_admin` is allowed to issue `gNMI.Get` method.
    * `cert_user_admin` is denied to issue `gRIBI.Get` method.

* Authz-1.2, "Test empty request"
  1. Use `gNSI.Rotate` method to push and finalize policy `policy-everyone-can-gribi-not-gnmi`, with `create_on` = `100` and `version` = `policy-everyone-can-gribi-not-gnmi_v1`.
  2. Ensure all results match per the following:
    * `cert_user_fake` is denied to issue `gRIBI.Get` method.
    * `cert_user_admin` is allowed to issue `gRIBI.Get` method.

* Authz-1.3, "Test that there can only be one policy"
  1. Use `gNSI.Rotate` method to push and finalize policy `policy-gribi-get`, with `create_on` = `100` and `version` = `policy-gribi-get_v1`.
  2. Ensure all results match per the following:
      * `cert_ready_only` is allowed to issue `gRIBI.Get` method.
      * `cert_ready_only` is denied to issue `gNMI.Get` method.
  3. Use `gNSI.Rotate` method to push and finalize policy `policy-gnmi-get`.
  4. Ensure all results changed to the following:
      * `cert_ready_only` is denied to issue `gRIBI.Get` method.
      * `cert_ready_only` is allowed to issue `gNMI.Get` method.

* Authz-1.4, "Test normal policy"
  1. Use `gNSI.Rotate` method to push and finalize policy `policy-normal-1`, with `create_on` = `100` and `version` = `policy-normal-1_v1`.
  2. Ensure all results match per the above table for policy `policy-normal-1`.

### Authz-2, test rotation behavior

* Authz-2.1, "Test only one rotation request at a time"
  1. Use `gNSI.Rotate` method to push policy `policy-everyone-can-gnmi-not-gribi`, but don't finalize it yet.
  2. Initial another `gNSI.Rotate` method to push policy `policy-everyone-can-gribi-not-gnmi`, and expect to receive an  `UNAVAILABLE` gRPC error.
  3. Ensure all actual client authorization result stays as per the following:
      * `cert_user_admin` is allowed to issue `gNMI.Get` method.
      * `cert_user_admin` is denied to issue `gRIBI.Get` method.

* Authz-2.2, "Test rollback when connection closed"
  1. Use `gNSI.Rotate` method to push and finalize policy `policy-gribi-get`.
  2. Ensure `gNSI.Probe` result matches the following:
      * `cert_ready_only` is allowed to issue `gRIBI.Get` method.
      * `cert_ready_only` is denied to issue `gNMI.Get` method.
  3. Use `gNSI.Rotate` method to push policy `policy-gnmi-get`, but don't finalize it yet.
  4. Ensure `gNSI.Probe` result matches the following:
      * `cert_ready_only` is denied to issue `gRIBI.Get` method.
      * `cert_ready_only` is allowed to issue `gNMI.Get` method.
  5. Close the gRPC session.
  6. Ensure `gNSI.Probe` result changed back to the following:
      * `cert_ready_only` is allowed to issue `gRIBI.Get` method.
      * `cert_ready_only` is denied to issue `gNMI.Get` method.

* Authz-2.3, "Test rollback on invalid policy"
  1. Use `gNSI.Rotate` method to push and finalize policy `policy-gribi-get`.
  2. Ensure `gNSI.Probe` result matches the following:
      * `cert_ready_only` is allowed to issue `gRIBI.Get` method.
      * `cert_ready_only` is denied to issue `gNMI.Get` method.
  3. Use `gNSI.Rotate` method to push policy `policy-invalid-no-allow-rules`, expect an error message and closed gRPC session.
  4. Ensure `gNSI.Probe` result remains as the following:
      * `cert_ready_only` is allowed to issue `gRIBI.Get` method.
      * `cert_ready_only` is denied to issue `gNMI.Get` method.

* Authz-2.4, "Test force_overwrite when the version does not change"
  1. Use `gNSI.Rotate` method to push and finalize policy `policy-gribi-get`.
  2. Use `gNSI.Rotate` method to try to push policy `policy-gnmi-get` with version value not changed. Expect error message and closed gRPC session.
  4. Validate that actual client authorization result stays as the following:
      * `cert_ready_only` is allowed to issue `gRIBI.Get` method.
      * `cert_ready_only` is denied to issue `gNMI.Get` method.
  3. Use `gNSI.Rotate` method to try to push policy `policy-gnmi-get` with version value, but `force_overwrite` set to true. Expect no error message, and the push can be finalized.
  4. Ensure actual client authorization results are changed to the following:
      * `cert_ready_only` is denied to issue `gRIBI.Get` method.
      * `cert_ready_only` is allowed to issue `gNMI.Get` method.


### Authz-3 Test Get behavior
  1. Use `gNSI.Rotate` method to push and finalize policy `policy-gribi-get`.
  2. Wait for 30s, intial `gNSI.Get` and validate the value of `version`, `created_on` and gRPC policy content does not change.


### Authz-4 reboot persistent

  1. Use `gNSI.Rotate` method to push and finalize policy `policy-normal-1`.
  2. Reboot the device.
  3. Reconnect to the device, issue `gNSI.Get` and `gNMI.Get` and validate the value of `version`, `created_on` and gRPC policy content does not change.
  4. Ensure actual corresponding clients are authorized per the the above table for policy `policy-normal-1`.