# gNMI-1.19: ConfigPush and ConfigPull after Control Card switchover

## Summary
This test verifies if a large config can be pushed via gNMI `SetRequest` within 120 seconds after Control Card switchover.

## Procedure

* Prepare a large OpenConfig config file to be pushed within a single setRequest.
  * 150 LAG interfaces w/ ip, ipv6 configuration
  * 800 Ethernet interfaces as member s of LAG
  * 28 IPv4 and 28 IPv6 BGP neighbors
  * ISIS on all trunk/LAG ports
* Store indexes of ACTIVE and BACKUP Controller Card in `previous_Active` and `previous_BACKUP`
* Initiate Control Card switchover using gNOI SwitchControlProcessorRequest; store timestamp in `SwitchControlProcessorRequest_time`
* Wait for `SwitchControlProcessorResponse` but no longer than 120 seconds. If not received, test FAILED.
* Immediately after receiving `SwitchControlProcessorResponse` for gNOI switchover, send gNMI `setRequest` with a prepared large config. Store timestamp as `SwitchControlProcessorResponse_time`.
* Wait for `SetResponse` but no longer than 30 seconds.
  * If `SetResponse` is not received, wait 10 seconds and retry the gNMI `setRequest` with the prepared large config. Repeat the wait for `SetResponse`.
  * If `SetResponse` is received at time <= `SwitchControlProcessorResponse_time` + 110 seconds and a non-zero grpc status code is returned, wait 10 seconds and retry the gNMI `setRequest` with the prepared large config. Repeat the wait for `SetResponse`.
  * If `SetResponse` is received at time > `SwitchControlProcessorResponse_time` + 110 seconds and a non-zero grpc status code is returned, test FAILED
  * If `SetResponse` is received at time <= `SwitchControlProcessorResponse_time` + 120 seconds and SUCCESS is returned, proceed
* Retrieve configuration from DUT DUT using gNMI `GetRequest`.
* Verify:
  * The gNMI `setResponse` has been received within 120 seconds after `setRequest` by comparing with `SwitchControlProcessorResponse_time`, and
  * The gNOI `SwitchControlProcessorResponse` has been received and switchover was executed by DUT (compare `previous_ACTIVE` with DUT state), and
  * The configuration retrieved from DUT is the same as one prepared^1
^1 some small deviations are expected. This is OK to verify that the retrieve configuration is not smaller in size than the prepared one, and has the same number of interfaces, BGP neighbors. 

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
