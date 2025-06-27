# PF-1.6: Policy forwarding of GUE tunnel traffic to default and non-default network-instance

## Summary
This test ensures NOS is able to host multiple VRFs, perform GUE tunnel traffic in the default VRF and also allows for gradual traffic migration from Default to Non-Default VRF using VRF selection policy.


## Test environment setup

### Topology
Create the following connections:
```mermaid
graph LR; 
subgraph ATE1 [ATE1]
    A1[Port1]
end
subgraph DUT1 [DUT1]
    B1[Port1]
    B2[Port2-default VRF]
    B3[Port3-nondefault VRF]
end 
subgraph ATE2 [ATE2]
    C1[Port1]
    C2[Port2]
end
B1 <--> B2;
B1 <--> B3;
A1 <-- IBGP(ASN100) & ISIS --> B1;
B2 <-- EBGP(ASN100:ASN200) --> C1;
B3 <-- EBGP(ASN100:ASN200)  --> C2;
```

### Configuration generation of DUT and ATE

#### DUT Configuration
* Configure ISIS[Level2] and IBGP[ASN100] as described in topology between ATE:Port1 and DUT:Port1
* Configure EBGP[ASN200] between DUT1:Port2 and ATE2:Port1
* Configure EBGP[ASN200] between DUT1:Port3 and ATE2:Port2
* Configure route leaking from the default VRF and non-default VRF and vice versa.
* Configure a policy based traffic steering from default to Non Default VRF, this policy should be able to steer the traffic from Default VRF to non default VRF and vice versa based on the destination IP/IPV6 address.
* DUT has the following VRF selection policy initially
    * Statement1: traffic matching IPv4Prefix1/24, Punt to default vrf
    * Statement2: traffic matching IPv4Prefix2/24, Punt to default vrf
    * Statement3: traffic matching IPv6Prefix3/64, Punt to default vrf
    * Statement4: traffic matching IPv6Prefix4/64, Punt to default vrf
    * DUT must also leak all the routes from the Default VRF to the non-default VRF

#### ATE Configuration
* Configure ISIS[Level2] & IBGP[ASN100] on ATE1:Port1
* Configure EBGP[ASN200] on ATE2:Port1 & ATE2:Port2

### Configure ATE Route Advertisements & Traffic Flows as below:
#### ATE Route Advertisements:

	ATE2:Port1 advertises following prefixes to DUT1:Port2 over EBGP
    - IPv4Prefix1/24
    - IPv4Prefix2/24
    - IPv6Prefix3/64
    - IPv6Prefix4/64

	ATE2:Port2 advertieses following prefixes to DUT1:Port3 over EBGP
    - IPv4Prefix1/24
    - IPv4Prefix2/24
    - IPv6Prefix3/64
    - IPv6Prefix4/64

#### ATE traffic Flows:

	From ATE1:Port1 to ATE2 destination prefixes
    - IPv4Prefix1/24 at a rate of 100 packets/sec
    - IPv4Prefix2/24 at a rate of 100 packets/sec
    - IPv6Prefix3/64 at a rate of 100 packets/sec 
    - IPv6Prefix4/64 at a rate of 100 packets/sec


## Procedure
### PF-1.6.1: [Baseline] Default VRF for all flows with regular traffic profile

#### In this case DUT1:Port1 sends the regular flows to ATE2:Port1.
  * ATE2:Port2 receives following IPv4 and IPv6 flows:
    * IPv4Prefix1/24
    * IPv4Prefix2/24
    * IPv6Prefix3/64
    * IPv6Prefix4/64

    * Expectations:
      * All traffic must be successful and there should be 0 packet loss.
      * Need to verify the packets sent by sender tester is equal to the packets on receiving tester port and also should be equal to the sum of packets seen in default.

### PF-1.6.2: Traffic from ATE1 to ATE2 Prefix 1 migrated to Non-Default VRF using the VRF selection policy
  * ATE1:Port1 sends following IPv4 and IPv6 flows:
    * IPv4Prefix1/24
    * IPv4Prefix2/24
    * IPv6Prefix3/64
    * IPv6Prefix4/64
   
  * VRF selection policy on DUT1:Port2 changes as follows: 
    * Statement1: traffic matching IPv4Prefix1/24, Punt to non-default vrf
    * Statement2: traffic matching IPv4Prefix2/24, Punt to default vrf
    * Statement3: traffic matching IPv6Prefix3/64, Punt to default vrf
    * Statement4: traffic matching IPv6Prefix4/64, Punt to default vrf

  * Expectations:
    * To validate the prefixes advertised by ATE2:Port1 and ATE2:Port2 are received on ATE1:Port1. 
    * Traffic for Prefix 1 received from ATE1:Port1 once punted to non-defailt VRF by the VRF selection policy, must be received by ATE2:Port2
    * Traffic for rest of the prefixes sent by ATE1:Port1 must be routed to ATE2:Port1 via the DEFAULT VRF in the DUT.
    * Need to verify the packets sent by sender tester is equal to the packets on receiving tester ports and also should be equal to the sum of packets seen in default & non default VRF.
    * There should be 0 packet loss. <br><br><br>

