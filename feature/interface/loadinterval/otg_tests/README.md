# RT-5.11: Interface Load-Interval for Statistics Sampling

## Summary

This test case verifies the DUT's ability to configure and utilize the load-interval parameter for statistics sampling on its interfaces. This parameter determines the time interval over which interface statistics like input/output rates are calculated.

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Configuration

1) Create the topology below:

    ```
    [ ATE Port 1 ] ----  |   DUT   | ---- | ATE Port 2 |
    ```

2) Configure a non-default load-interval value on an interface on the DUT  (30 seconds).

### Traffic:
*   Establish a stable traffic flow between ATE1 and ATE2 through the DUT on the configured interface.

### Verification:

*   Initial Observation:
    *   Observe the interface statistics on the DUT.
    *   Record the initial input/output rate values.
*   Load-Interval Impact:
    *   Clear the interfaces counters.
    *   Configure the load-interval to a value equal to 30 seconds.
    *   Use gnmi.Subscribe SAMPLE mode with 10 second interval, capture 3 samples and compute the average pps and bps and compare to the ATE generated flow
    *   Verify that the input/output rate values have been updated and reflect the average traffic rate over the configured load-interval.
*   Varying Load-Interval:
    *   Clear the interfaces counters.
    *   Change the load-interval to a different value (60 seconds).
    *   Use gnmi.Subscribe SAMPLE mode with 10 second interval, capture 3 samples and compute the average pps and bps.
    *   Verify that the input/output rates were not incremented as the sampling time is less that the load-interval
    *   Use gnmi.Subscribe SAMPLE mode with 10 second interval, capture 6 samples and compute the average pps and bps and compare to the ATE generated flow
    *   Verify that the input/output rates now reflect the average traffic rate over the new load-interval.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config paths
  /interfaces/interface/rates/config/load-interval:
  ## State paths
  /interfaces/interface/rates/state/load-interval:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/out-octets:
rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Minimum DUT platform requirement
* FFF