# RT-7.10 Routing policy statement insertion and removal

> WARNING: WORK IN PROGRESS

## Summary

This test verify that using gNMI setReqyest(Replace) we can insert statement in the middle of pre-existing policy, then remove any statement form policy, the re add this statement.
Even if statement is not first and not last one.

This test verify correctness of gNMI setReques REPLACE operation for routing policy.

## Subtests

* RT-7.10.1 Initial Policy
  * Establish external BGP session between ATE port1 and DUT port1
  * Configure policy "test-policy" and apply using setRequest Replace at `openconfig/routing-policy/`
  ```
  {
    "openconfig-routing-policy:routing-policy": {
      "defined-sets": {
        "openconfig-bgp-policy:bgp-defined-sets": {
          "community-sets": {
            "community-set": [
              {
                "community-set-name": "Comm_100_1",
                "config": {
                  "community-set-name": "Comm_100_1",
                  "community-member": [
                    "100:1"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_3",
                "config": {
                  "community-set-name": "Comm_100_3",
                  "community-member": [
                    "100:3"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_5",
                "config": {
                  "community-set-name": "Comm_100_5",
                  "community-member": [
                    "100:5"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_7",
                "config": {
                  "community-set-name": "Comm_100_7",
                  "community-member": [
                    "100:7"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_9",
                "config": {
                  "community-set-name": "Comm_100_9",
                  "community-member": [
                    "100:9"
                  ]
                }
              }
            ]
          }
        }
      },
      "policy-definitions": {
        "policy-definition": [
          {
            "name": "test-policy",
            "config": {
              "name": "test-policy"
            },
            "statements": {
              "statement": [
                {
                  "name": "Stmnt_1",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_1"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_1",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_3",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_3"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_3",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_5",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_5"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_5",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_7",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_7"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_7",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_9",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_9"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_9",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_Last",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_Last"
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "ACCEPT_ROUTE"
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
  * Attach policy "test-policy" to session via:
  ```
  {
    "openconfig-network-instance:network-instances": {
      "network-instance": [
        {
          "name": "DEFAULT",
          "config": {
            "name": "DEFAULT"
          },
          "protocols": {
            "protocol": [
              {
                "name": "DEFAULT",
                "identifier": "openconfig-policy-types:BGP",
                "bgp": {
                  "neighbors": {
                    "neighbor": [
                      {
                        "neighbor-address": "<ATE port1 IPv4>",
                        "config": {
                          "neighbor-address": "<ATE port1 IPv4>"
                        },
                        "afi-safis": {
                          "afi-safi": [
                            {
                              "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                              "config": {
                                "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST"
                              },
                              "apply-policy": {
                                "config": {
                                  "import-policy": [
                                    "test-policy"
                                  ],
                                  "export-policy": [
                                    "test-policy"
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
              }
            ]
          }
        }
      ]
    }
  }
  ```
  * Verify that DUT accepted configuration without errors.
  * Retrive  "test-policy" from device using subscribeRequest once for `/routing-policy/policy-definitions/policy-definition[name="test-policy"]/*`. Compare with policy configured above.

* RT-7.10.2 Policy statement insertion
  * Configure policy "test-policy" and apply using setRequest Replace at `openconfig/routing-policy/`
  ```
  {
    "openconfig-routing-policy:routing-policy": {
      "defined-sets": {
        "openconfig-bgp-policy:bgp-defined-sets": {
          "community-sets": {
            "community-set": [
              {
                "community-set-name": "Comm_100_1",
                "config": {
                  "community-set-name": "Comm_100_1",
                  "community-member": [
                    "100:1"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_2",
                "config": {
                  "community-set-name": "Comm_100_2",
                  "community-member": [
                    "100:2"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_6",
                "config": {
                  "community-set-name": "Comm_100_6",
                  "community-member": [
                    "100:6"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_7",
                "config": {
                  "community-set-name": "Comm_100_7",
                  "community-member": [
                    "100:7"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_8",
                "config": {
                  "community-set-name": "Comm_100_8",
                  "community-member": [
                    "100:8"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_10",
                "config": {
                  "community-set-name": "Comm_100_10",
                  "community-member": [
                    "100:10"
                  ]
                }
              }
            ]
          }
        }
      },
      "policy-definitions": {
        "policy-definition": [
          {
            "name": "test-policy",
            "config": {
              "name": "test-policy"
            },
            "statements": {
              "statement": [
                {
                  "name": "Stmnt_1",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_1"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_1",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_2",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_2"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_2",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_6",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_6"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_6",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_7",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_7"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_7",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_8",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_8"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_8",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_10",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_10"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_10",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_Last",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_Last"
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "ACCEPT_ROUTE"
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
  * Verify that DUT accepted configuration without errors.
  * Retrive  "test-policy" from device using subscribeRequest once for `/routing-policy/policy-definitions/policy-definition[name="test-policy"]/*`. Compare with policy configured above.

* RT-7.10.3 Policy statement removal
  * Configure policy "test-policy" and apply using setRequest Replace at `openconfig/routing-policy/`
  ```
  {
    "openconfig-routing-policy:routing-policy": {
      "defined-sets": {
        "openconfig-bgp-policy:bgp-defined-sets": {
          "community-sets": {
            "community-set": [
              {
                "community-set-name": "Comm_100_1",
                "config": {
                  "community-set-name": "Comm_100_1",
                  "community-member": [
                    "100:1"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_2",
                "config": {
                  "community-set-name": "Comm_100_2",
                  "community-member": [
                    "100:2"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_6",
                "config": {
                  "community-set-name": "Comm_100_6",
                  "community-member": [
                    "100:6"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_7",
                "config": {
                  "community-set-name": "Comm_100_7",
                  "community-member": [
                    "100:7"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_8",
                "config": {
                  "community-set-name": "Comm_100_8",
                  "community-member": [
                    "100:8"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_10",
                "config": {
                  "community-set-name": "Comm_100_10",
                  "community-member": [
                    "100:10"
                  ]
                }
              }
            ]
          }
        }
      },
      "policy-definitions": {
        "policy-definition": [
          {
            "name": "test-policy",
            "config": {
              "name": "test-policy"
            },
            "statements": {
              "statement": [
                {
                  "name": "Stmnt_1",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_1"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_1",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_2",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_2"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_2",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_6",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_6"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_6",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_7",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_7"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_7",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_8",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_8"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_8",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_10",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_10"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_10",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_Last",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_Last"
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "ACCEPT_ROUTE"
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
  * Verify that DUT accepted configuration without errors.
  * Retrive  "test-policy" from device using subscribeRequest once for  `/routing-policy/policy-definitions/policy-definition[name="test-policy"]/*`. Compare with policy configured above.

* RT-7.10.3 Policy statement re-insertion
  * Configure policy "test-policy" and apply using setRequest Replace at `openconfig/routing-policy/`
  ```
  {
    "openconfig-routing-policy:routing-policy": {
      "defined-sets": {
        "openconfig-bgp-policy:bgp-defined-sets": {
          "community-sets": {
            "community-set": [
              {
                "community-set-name": "Comm_100_1",
                "config": {
                  "community-set-name": "Comm_100_1",
                  "community-member": [
                    "100:1"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_2",
                "config": {
                  "community-set-name": "Comm_100_2",
                  "community-member": [
                    "100:2"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_3",
                "config": {
                  "community-set-name": "Comm_100_3",
                  "community-member": [
                    "100:3"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_4",
                "config": {
                  "community-set-name": "Comm_100_4",
                  "community-member": [
                    "100:4"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_5",
                "config": {
                  "community-set-name": "Comm_100_5",
                  "community-member": [
                    "100:5"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_6",
                "config": {
                  "community-set-name": "Comm_100_6",
                  "community-member": [
                    "100:6"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_7",
                "config": {
                  "community-set-name": "Comm_100_7",
                  "community-member": [
                    "100:7"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_8",
                "config": {
                  "community-set-name": "Comm_100_8",
                  "community-member": [
                    "100:8"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_9",
                "config": {
                  "community-set-name": "Comm_100_9",
                  "community-member": [
                    "100:9"
                  ]
                }
              },
              {
                "community-set-name": "Comm_100_10",
                "config": {
                  "community-set-name": "Comm_100_10",
                  "community-member": [
                    "100:10"
                  ]
                }
              }
            ]
          }
        }
      },
      "policy-definitions": {
        "policy-definition": [
          {
            "name": "test-policy",
            "config": {
              "name": "test-policy"
            },
            "statements": {
              "statement": [
                {
                  "name": "Stmnt_1",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_1"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_1",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_2",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_2"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_2",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_3",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_3"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_3",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_4",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_4"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_4",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_5",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_5"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_5",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_6",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_6"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_6",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_7",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_7"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_7",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_8",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_8"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_8",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_9",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_9"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_9",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_10",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_10"
                  },
                  "openconfig-routing-policy:conditions": {
                    "openconfig-bgp-policy:bgp-conditions": {
                      "match-community-set": {
                        "config": {
                          "community-set": "Comm_100_10",
                          "match-set-options": "ANY"
                        }
                      }
                    }
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "NEXT_STATEMENT"
                    }
                  }
                },
                {
                  "name": "Stmnt_Last",
                  "openconfig-routing-policy:config": {
                    "name": "Stmnt_Last"
                  },
                  "openconfig-routing-policy:actions": {
                    "config": {
                      "policy-result": "ACCEPT_ROUTE"
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
  * Verify that DUT accepted configuration without errors.
  * Retrive  "test-policy" from device using subscribeRequest once for  `/routing-policy/policy-definitions/policy-definition[name="test-policy"]/*`. Compare with policy configured above.


## Config Parameter Coverage

### Policy definition



### Policy for community-set match



## Telemetry Parameter Coverage

### Policy definition state



### Policy for community-set match state



### Paths to verify policy state



### Paths to verify prefixes sent and received



## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths
  ### Policy definition
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  ### Policy for community-set match
  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-set-name:
  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/config/community-member:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set/config/community-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set/config/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy:

  ## State paths
  ### Policy definition state

  /routing-policy/policy-definitions/policy-definition/state/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/state/name:

  ### Policy for community-set match state

  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-set-name:
  /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/state/community-member:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set/state/community-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set/state/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/state/policy-result:


  ### Paths to verify policy state

  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/state/import-policy:


rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```
