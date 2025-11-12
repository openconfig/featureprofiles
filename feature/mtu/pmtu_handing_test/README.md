# MTU-1.5: Path MTU handing

## Summary

This tests ensures that DUT generates ICMP "Fragmentation Needed and Don't Fragment was Set" for packets exceeding egress interface MTU. 

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

    ```
                             |         |      |            |
        [ ATE Port 1 ] ----  |   DUT   | ---- | ATE Port 2 |
                             |         |      |            |
    ```


### Configuration

* Configure DUT with routed ports on DUT.
* Configure Ethernet MTU of 9216 on DUT port 1 and Ethernet MTU of 1514 on DUT port 2.
* Configure static routes on DUT for IPV4-DST and IPV6-DST to ATE Port 2  


### MTU-1.5.1 IPv4 Path MTU 

Run traffic flows to IPV4-DST with the sizes at 50% linerate for 30 seconds:
- 2000 Bytes
- 4000 Bytes
- 9000 Bytes

Verify:
* Ensure that ATE Port-1 receives ICMP type-3, code-4 for packet of every flow sent.
* DUT pipeline counters report fragment packet discards.
* Verify the amount of traffic forwarded and dropped to the control-plane and compare to the amount of packets sent. Default rate-limiting of fragment  
  traffic is permitted.
* Verify low CPU (<20%) utilization on control plane.

### MTU-1.5.2 IPv6 Path MTU 

Run traffic flows to IPV6-DST with the sizes at 50% linerate for 30 seconds:
- 2000 Bytes
- 4000 Bytes
- 9000 Bytes

* Ensure that ATE Port-1 receives ICMPv6 type-2 code-0 for packet of every flow sent.
* Verify that DUT pipeline counters report fragment packet discards.
* Verify the amount of traffic forwarded and dropped to the control-plane and compare to the amount of packets sent. Default rate-limiting of fragment  
  traffic is permitted.
* Verify low CPU (<20%) utilization on control plane.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # tunnel interfaces
    /interfaces/interface/config/mtu:
    # telemetry
    /components/component/integrated-circuit/pipeline-counters/drop/state/packet-processing-aggregate:
      platform_type: [ "INTEGRATED_CIRCUIT" ]
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/fragment-total-drops:
      platform_type: [ "INTEGRATED_CIRCUIT" ]


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

The device may support some vendor proprietary leafs to count MTU exceeded packets which are dropped due to control plane policing rules in the `components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor` tree.  
Implementation should add code with a switch statement to expose these counters, if they exist.

## Required DUT platform

* FFF
