# RT-5.8: IPv6 Link Local

## Summary

Configure an IPv6 address which is in link local scope. Verify the link local IPv6 address exists by checking the state path.

## Procedure

* Subtest #1 - Configure IPv6 link local
  * Configure DUT port 1 and OTG port 1 with an IPv6 link local scope IP address
  * Configure DUT port 2 and OTG port 2 with an IPv6 link local scope IP address
  * Validate config and state paths are set

* Subtest #2 - Verify the interface will pass IPv6 traffic as expected
  * Send IPv6 traffic from OTG port 1 to OTG port 2, validate OTG port 2 does not receive the traffic
  * Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1 receives the traffic
  
* Subtest #3 - Verify adding and removing global unicast address does not affect link local address
  * Add configuration for a global unicast address on DUT port 1 
  * Validate config and state paths are set
  * Remove configuration for the global unitcast address on DUT port 1
  * Validate that DUT port 1 link local address is still configured
  * Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1 receives the traffic

* Subtest #4 - Verify enable/disable of DUT port 1 does not affect link local address
  * Disable/enable the port and see if the configured link-local address stays?
  * Validate that DUT port 1 link local address config and state paths continue to contain the address assignment
  * Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1 receives the traffic

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
