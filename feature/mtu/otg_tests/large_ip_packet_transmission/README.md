# MTU-1.3: Large IP Packet Transmission

## Summary

Test that ports with sufficiently high MTU do not fragment any packets when flows of various size 
IPv4 and IPv6 packet sizes are sent over them.

## Procedure

* Test environment setup
  * Configure DUT with routed input and output interfaces with an Ethernet MTU of 9216.
  * Test should be executed with two different interface/connectivity profiles:
    1) Standalone -- one input and one output port
    2) Bundle with four input members and four output members

* MTU-1.3.1: Test with Physical and Bundle interfaces
  * Run traffic flows of the following size over IPv4 and IPv6 between ATE ports. 
    * 1500 Bytes
    * 2000 Bytes
    * 4000 Bytes
    * 9202 Bytes
  * Assert ATE reports packets sent and received count are the same, indicating no fragmentation, and 
    successful transit.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
  # Config Paths
  /interfaces/interface/config/mtu:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/mtu:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/mtu:
  # State Paths
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/counters/in-errors:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/in-discards:
  /interfaces/interface/state/counters/out-errors:
  /interfaces/interface/state/counters/out-unicast-pkts:
  /interfaces/interface/state/counters/out-discards:

rpcs:
  gnmi:
    gNMI.Set:
```

## Minimum DUT platform requirement

N/A

