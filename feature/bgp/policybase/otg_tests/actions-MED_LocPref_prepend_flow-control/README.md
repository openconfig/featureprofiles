# RT-1.XX: BGP policy actions - MED, LocPref, prepend, flow-control

## Summary

- Verify abilty to set MED to fixed value in export and import policy
- Verify abilty to increment MED by fixed value in export and import policy
- Verify abilty to set Local Preference to fixed value in export and import policy
- Verify abilty to prepend AS path with 10 additional repetitions of local ASN in export and import policy
- Verify abilty to prepend AS path with 10 additional repetitions of configured ASN in export and import policy
- Applicable to both IPv4 and IPv6 BGP neighbors

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

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
    *   Advertise ```ipv4-network-2 = 192.168.20.0/24``` and ```ipv6-network-2 = 2024:db8:64:64::/64``` from ATE to DUT over the IPv4 and IPv6 eBGP session on port-2 with:
        * MED = 50
        * Local Preference = 50

### RT-1.XX.1 []
#### IPv4, IPv6 iBGP set MED
---
##### Configure a route-policy to set MED
*   Configure an route-policy definition with the name ```med-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set MED as ```100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/med
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-1
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply policy as import and export as a chain/list ```[med-policy]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Configure  default policies for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
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
*   Validate that the ATE receives the prefix ```ipv4-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 100
*   Validate that the ATE receives the prefix ```ipv6-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 100
*   Validate that the ATE receives the prefix ```ipv4-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 100
*   Validate that the ATE receives the prefix ```ipv6-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 100

### RT-1.XX.2 []
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
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/med
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply policy as import and export as a chain/list ```[med-policy]```
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

### RT-1.XX.3 []
#### IPv4, IPv6 iBGP increase MED
---
##### Configure a route-policy to set MED
*   Configure an route-policy definition with the name ```med-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set MED as ```+100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/med
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-1
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply policy as import and export as a chain/list ```[med-policy]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Configure  default policies and remove any import, export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Remove policy as import and export as a chain/list ```[med-policy]```
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
*   Validate that the ATE receives the prefix ```ipv4-network-1```  from DUT neighbor on ATE Port-1 and it has MED == 150
*   Validate that the ATE receives the prefix ```ipv6-network-1```  from DUT neighbor on ATE Port-1 and it has MED == 150
*   Validate that the ATE receives the prefix ```ipv4-network-2```  from DUT neighbor on ATE Port-2 and it has MED == 150
*   Validate that the ATE receives the prefix ```ipv6-network-2```  from DUT neighbor on ATE Port-2 and it has MED == 150
  
### RT-1.XX.4 []
#### IPv4, IPv6 eBGP increase MED
---
##### Configure a route-policy to set MED
*   Configure an route-policy definition with the name ```med-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set MED as ```+100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/med
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply policy as import and export as a chain/list ```[med-policy]```
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

### RT-1.XX.5 []
#### IPv4, IPv6 iBGP set LocalPreference
---
##### Configure a route-policy to set MED
*   Configure an route-policy definition with the name ```lp-policy```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy``` configure a statement with the name ```match-statement-1```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   For routing-policy ```med-policy``` statement ```match-statement-1``` set Local Preference as ```100```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/med
##### Configure  bgp import and export policy for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-1
*   Set default import and export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply policy as import and export as a chain/list ```[med-policy]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy
##### Configure  default policies for the DUT IPv4 and IPv6 BGP neighbors on ATE Port-2
*   Set default import and export policy to ```ACCEPT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
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
*   Validate that the ATE receives the prefix ```ipv4-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 100
*   Validate that the ATE receives the prefix ```ipv6-network-1```  from DUT neighbor on ATE Port-2 and it has MED == 100
*   Validate that the ATE receives the prefix ```ipv4-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 100
*   Validate that the ATE receives the prefix ```ipv6-network-2```  from DUT neighbor on ATE Port-1 and it has MED == 100


### RT-1.XX.6 []
#### IPv4, IPv6 eBGP set LocalPreference

### RT-1.XX.7 []
#### IPv4, IPv6 iBGP prepend 10 x local ASN

### RT-1.XX.8 []
#### IPv4, IPv6 eBGP prepend 10 x local ASN

### RT-1.XX.9 []
#### IPv4, IPv6 iBGP prepend 10 x ASN

### RT-1.XX.10 []
#### IPv4, IPv6 eBGP prepend 10 x ASN

### RT-1.XX.11 []
#### IPv4, IPv6 iBGP NEXT-STATEMENT

### RT-1.XX.12 []
#### IPv4, IPv6 eBGP NEXT-STATEMENT

### RT-1.XX.13 []
#### IPv4, IPv6 iBGP NEXT-REJECT

### RT-1.XX.14 []
#### IPv4, IPv6 eBGP NEXT-REJECT

### RT-1.XX.15 []
#### IPv4, IPv6 iBGP NEXT-ACCEPT

### RT-1.XX.16 []
#### IPv4, IPv6 eBGP NEXT-ACCEPT

----------------------------------
##### Configure a route-policy to prepend AS-PATH
*   Configure an IPv4 route-policy definition with the name ```asp-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```asp-policy-v4``` configure a statement with the name ```asp-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```asp-policy-v4``` statement ```asp-statement-v4``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to prepend AS
*   For routing-policy ```asp-policy-v4``` statement ```asp-statement-v4``` set AS-PATH prepend to the ASN of the DUT
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn

