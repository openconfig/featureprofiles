# RT-1.32: BGP policy actions - MED, LocPref, prepend, flow-control

## Summary

- Verify abilty to set MED to fixed value in export and import policy
- Verify abilty to increment MED by fixed value in export and import policy
- Verify abilty to set Local Preference to fixed value in export and import policy
- Verify abilty to prepend AS path with 10 additional repetitions of local ASN in export and import policy
- Verify abilty to prepend AS path with 10 additional repetitions of configured ASN in export and import policy
- verify ```NEXT-STATEMENT``` flow-control action
- Applicable to both IPv4 and IPv6 BGP neighbors

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure
### Applying configuration

For each section of configuration below, prepare a gnmi.SetBatch  with all the configuration items appended to one SetBatch.  Then apply the configuration to the DUT in one gnmi.Set using the `replace` option.
> WARNING: Replace operations should be performed at an appropriate level in the config tree to ensure that preexisting configuration objects necessary for DUT management access and base operation are not removed.

#### Initial Setup:

*   Connect DUT port-1, 2 to ATE port-1, 2
*   Configure IPv4/IPv6 addresses on the ports
*   Create an IPv4 networks i.e. ```ipv4-network-1 = 192.168.10.0/24``` attached to ATE port-1
*   Create an IPv6 networks i.e. ```ipv6-network-1 = 2024:db8:128:128::/64``` attached to ATE port-1
*   Create an IPv4 networks i.e. ```ipv4-network-2 = 192.168.20.0/24``` attached to ATE port-2
*   Create an IPv6 networks i.e. ```ipv6-network-2 = 2024:db8:64:64::/64``` attached to ATE port-2
*   Configure IPv4 and IPv6 iBGP between DUT Port-1 and ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network-1 = 192.168.10.0/24``` and ```ipv6-network-1 = 2024:db8:128:128::/64``` from ATE to DUT over the IPv4 and IPv6 eBGP session on port-1 with:
        * MED = 50
        * Local Preference = 50
*   Configure IPv4 and IPv6 eBGP between DUT Port-2 and ATE Port-2
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config
    *   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
    *   Advertise ```ipv4-network-2 = 192.168.20.0/24``` and ```ipv6-network-2 = 2024:db8:64:64::/64``` from ATE to DUT over the IPv4 and IPv6 eBGP session on port-2.  The ATE should advertise both prefixes with:
        * MED = 50
        * Local Preference = 50

### RT-1.32.1 [TODO: https://github.com/openconfig/featureprofiles/issues/2615]
#### IPv4, IPv6 eBGP set MED
---
##### Configure a route-policy to set MED
*   Configure an route-policy definition with the name ```med-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set MED as ```100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Add `policy-definition["med-policy"]` to import-policy and export-policy leaf-lists.
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Configure  default policies and remove any import, export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-1
*   Set default import and export policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Remove policy as import and export as a chain/list ```[med-policy]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Verification
*   Verify that policies are successfully applied to the DUT BGP neighbor on ATE Port-2 and default policies are set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Verify that there is no policies applied to the DUT BGP neighbor on ATE Port-1 and default policies are set to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
#### Validate test results
*   Validate that the ATE receives the prefix ```ipv4-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 100
*   Validate that the ATE receives the prefix ```ipv6-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 100
*   Validate that the ATE receives the prefix ```ipv4-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 100
*   Validate that the ATE receives the prefix ```ipv6-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 100

