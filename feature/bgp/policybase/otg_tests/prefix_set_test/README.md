# RT-1.33: BGP Policy with prefix-set matching

## Summary

BGP policy configuration with prefix-set matching

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure
Establish eBGP sessions between:
	•	ATE port-1 and DUT port-1
	•	ATE port-2 and DUT port-2
	•	Configure Route-policy under BGP neighbor/session address-family

For IPv4:
Create two prefix-sets as below:
IPv4-prefix-set-1  - exact match on 10.23.15.0/26
IPv4-prefix-set-2  - match on 10.23.0.0/16
For IPv6:
Create two prefix-sets as below:
IPv6-prefix-set-1  - exact match on 2001:4860:f804::/48
IPv6-prefix-set-2  - 65-128 match on ::/0
For IPv4 and IPv6:
	•	Configure BGP policy on DUT to allow routes based on IPv4-prefix-set-2 and reject routes based on IPv4-prefix-set-1 
	•	Configure BGP policy on DUT to allow routes based on IPv6-prefix-set-1
	•	and reject routes based on IPv6-prefix-set-2 
	•	Validate that the prefixes are accepted after policy application.
	•	DUT conditionally advertises prefixes received from ATE port-1 to ATE port-2 after policy application. Ensure that multiple routes are accepted and advertised to the neighbor on ATE port-2.

## Config Parameter Coverage
/routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
/routing-policy/defined-sets/prefix-sets/prefix-set/config/name
/routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
/routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range

/routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
/routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range
/routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
/routing-policy/defined-sets/prefix-sets/prefix-set/state/name

/routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
/routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set

## Telemetry Parameter coverage
N/A
Protocol/RPC Parameter coverage
N/A
Minimum DUT platform requirement
vRX