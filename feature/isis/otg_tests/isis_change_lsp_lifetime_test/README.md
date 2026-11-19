# RT-2.10: IS-IS change LSP lifetime

## Summary

* Changing the lsp lifetime and verifying isis lsp parameters

## Canonical OC

```json
{
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "config": {
                "identifier": "openconfig-policy-types:ISIS",
                "name": "DEFAULT"
              },
              "identifier": "openconfig-policy-types:ISIS",
              "isis": {
                "global": {
                  "afi-safi": {
                    "af": [
                      {
                        "afi-name": "openconfig-isis-types:IPV4",
                        "config": {
                          "afi-name": "openconfig-isis-types:IPV4",
                          "enabled": true,
                          "safi-name": "openconfig-isis-types:UNICAST"
                        },
                        "safi-name": "openconfig-isis-types:UNICAST"
                      },
                      {
                        "afi-name": "openconfig-isis-types:IPV6",
                        "config": {
                          "afi-name": "openconfig-isis-types:IPV6",
                          "enabled": true,
                          "safi-name": "openconfig-isis-types:UNICAST"
                        },
                        "safi-name": "openconfig-isis-types:UNICAST"
                      }
                    ]
                  },
                  "config": {
                    "level-capability": "LEVEL_2"
                  },
                  "timers": {
                    "config": {
                      "lsp-lifetime-interval": 500,
                      "lsp-refresh-interval": 60
                    }
                  }
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

## Topology

* ATE:port1 <-> port1:DUT:port2 <-> ATE:port2

## Procedure

    * Configure IS-IS for ATE port-1 and DUT port-1.
    * Modify the default lifetime of the LSP PDU.
    * The default lifetime of the LSP PDU is 1200 seconds.
        This parameter can be updated using the LSP lifetime parameter.
        LSP lifetime indicates how long the LSP PDU originated by the DUT should remain in the network. 
        The DUT regenerates the LSP PDU typically ~300 seconds before its expiration.
    * Change the LSP lifetime to 500 seconds.
    * Configure the LSP refresh interval to 60 seconds so that LSP regeneration can be verified deterministically.
    * Verify that IS-IS adjacency for IPv4 and IPV6 address family is coming up.
    * Verify that IPv4 and IPv6 prefixes that are advertised by ATE correctly installed into DUTs route and forwarding table.
    * Verify that the updated LSP lifetime is reflected in isis database output.
    * Verify that the remaining lifetime of the lsp is remaining lifetime = configured lifetime - time passed since the LSP PDU generation.
    * Verify that once the new LSP PDU is generated the sequence number and checksum of the new LSP PDU is updated

## OpenConfig Path and RPC Coverage
```yaml
paths:
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/isis/global/timers/config/lsp-lifetime-interval:
  /network-instances/network-instance/protocols/protocol/isis/global/timers/config/lsp-refresh-interval:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/isis/global/timers/state/lsp-lifetime-interval:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/state/remaining-lifetime:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```
