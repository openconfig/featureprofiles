
# RT-10.2: Non-default Route Generation based on 192.168.2.2/32 Presence in ISIS

## Testcase summary

## Testbed type 

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Topology

*  Connect ATE Port-1 to DUT Port-1

*  Preconditions:
    * A ISIS speaker DUT is configured.

    * ISIS peering is established and stable between DUT and at least one neighbor ATE-1.

    * A policy to generate default route 192.168.2.0/30 upon receiving a isis route 192.168.2.2/32

    * Initial routing tables are verified to be free of any 192.168.2.0/30 or 192.168.2.2/32 .




## Procedure for generate non-default route

## Scenario: 192.168.2.2/32 route is present		

1.1	On ATE-1, advertise a isis route 192.168.2.2/32 towards DUT.	

Expected result: The 192.168.2.2/32 route is visible in DUT's routing table.

1.2	Wait for isis to converge.	

Expected result: The isis session remains in the UP state.

1.3	On DUT, inspect the routing table.	

Expected result: A non-default route (192.168.2.0/30) is now generated in DUT.

1.4	On ATE-1, stop advertising the route 192.168.2.2/32.	

Expected result: The 192.168.2.2/32 route is removed from DUT's routing table.

1.5	Wait for isis to converge.	

Expected result: The isis session remains in the UP state.

1.6	On DUT, inspect the routing table.	

Expected result: The non-default route 192.168.2.0/30 is withdrawn and is no longer generated in DUT.



## Conclusion:

Pass: The test case passes if all expected results are achieved.

Fail: The test case fails if any of the following occur:

A non-default route is generated and when the 192.168.2.2/32 route is not present.
A non-default route is not generated and advertised when the 192.168.2.2/32 route is present.
The isis session drops or gets stuck in an intermediate state.

## Canonical OC 

```
TODO: Add advertise-aggregate path to the OpenConfig public data models. https://github.com/openconfig/public/issues/1368

/routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/advertise-aggregate = `"LOCAL-AGG"

{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "default-import-policy": "REJECT_ROUTE"
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST"
                            }
                          }
                        ]
                      },
                      "config": {
                        "neighbor-address": "192.1.1.1"
                      },
                      "neighbor-address": "192.1.1.1"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "default-import-policy": "REJECT_ROUTE"
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV6_UNICAST"
                            }
                          }
                        ]
                      },
                      "config": {
                        "neighbor-address": "2001:db9::1"
                      },
                      "neighbor-address": "2001:db9::1"
                    }
                  ]
                }
              },
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "identifier": "BGP",
              "name": "BGP"
            },
            {
              "config": {
                "identifier": "LOCAL_AGGREGATE",
                "name": "DEFAULT-AGG"
              },
              "identifier": "LOCAL_AGGREGATE",
              "local-aggregates": {
                "aggregate": [
                  {
                    "config": {
                      "prefix": "0.0.0.0/0"
                    },
                    "prefix": "0.0.0.0/0"
                  }
                ]
              },
              "name": "DEFAULT-AGG"
            }
          ]
        }
      }
    ]
  },
  "routing-policy": {
    "defined-sets": {
      "prefix-sets": {
        "prefix-set": [
          {
            "config": {
              "mode": "IPV4",
              "name": "EBGP-IMPORT-IPV4"
            },
            "name": "EBGP-IMPORT-IPV4",
            "prefixes": {
              "prefix": [
                {
                  "config": {
                    "ip-prefix": "192.0.2.0/32",
                    "masklength-range": "exact"
                  },
                  "ip-prefix": "192.0.2.0/32",
                  "masklength-range": "exact"
                }
              ]
            }
          }
        ]
      }
    },
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "EBGP-IMPORT-IPV4"
          },
          "name": "EBGP-IMPORT-IPV4",
          "statements": {
            "statement": [
              {
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "conditions": {
                  "match-prefix-set": {
                    "config": {
                      "match-set-options": "INVERT",
                      "prefix-set": "EBGP-IMPORT-IPV4"
                    }
                  }
                },
                "config": {
                  "name": "10"
                },
                "name": "10"
              }
            ]
          }
        }
      ]
    }
  }
}

```

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
