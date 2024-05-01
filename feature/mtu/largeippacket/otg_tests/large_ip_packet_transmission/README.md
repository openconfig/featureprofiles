# MTU-1.3: Large IP Packet Transmission

## Summary

Test that ports with sufficiently high MTU do not fragment any packets when flows of various size 
IPv4 and IPv6 packet sizes are sent over them.

## Procedure

* Configure DUT with routed input and output interfaces with an Ethernet MTU of 9216.
  * Test should be executed with two different interface/connectivity profiles:
    1) Standalone -- one input and one output port
    2) Bundle with four input members and four output members
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
  ## Config Paths ##
  /interfaces/interface/config/mtu:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/mtu:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/mtu:

  ## State Paths ##
  # No coverage, validates success by checking flow statistics between ATE ports.

rpcs:
  gnmi:
    gNMI.Set:
```

## Minimum DUT platform requirement

N/A

