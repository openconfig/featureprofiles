# TUN-2.8 ECMP hashing based with MPLSoUDP decap 

## Summary
Test ECMP hashing with ingress MPLSoUDP decapsulation and POP actions.

## Configuration:

* Configure 2 LAG interfaces between the DUT and ATE with 4 member links in each bundle. One for ingress and one for egress.
* Assign IPv4 and IPv6 address on both bundles for point-to-point IP connectivity.
* On the ingress interface, configure a GUE decapsulation filter based on any target IPv4 address.
* Configure static MPLS pop rule for the MPLS label 100000.

## Traffic

* Test case 1: IPv4 Inner header
	* GUE destination address: Target of GUE decapsulation filter
	* MPLS label: 100000
	* Traffic payload will be a varierty of 64 flows with different TCP src/dst ports and source src IPv4. The dst IPv4 will be the address of the egress interface.

* Test case 2: IPv6 Inner header
	* GUE destination address: Target of GUE decapsulation filter
	* MPLS label: 100000
	* Traffic payload will be a varierty of 64 flows with different TCP src/dst ports and source src IPv6. The dst IPv6 will be the address of the egress interface.

## Verification

Ensure that the traffic is balanced across all member links of the egress interface with a 6% tolerance.

## OC Paths
```

    # Bundle configuration
    /interfaces/interface/ethernet/config/port-speed:
    /interfaces/interface/ethernet/config/duplex-mode:
    /interfaces/interface/ethernet/config/aggregate-id:
    /interfaces/interface/aggregation/config/lag-type:
    /interfaces/interface/aggregation/config/min-links:
    # match condition
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address:
    # decap action
	/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-mpls-in-udp:
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



# TUN-2.9 ECMP hashing based with GUE decap 

## Summary
Test ECMP hashing with ingress GUE decapsulation.

## Configuration:

* Configure 2 LAG interfaces between the DUT and ATE with 4 member links in each bundle. One for ingress and one for egress.
* Assign IPv4 and IPv6 address on both bundles for point-to-point IP connectivity.
* On the ingress interface, configure a GUE decapsulation filter based on any target IPv4 address.

## Traffic

* Test case 1: IPv4 Inner header
	* GUE destination address: Target of GUE decapsulation filter
	* Traffic payload will be a varierty of 64 flows with different TCP src/dst ports and source src IPv4. The dst IPv4 will be the address of the egress interface.

* Test case 2: IPv6 Inner header
	* GUE destination address: Target of GUE decapsulation filter
	* Traffic payload will be a varierty of 64 flows with different TCP src/dst ports and source src IPv6. The dst IPv6 will be the address of the egress interface.

## Verification

Ensure that the traffic is balanced across all member links of the egress interface with a 6% tolerance.

## OpenConfig paths
```
paths:
    # Bundle configuration
    /interfaces/interface/ethernet/config/port-speed:
    /interfaces/interface/ethernet/config/duplex-mode:
    /interfaces/interface/ethernet/config/aggregate-id:
    /interfaces/interface/aggregation/config/lag-type:
    /interfaces/interface/aggregation/config/min-links:
    # match condition
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address:
    # decap action
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gue
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


# TUN-2.10 ECMP hashing based with GRE encap

## Summary
Test ECMP hashing with ingress GRE encapsulation.

## Configuration:

* Configure 2 LAG interfaces between the DUT and ATE with 4 member links in each bundle. One for ingress and one for egress.
* Assign IPv4 and IPv6 address on both bundles for point-to-point IP connectivity.
* On the ingress interface, configure a GRE encapsulation tunnel with the destination IP of the egress interface.

## Traffic

* Test case 1: IPv4 Inner header
	* Generate a varierty of 64 flows with different TCP src/dst ports. The src/dst IPv4 address will be in the range specified in the GRE filter so that it gets encapsulated.

* Test case 2: IPv6 Inner header
	* Generate a varierty of 64 flows with different TCP src/dst ports. The src/dst IPv6 address will be in the range specified in the GRE filter so that it gets encapsulated.

## Verification

Ensure that the traffic is balanced across all member links of the egress interface with a 6% tolerance.

## OpenConfig paths
```
paths:
    # Bundle configuration
    /interfaces/interface/ethernet/config/port-speed:
    /interfaces/interface/ethernet/config/duplex-mode:
    /interfaces/interface/ethernet/config/aggregate-id:
    /interfaces/interface/aggregation/config/lag-type:
    /interfaces/interface/aggregation/config/min-links:
    # match condition
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/source-address:
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address:

   	# TODO: Add new OC for GRE encap headers
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/index:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/next-hop:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/config/index:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/type:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dst-ip:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/src-ip:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dscp:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/ip-ttl:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/index:

    # TODO: Add new OC for policy forwarding actions
    #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/next-hop-group:
    #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/config/packet-type:
    #/network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/count:


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true

```
