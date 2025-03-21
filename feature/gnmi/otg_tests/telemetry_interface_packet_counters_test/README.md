# gNMI-1.11: Telemetry: Interface Packet Counters

## Summary

Validate interfaces counters including both IPv4 and IPv6 counters.

## Procedure

In the automated ondatra test, verify the presence of the telemetry paths of the
following features:

*   Configure Interface and add load-interval:

    *   /interfaces/interface/rates/config/load-interval

*   Configure IPv4 and IPv6 addresses under subinterface:

    *   /interfaces/interface/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled

    Validate that IPv4 and IPv6 addresses are enabled:

    *   /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/enabled
    *   /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/enabled

*   Validate that Interface has load-interval configured:

    *   /interfaces/interface/rates/state/load-interval

*   For the parent interface counters in-pkts and out-pkts:

    Check the presence of packet counter paths and monitor counters every
    30 seconds:

    *   /interfaces/interface[name='port']/state/counters/in-pkts
    *   /interfaces/interface[name='port']/state/counters/out-pkts

*   Subinterfaces counters:

    Check the presence of packet counter paths

    *   TODO:
        /interfaces/interface[name=port]/subinterfaces/subinterface[index='index']/ipv4/state/counters/in-pkts
    *   TODO:
        /interfaces/interface[name=port]/subinterfaces/subinterface[index='index']/ipv4/state/counters/out-pkts
    *   TODO:
        /interfaces/interface[name=port]/subinterfaces/subinterface[index='index']/ipv6/state/counters/in-discarded-pkts
    *   TODO:
        /interfaces/interface[name=port]/subinterfaces/subinterface[index='index']/ipv6/state/counters/out-discarded-pkts

*   Ethernet interface counters

    Check the presence of counter path including 'in-maxsize-exceeded'

    *   TODO: /interfaces/interface/ethernet/state/counters/in-maxsize-exceeded
    *   /interfaces/interface/ethernet/state/counters/in-mac-pause-frames
    *   /interfaces/interface/ethernet/state/counters/out-mac-pause-frames
    *   /interfaces/interface/ethernet/state/counters/in-crc-errors
    *   /interfaces/interface/ethernet/state/counters/in-fragment-frames
    *   /interfaces/interface/ethernet/state/counters/in-jabber-frames

*   Interface CPU and management

    Check the presence of CPU and management paths

    *   TODO: /interfaces/interface/state/cpu
    *   TODO: /interfaces/interface/state/management

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Test environment setup
The test uses a 2 port ATE setup where 2 ports are used as a singleton interface
Ports are configured with ipv4, ipv6 interfaces on DUT and ATE. Traffic is sent
and from ATE to DUT and the counters are verified.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.
OC paths used for test setup are not listed here.

```yaml
paths:
  ## Config Paths ##
  /interfaces/interface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /interfaces/interface/rates/config/load-interval:

  ## State Paths ##
  /interfaces/interface/state/counters/carrier-transitions:
  /interfaces/interface/state/counters/in-broadcast-pkts:
  /interfaces/interface/state/counters/in-discards:
  /interfaces/interface/state/counters/in-errors:
  /interfaces/interface/state/counters/in-fcs-errors:
  /interfaces/interface/state/counters/in-multicast-pkts:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/out-broadcast-pkts:
  /interfaces/interface/state/counters/out-discards:
  /interfaces/interface/state/counters/out-errors:
  /interfaces/interface/state/counters/out-multicast-pkts:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/out-unicast-pkts:
  /interfaces/interface/rates/state/load-interval:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv4/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/in-discarded-pkts:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/counters/out-discarded-pkts:
  /interfaces/interface/ethernet/state/counters/in-maxsize-exceeded:
  /interfaces/interface/ethernet/state/counters/in-mac-pause-frames:
  /interfaces/interface/ethernet/state/counters/out-mac-pause-frames:
  /interfaces/interface/ethernet/state/counters/in-crc-errors:
  /interfaces/interface/ethernet/state/counters/in-fragment-frames:
  /interfaces/interface/ethernet/state/counters/in-jabber-frames:
  /interfaces/interface/state/cpu:
  /interfaces/interface/state/management:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Required DUT platform

* Specify the minimum DUT-type
  * FFF - fixed form factor is enough for this test. However it can run also
  on a MFF testbed.
    gNMI.Set:

## Minimum DUT platform requirement
  * FFF - fixed form factor

