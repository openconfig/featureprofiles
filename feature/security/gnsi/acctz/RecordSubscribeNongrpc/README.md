# ACCTZ-3.1 - gNSI.acctz.v1 (Accounting) Test Record Subscribe Non-gRPC

## Summary
Test Accounting for non-gRPC records

## Procedure
- Record the time T0 for use later in this test
- For each of the supported service classes in gnsi.acctz.v1.CommandService.CmdServiceType:
	- Connect to the DUT, recording the local and remote IP addresses and port numbers,
	- Run a few commands that should be permitted and a few that should be denied. If command abbreviation is permitted, at least one command and its arguments should be abbreviated.
	- disconnect
- Establish gNSI connection to the DUT.
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = T0
- Verify that accurate accounting records are returned for the commands/RPCs that were run, both permitted and denied.
- If start/stop accounting is supported, each connection's accounting should be preceded by a start (login) record for the service and the records associated with the command/RPCs sent during the connection should be followed by a logout record.
- For each RecordResponse returned, check/confirm that:
	- session_info. :
		- .{layer4_proto, local_address, local_port, remote_address, remote_port}, ip_proto must match those recorded earlier
		- channel_id = 0 for ssh and grpc.
		- .tty must be populated and correct, if applicable to the platform & access method, else omitted
		- .status must equal the operation, else UNSPECIFIED if there is no corresponding enumeration.  It must equal ONCE for connections where each command/RPC is authenticated (eg: HTTP metadata authen).  If the operation was not LOGIN, ONCE, or ENABLE, authen must be omitted, else it must be populated:
			- .authen.type must equal the authentication method used.
			- .authen.status must equal the status of the authentication operation.  if FAIL or ERROR, cause should be populated, if SUCCESS, cause might be populated.
		- .user.identity must match the username used to authenticate to the DUT
		- .user.privilege_level must match the user's privilege level, if applicable to the platform
	- timestamp is after (greater than) RecordRequest.timestamp
	- session_info.service_request must be a CmdService.
	- cmd_service. : 
		- .service_type must equal the service used
		- .cmd must equal the full name of the command sent, expanded by the DUT, if abbreviated
		- .cmd_args must equal the arguments to the command.  Abbreviated arguments that are not user-freeform, must be expanded by the DUT.
		- If any of the .cmd or .cmd_args are truncated, {cmd,cmd_args}_istruncated must be true, else false.
	- for authorization:
		- .status must equal the expected and true authorization status for the command/RPC
		- if .status is PERMIT, .detail  might be populated with additional information
		- if .status is DENY or ERROR, .detail should be populated with the reason
		- command_service.status must equal the status of the command/RPC operation, SUCCESS or FAILURE
			- If SUCCESS, task_ids should be populated with task IDs
			- If FAILURE, failure_cause must be populated with a failure message
	- task_ids might be populated with platform-specific information
- If applicable to the service type, and session_info.stats != ONCE, ensure records for each connection are bracketed by LOGIN/LOGOUT records.


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

