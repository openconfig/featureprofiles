# RT-5.8: IPv6 Link Local

## Summary

Configure an IPv6 address which is in link local scope. Verify the link local IPv6 address exists by checking the state path.

## Testbed type

* Use the [OTG DUT 2 port testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/otgdut_2.binding)

## Procedure

* Sub Test #1 - Configure IPv6 link local
  * Configure DUT port 1 and OTG port 1 with an IPv6 link local scope IP address
  * Validate config and state paths are set

* Sub Test #2 - Verify the interface will pass IPv6 traffic
  * Send IPv6 traffic from OTG port 1 to OTG port 2
  * Validate OTG port 2 receives the traffic

## Config Parameter Coverage

```
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/type
```

## Telemetry Parameter Coverage

```
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/prefix-length
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/type
```

## Protocol/RPC Parameter Coverage

None

## Required DUT platform

* FFF - fixed form factor
