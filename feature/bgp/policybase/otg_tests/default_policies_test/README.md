# RT-7: BGP default policies

## Summary

Following expectation for default-policies at the peer-group and neighbor levels
*   Default import and export policies are expected to be functional for both IBGP and EBGP peers when policy definition in the policy chain is not satisfied.
*   For IBGP peers, when no policy is attached, actions defined in the default-policy (when one exists) should apply.
*   For EBGP peers, when no policy is attached, actions defined in the default-policy (when one exists) should apply.

## Topology
```mermaid
graph LR; 
A[OTG:Port1] <-- EBGP --> B[Port1:DUT:Port2];
B <-- IBGP+IS-IS --> C[Port2:OTG];
```

## Procedure
* DUT:Port1 has EBGP peering with ATE:Port1. Ensure ATE:Port1 advertises IPv4-prefix1, IPv4-prefix2, IPv4-prefix3, IPv6-prefix1, IPv6-prefix2 and IPv6-prefix3
* DUT:Port2 has IBGP peering with ATE:PORT2 using its loopback interface. The loopback interface is reachable via IS-IS. Ensure ATE:Port2 advertises IPv4-prefix4, IPv4-prefix5, IPv4-prefix6, IPv6-prefix4, IPv6-prefix5 and IPv6-prefix6
* Conduct following test procedures by applying policies at the Peer-group and Neighbor AFI-SAFI levels.

* RT-7.1 : Policy definition in policy chain is not satisfied and Default Policy has REJECT_ROUTE action
  * Create a default policy REJECT-ALL with action as REJECT_ROUTE and apply the same to both IPV4-unicast and IPV6-unicast AFI-SAFI
  * Create policy EBGP-IMPORT-IPV4 that only accepts IPv4-prefix1 and IPv4-prefix2 and then terminates
  * Create policy EBGP-IMPORT-IPV6 that only accepts IPv6-prefix1 and IPv6-prefix2 and then terminates
  * Create policy EBGP-EXPORT-IPV4 that only allows IPv4-prefix4 and terminates
  * Create policy EBGP-EXPORT-IPV6 that only allows IPv6-prefix4 and terminates
  * Create policy IBGP-IMPORT-IPV4 that only accepts IPv4-prefix4 and IPv4-prefix5 and then terminates
  * Create policy IBGP-IMPORT-IPV6 that only accepts IPv6-prefix4 and IPv6-prefix5 and then terminates
  * Create policy IBGP-EXPORT-IPV4 that only allows IPv4-prefix1 and terminates
  * Create policy IBGP-EXPORT-IPV6 that only allows IPv6-prefix1 and terminates
  * Apply the above policies to the respective peering at the repective AFI-SAFI levels
  * Following test expectations. If expectations not met, the test should fail.
    * DUT:Port1 should reject import of IPv4-prefix3 and IPv6-prefix3
    * DUT:Port1 should reject export of IPv4-prefix5 and IPv6-prefix5
    * DUT:Port2 should reject import of IPv4-prefix6 and IPv6-prefix6
    * DUT:Port2 should reject export of IPv4-prefix2 and IPv6-prefix2

* RT-7.2 : Policy definition in policy chain is not satisfied and Default Policy has ACCEPT_ROUTE action  
  * Continue with the same configuration as RT-7.1
  * Replace the default-policy REJECT-ALL with default-policy ACCEPT-ALL which has action ACCEPT_ROUTE.
  * Ensure ACCEPT-ALL policy is applied to both IPv4-unicast and IPv6-unicast AFI-SAFI
  * Following test expectations. If expectations not met, the test should fail.
    * DUT:Port1 should accept import of IPv4-prefix1, IPv4-prefix2, IPv4-prefix3, IPv6-prefix1, IPv6-prefix2 and IPv6-prefix3
    * DUT:Port1 should allow export of IPv4-prefix4, IPv4-prefix5, IPv4-prefix6, IPv6-prefix4, IPv6-prefix5 and IPv6-prefix6
    * DUT:Port2 should accept import of IPv4-prefix4, IPv4-prefix5, IPv4-prefix6, IPv6-prefix4, IPv6-prefix5 and IPv6-prefix6
    * DUT:Port2 should allow export of IPv4-prefix1, IPv4-prefix2, IPv4-prefix3, IPv6-prefix1, IPv6-prefix2 and IPv6-prefix3
    * Ensure that DUT:Port1 is not exporting non BGP routes to ATE:PORT1
