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

* Configure DUT with routed input with Ethernet MTU of 9216 and output interfaces with Ethernet MTU of 1514
* Configure static routes to IPV4-DST and IPV6-DST to ATE Port 2


### MTU-1.5.1 IPv4 Path MTU 

Run traffic flows to IPV4-DST with the sizes:
- 2000 Bytes
- 4000 Bytes
- 9000 Bytes

Ensure that ATE Port-1 receives ICMP type-3, code-4 for packet of every flow sent.
Verify that DUT pipeline counters report fragment packet discards.

### MTU-1.5.2 IPv6 Path MTU 

Run traffic flows to IPV6-DST with the sizes:
- 2000 Bytes
- 4000 Bytes
- 9000 Bytes

Ensure that ATE Port-1 receives ICMPv6 type-2 code-0 for packet of every flow sent.
Verify that DUT pipeline counters report fragment packet discards.

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # tunnel interfaces
    /interfaces/interface/config/mtu:
    # telemetry
    /components/component/integrated-circuit/pipeline-counters/drop/lookup-block/state/fragment-total-drops:


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF