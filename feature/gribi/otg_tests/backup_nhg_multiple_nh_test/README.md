# TE-11.2: Backup NHG: Multiple NH

## Summary

Ensure that backup NHGs are honoured when the primary NextHopGroup entries contain >1 NH.

## Procedure

*   Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, ATE port-3 to
    DUT port-3, ATE port-4 to DUT port-4, and ATE port-5 to DUT port-5.
*   Create a L3 routing instance (VRF-A), and assign DUT port-1 to VRF-A.
*   Connect a gRIBI client to the DUT, make it become leader and inject the
    following:
    *   An IPv4Entry in VRF-A for `203.0.113.1/32`, pointing to a NextHopGroup (in DEFAULT VRF)
        containing:
        *   Two primary next-hops with weights:
            *   IP of ATE port-2 (Weight 80)
            *   IP of ATE port-3 (Weight 20)
        *   A backup NextHopGroup.
    *   The backup NextHopGroup (in DEFAULT VRF) contains:
        *   Two next-hops with weights:
            *   IP of ATE port-4 (Weight 60)
            *   IP of ATE port-5 (Weight 40)
*   Send traffic from ATE port-1 destined to `203.0.113.1/32`.
*   Ensure that traffic is received at ATE port-2 and ATE port-3 in an 80:20 ratio.
    Validate that AFT telemetry covers this case.
*   Disable DUT port-2. Ensure that traffic for the destination is now received only at
    ATE port-3.
*   Disable DUT port-3. Ensure that traffic for the destination fails over to the backup NHG
    and is received at ATE port-4 and ATE port-5 in a 60:40 ratio.

## Config Parameter coverage

*   No new configuration covered.

## Telemetry Parameter coverage

*   No new telemetry covered.

## Protocol/RPC Parameter coverage

*   gRIBI:
    *   Modify
        *   ModifyRequest
            *   NextHopGroup
                *   backup_nexthop_group

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/config/interface:
  /network-instances/network-instance/interfaces/interface/config/subinterface:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy:
  /network-instances/network-instance/policy-forwarding/interfaces/interface/config/interface-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/policy-id:
  /network-instances/network-instance/policy-forwarding/policies/policy/config/type:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/protocol:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

vRX

