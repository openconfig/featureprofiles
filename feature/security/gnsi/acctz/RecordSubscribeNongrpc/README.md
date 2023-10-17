# gNSI.acctz.v1 (Accounting) Test Record Subscribe Non-gRPC

Test Accounting for non-gRPC records

## Procedure

- For each of the applicable service classes in gnmi. gnsi.acctz.v1.CommandService.CmdServiceType:
	- Connect to a non-”System” address on the DUT, recording the local and remote IP addresses and port numbers,
	- Run a few commands that should be permitted and a few that should be denied. If command abbreviation is permitted, at least one command and its arguments should be abbreviated.
	- disconnect
- Establish gNSI connection to the DUT.
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = to the timestamp retained in ACCT1.2
- Retain a copy of the Record.timestamp of the first returned record for ACCT1.2
- Verify that accurate accounting records are returned for the commands/RPCs that were run, both permitted and denied.
- If start/stop accounting was enabled, each connection’s accounting should be preceded by a start (login) record for the service and the records associated with the RPCs sent during the connection should be followed by a logout record.
- For each RecordResponse returned, check/confirm that:
	- session_info. :
		- .{layer4_proto, local_address, local_port, remote_address, remote_port}, ip_proto must match those recorded earlier
		- channel_id = 0 for ssh and grpc.
		- .tty must be populated and correct, if applicable to the platform & access method, else omitted
		- .status must equal the operation, else UNSPECIFIED if there is no corresponding enumeration.  It must equal ONCE for connections where each RPC/command is authenticated (eg: gRPC metadata authen).  If the operation was not LOGIN, ONCE, or ENABLE, authen must be omitted, else it must be populated:
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
		- .status must equal the expected and true authorization status for the RPC
		- if .status is PERMIT, .detail  might be populated with additional information
		- if .status is DENY or ERROR, .detail should be populated with the reason
		- grpc_service.status must equal the status of the RPC operation, SUCCESS or FAILURE
			- If SUCCESS, task_ids should be populated with task IDs
			- If FAILURE, failure_cause must be populated with a failure message
	- task_ids might be populate with platform-specific information
- If applicable to the service type, and session_info.stats != ONCE, ensure records for each connection are bracketed by LOGIN/LOGOUT records.


## Config Parameter
### Prefix:
/gnsi/acctz/v1/Acctz/RecordSubscribe

### Parameter:
RecordRequest.timestamp!=0
RecordResponse.service_request = CommandService

## Telemetry Coverage
### Prefix:
Accounting does not currently support any telemetry; see https://github.com/openconfig/gnsi/issues/97 where it might become /system/aaa/acctz/XXX

## Protocol/RPC
gnsi.acctz.v1

## Minimum DUT
vRX
