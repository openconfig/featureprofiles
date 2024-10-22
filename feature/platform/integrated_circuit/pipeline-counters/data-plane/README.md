# IC-1.1: Test different drop counters for data-plane traffic

## Summary
This is to test the different data-plane counters under the pipeline container. 


## Procedure

* Test environment setup
    ```mermaid
    graph LR; 
    A[ATE1:interface1]  <-- L3 connection --> B[interface1:DUT];
    ```
    * Make a L3 connection between the ATE and the DUT. This doesnt have to be a lag bundle.

* IC-1.1.1 - [TODO:] Exercise adverse-aggregate drop counter

* IC-1.1.2 - [TODO:] Exercise congestion-aggregate drop counter

* IC-1.1.3 - [TODO:] Exercise packet-processing-aggregate drop counter

* IC-1.1.4 - [TODO:] Exercise no-route drop counter

* IC-1.1.4 - [TODO:] Exercise vendor drop counter


  
## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

```yaml
paths:
  # interface configuration
/components/component/integrated-circuit/pipeline-counters/drop/state/adverse-aggregate
/components/component/integrated-circuit/pipeline-counters/drop/state/congestion-aggregate
/components/component/integrated-circuit/pipeline-counters/drop/state/no-route
/components/component/integrated-circuit/pipeline-counters/drop/state/urpf-aggregate
/components/component/integrated-circuit/pipeline-counters/drop/vendor

rpcs:
  gnmi:
    gNMI.Subscribe:
      sample_interval: 5000000000
```
