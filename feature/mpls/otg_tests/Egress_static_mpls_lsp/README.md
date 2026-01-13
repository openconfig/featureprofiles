# TE 9.2 Egress Static MPLS LSP Verification

## Summary

Verify that the Router (DUT) correctly processes incoming MPLS traffic, 
matches the configured static LSP label, pops the label, and forwards the remaining payload (with the next label in the stack) 
to the correct egress interface connected to the ATE.


## Testbed type:

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)


## Procedure

### A. Router (DUT) Configuration
Enable MPLS: Enable MPLS forwarding globally and on the interfaces connected to ATE1 and ATE2.

* Configure a Static LSP:

In-Label: 1000001 for ipv4 payload & 1000002 for ipv6 payload

Action: Pop

Next-hop: IP/IPV6 address of ATE2 interface.

Egress Interface: Interface connected to ATE2.

### B. ATE Configuration (e.g., IXIA)

* Traffic Stream 1 & 2: Configure two flows with the following encapsulation stack:

Layer 2: Ethernet Header

Layer 2.5 (Inner Label): MPLS Label 1000001 for ipv4 payload and  MPLS Label 1000002 for ipv6 payload

Layer 3: IPv4 and IPV6 Payload


## Pass/Fail Criteria


### Tests:

1. Send traffic from ATE1 towards ATE2 , ipv4 traffic with mpls label 1000001 

2. Send traffic from ATE1 towards ATE2 ,  ipv6 traffic with label 1000002 

3. Success criteria: The traffic rate (PPS/Mbps) sent from ATE1 matches the traffic rate received at ATE2 (zero packet loss). 

4. Failure criteria: Any packet loss is detected. 

## OC paths in json format:
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
paths:
  ## Config paths
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/config/incoming-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/lsp-next-hops/lsp-next-hop/index:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/lsp-next-hops/lsp-next-hop/config/index:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/lsp-next-hops/lsp-next-hop/config/ip-address:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/lsp-next-hops/lsp-next-hop/config/interface:

  ## State paths
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/state/incoming-label:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/lsp-next-hops/lsp-next-hop/state/ip-address:
  /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/egress/lsp-next-hops/lsp-next-hop/state/interface:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform
  * vRX - virtual router device
