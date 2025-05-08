# PF-1.21 ECMP hashing based with GUE decap 

## Summary
Test ECMP hashing with ingress GUE decapsulation.

## Configuration:

```

                         |         |
    [ ATE Agg1 ]---------|   DUT   |----------[ ATE Agg2 ]
                         |         |
```


* Configure 2 LAG interfaces between the DUT and ATE with 4 member links in each bundle. One for ingress and one for egress.
* Assign IPv4 and IPv6 address on both bundles for point-to-point IP connectivity.
* On the ingress interface, configure a GUE decapsulation filter based on any target IPv4 address.

## Traffic

* Test case 1: IPv4 Inner header
	* GUE destination address: Target of GUE decapsulation filter (IPv4 address and port).
	* Traffic payload will be a varierty of 64 flows with different TCP src/dst ports and source src IPv4. The dst IPv4 will be the address of the egress interface.

* Test case 2: IPv6 Inner header
	* GUE destination address: Target of GUE decapsulation filter (IPv4 address and port).
	* Traffic payload will be a varierty of 64 flows with different TCP src/dst ports and source src IPv6. The dst IPv6 will be the address of the egress interface.

## Verification

Ensure that the traffic is balanced across all member links of the egress interface with a 6% tolerance.

## OpenConfig paths
```yaml
paths:
    # Bundle configuration
    /interfaces/interface/ethernet/config/port-speed:
    /interfaces/interface/ethernet/config/duplex-mode:
    /interfaces/interface/ethernet/config/aggregate-id:
    /interfaces/interface/aggregation/config/lag-type:
    /interfaces/interface/aggregation/config/min-links:
    # match condition
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/destination-address:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/transport/config/destination-port:
    # decap action
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gue:
    # application to the interface
    /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-forwarding-policy:


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true

```