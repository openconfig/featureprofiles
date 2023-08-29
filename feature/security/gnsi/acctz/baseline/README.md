# gNSI.acctz.v1 (Accounting) Test Baseline Pre-requisites

## Minimum DUT pre-requisites:

- Virtual device (vRX)
- DUT must support gnsi.acctz.v1
- DUT is expected to support all or some of the accounting record generating subsystems listed in acctz.CommandService.CmdServiceType and GrpcService.GrpcServiceType.  These tests concentrate on the functionality of the given gNSI operations, not the underlying protocols nor those subsystems.

## Minimum Accounting Remote Collector Pre-requisites:
- A gNSI accounting client (aka. "Remote Collector") is needed to make acctz gRPCs requests to/from the DUT.  It must be possible to inspect the received data.
- Clients for the various accounting record generating subsystems listed in acctz.CommandService.CmdServiceType and GrpcService.GrpcServiceType and console access, if applicable.

## Other
- An NTP source
