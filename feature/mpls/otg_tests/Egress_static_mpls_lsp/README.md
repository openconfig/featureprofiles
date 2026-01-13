# TE 9.2 Egress Static MPLS LSP Verification

## Summary

Verify that the Router (DUT) correctly processes incoming MPLS traffic, 
matches the configured static LSP label, pops the label, and forwards the remaining payload 
to the correct egress interface.


## Testbed type:

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)


## Procedure

### A. Router (DUT) Configuration

Enable MPLS: Enable MPLS forwarding globally and on the interfaces connected to ATE1 and ATE2.

* Configure a Static LSP:

MPLS Label: 1000001 for ipv4 payload & 1000002 for ipv6 payload

Action: Pop

Next-hop: IP/IPV6 address of ATE2 interface.

Egress Interface: Interface connected to ATE2.

### B. ATE Configuration (e.g., IXIA)

* Traffic Stream 1 & 2: Configure two flows with the following encapsulation stack:

Layer 2: Ethernet Header

Layer 2.5 (MPLS Label): MPLS Label 1000001 for ipv4 payload and  MPLS Label 1000002 for ipv6 payload

Layer 3: IPv4 and IPV6 Payload


## Pass/Fail Criteria


### Tests:

1. Send IPv4 traffic with MPLS label 1000001 from ATE1 towards ATE2.

2. Send IPv6 traffic with MPLS label 1000002 from ATE1 towards ATE2. 

3. Success criteria: The traffic rate (PPS/Mbps) sent from ATE1 matches the traffic rate received at ATE2 (zero packet loss). 

4. Failure criteria: Any packet loss is detected. 

## Canonical OC:
```json

{
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "default",
        "mpls": {
          "lsps": {
            "static-lsps": {
              "static-lsp": [
                {
                  "name": "STATIC_LSP_1000001",
                  "config": {
                    "name": "STATIC_LSP_1000001"
                  },
                  "ingress": {
                    "config": {
                      "incoming-label": 1000001
                    }
                  },
                  "transit": {
                    "config": {
                      "next-hop": "23.130.0.252"
                    }
                  }
                },
                {
                  "name": "STATIC_LSP_1000002",
                  "config": {
                    "name": "STATIC_LSP_1000002"
                  },
                  "ingress": {
                    "config": {
                      "incoming-label": 1000002
                    }
                  },
                  "transit": {
                    "config": {
                      "next-hop": "2605:ad80:7f:cffe:23:130:0:252"
                    }
                  }
                }
              ]
            }
          }
        }
      }
    ]
  }
}
```


## OpenConfig Path and RPC Coverage

```yaml

## Config paths
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/incoming-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/next-hop:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/push-label:

## State paths
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/incoming-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/next-hop:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/push-label:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform
  * vRX - virtual router device
