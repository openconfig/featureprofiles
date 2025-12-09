# TestID-16.4: gRIBI to BGP Route Redistribution for IPv4

## Summary

This test validates the gRIBI route redistribution from gRIBI to BGP for IPv4 in a network instance.

## Testbed type

* Specify the .testbed topology file from the
[TESTBED_DUT_ATE_2 LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

* DUT and ATE ports will be connected and configured as following:
  * DUT Port 1 (192.0.2.1/30) <> (192.0.2.2/30) ATE Port 1 
  * DUT Port 2 (203.0.113.1/30) <> (203.0.113.2/30) ATE Port 2

* VRFs: TEST_VRF (L3), DEFAULT
* gRIBI: Enabled

* DUT Port 2 <> ATE Port 2:
  * DUT AS: 65500, ATE AS: 65502
  * Import Policy (bgp-import-policy): (Same as DUT Port 1 Import)
    * 198.51.100.0/26 & /32 & EF_ALL Community: Accept
    * Default: Reject
  * Export Policy (bgp-export-policy):
    * Redistributed gRIBI routes matching (198.51.100.0/26 & /32 & EF_ALL Community): Accept
    * Default: Reject
      
* Redistribution Policy (TEST_VRF):
  * Source: gRIBI, Destination: BGP
  * Import Policy:
    * Prefixes within 198.51.100.0/26 with mask /32: Add Communities EF_ALL, NO-CORE, then Accept.
    * Default: Reject

### Canonical OC - Defined Sets

The following sets are used in the policies below.

```json
{
  "routing-policy": {
    "defined-sets": {
      "prefix-sets": {
        "prefix-set": [
          {
            "name": "EF_AGG",
            "config": {
              "name": "EF_AGG",
              "mode": "IPV4"
            },
            "prefixes": {
              "prefix": [
                {
                  "ip-prefix": "198.51.100.0/26",
                  "masklength-range": "32..32",
                  "config": {
                    "ip-prefix": "198.51.100.0/26",
                    "masklength-range": "32..32"
                  }
                }
              ]
            }
          }
        ]
      },
      "openconfig-bgp-defined-sets": {
        "community-sets": {
          "community-set": [
            {
              "community-set-name": "EF_ALL",
              "config": {
                "community-set-name": "EF_ALL",
                "community-member": [
                  "65535:65535"
                ]
              }
            },
            {
              "community-set-name": "NO-CORE",
              "config": {
                "community-set-name": "NO-CORE",
                "community-member": [
                  "65534:20420"
                ]
              }
            },
            {
              "community-set-name": "GSHUT-COMMUNITY",
              "config": {
                "community-set-name": "GSHUT-COMMUNITY",
                "community-member": [
                  "65535:0"
                ]
              }
            }
          ]
        }
      }
    }
  }
}
```

### TestID-16.4.1 - gRIBI to BGP Redistribution

* Step 1 - Generate DUT configuration
  * Configure network-instance 'TEST_VRF' with DUT and ATE interfaces and IP addresses.
  * Configure eBGP & multipath with import and export policies.
  * Configure gRIBI to BGP redistribution policy and table connection.

#### Canonical OC

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "GRIBI-TO-BGP",
          "config": {
            "name": "GRIBI-TO-BGP"
          },
          "statements": {
            "statement": [
              {
                "name": "REDISTRIBUTE_GRIBI_IPV4",
                "config": {
                  "name": "REDISTRIBUTE_GRIBI_IPV4"
                },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "prefix-set": "EF_AGG",
                      "match-set-options": "ANY"
                    }
                  }
                },
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  },
                  "openconfig-bgp-policy:bgp-actions": {
                    "set-community": {
                      "config": {
                        "method": "REFERENCE",
                        "options": "ADD"
                      },
                      "reference": {
                        "config": {
                          "community-set-refs": [
                            "EF_ALL",
                            "NO-CORE"
                          ]
                        }
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
  },
  "network-instances": {
    "network-instance": [
      {
        "name": "TEST_VRF",
        "config": {
          "name": "TEST_VRF"
        },
        "table-connections": {
          "table-connection": [
            {
              "src-protocol": "GRIBI",
              "dst-protocol": "BGP",
              "address-family": "IPV4",
              "config": {
                "import-policy": [
                  "GRIBI-TO-BGP"
                ],
                "default-import-policy": "REJECT_ROUTE"
              }
            }
          ]
        }
      }
    ]
  }
}
```


* Step 2 - Program a gRIBI route in TEST_VRF

```yaml
'operation: { op: ADD network_instance: "TEST_VRF" next_hop: { index: 1001 next_hop { ip_address: { value: "192.0.2.2" } } } }'
'operation: { op: ADD network_instance: "TEST_VRF" next_hop_group: { id: 2001 next_hop_group { next_hop { index: 1001 weight: { value: 1 } } } } }'
'operation: { op: ADD network_instance: "TEST_VRF" ipv4: { prefix: "198.51.100.1/32" ipv4_entry { next_hop_group: { value: 2001 } } } }'
```

* Step 3 - Verify gRIBI route '198.51.100.1/32' is received over eBGP session at ATE Port 2
* Step 4 - Send Traffic from ATE port 2 to ATE 1 (towards destination address 198.51.100.1)
* Step 5 - Validate traffic is received at ATE Port 1 without any loss.
* Step 6 - Delete gRIBI route '198.51.100.1/32' from TEST_VRF

```yaml
'operation: { op: DELETE network_instance: "TEST_VRF" ipv4: { prefix: "198.51.100.1/32" } }'
'operation: { op: DELETE network_instance: "TEST_VRF" next_hop_group: { id: 2001 } }'
'operation: { op: DELETE network_instance: "TEST_VRF" next_hop: { index: 1001 } }'
```

* Step 7 - Verify gRIBI route '198.51.100.1/32' is deleted from TEST_VRF
* Step 8 - Validate full traffic loss at ATE Port 1

### TestID-16.4.2 - Drain Policy Validation

* Step 1 - Generate DUT configuration
  * Configure network-instance 'TEST_VRF' with DUT and ATE interfaces and IP addresses.
  * Configure eBGP & multipath with import and export policies.
  * Configure gRIBI to BGP redistribution policy and table connection.
    
* Step 2 - Program a gRIBI route in TEST_VRF

```yaml
'operation: { op: ADD network_instance: "TEST_VRF" next_hop: { index: 1001 next_hop { ip_address: { value: "192.0.2.2" } } } }'
'operation: { op: ADD network_instance: "TEST_VRF" next_hop_group: { id: 2001 next_hop_group { next_hop { index: 1001 weight: { value: 1 } } } } }'
'operation: { op: ADD network_instance: "TEST_VRF" ipv4: { prefix: "198.51.100.1/32" ipv4_entry { next_hop_group: { value: 2001 } } } }'
```
* Step 3 - Verify gRIBI route '198.51.100.1/32' is received over eBGP session at ATE Port 2 with EF_ALL community, without GSHUT.

* Step 4 - Generate drain policy configuration
  * Configure and append a drain policy 'peer_drain' to existing bgp export policy towards ATE Port 2 BGP session.

#### Canonical OC

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "peer_drain",
          "config": {
            "name": "peer_drain"
          },
          "statements": {
            "statement": [
              {
                "name": "DRAIN-ACTIONS",
                "config": {
                  "name": "DRAIN-ACTIONS"
                },
                "actions": {
                  "config": {
                    "policy-result": "NEXT_STATEMENT"
                  },
                  "bgp-actions": {
                    "config": {
                      "set-med": 100,
                      "set-med-action": "ADD"
                    },
                    "set-as-path-prepend": {
                      "config": {
                        "repeat-n": 5
                      }
                    },
                    "set-community": {
                      "config": {
                         "options": "ADD"
                      },
                      "reference": {
                        "config": {
                          "community-set-refs": [
                            "GSHUT-COMMUNITY"
                          ]
                        }
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
}
```

* Step 5 - Append drain policy 'peer_drain' to existing bgp export policy towards ATE Port 2 BGP session.
* Step 6 - Verify route '198.51.100.1/32' is received with community EF_ALL, MED, 5 AS numbers and GSHUT community at ATE Port 2
* Step 7 - Delete drain policy 'peer_drain'
* Step 8 - Verify route '198.51.100.1/32' BGP attributes are reverted back to original attributes (including EF_ALL community) at ATE Port 2
* Step 9 - Delete gRIBI route '198.51.100.1/32' from TEST_VRF and verify route is removed from RIB and FIB

```yaml
'operation: { op: DELETE network_instance: "TEST_VRF" ipv4: { prefix: "198.51.100.1/32" } }'
'operation: { op: DELETE network_instance: "TEST_VRF" next_hop_group: { id: 2001 } }'
'operation: { op: DELETE network_instance: "TEST_VRF" next_hop: { index: 1001 } }'
```

### TestID-16.4.3 - Disable BGP session with drain policy

* Step 1 - Generate DUT configuration
  * Configure network-instance 'TEST_VRF' with DUT and ATE interfaces and IP addresses.
  * Configure eBGP & multipath with import and export policies.
  * Configure gRIBI to BGP redistribution policy and table connection.
    
* Step 2 - Program a gRIBI route in TEST_VRF

```yaml
'operation: { op: ADD network_instance: "TEST_VRF" next_hop: { index: 1001 next_hop { ip_address: { value: "192.0.2.2" } } } }'
'operation: { op: ADD network_instance: "TEST_VRF" next_hop_group: { id: 2001 next_hop_group { next_hop { index: 1001 weight: { value: 1 } } } } }'
'operation: { op: ADD network_instance: "TEST_VRF" ipv4: { prefix: "198.51.100.1/32" ipv4_entry { next_hop_group: { value: 2001 } } } }'
```
* Step 3 - Verify gRIBI route '198.51.100.1/32' is received over eBGP session at ATE Port 2 with EF_ALL community, without GSHUT.

* Step 4 - Generate drain policy configuration
  * Configure and append a drain policy 'peer_drain' to existing bgp export policy towards ATE Port 2 BGP session.

#### Canonical OC

```json
{
"routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "peer_drain",
          "config": {
            "name": "peer_drain"
          },
          "statements": {
            "statement": [
              {
                "name": "DRAIN-ACTIONS",
                "config": {
                  "name": "DRAIN-ACTIONS"
                },
                "actions": {
                  "config": {
                    "policy-result": "NEXT_STATEMENT"
                  },
                  "bgp-actions": {
                    "config": {
                      "set-med": 100,
                      "set-med-action": "ADD"
                    },
                    "set-as-path-prepend": {
                      "config": {
                        "repeat-n": 5,
                        "asn": 64701
                      }
                    },
                    "set-community": {
                      "config": {
                         "options": "ADD"
                      },
                      "reference": {
                        "config": {
                          "community-set-refs": [
                            "GSHUT-COMMUNITY"
                          ]
                        }
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
}
```

* Step 5 - Append drain policy 'peer_drain' to existing bgp export policy towards ATE Port 2 BGP session.
* Step 6 - Verify route '198.51.100.1/32' is received with community EF_ALL, MED, 5 AS numbers and GSHUT community at ATE Port 2
* Step 7 - Disable bgp session on ATE Port 2
* Step 8 - Re-enable bgp session on ATE Port 2
* Step 9 - Verify route '198.51.100.1/32' is received with community EF_ALL, MED, 5 AS numbers and GHUT community at ATE Port 2
* Step 10 - Delete drain policy 'peer_drain'
* Step 11 - Verify route '198.51.100.1/32' BGP attributes are reverted back to original attributes (including EF_ALL community) at ATE Port 2
* Step 12 - Delete gRIBI route '198.51.100.1/32' from TEST_VRF and verify route is removed from RIB and FIB

```yaml
'operation: { op: DELETE network_instance: "TEST_VRF" ipv4: { prefix: "198.51.100.1/32" } }'
'operation: { op: DELETE network_instance: "TEST_VRF" next_hop_group: { id: 2001 } }'
'operation: { op: DELETE network_instance: "TEST_VRF" next_hop: { index: 1001 } }'
```

## OpenConfig Path and RPC Coverage

This yaml stanza defines the OC paths intended to be covered by this test.  OC
paths used for test environment setup are not required to be listed here. This
content is parsed by automation to derive the test coverage.  If any new OC
paths are required, they should also be included here as a TODO comment.

```yaml
paths:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* Specify the minimum DUT-type:
  * FFF - fixed form factor
