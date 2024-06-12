# SYS-2.1: Ingress control-plane ACL.

## Summary

The test verifies securing device control-plane access with an ingress access-control-list (ACL).

## Testbed type

*  [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

*   DUT has an single ingress port with IPv4/IPv6 enabled.

   ATE <> DUT

### Configuration

1.  Configure test address on the device loopback (secondary)

2.  Configure IPv4/IPv6 ACL/filters with following terms:
    - Allow gRPC from any (lab management access)
    - Allow SSH from MGMT-SRC
    - Allow ICMP from MGMT-SRC
    - Explicit deny

3.  Apply filter to control-plane ingress.

## Test cases

### SYS-2.1.1: Verify ingress control-plane ACL permit
Generate ICMP traffic to device loopback from MGMT-SRC
Generate SSH SYN packets to device loopback from MGMT-SRC

Verify:

*  ACL counters for corresponding ACL entries are incrementing.
*  Device responds to ICMP permitted
*  Device sends TCP-ACK for SSH session

### SYS-2.1.2: Verify control-plane ACL deny
Generate ICMP traffic to device loopback from UNKNOWN-SRC
Generate SSH SYN packets to device loopback from UNKNOWN-SRC

Verify:

*  Explicit deny ACL counter is incrementing.
*  Device does not respond to ICMP
*  Device sends TCP-ACK for SSH session

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # acl definition
    /acl/acl-sets/acl-set/config/name:
    /acl/acl-sets/acl-set/config/type:
    /acl/acl-sets/acl-set/config/description:
    /acl/acl-sets/acl-set/acl-entries/acl-entry/config/sequence-id:
    /acl/acl-sets/acl-set/acl-entries/acl-entry/config/description:
    /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address:
    /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol:
    /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address:
    /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol:
    /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port:
    # acl application
    /system/control-plane-traffic/ingress/acl/acl-set/config/set-name:
    /system/control-plane-traffic/ingress/acl/acl-set/config/type:

    # telemetry
    /system/control-plane-traffic/ingress/acl/acl-set/state/set-name:
    /system/control-plane-traffic/ingress/acl/acl-set/acl-entries/acl-entry/state/sequence-id:
    /system/control-plane-traffic/ingress/acl/acl-set/acl-entries/acl-entry/state/matched-packets:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* MFF
* FFF
* VRX