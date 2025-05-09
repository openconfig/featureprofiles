# ACCTZ-9.1 - gNSI.acctz.v1 (Accounting) Test Accounting Privilege Escalation

## Summary
Test Accounting for changing current privilege level, if supported.

## Procedure

- Record the time T0 for use later in this test
- For each of the supported RecordResponse.service_request.service_type that supports privilege escalation:
	- Connect to the DUT, recording the local and remote IP addresses and port numbers,
	- change privilege level,
	- disconnect
- Establish gNSI connection to the DUT.
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = T0
- Verify that accurate accounting records are returned for the commands/RPCs authentication failures.
- If start/stop accounting is supported, each authentication failure should have an accompanying session_info.status = LOGIN accounting record.
- For each RecordResponse correlated to each privilege escalation, check/confirm that:
	- session_info. :
		- .{layer4_proto, local_address, local_port, remote_address, remote_port}, ip_proto must match those recorded earlier
		- channel_id = 0 for ssh and grpc.
		- .tty must be populated and correct, if applicable to the platform & access method, else omitted
		- .status must equal ENABLE:
			- .authen.type must equal the authentication method used.
			- .authen.status must equal FAIL, and cause should be populated.
			- .authen.cause should be populated with reason(s) for the failure.
		- .user.identity must match the username sent to authenticate to the DUT
		- .user.privilege_level must be populated with the new privilege level
	- timestamp is after (greater than) RecordRequest.timestamp
	- session_info.service_request.serivce_type must equal the service used.
	- cmd_service. or grpc_service: 
		- .service_type must equal the service used
		- all other fields should be omitted.
	- for authorization:
		- all other fields should be omitted.
	- task_ids might be populate with platform-specific information

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

TODO(OCRPC): Record may not be complete

```yaml
paths:
    ### Prefix:
    # Accounting does not currently support any telemetry; see https://github.com/openconfig/gnsi/issues/97 where it might become /system/aaa/acctz/XXX
rpcs:
  gnsi:
    acctz.v1.Acctz.RecordSubscribe:
      "RecordRequest.timestamp!=0": true
      "RecordResponse.service_request = CommandService": true
```

## Minimum DUT
vRX