### RT-1.32.2 [TODO: https://github.com/openconfig/featureprofiles/issues/2615]
#### IPv4, IPv6 eBGP increase MED
---
##### Configure a route-policy to increase MED
*   Configure an route-policy definition with the name ```med-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set MED as ```+100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Add `policy-definition["med-policy"]` to import-policy and export-policy leaf-lists.
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Configure  default policies and remove any import, export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-1
*   Set default import and export policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Remove policy as import and export as a chain/list ```[med-policy]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Verification
*   Verify that policies are successfully applied to the DUT BGP neighbor on ATE Port-2 and default policies are set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Verify that there is no policies applied to the DUT BGP neighbor on ATE Port-1 and default policies are set to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
#### Validate test results
*   Validate that the ATE receives the prefix ```ipv4-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 150
*   Validate that the ATE receives the prefix ```ipv6-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 150
*   Validate that the ATE receives the prefix ```ipv4-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 150
*   Validate that the ATE receives the prefix ```ipv6-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 150

### RT-1.32.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2615]
#### IPv4, IPv6 iBGP set Local Preference
---
##### Configure a route-policy to set Local Preference
*   Configure an route-policy definition with the name ```lp-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```lp-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```lp-policy``` statement ```match-statement-1``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```lp-policy``` statement ```match-statement-1``` set Local Preference as ```100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-1
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Add `policy-definition["lp-policy"]` to import-policy and export-policy leaf-lists.
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Configure  default policies and remove any import, export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Remove all import and export policies
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Verification
*   Verify that policies are successfully applied to the DUT BGP neighbor on ATE Port-1 and default policies are set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Verify that there is no policies applied to the DUT BGP neighbor on ATE Port-2 and default policies are set to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
#### Validate test results
*   Validate that the ATE receives the prefix ```ipv4-network-1```  from DUT neighbor on ATE Port-2 and it has LocPref == 100
*   Validate that the ATE receives the prefix ```ipv6-network-1```  from DUT neighbor on ATE Port-2 and it has LocPref == 100
*   Validate that the ATE receives the prefix ```ipv4-network-2```  from DUT neighbor on ATE Port-1 and it has LocPref == 100
*   Validate that the ATE receives the prefix ```ipv6-network-2```  from DUT neighbor on ATE Port-1 and it has LocPref == 100

### RT-1.32.4 [TODO: https://github.com/openconfig/featureprofiles/issues/2615]
#### IPv4, IPv6 eBGP NEXT-STATEMENT
---
##### Configure a route-policy set MED, LocalPreferemce is separate statements
*   Configure an route-policy definition with the name ```flow-control-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```flow-control-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```flow-control-policy``` statement ```match-statement-1``` set policy-result as ```NEXT-STATEMENT```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```flow-control-policy``` statement ```match-statement-1``` set MED to 70
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med
*   For routing-policy ```flow-control-policy``` configure a statement with the name ```match-statement-2```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```flow-control-policy``` statement ```match-statement-2``` set policy-result as ```ACCEPT-ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```flow-control-policy``` statement ```match-statement-2``` prepend as-path with local ASN  ```10``` times
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply as import and export only policy - ```[flow-control-policy]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Configure  default policies and remove any import, export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-1
*   Set default import and export policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Remove all import and export policies
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Verification
*   Verify that policies are successfully applied to the DUT BGP neighbor on ATE Port-2 and default policies are set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Verify that there is no policies applied to the DUT BGP neighbor on ATE Port-1 and default policies are set to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
#### Validate test results
*   Validate that the ATE receives the prefix ```ipv4-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 70 and it has 11 ASN on as-path. All equal to DUT's ASN.
*   Validate that the ATE receives the prefix ```ipv6-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 70 and it has 11 ASN on as-path. All equal to DUT's ASN.
*   Validate that the ATE receives the prefix ```ipv4-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 70 and it has 11 ASN on as-path. All equal to DUT's ASN.
*   Validate that the ATE receives the prefix ```ipv6-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 70 and it has 11 ASN on as-path. All equal to DUT's ASN.

### RT-1.32.5 [TODO: https://github.com/openconfig/featureprofiles/issues/2615]
#### IPv4, IPv6 eBGP prepend 10 x local ASN
---
##### Configure a route-policy to prepend 10
*   Configure an route-policy definition with the name ```prepend-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```prepend-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```prepend-policy``` statement ```match-statement-1``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```prepend-policy``` statement ```match-statement-1``` prepend as-path with local ASN  ```10``` times
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply as import and export only policy - ```[prepend-policy]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Configure  default policies and remove any import, export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-1
*   Set default import and export policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Remove all import and export policies
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Verification
*   Verify that policies are successfully applied to the DUT BGP neighbor on ATE Port-2 and default policies are set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Verify that there is no policies applied to the DUT BGP neighbor on ATE Port-1 and default policies are set to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
#### Validate test results
*   Validate that the ATE receives the prefix ```ipv4-network-1```  from DUT neighbor on ATE Port-2 and it has 11 ASN on as-path. All equal to DUT's ASN.
*   Validate that the ATE receives the prefix ```ipv6-network-1```  from DUT neighbor on ATE Port-2 and it has 11 ASN on as-path. All equal to DUT's ASN.
*   Validate that the ATE receives the prefix ```ipv4-network-2```  from DUT neighbor on ATE Port-1 and it has 11 ASN on as-path. First equial to ATE port-2 ASN and other equal to DUT's ASN.
*   Validate that the ATE receives the prefix ```ipv6-network-2```  from DUT neighbor on ATE Port-1 and it has 11 ASN on as-path. First equial to ATE port-2 ASN and other equal to DUT's ASN.

### RT-1.32.6 [TODO: https://github.com/openconfig/featureprofiles/issues/2615]
#### IPv4, IPv6 eBGP prepend 10 x ASN
---
##### Configure a route-policy to prepend 10
*   Configure an route-policy definition with the name ```prepend-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```prepend-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```prepend-policy``` statement ```match-statement-1``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```prepend-policy``` statement ```match-statement-1``` prepend as-path with ```23456``` ASN  ```10``` times
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repead-n
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply as import and export only policy - ```[prepend-policy]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Configure  default policies and remove any import, export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-1
*   Set default import and export policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Remove all import and export policies
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Verification
*   Verify that policies are successfully applied to the DUT BGP neighbor on ATE Port-2 and default policies are set to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Verify that there is no policies applied to the DUT BGP neighbor on ATE Port-1 and default policies are set to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
#### Validate test results
*   Validate that the ATE receives the prefix ```ipv4-network-1```  from DUT neighbor on ATE Port-2 and it has 11 ASN on as-path. First 10 equal to ```23456``` and last equal to DUT's ASN.
*   Validate that the ATE receives the prefix ```ipv6-network-1```  from DUT neighbor on ATE Port-2 and it has 11 ASN on as-path. First 10 equal to ```23456``` and last equal to DUT's ASN.
*   Validate that the ATE receives the prefix ```ipv4-network-2```  from DUT neighbor on ATE Port-1 and it has 11 ASN on as-path. First equal to ATE port-2 ASN and other 10 equal to ```23456``` ASN.
*   Validate that the ATE receives the prefix ```ipv6-network-2```  from DUT neighbor on ATE Port-1 and it has 11 ASN on as-path. First equal to ATE port-2 ASN and other 10 equal to ```23456``` ASN.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
    ## Config parameter coverage
    /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/enabled:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy:
    /routing-policy/policy-definitions/policy-definition/config/name:
    /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref:
    /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med:
    /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn:
    /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n:
    /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
    /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:

    ## Telemetry parameter coverage
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/default-import-policy:
    /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/default-export-policy:

    ## Protocol/RPC Parameter Coverage
rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Required DUT platform

* FFF
