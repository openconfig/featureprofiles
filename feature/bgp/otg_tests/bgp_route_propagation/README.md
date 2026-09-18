# RT-1.108: Two-Tier EdgeBGP Route Propagation and Attribute Transparency

## Summary

Validate end-to-end route propagation across the two-tier EdgeBGP architecture: Customer Edge (CE) $\leftrightarrow$ Edge Router (DUT/anPF) $\leftrightarrow$ Cloud Router (VRC). Verify full path attribute transparency across boundaries, including preservation of customer Multi-Exit Discriminator (MED), passing of cloud priority metrics from core to customer via PE metric-preservation policies, and AS-Path integrity during multiprotocol L3VPN encapsulation.

> [!NOTE]
> Correlates with Google internal trackers [b/535295388](http://b/535295388) and [b/494322099](http://b/494322099).
> Synthesized from CSE EdgeBGP LLD and KNE runbook validations (`test_rp_1_1`, `test_rp_1_5`, `test_rp_1_6`, `test_rp_1_9`, `test_rp_1_10`).

## Testbed type

* `TESTBED_DUT_ATE_4LINKS` (ATE Port 1 simulating Customer CE1; ATE Port 2 simulating Customer CE2; ATE Port 3 simulating Cloud Router VRC1; ATE Port 4 simulating Cloud Router VRC2).

## Topology

```mermaid
graph LR;
  ce1[ATE Port 1: CE1 (AS 65001)] -- "eBGP (VRF ce1)" --> dut1[DUT Port 1]
  ce2[ATE Port 2: CE2 (AS 65002)] -- "eBGP (VRF ce1)" --> dut2[DUT Port 2]
  dut3[DUT Port 3] -- "iBGP MP-BGP (AS 64500)" --> vrc1[ATE Port 3: VRC1]
  dut4[DUT Port 4] -- "iBGP MP-BGP (AS 64500)" --> vrc2[ATE Port 4: VRC2]
```

## Procedure

### Test environment setup

1. Configure tenant VRF `ce1` on DUT with assigned Route Distinguisher (e.g. `64500:100`) and Route Targets (`target:64500:100`).
2. Establish eBGP peering on DUT Port 1 (`192.0.2.1/30`) with ATE CE1 (`192.0.2.2`, AS 65001) in VRF `ce1`.
3. Establish eBGP peering on DUT Port 2 (`192.0.2.5/30`) with ATE CE2 (`192.0.2.6`, AS 65002) in VRF `ce1`.
4. Establish dynamic or static iBGP peering over IPv6 transit on DUT Port 3/4 with ATE VRC1/VRC2 in default/transit VRF with `vpn-ipv4` and `vpn-ipv6` enabled.
5. Apply metric preservation policy on DUT:
   - Ingress eBGP: preserve customer-signaled MED when translating into VPNv4/VPNv6 updates.
   - Egress eBGP: pass received core MED to customer CE via metric preservation policy (`set metric +0` or policy-definition `set-med`).

### RT-1.108.1 - Ingress Route Propagation (Customer CE to Cloud Router)

* **Step 1**: ATE CE1 advertises IPv4 prefix `198.51.100.0/24` with AS-Path `[65001]` and IPv6 prefix `2001:db8:10::/64` with AS-Path `[65001]`.
* **Step 2**: Verify DUT receives routes in VRF `ce1` Adj-RIB-In.
* **Step 3**: Verify DUT encapsulates routes into MP-BGP L3VPN updates and transmits them over iBGP to ATE VRC1.
* **Step 4**: Verify ATE VRC1 receives both prefixes with:
  - Preserved AS-Path: `[65001]`.
  - Correct RD: `64500:100`.
  - Matching Route Target: `target:64500:100`.

### RT-1.108.2 - Egress Route Propagation (Cloud Router to Customer CE)

* **Step 1**: ATE VRC1 advertises core VPC prefix `203.0.113.0/24` and `2001:db8:250::/64` with RD `64500:100` and RT `target:64500:100` to DUT over iBGP.
* **Step 2**: Verify DUT imports the VPN routes into tenant VRF `ce1`.
* **Step 3**: Verify DUT advertises `203.0.113.0/24` and `2001:db8:250::/64` to ATE CE1 over eBGP.
* **Step 4**: Verify ATE CE1 receives the prefix with DUT's autonomous system (`64500`) prepended to the AS-Path.

### RT-1.108.3 - Customer MED Preservation (eBGP to iBGP)

* **Step 1**: ATE CE1 advertises prefix `198.51.100.0/25` and `2001:db8::cafe/128` with explicit **MED = 200**.
* **Step 2**: Query DUT telemetry at `/network-instances/network-instance[name=ce1]/protocols/protocol[identifier=openconfig-policy-types:BGP][name=BGP]/bgp/rib/attr-sets/attr-set/state/med` (referenced by route entry under `/network-instances/network-instance[name=ce1]/protocols/protocol[identifier=openconfig-policy-types:BGP][name=BGP]/bgp/rib/afi-safis/afi-safi[afi-safi-name=openconfig-bgp-types:IPV4_UNICAST]/ipv4-unicast/neighbors/neighbor[neighbor-address=192.0.2.2]/adj-rib-in-post/routes/route/state/attr-index`); verify MED value is `200`.
* **Step 3**: Monitor MP-BGP L3VPN update received at ATE VRC1; verify the MED attribute value `200` is retained unaltered in the update.

### RT-1.108.4 - Cloud Priority (MED) Passing to Customer (iBGP to eBGP)

* **Step 1**: ATE VRC1 advertises IPv4 prefix `203.0.113.0/24` and IPv6 prefix `2001:db8:250::/64` with **MED = 123** representing GCP Cloud Router active/backup priority.
* **Step 2**: Verify DUT receives MED `123` on the iBGP session for both IPv4 and IPv6 prefixes.
* **Step 3**: Verify DUT propagates both IPv4 and IPv6 updates to ATE CE1 retaining **MED = 123** on the egress eBGP updates (preventing reset to 0).

### RT-1.108.5 - Multiple Paths / Add-Paths with Disparate AS Paths

* **Step 1**: ATE CE1 advertises prefix `198.51.100.128/25` with AS-Path `[65001, 65010]`.
* **Step 2**: ATE CE2 advertises the same prefix `198.51.100.128/25` with alternate AS-Path `[65002, 65020, 65030]`.
* **Step 3**: Verify DUT installs both paths in BGP RIB when BGP multipath/add-path is enabled.
* **Step 4**: Verify best-path selection favors CE1 (shorter AS-Path length).
* **Step 5**: Send continuous data plane traffic destined to `198.51.100.128/25` and withdraw the prefix from ATE CE1; validate sub-second cutover to the CE2 path by measuring packet loss and verifying total traffic loss duration is less than 1 second.

## Canonical OC

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "PRESERVE-MED",
          "config": {
            "name": "PRESERVE-MED"
          }
        },
        {
          "name": "PASS-CLOUD-MED",
          "config": {
            "name": "PASS-CLOUD-MED"
          }
        }
      ]
    }
  },
  "network-instances": {
    "network-instance": [
      {
        "name": "ce1",
        "config": {
          "name": "ce1",
          "type": "openconfig-network-instance-types:L3VRF",
          "route-distinguisher": "64500:100"
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP",
              "bgp": {
                "global": {
                  "config": {
                    "as": 64500
                  }
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": {
                        "neighbor-address": "192.0.2.2",
                        "peer-as": 65001
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": ["PRESERVE-MED"],
                          "export-policy": ["PASS-CLOUD-MED"]
                        }
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # BGP RIB Attributes and Routes
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-out-post/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```

## Required DUT platform

* FFF
