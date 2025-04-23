# gNOI-5.2: Traceroute Test

## Summary

Validate that the device supports traceroute functionality using different
source, destination, max_ttl, do_not_fragment and L4Protocols.

## Procedure

*   Issue gnoi.system Traceroute command. Provide following parameters:

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
    *   Max_TTL: Check the following cases of TTL values:
        *   Not set(default of 30)
        *   TODO: Set to -1: *Check if test is abandoned once TTL reaches higher
            value(of say 100)
        *   Set to 1
        *   TODO: Set to 255
    *   Do_not_fragment: Check the following cases when DF bit is:
        *   TODO: Unset
    *   L4Protocol: set as:
        *   ICMP
        *   TCP
        *   UDP

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:

rpcs:
  gnoi:
    system.System.Traceroute:
```
