# RT-1.66: IPv4 Static Route with IPv6 Next-Hop

## Summary
This test verified the functionality of IPv4 static route configured to redirect packets to a IPv6 destination. 

## Testbed type
[TESTBED_DUT_ATE_4 LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup:
 * Connect DUT port-1, port-2, port-3 and port-4 to ATE port-1, port-2, port-3 and port-4 respectively
 * Configure IPv4 addresses on port-1 of DUT '192.0.1.1/24' and ATE '192.0.1.2/24'
 * Configure IPv4 addresses on port-2 of DUT '192.0.2.1/24' and ATE '192.0.2.2/24'
 * Configure [IPv4, IPv6] addresses on port-3 of DUT ['192.0.3.1/24', '2001:db8:128:128::1/64'] and ATE '['192.0.3.2/24', 2001:db8:128:128::2/64']
 * Configure [IPv4, IPv6] addresses on port-4 of DUT ['192.0.4.1/24', '2001:db8:128:129::1/64'] and ATE '['192.0.4.2/24', 2001:db8:128:129::2/64']
 * Enable ECMP for static route

### RT-1.66.1: IPv4 static route with an IPv6 next-hop in default network-instance

  * Step 1 - Generate DUT Configuration
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port3 '2001:db8:128:128::2' in a default network-instance
  * Step 2 - Generate ATE Configuration
    - Configure IPv4 traffic profile for source '192.0.1.2/24' and destination '192.168.1.0/24' with udp payload and random src/dest ports
  * Step 3 - Traffic Test
    - Start the traffic
    - Monitor for 60 seconds
  * Step 4 - Test Validations
    - Traffic must be received on ATE:port3 without any loss
    - Configuration must be accepted by device
      
#### Canonical OC

```json
    {
      "network-instances": {
        "network-instance": [
          {
            "name": "DEFAULT",
            "protocols": {
              "protocol": [
                {
                  "identifier": "STATIC",
                  "name": "static",
                  "static-routes": {
                    "static": [
                      {
                        "prefix": "192.168.1.0/24",
                        "next-hops": {
                          "next-hop": [
                            {
                              "index": "0",
                              "config": {
                                "index": "0",
                                "next-hop": "2001:db8:128:128::2"
                              }
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            }
          }
        ]
      }
    }
```

### RT-1.66.2: IPv4 static route with multiple IPv6 next-hop in default network-instance

  * Step 1 - Generate DUT Configuration
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port3 '2001:db8:128:128::2' in a default network-instance
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port4 '2001:db8:128:129::2' in a default network-instance
  * Step 2 - Generate ATE Configuration
    - Configure IPv4 traffic profile for source '192.0.1.2/24' and destination '192.168.1.0/24' with udp payload and random src/dest ports
  * Step 3 - Traffic Test
    - Start the traffic
    - Monitor for 60 seconds
  * Step 4 - Test Validations
    - Traffic must be equally received on ATE:port3 and ATE:port4 without any loss
    - Configuration must be accepted by device
      
#### Canonical OC

```json
    {
      "network-instances": {
        "network-instance": [
          {
            "name": "DEFAULT",
            "protocols": {
              "protocol": [
                {
                  "identifier": "STATIC",
                  "name": "static",
                  "static-routes": {
                    "static": [
                      {
                        "prefix": "192.168.1.0/24",
                        "next-hops": {
                          "next-hop": [
                            {
                              "index": "0",
                              "config": {
                                "index": "0",
                                "next-hop": "2001:db8:128:128::2"
                              }
                            },
                            {
                              "index": "1",
                              "config": {
                                "index": "1",
                                "next-hop": "2001:db8:128:129::2"
                              }
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            }
          }
        ]
      }
    }
```



### RT-1.66.3: IPv4 static route with an IPv6 and an IPv4 next-hop in default network-instance

  * Step 1 - Generate DUT Configuration
    - Configure a ipv4 static route '192.168.1.0/24' with IPv4 next-hop of ATE:port3 '192.0.3.2/24' in a default network-instance
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port4 '2001:db8:128:129::2' in a default network-instance
  * Step 2 - Generate ATE Configuration
    - Configure IPv4 traffic profile for source '192.0.1.2/24' and destination '192.168.1.0/24' with udp payload and random src/dest ports
  * Step 3 - Traffic Test
    - Start the traffic
    - Monitor for 60 seconds
  * Step 4 - Test Validations
    - Traffic must be equally received on ATE:port3 and ATE:port4 without any loss
    - Configuration must be accepted by device
      
#### Canonical OC

