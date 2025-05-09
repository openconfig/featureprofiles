# OC-1.2: Default Address Families

TODO(robshakir): fill in test plan from code already written.

## Description

This test verifies that the IPv4 and IPv6 address families are enabled within a network instance by default.

## Test Procedure

* Configure an ATE with port1 connected to DUT port1, and port2 connected to DUT port2.
* Configure the DUT to have:
  * these interfaces within the `DEFAULT` network instance and validate that traffic can be forwarded between ATE port1 and ATE port2.
  * these interfaces within a non-default `L3VRF` and validate that traffic can be forwarded between ATE port1 and ATE port2.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
paths:
rpcs:
  gnmi:
    gNMI.Subscribe:
```

