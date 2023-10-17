# gNSI.acctz.v1 (Accounting) Test Record Subscribe Full

Test RecordSubscribe for all (since epoch) records

## Procedure

- For each of the applicable service types in gnsi.acctz.v1.GrpcService.GrpcServiceType:
	- Alternate connecting to the IPv4 and Ipv6 addresses of the DUT, recording the local and remote IP addresses and port numbers,
	- Call a few RPCs that will generate accounting records and that, by authorization configuration, should be permitted and a few that should be denied, and some that include arbitrary arguments (eg: interface description), pause for 1 second after the first RPC of this test to ensure its timestamp differs from subsequent RPCs.
	- disconnect
- Establish gNSI connection to the DUT.
- Call gnsi.acctz.v1.Acctz.RecordSubscribe with RecordRequest.timestamp = 0
- Retain a copy of the Record.timestamp of the record corresponding to the first RPC sent above for use in the [Record Subscribe Partial](../RecordSubscribePartial) test.
- Verify that accurate accounting records are returned for the commands/RPCs that were run, both permitted and denied.
	- If start/stop accounting is supported, each connection’s accounting should be preceded by a start (login) record for the service and the records associated with the RPCs sent during the connection should be followed by a logout record.
	- For each RecordResponse returned, check/confirm that:
		- session_info. :
			- .{layer4_proto, local_address, local_port, remote_address, remote_port}, ip_proto must match those recorded earlier
			- .channel_id = 0 for ssh and grpc.
			- .tty must be populated and correct, if applicable to the platform & access method, else omitted
			- .status must equal the operation, else UNSPECIFIED if there isn’t a corresponding enumeration.  It must equal ONCE for connections where each RPC/command is authenticated (eg: gRPC metadata authen). If the operation was not LOGIN, ONCE, or ENABLE, authen must be omitted, else it must be populated:
				- .authen.type must equal the authentication method used.
				- .authen.status must equal the status of the authentication operation.  if FAIL or ERROR, cause should be populated, if SUCCESS, cause might be populated.
			- .user.identity must match the username used to authenticate to the DUT
			- .user.privilege_level must match the user’s privilege level, if applicable to the platform
		- timestamp is after (greater than) RecordRequest.timestamp
		- session_info.service_request must be a GrpcService.
		- grpc_service. : 
			- .service_type must equal the service used
			- .rpc_name must equal the path of the RPC call made
			- .payloads must equal the payload of the RPC sent.
			- If any of the payloads is truncated, payload_istruncated must be true, else false.
		- for authorization:
			- .status must equal to the expected and actual authorization status for the RPC
			- if .status is PERMIT, .detail  might be populated with additional information
			- if .status is DENY or ERROR, .detail should be populated with the reason
 
- task_ids might be populate with platform-specific information

## Config Parameter
### Prefix:
/gnsi/acctz/v1/Acctz/RecordSubscribe

### Parameter:
RecordRequest.timestamp=0
RecordResponse.service_request = GrpcService

## Telemetry Coverage
### Prefix:
Accounting does not currently support any telemetry; see https://github.com/openconfig/gnsi/issues/97 where it might become /system/aaa/acctz/XXX

## Protocol/RPC
gnsi.acctz.v1

## Minimum DUT
vRX
