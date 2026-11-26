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

Run traffic flows to IPV4-DST with the sizes at 50% linerate for 15 seconds:
- 2000 Bytes
- 4000 Bytes
- 9000 Bytes

Verify:
* Require at least one ICMP type-3, code-4 response addressed to ATE Port-1 for each flow.
* Require traffic transmitted on ATE Port-1 and zero flow packets received on ATE Port-2.
* Require present `packet-processing-aggregate` counters on matching integrated circuits, with a positive combined delta within 30 seconds after traffic. Take a per-flow baseline immediately before traffic and fail on missing counters or counter decreases. Aggregate drops may include unrelated causes; the ICMP check establishes PMTU response coverage. Default ICMP rate-limiting is permitted.
* Verify that post-test CPU utilization does not exceed the pre-test baseline by more than 10%.

#### Canonical OC
```json
{

}
```

### MTU-1.5.2 IPv6 Path MTU 

Run UDP/IPv6 traffic flows to IPV6-DST with the sizes at 50% linerate for 15 seconds:
- 2000 Bytes
- 4000 Bytes
- 9000 Bytes

* Require at least one ICMPv6 type-2, code-0 response addressed to ATE Port-1 for each flow.
* Require traffic transmitted on ATE Port-1 and zero flow packets received on ATE Port-2.
* Require present `packet-processing-aggregate` counters on matching integrated circuits, with a positive combined delta within 30 seconds after traffic. Take a per-flow baseline immediately before traffic and fail on missing counters or counter decreases. Aggregate drops may include unrelated causes; the ICMP check establishes PMTU response coverage. Default ICMP rate-limiting is permitted.
* Verify that post-test CPU utilization does not exceed the pre-test baseline by more than 10%.

#### Canonical OC
```json
{

}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # tunnel interfaces
    /interfaces/interface/config/mtu:
    # telemetry
    /interfaces/interface/state/mtu:
    /components/component/integrated-circuit/pipeline-counters/drop/state/packet-processing-aggregate:
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
