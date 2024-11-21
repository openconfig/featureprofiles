# TUN-1.9: GRE inner packet DSCP

## Summary

Verify the DSCP value of original packet header after GRE acl based tunnel encapsulation.

## Procedure

*   Configure DUT with ingress and egress routed interfaces.
*   Configure acl based tunnel configuration with action as encapsulation.
*   Attach the filter on ingress interface.
*   Configure the static route for the tunnel end point destination.
*   Capture packet on ATE on the recieving end(port-2).
*   verify dscp value of original packet after encapsulation.
*   verify that no traffic drops in all flows.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/set-name:
  /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/config/type:
rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Validation coverage

*   No packet drop should be oberserved.
*   Capture the packet on recieving end and verify the orginal DSCP value. Orginal value should not be altered.
    
