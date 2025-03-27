# PF-1.13 - MPLSoGRE IPV4 decapsulation of IPV4/IPV6 payload scale test 

## Summary
This test verifies scaling of MPLSoGRE decapsulation of IP traffic using static MPLS LSP configuration. MPLSoGRE Traffic on ingress to the DUT is decapsulated and IPV4/IPV6 payload is forwarded towards the IPV4/IPV6 egress nexthop.

## Testbed type
* [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure
### Test environment setup

```
DUT has 2 ingress aggregate interfaces and 1 egress aggregate interface.

                         |         | --eBGP-- | ATE Ports 3,4 |
    [ ATE Ports 1,2 ]----|   DUT   |          |               |
                         |         | --eBGP-- | ATE Port 5,6  |
```

Test uses aggregate 802.3ad bundled interfaces (Aggregate Interfaces).

* Ingress Ports: Aggregate2 and Aggregate3
    * Aggregate2 (ATE Ports 3,4) and Aggregate3 (ATE Ports 5,6) are used as the source ports for encapsulated traffic.

* Egress Ports: Aggregate1
    * Traffic is forwarded (egress) on Aggregate1 (ATE Ports 1,2) .

## PF-1.13.1: Generate config for MPLS in GRE decap and push to DUT 
Please generate config using PF-1.5.1

## PF-1.13.2: Verify IPV4/IPV6 traffic scale 
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGRE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGRE header
* Inner payload with all possible DSCP range 0-56 : 
  * Both IPV4 and IPV6 unicast payloads, with random source address, destination address, TCP/UDP source port and destination ports
  * Multicast traffic with random source address, TCP/UDP header with random source and destination ports
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size
* Increase the number of VLANs to 2000 under Aggregate1 with traffic running on the device

Verify:
* All traffic received on Aggregate2 and Aggregate3 gets decapsulated and forwarded as IPV4/IPV6 unicast on the respective egress interfaces under Aggregate1
* No packet loss when forwarding with counters incrementing corresponding to traffic
* Traffic equally load-balanced across member links of the aggregate interfaces
* Inner payload DSCP and TTL values are not altered by the device
* Device can achieve the maximum interface scale on the device
* Entire static label range is usable and functional by sending traffic across the entire label range


## OpenConfig Path and RPC Coverage
TODO: Finalize and update the below paths after the review and testing on any vendor device.

```yaml
paths:
  # Telemetry for GRE decap rule    
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
    
  # Config paths for GRE decap
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gre:

  /network-instances/network-instance/policy-forwarding/policies/policy/policy-counters/state/out-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/sequence-id:

  rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform
  * MFF
  * FFF