### PF-1.6.3: Traffic from ATE1 to ATE2 Prefix 1 migrated to Non-Default VRF using the VRF selection policy
  * ATE1:Port1 sends following IPv4 and IPv6 flows:
    * IPv4Prefix1/24
    * IPv4Prefix2/24
    * IPv6Prefix3/64
    * IPv6Prefix4/64
   
  * VRF selection policy on DUT1:Port2 changes as follows: 
    * Statement1: traffic matching IPv4Prefix1/24, Punt to non-default vrf
    * Statement2: traffic matching IPv4Prefix2/24, Punt to non-default vrf
    * Statement3: traffic matching IPv6Prefix3/64, Punt to default vrf
    * Statement4: traffic matching IPv6Prefix4/64, Punt to default vrf

  * Expectations:
    * To validate the prefixes advertised by ATE2:Port1 and ATE2:Port2 are received on ATE1:Port1 . 
    * Traffic for Prefix 1 & 2 received from ATE1:Port1 once punted to non-defailt VRF by the VRF selection policy, must be received by ATE2:Port2
    * Traffic for rest of the prefixes sent by ATE1:Port1 must be routed to ATE2:Port1 via the DEFAULT VRF in the DUT.
    * Need to verify the packets sent by sender tester is equal to the packets on receiving tester ports and also should be equal to the sum of packets seen in default & non default VRF.
    * There should be 0 packet loss. <br><br><br>

### PF-1.6.4: Traffic from ATE1 to ATE2 Prefix 1 migrated to Non-Default VRF using the VRF selection policy
  * ATE1:Port1 sends following IPv4 and IPv6 flows:
    * IPv4Prefix1/24
    * IPv4Prefix2/24
    * IPv6Prefix3/64
    * IPv6Prefix4/64
   
  * VRF selection policy on DUT1:Port2 changes as follows: 
    * Statement1: traffic matching IPv4Prefix1/24, Punt to non-default vrf
    * Statement2: traffic matching IPv4Prefix2/24, Punt to non-default vrf
    * Statement3: traffic matching IPv6Prefix3/64, Punt to non-default vrf
    * Statement4: traffic matching IPv6Prefix4/64, Punt to default vrf

  * Expectations:
    * To validate the prefixes advertised by ATE1:Port1 are received on ATE2:Port1 and ATE2:Port2. 
    * Traffic for Prefix 1,2 & 3 received from ATE1:Port1 once punted to non-defailt VRF by the VRF selection policy, must be received by ATE2:Port2
    * Traffic for rest of the prefixes sent by ATE1:Port1 must be routed to ATE2:Port1 via the DEFAULT VRF in the DUT.
    * Need to verify the packets sent by sender tester is equal to the packets on receiving tester ports and also should be equal to the sum of packets seen in default & non default VRF.
    * There should be 0 packet loss. <br><br><br>

### PF-1.6.5: Traffic from ATE1 to ATE2 Prefix 1 migrated to Non-Default VRF using the VRF selection policy
  * ATE1:Port1 sends following IPv4 and IPv6 flows:
    * IPv4Prefix1/24
    * IPv4Prefix2/24
    * IPv6Prefix3/64
    * IPv6Prefix4/64
   
  * VRF selection policy on DUT1:Port2 changes as follows: 
    * Statement1: traffic matching IPv4Prefix1/24, Punt to non-default vrf
    * Statement2: traffic matching IPv4Prefix2/24, Punt to non-default vrf
    * Statement3: traffic matching IPv6Prefix3/64, Punt to non-default vrf
    * Statement4: traffic matching IPv6Prefix4/64, Punt to non-default vrf

  * Expectations:
    * To validate the prefixes advertised by ATE1:Port1 are received on ATE2:Port1 and ATE2:Port2. 
    * Traffic for all Prefixes received from ATE1:Port1 once punted to non-defailt VRF by the VRF selection policy, must be received by ATE2:Port2
    * No traffic should be routed to ATE2:Port1 via the DEFAULT VRF in the DUT in this case.
    * Need to verify the packets sent by sender tester is equal to the packets on receiving tester ports and also should be equal to the sum of packets seen in default & non default VRF.
    * There should be 0 packet loss. <br><br><br>


## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Set:
    /network-instances/network-instance/name
    /network-instances/network-instance/config
    /network-instances/network-instance/config/name
    /network-instances/network-instance/config/type
    /network-instances/network-instance/config/description
    /network-instances/network-instance/config/router-id
    /network-instances/network-instance/config/route-distinguisher
    /network-instances/network-instance/policy-forwarding/interfaces/interface/configa
    /network-instances/network-instance/policy-forwarding/interfaces/interface/config/interface-id
    /network-instances/network-instance/policy-forwarding/interfaces/interface/config/apply-vrf-selection-policy

    
    gNMI.Get:

    /network-instances/network-instance/state
    /network-instances/network-instance/policy-forwarding/interfaces/interface/state/apply-vrf-selection-policy
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv4/state/dscp-set
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/state/dscp-set
    gNMI.Subscribe:
```