# IC-1: Test aggregate drop counters for control-plane traffic

## Summary
This is to test the different control-plane counters under the pipeline container. 


## Procedure

* Test environment setup
    ```mermaid
    graph LR; 
    A[ATE1:interface1]  <-- L3 connection --> B[interface1:DUT];
    ```
    * Make a L3 connection between the ATE and the DUT. This doesnt have to be a lag bundle.


* IC-1.0.1 - Exercise aggregate counters

  * Generate significant amount of traffic with the IP Options field set. This can be Option type 7 for record route. Also set the TTL of the packet to "1".
  * Check changes in the different state paths under the pipeline-counters container.
  * Check if the NOS has support for the leaves under the `vendor` container to identify the type of packet causing Queue increase as well as changes in the drop counters.


## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

```yaml
paths:
  # interface configuration
/components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/dropped-aggregate
/components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/dropped-bytes-aggregate
/components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/queued-aggregate
/components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/queued-bytes-aggregate
/components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor

rpcs:
  gnmi:
    gNMI.Subscribe:
      on_change: true
```
