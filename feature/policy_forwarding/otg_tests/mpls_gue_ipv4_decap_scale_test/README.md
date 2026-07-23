# PF-1.20: MPLSoGUE IPV4 decapsulation of IPV4/IPV6 payload scale test 

## Summary
This test verifies scaling of MPLSoGUE decapsulation of IP traffic using static MPLS LSP configuration. MPLSoGUE Traffic on ingress to the DUT is decapsulated and IPV4/IPV6 payload is forwarded towards the IPV4/IPV6 egress nexthop.

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

## PF-1.20.1: Generate config for MPLS in GRE decap and push to DUT 
Please generate config using PF-1.5.1

## PF-1.20.2: Verify IPV4/IPV6 traffic scale 
Generate traffic on ATE Ports 3,4,5,6 having:
* Outer source address: random combination of 1000+ IPV4 source addresses from 100.64.0.0/22
* Outer destination address: Traffic must fall within the configured IPV4 unicast decap prefix range for MPLSoGUE traffic on the device
* MPLS Labels: Configure streams that map to every egress interface by having associated IPV4/IPV6/Multicast static MPLS labels in the MPLSoGUE header
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

## PF-1.20.v6: Validate scaled decapsulation of MPLS over GUE with 1000 unique IPv6 outer header flows

### Test setup
Connect DUT to ATE. Ensure ATE is capable of generating thousands of unique traffic streams.
The test topology uses the same ingress and egress aggregate interfaces as the IPv4 scale test.

### Configuration
Utilize a `gNMI Set` operation to program 1000 unique Policy-Based Routing (PBR) decapsulation rules. Each rule should match a unique IPv6 outer header. Alternatively, configure a single rule matching a `/64` IPv6 prefix that routes to a decapsulation loopback/interface, depending on platform support.
Ensure the routing table contains valid routes for the inner payloads to egress out of the correct egress interface (Aggregate1).

## Canonical OC
```json
{
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "default",
        "policy-forwarding": {
          "policies": {
            "policy": [
              {
                "policy-id": "gue-decap-scale-v6",
                "config": {
                  "policy-id": "gue-decap-scale-v6",
                  "type": "PBR_POLICY"
                },
                "rules": {
                  "rule": [
                    {
                      "sequence-id": 1,
                      "config": {
                        "sequence-id": 1
                      },
                      "ipv6": {
                        "config": {
                          "source-address": "2001:db8:1::1/128",
                          "protocol": "openconfig-packet-match-types:IP_UDP"
                        }
                      },
                      "action": {
                        "config": {
                          "decapsulate-mpls-in-udp": true
                        }
                      }
                    }
                  ]
                }
              }
            ]
          }
        }
      }
    ]
  }
}
```

### Telemetry
Establish a `gNMI Subscribe` to monitor device health and egress packet rates:
*   Subscribe to `/system/state/memory/utilization` to track memory usage.
*   Subscribe to `/system/cpus/cpu/state/total-utilization` to track CPU utilization across all cores.
*   Subscribe to egress interface packet rate counters on the egress ports (e.g., Aggregate1 member links).

### Traffic execution
1.  ATE generates 1000 simultaneous traffic flows from the ingress aggregate interfaces (Aggregate2 and Aggregate3).
2.  Each flow must have a unique outer IPv6 Source Address, unique UDP source port, and unique inner payload IP and UDP source port (varying within the ephemeral range 49152-65535).
3.  The total line rate for the combined streams should be at least 50% of the ingress port capacity.

### Pass/Fail Criteria
*   **Pass Criteria:**
    *   `gNMI Set` configures all 1000 flows successfully without timing out.
    *   DUT successfully decapsulates all 1000 flows simultaneously.
    *   ATE reports 0% packet loss for all generated streams.
    *   Telemetry indicates that CPU and memory utilization remain within safe, stable thresholds (e.g., `<80%`) during the scale operation.
*   **Fail Criteria:**
    *   `gNMI Set` rejects the configuration due to hardware resource or TCAM exhaustion for the IPv6 decapsulation rules.
    *   Packet loss is observed on any specific streams, indicating incomplete hardware programming or processing failures for scaled traffic.

## OpenConfig Path and RPC Coverage
TODO: Finalize and update the below paths after the review and testing on any vendor device.

```yaml
paths:
  # Telemetry for GRE decap rule    
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-pkts:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/matched-octets:
    
  # Config paths for GRE decap
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-mpls-in-udp:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/source-address:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/ipv6/config/protocol:

  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/packets-forwarded:
  /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/octets-forwarded:
  /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/state/sequence-id:

  # Telemetry for device health monitoring
  /system/state/memory/utilization:
  /system/cpus/cpu/state/total-utilization:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

FFF
