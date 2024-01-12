# gNSI.acctz.v1 (Accounting) Tests

## gNSI Accounting (acctz) Test prerequisites:

### Minimum DUT pre-requisites:

- Virtual device (vRX)
- DUT must support gnsi.acctz.v1
- DUT is expected to support all or some of the accounting record generating subsystems listed in acctz.CommandService.CmdServiceType and GrpcService.GrpcServiceType.  These tests concentrate on the functionality of the given gNSI operations, not the underlying protocols nor those subsystems.
- DUT must have a NTP source

### Minimum Accounting Remote Collector Pre-requisites:
- A gNSI accounting client (aka. "Remote Collector") is needed to make acctz gRPCs requests to the DUT.  It must be possible to inspect the received data.
- Clients for the various accounting record generating subsystems listed in acctz.CommandService.CmdServiceType and GrpcService.GrpcServiceType and console access, if applicable.
- The accounting client must have a NTP source.

### Other
- An NTP source

## gNSI Accounting (acctz) Test Configuration library:

Create a library of device configuration to be used for all of the gNSI.acctz.v1 tests with the following:

- Configure both IPv4 and IPv6, if supported by the implementation.
- DUT must be configured to, or by default have, a large enough accounting record history to accommodate these tests.  The minimum size is determined by the number of accounting record generating subsystems supported by the DUT and the size of the requests made.
- If not the default, accounting record generating subsystems must be configured to generate accounting records to gNSI.
- The DUT must be configured for login/logout (aka. start/stop), "enable" (privilege escalation), and "watchdog" (idle) accounting, if supported
- DUT has basic configuration to facilitate a gNSI connection and connections for each of those subsystems.
- DUT-local user accounts are required:
	- A User permitted to make gnsi.acctz requests
	- A User not permitted to make gnsi.acctz requests
	- A User permitted to call some grpc, but not all
	- A User permitted to run some commands in each of the service classes of gnsi.acctz.v1.CommandService.CmdServiceType & gnsi.acctz.v1.GrpcService.GrpcServiceType, but not all

## gNSI Accounting (acctz) Tests:
- [ACCTZ-1.1 Record Subscribe Full](RecordSubscribeFull)
- [ACCTZ-2.1 Record Subscribe Partial](RecordSubscribePartial)
- [ACCTZ-3.1 Record Subscribe Non-gRPC](RecordSubscribeNongrpc)
- [ACCTZ-4.1 Record History Truncation](RecordHistoryTruncation/)
- [ACCTZ-4.2 Record Payload Truncation](RecordPayloadTruncation/)
- [ACCTZ-5.1 Record Subscribe Idle Timeout](RecordSubscribeIdleTimeout/)
- [ACCTZ-6.1 Record Subscribe Idle Timeout DoA](RecordSubscribeIdleTimeoutDoA/)
- [ACCTZ-7.1 Accounting Authentication Failure - Multi-transaction](AccountingAuthenFailMulti/)
- [ACCTZ-8.1 Accounting Authentication Failure - Uni-transaction](AccountingAuthenFailUni/)
- [ACCTZ-9.1 Accounting Privilege Escalation](AccountingPrivEscalation/)
- [ACCTZ-10.1 Accounting Authentication Error - Multi-transaction](AccountingAuthenErrorMulti/)
