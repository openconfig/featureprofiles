# gNMI-x.x: Exception Configlets and union_replace

## Summary

Exception configlets are stanzas of native vendor config in CLI format.  The operational use case is in an emergency, configuration may need to be quickly applied to mitigate an impaired network.  When using CLI for configuration, config generation software creates a complete configuration for a device with an ordered list of CLI commands.  The exception configlets are added to the end of the ordered list of CLI commands.  CLI commands in the exception configlet are applied in order and override any conflicting commands that come before it.

With gNMI union_replace, the goal is to achieve the same behavior.  Configuration generation will generate a complete device configuration using both origin openconfig and origin CLI.  This data will then be placed into a single gnmi.Set using the union_replace action.

The exception configuration items may be modeled and used in openconfig with the exception configlet overriding what is set in openconfig.  The configuration items may be vendor specific and not modeled in openconfig.

The expected usage of exception configlets is:

The origin CLI configuration will contain an ordered list of commands, with the exception configlet(s) appended to the end.   The device should merge the origin CLI configuration as per the union_replace specification.  If a CLI command appears which conflicts with other CLI, the latest CLI should be used.  If the resulting CLI command conflicts with origin openconfig, then the [union_replace conflict resolution method](https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-union_replace.md#533-resolving-issues-with-union-between-the-origins) should be used.

## Testbed type

* [2 port ATE to DUT](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### gNMI-x.x.x - Add configuration to existing protocol (Adding an import policy to BGP)

Existing configuration defines a routing protocol in OpenConfig.

Exception configuration uses CLI to create a policy and apply it to the existing routing protocol.

```
route-map MY-INPUT permit 10
  match community ROUTE-GENERATOR
  set local-preference 5000
route-map PROD-IN permit 20
!

router bgp 65500
  neighbor ONE route-map MY-INPUT in
  neighbor TWO route-map MY-INPUT in
!
```

### gNMI-x.x.x - Change configuration of a routing protocol (Change routing protocol preference)

Existing configuration defines BGP routing process using OpenConfig

Exception configuration uses CLI to change a property of the BGP routing process. (Global parameter routing-preference)

```
router bgp 65500
   distance bgp 20 200 200
```

###  gNMI-x.x.x - Delete existing configuration (Remove BGP neighbors)

Existing configuration defines the neighbors using OpenConfig.

Exception configuration uses CLI to removes some neighbors.

```
protocols {
    bgp {
        group IBGP {
            delete: neighbor <ipv4.1>;
            delete: neighbor <ipv6.1>;
        }
    }
}
```

### gNMI-x.x.x - Delete and add a routing-policy (policy-forwarding example)

Existing configuration defines a routing-policy using OpenConfig.

Exception configuration uses CLI to remove the routing-policy and create a new one in its place.

```
no policy-map type pbr my_pbr_policy
policy-map type pbr my_pbr_policy
   10 class ingress-10-pbr
      set nexthop-group my_nexthop_group
!
```

## OpenConfig Coverage

[TODO: Add OC paths covered]

gnmi.Set.SetRequest.Update.union_replace

## Minimum DUT Required

vRX - Virtual Router Device