```json
    {
      "network-instances": {
        "network-instance": [
          {
            "name": "DEFAULT",
            "protocols": {
              "protocol": [
                {
                  "identifier": "STATIC",
                  "name": "static",
                  "static-routes": {
                    "static": [
                      {
                        "prefix": "192.168.1.0/24",
                        "next-hops": {
                          "next-hop": [
                            {
                              "index": "0",
                              "config": {
                                "index": "0",
                                "next-hop": "192.0.3.2"
                              }
                            },
                            {
                              "index": "1",
                              "config": {
                                "index": "1",
                                "next-hop": "2001:db8:128:129::2"
                              }
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            }
          }
        ]
      }
    }
```

### RT-1.66.4: IPv4 static route with an invalid IPv6 next-hop in default network-instance

  * Step 1 - Generate DUT Configuration
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port3 '2001:db8:128:130::2' in a default network-instance
  * Step 2 - Generate ATE Configuration
    - Configure IPv4 traffic profile for source '192.0.1.2/24' and destination '192.168.1.0/24' with udp payload and random src/dest ports
  * Step 3 - Traffic Test
    - Start the traffic
    - Monitor for 60 seconds
  * Step 4 - Test Validations
    - Traffic must not be received on ATE:port3 and there should be 100% traffic loss.
    - Configuration must be accepted by device
      
#### Canonical OC

```json
    {
      "network-instances": {
        "network-instance": [
          {
            "name": "DEFAULT",
            "protocols": {
              "protocol": [
                {
                  "identifier": "STATIC",
                  "name": "static",
                  "static-routes": {
                    "static": [
                      {
                        "prefix": "192.168.1.0/24",
                        "next-hops": {
                          "next-hop": [
                            {
                              "index": "0",
                              "config": {
                                "index": "0",
                                "next-hop": "2001:db8:128:130::2"
                              }
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            }
          }
        ]
      }
    }
```


### RT-1.66.5: IPv4 static route with an IPv6 next-hop in non-default network-instance

  * Step 1 - Generate DUT Configuration
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port3 '2001:db8:128:128::2' in a non-default network-instance 'VRF1'
    - Assosiate ATE:port1 and ATE:port3 with 'VRF1'
  * Step 2 - Generate ATE Configuration
    - Configure IPv4 traffic profile for source '192.0.1.2/24' and destination '192.168.1.0/24' with udp payload and random src/dest ports
  * Step 3 - Traffic Test
    - Start the traffic
    - Monitor for 60 seconds
  * Step 4 - Test Validations
    - Traffic must be received on ATE:port3 without any loss
    - Configuration must be accepted by device
      
#### Canonical OC

```json
    {
      "network-instances": {
        "network-instance": [
          {
            "name": "VRF1",
            "interfaces": {
              "interface": [
                {
                  "id": "Ethernet1/1",
                  "config": {
                    "id": "Ethernet1/1"
                  }
                },
                {
                  "id": "Ethernet1/3",
                  "config": {
                    "id": "Ethernet1/3"
                  }
                }
              ]
            },
            "protocols": {
              "protocol": [
                {
                  "identifier": "STATIC",
                  "name": "static",
                  "static-routes": {
                    "static": [
                      {
                        "prefix": "192.168.1.0/24",
                        "next-hops": {
                          "next-hop": [
                            {
                              "index": "0",
                              "config": {
                                "index": "0",
                                "next-hop": "2001:db8:128:128::2"
                              }
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            }
          }
        ]
      }
    }
```

### RT-1.66.6: IPv4 static route with multiple IPv6 next-hop in non-default network-instance

  * Step 1 - Generate DUT Configuration
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port3 '2001:db8:128:128::2' in a non-default network-instance 'VRF1'
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port4 '2001:db8:128:129::2' in a non-default network-instance 'VRF1'
    - Assosiate ATE:port1, ATE:port3 and ATE:port4 with 'VRF1'
  * Step 2 - Generate ATE Configuration
    - Configure IPv4 traffic profile for source '192.0.1.2/24' and destination '192.168.1.0/24' with udp payload and random src/dest ports
  * Step 3 - Traffic Test
    - Start the traffic
    - Monitor for 60 seconds
  * Step 4 - Test Validations
    - Traffic must be equally received on ATE:port3 and ATE:port4 without any loss
    - Configuration must be accepted by device
      
#### Canonical OC

