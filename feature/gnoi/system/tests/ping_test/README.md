# gNOI-5.1: Ping Test

## Summary

Validate ping request and response work on the device, with parameters covering
VRF, source-IP address and different packet sizes. L3 Protocol used will be
ICMP, which is default.

## Topology

*   DUT
    *   Note: The current test only pings its looplack address to exercise the
        gNOI API. The test may require the use of ATE for new tests in future.

## Procedure

*   Issue gnoi.system Ping command. Provide following parameters:
    *   Destination: populate this field with the
        *   target device loopback IP address
        *   TODO: an IP-in-IP tunnel-end-point address
        *   TODO: an address matching regular non-default route
        *   TODO: an address matching the default route
        *   TODO: an address requiring VRF fallback lookup into default vrf
        *   TODO: supervisor's physical management port address
        *   TODO: floating management address
    *   Source: populate this field with
        *   loopback IP address
        *   regular interface address
        *   TODO: an IP-in-IP tunnel-end-point address
        *   TODO: supervisor's physical management port address
        *   TODO: floating management address
    *   VRF:
        *   TODO: Set the VRF to be management VRF, TE VRF and default fallback
            VRF
    *   Size:
        *   Set to min packet size of 64, ethernet packet size of 1512, max mtu
            of jumbo frame 9202, and value slightly bigger than the egress
            interface MTU of a transit router to test do_not_fragment.
        *   TODO: verify these for vlan tagged vs untagged packets. May need +4
            bytes

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
rpcs:
  gnoi:
    system.System.Ping:
```

