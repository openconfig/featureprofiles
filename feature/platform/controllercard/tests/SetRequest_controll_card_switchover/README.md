# gNMI-1.19 ConfigPush after Control Card switchover

## Summary
This test verifies if a large config can be bushed via gNMI SetRequest within 2 minutes after Control Card switchover. 

## Procedure

* Prepare a large OpenConfig config file to be pushed within a single setRequest.
  * 150 LAG interfaces w/ ip, ipv6 configuration
  * 800 Ethernet interfaces as member s of LAG
  * 28 IPv4 and 28 IPv6 BGP neighbors
  * ISIS on all trunk/LAG ports
* Store indexes of ACTIVE and BACKUP Controller Card in "previous_Active" and "previous_BACKUP"
* Initiate Control Card switchover using gNOI SwitchControlProcessorRequest; store timestamp in "SwitchControlProcessorRequest_time"
* Wait for `SwitchControlProcessorResponce` but no longer then 120s. If not received, test FAILED.
* Immediately after receiving `SwitchControlProcessorResponce` for  gNOI switchover, send gNMI `setRequest` with a prepared large config. Store timestamp as "SwitchControlProcessorResponce_time".
* Wait for `SetResponce` but no longer than 120s.
  * If not received, the test FAILED.
  * If received at time <= "SwitchControlProcessorResponce"+110s and ERROR is returned, send gNMI `setRequest` with prepared large config. Reaped form Wait for `SetResponce`
  * If received at time > "SwitchControlProcessorResponce"+110s and ERROR is returned, test FAILED
  * If received at time <= "SwitchControlProcessorResponce"+120s and SUCCESS is returned, proceed
* Retrieve configuration from DUT DUT using gNMI `GetRequest`.
* Verify:
  * The gNMI `setResponce` has been received within 120s after `setRequest` by comparing with "SwitchControlProcessorResponse_time", and 
  * The gNOI `SwitchControlProcessorResponce` has been received and switchover was executed by DUT (compare "previous_ACRIVE" with DUT state), and
  * The configuration retrieved from DUT is the same as one prepared^1

^1 some small deviations are expected. This is OK to verify that the retrieve configuration configuration is not smaller in size than the prepared one, and has the same number of interfaces, BGP neighbors.

## Testbed topology
dut.testbed

## Config Parameter coverage
N/A

## Telemetry Parameter coverage
N/A

##
MFF