```json
    {
      "network-instances": {
        "network-instance": [
          {
            "name": "VRF1",
            "interfaces": {
              "interface": [
                {
                  "id": "Ethernet1/1",
                  "config": {
                    "id": "Ethernet1/1"
                  }
                },
                {
                  "id": "Ethernet1/3",
                  "config": {
                    "id": "Ethernet1/3"
                  }
                },
                {
                  "id": "Ethernet1/4",
                  "config": {
                    "id": "Ethernet1/4"
                  }
                }
              ]
            },
            "protocols": {
              "protocol": [
                {
                  "identifier": "STATIC",
                  "name": "static",
                  "static-routes": {
                    "static": [
                      {
                        "prefix": "192.168.1.0/24",
                        "next-hops": {
                          "next-hop": [
                            {
                              "index": "0",
                              "config": {
                                "index": "0",
                                "next-hop": "2001:db8:128:128::2"
                              }
                            },
                            {
                              "index": "1",
                              "config": {
                                "index": "1",
                                "next-hop": "2001:db8:128:129::2"
                              }
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            }
          }
        ]
      }
    }
```



### RT-1.66.7: IPv4 static route with an IPv6 and an IPv4 next-hop in non-default network-instance

  * Step 1 - Generate DUT Configuration
    - Configure a ipv4 static route '192.168.1.0/24' with IPv4 next-hop of ATE:port3 '192.0.3.2/24' in a non-default network-instance 'VRF1'
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port4 '2001:db8:128:129::2' in a non-default network-instance 'VRF1'
    - Assosiate ATE:Port1, ATE:port3 and ATE:port4 with 'VRF1'
  * Step 2 - Generate ATE Configuration
    - Configure IPv4 traffic profile for source '192.0.1.2/24' and destination '192.168.1.0/24' with udp payload and random src/dest ports
  * Step 3 - Traffic Test
    - Start the traffic
    - Monitor for 60 seconds
  * Step 4 - Test Validations
    - Traffic must be equally received on ATE:port3 and ATE:port4 without any loss
    - Configuration must be accepted by device
      
#### Canonical OC

```json
    {
      "network-instances": {
        "network-instance": [
          {
            "name": "VRF1",
            "interfaces": {
              "interface": [
                {
                  "id": "Ethernet1/1",
                  "config": {
                    "id": "Ethernet1/1"
                  }
                },
                {
                  "id": "Ethernet1/3",
                  "config": {
                    "id": "Ethernet1/3"
                  }
                },
                {
                  "id": "Ethernet1/4",
                  "config": {
                    "id": "Ethernet1/4"
                  }
                }
              ]
            },
            "protocols": {
              "protocol": [
                {
                  "identifier": "STATIC",
                  "name": "static",
                  "static-routes": {
                    "static": [
                      {
                        "prefix": "192.168.1.0/24",
                        "next-hops": {
                          "next-hop": [
                            {
                              "index": "0",
                              "config": {
                                "index": "0",
                                "next-hop": "192.0.3.2"
                              }
                            },
                            {
                              "index": "1",
                              "config": {
                                "index": "1",
                                "next-hop": "2001:db8:128:129::2"
                              }
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            }
          }
        ]
      }
    }
```

### RT-1.66.8: IPv4 static route with an invalid IPv6 next-hop in non-default network-instance

  * Step 1 - Generate DUT Configuration
    - Configure a ipv4 static route '192.168.1.0/24' with IPv6 next-hop of ATE:port3 '2001:db8:128:130::2' in a non-default network-instance 'VRF1'
    - Assosiate ATE:port1 and ATE:port3 with VRF1
  * Step 2 - Generate ATE Configuration
    - Configure IPv4 traffic profile for source '192.0.1.2/24' and destination '192.168.1.0/24' with udp payload and random src/dest ports
  * Step 3 - Traffic Test
    - Start the traffic
    - Monitor for 60 seconds
  * Step 4 - Test Validations
    - Traffic must not be received on ATE:port3 and there should be 100% traffic loss.
    - Configuration must be accepted by device
      
#### Canonical OC

```json
    {
      "network-instances": {
        "network-instance": [
          {
            "name": "VRF1",
            "interfaces": {
              "interface": [
                {
                  "id": "Ethernet1/1",
                  "config": {
                    "id": "Ethernet1/1"
                  }
                },
                {
                  "id": "Ethernet1/3",
                  "config": {
                    "id": "Ethernet1/3"
                  }
                }
              ]
            },
            "protocols": {
              "protocol": [
                {
                  "identifier": "STATIC",
                  "name": "static",
                  "static-routes": {
                    "static": [
                      {
                        "prefix": "192.168.1.0/24",
                        "next-hops": {
                          "next-hop": [
                            {
                              "index": "0",
                              "config": {
                                "index": "0",
                                "next-hop": "2001:db8:128:130::2"
                              }
                            }
                          ]
                        }
                      }
                    ]
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
 #/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform
*   FFF