##### Configure another route-policy to set the MED
*   Configure an IPv4 route-policy definition with the name ```med-policy-v4```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy-v4``` configure a statement with the name ```med-statement-v4```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy-v4``` statement ```med-statement-v4``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set MED
*   For routing-policy ```med-policy-v4``` statement ```med-statement-v4``` set MED to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med

##### Configure chained bgp export policies for the DUT BGP neighbor on ATE Port-1
*   Set default export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply both the policies as a chain/list ```[asp-policy-v4, med-policy-v4]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy

##### Verification
*   Verify that chained export policies are successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy

##### Validate test results
*   Validate that the ATE receives the prefix ``ipv4-network-2``` i.e. ```192.168.20.0/24``` from BGP neighbor on DUT Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ``ipv4-network-2``` i.e. ```192.168.20.0/24``` on ATE from BGP neighbor on DUT Port-1 has AS-PATH with the ASN of DUT occuring twice
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
*   Validate that the prefix ``ipv4-network-2``` i.e. ```192.168.20.0/24``` from BGP neighbor on DUT Port-1 has MED set to ```1000```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

----------------------------



### RT-1.29.3 [TODO: https://github.com/openconfig/featureprofiles/issues/2594]
#### IPv6 BGP chained import policy test
---
##### Configure a route-policy to match the prefix
*   Configure an IPv6 route-policy definition with the name ```match-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```match-policy-v6``` configure a statement with the name ```match-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```match-policy-v6``` statement ```match-statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure a prefix-set for route filtering/matching
*   Configure a prefix-set with the name ```prefix-set-v6``` and mode ```IPV6```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   For prefix-set ```prefix-set-v6``` set the ip-prefix to ```ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` and masklength to ```exact```
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
##### Attach the prefix-set to route-policy
*   For routing-policy ```match-policy-v6``` statement ```match-statement-v6``` set match options to ```ANY```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   For routing-policy ```match-policy-v6``` statement ```match-statement-v6``` set prefix set to ```prefix-set-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set

##### Configure another route-policy to set the local preference
*   Configure an IPv6 route-policy definition with the name ```lp-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```lp-policy-v6``` configure a statement with the name ```lp-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```lp-policy-v6``` statement ```lp-statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set local-pref
*   For routing-policy ```lp-policy-v6``` statement ```lp-statement-v6``` set local-preference to ```200```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-local-pref

##### Configure chained bgp import policies for the DUT BGP neighbor on ATE Port-2
*   Set default import policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   Apply both the policies as a chain/list ```[match-policy-v6, lp-policy-v6]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy

##### Verification
*   Verify that chained import policies are successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy

##### Validate test results
*   Validate that the DUT receives the prefix ``ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` from BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ``ipv6-network-1``` i.e. ```2024:db8:128:128::/64``` from BGP neighbor on ATE Port-1 has local preference set to 200
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

### RT-1.29.4 [TODO: https://github.com/openconfig/featureprofiles/issues/2594]
#### IPv6 BGP chained export policy test
---
##### Configure a route-policy to prepend AS-PATH
*   Configure an IPv6 route-policy definition with the name ```asp-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```asp-policy-v6``` configure a statement with the name ```asp-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```asp-policy-v6``` statement ```asp-statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to prepend AS
*   For routing-policy ```asp-policy-v6``` statement ```asp-statement-v6``` set AS-PATH prepend to the ASN of the DUT
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn

##### Configure another route-policy to set the MED
*   Configure an IPv6 route-policy definition with the name ```med-policy-v6```
    *   /routing-policy/policy-definitions/policy-definition/config/name
*   For routing-policy ```med-policy-v6``` configure a statement with the name ```med-statement-v6```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   For routing-policy ```med-policy-v6``` statement ```med-statement-v6``` set policy-result as ```ACCEPT_ROUTE```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
##### Configure BGP actions to set MED
*   For routing-policy ```med-policy-v6``` statement ```med-statement-v6``` set MED to ```1000```
    *   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med

##### Configure chained bgp export policies for the DUT BGP neighbor on ATE Port-1
*   Set default export policy to ```REJECT_ROUTE```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   Apply both the policies as a chain/list ```[asp-policy-v6, med-policy-v6]```
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy

##### Verification
*   Verify that chained export policies are successfully applied to the DUT BGP neighbor on ATE Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy

##### Validate test results
*   Validate that the ATE receives the prefix ``ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` from BGP neighbor on DUT Port-1
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   Validate that the prefix ``ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` on ATE from BGP neighbor on DUT Port-1 has AS-PATH with the ASN of DUT occuring twice
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
*   Validate that the prefix ``ipv6-network-2``` i.e. ```2024:db8:64:64::/64``` from BGP neighbor on DUT Port-1 has MED set to ```1000```
    *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med


## Config parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/global/config
*   /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/config/
*   /routing-policy/policy-definitions/policy-definition/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/config/name
*   /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result
*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
*   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
*   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options
*   /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-import-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/default-export-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy

## Telemetry parameter coverage

*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy
*   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy
*   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
*   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/prefix
*   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/prefix
*   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med

## Protocol/RPC Parameter Coverage

* gNMI
  * Get
  * Set

## Required DUT platform

* FFF
