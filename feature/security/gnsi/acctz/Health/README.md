# gNSI.acctz.v1 (Accounting) Test DUT Health Check

Verify the healthy state of the DUT and Remote Collector:

- The system clock must be synchronized between DUT and accounting Collector
- Capture the generic health check of the DUT, used modularly in pre/post and during various different tests
- No system/kernel/process/component coredumps 
- No high CPU spike or usage on control or forwarding plane
- No high memory utilization or usage on control or forwarding plane
- No processes/daemons high CPU/Memory utilization
- No SWAP memory utilization
- No generic drop counters
- QUEUE drops
- Interfaces
- VOQ
- Fabric Drops
- ASIC drops
- DDOS/COPP violations
- No flow control frames tx/rx
- No CRC or Layer 1 errors on interfaces or fabric links
- No system errors or logs
- No config commit errors
- No system level alarms 
- No memory leaks
- In spec hardware should be in proper state
- No hardware errors
- Major Alarms 
- No HW component or SW processes crash
