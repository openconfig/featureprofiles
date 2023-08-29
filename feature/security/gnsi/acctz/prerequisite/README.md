# gNSI.acctz.v1 (Accounting) Test Prerequisites

Minimum DUT Configuration pre-requisites:

- Configure both IPv4 and IPv6, if possible on the implementation.
- DUT must be configured to, or by default have, a large enough accounting record history to accommodate these tests
- DUT accounting record generating subsystems are configured to generate accounting records to gNSI
- The DUT must be configured for login/logout (aka. start/stop), "enable" (privilege escalation), and "watchdog" (idle) accounting, if supported
- DUT has basic configuration to facilitate a gNSI connection and for those subsystems
- DUT-local user accounts are required
	- User permitted to make gnsi.acctz requests
	- User not permitted make gnsi.acctz requests
	- User permitted to call some grpc, but not all
	- User permitted to run some commands in each of the service classes of gnsi.acctz.v1.CommandService.CmdServiceType & gnsi.acctz.v1.GrpcService.GrpcServiceType, but not all
- DUT must have an NTP source


Minimum Remote Collector Configuration pre-requisites:
- Collector must have an NTP source
