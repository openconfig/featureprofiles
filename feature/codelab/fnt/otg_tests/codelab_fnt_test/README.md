# CL-1.1: FNT Codelab covering basic interface setup

## Summary

Validates connectivity and basic forwarding between DUT and ATE.

## Testbed type

Topology: https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

ATE port-1 <------> port-1 DUT

DUT port-2 <------> port-2 ATE

## Procedure

*   Connect DUT port-1 and port-2 to ATE port-1 and port-2 respectively.
*   Configure IPv4/IPv6 addresses on the DUT ports.
*   Configure a logical interface to the ATE ports.
*   Generate fixed packets(30000) traffic from ATE port-1 to ATE port-2.
*   Validate in/out packets on ATE and also validate packet loss.

## OpenConfig Path and RPC Coverage

This yaml stanza defines the OC paths intended to be covered by this test. OC
paths used for test environment setup are not required to be listed here. This
content is parsed by automation to derive the test coverage

```yaml
paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

vRX