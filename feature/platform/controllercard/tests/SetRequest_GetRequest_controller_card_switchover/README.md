# gNMI-1.19: ConfigPush and ConfigPull after Control Card switchover

## Summary
This test verifies if a large config can be pushed and/or pulled via gNMI SetRequest/GetRequest within 2 minutes after Control Card switchover.

## Procedure

* Prepare a large OpenConfig config file to be pushed within a single setRequest.
  * 150 LAG interfaces w/ ip, ipv6 configuration
  * 800 Ethernet interfaces as member s of LAG
  * 28 IPv4 and 28 IPv6 BGP neighbors
  * ISIS on all trunk/LAG ports

### sub-Test 1: SetRequest
[TODO: [issue #2407](https://github.com/openconfig/featureprofiles/issues/2407) ]
* Store indexes of ACTIVE and BACKUP Controller Card in "previous_Active" and "previous_BACKUP"
* Initiate Control Card switchover using gNOI SwitchControlProcessorRequest; store timestamp in "SwitchControlProcessorRequest_time"
* Wait for `SwitchControlProcessorResponce` but no longer then 120s. If not received, test FAILED.
* Immediately after receiving `SwitchControlProcessorResponce` for  gNOI switchover, send gNMI `setRequest` with a prepared large config. Store timestamp as "SwitchControlProcessorResponce_time".
* Wait for `SetResponce` but no longer than 30s.
  * If not received, the test  wait 10s and send gNMI `setRequest` with prepared large config. Repaet Wait for `SetResponce`.
  * If received at time <= "SwitchControlProcessorResponce"+110s and a non-zero grpc status code is returned, wait 10s and send gNMI `setRequest` with prepared large config. Repeat Wait for `SetResponce`
  * If received at time > "SwitchControlProcessorResponce"+110s and a non-zero grpc status code is returned, test FAILED
  * If received at time <= "SwitchControlProcessorResponce"+120s and SUCCESS is returned, proceed
* Retrieve configuration from DUT DUT using gNMI `GetRequest`.
* Verify:
  * The gNMI `setResponce` has been received within 120s after `setRequest` by comparing with "SwitchControlProcessorResponse_time", and
  * The gNOI `SwitchControlProcessorResponce` has been received and switchover was executed by DUT (compare "previous_ACRIVE" with DUT state), and
  * The configuration retrieved from DUT is the same as one prepared^1
^1 some small deviations are expected. This is OK to verify that the retrieve configuration is not smaller in size than the prepared one, and has the same number of interfaces, BGP neighbors. 

### sub-Test 2: GetRequest
[TODO: [issue #2451](https://github.com/openconfig/featureprofiles/issues/2451) ]
* Store indexes of ACTIVE and BACKUP Controller Card in "previous_Active" and "previous_BACKUP"
* Retrive full configuration using gNMI `getRequest` and store as "PreviousFullConfig"
* Initiate Control Card switchover using gNOI SwitchControlProcessorRequest; store timestamp in "SwitchControlProcessorRequest_time"
* Wait for `SwitchControlProcessorResponce` but no longer then 120s. If not received, test FAILED.
* Immediately after receiving `SwitchControlProcessorResponce` for  gNOI switchover, send gNMI `getRequest`. Store timestamp as "SwitchControlProcessorResponce_time".
* Wait for `getResponce` but no longer than 10s.
  * If not received, the test  wait 10s and send gNMI `getRequest`. Reapeat from "Wait for `getResponce` but no longer than 10s" above.
  * If received at time <= "SwitchControlProcessorResponce"+110s and a non-zero grpc status code is returned, wait 10s and send gNMI `getRequest`. Reapeat from "Wait for `getResponce` but no longer than 10s" above.
  * If received at time > "SwitchControlProcessorResponce"+110s and a non-zero grpc status code is returned, test FAILED
  * If received at time <= "SwitchControlProcessorResponce"+120s and SUCCESS is returned, store retrived configuration as "CurrentFullConfiguration" than proceed

* Verify if "PreviousFullConfig" == "CurrentFullConfiguration". If TRUE, test passed, elese test failed.

## Testbed topology
dut.testbed

## Config Parameter coverage
N/A

## Telemetry Parameter coverage
N/A

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## DUT

MFF
