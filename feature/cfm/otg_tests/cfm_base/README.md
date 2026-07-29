# CFM-1.1: CFM over ETHoCWoMPLSoGRE

## Summary

This test verifies CFM "DOWN MEP" can be established over "EthoCWoMPLSoGRE" dataplane.
## Testbed type

*  [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure

### Test environment setup

```text
DUT has an ingress and 2 egress aggregate interfaces.
                         
    [ ATE Port 1 ]----| Port1  :DUT1:  Port2 | ---- |Port2 :DUT2: Port1 | ---- |ATE Port 2 |  
```

Test uses aggregate 802.3ad bundled interfaces (Aggregate Interfaces).

* Ingress Port: nTraffic is generated from Aggregate1 (ATE Ports 1).

* Egress Ports: Aggregate2 (ATE Port 2) is used as the destination ports for encapsulated traffic.

* Transit Ports: Aggregate3 (DUT1 port 2 and DUT2 Port 2)

### CFM-1.1.1: Generate DUT Configuration

Aggregate 1 and Aggregate2 i.e "customer interface's" are the  customer facing ports that could either have port mode configuration or attachment mode configuration as described below. 

EACH test should be run twice - once with port mode configuration and once with attachment in sub-interface mode configuration.


port mode configuration implies CFM configured on main Port as an attachment and vlan-mode is when cfm is confirued between attachments on ansub-interface.

Port mode will have Link loss forwarding enabled while sub-interface mode may not.

#### Aggregate 1 "customer interface" Port mode configuration

* Configure DUT1 port 1 to be a member of aggregate interface named "customer interface"
* "customer interface" is a static Layer 2 bundled interface part of pseudowire that accepts packets from all VLANs.
* MTU default 9216

#### Aggregate 1 "customer interface" Attachment mode configuration

* Configure DUT1 port 1 to be a member of aggregate interface named "customer interface"
* Create a sub interface of the aggregated interface and assign a VLAN ID to it. 
* This sub interface will be a static Layer 2 bundled interface part of pseudowire that accepts packets from vlan ID associated with it. 
* MTU default 9216

#### Aggregate 2 "customer interface" Port mode configuration

* Configure DUT2 port 1 to be a member of aggregate interface named "customer interface"
* "customer interface" is a static Layer 2 bundled interface part of pseudowire that accepts packets from all VLANs.
* MTU default 9216

#### Aggregate 2 "customer interface" Attachment mode configuration

* Configure DUT2 port 1 to be a member of aggregate interface named "customer interface"
* Create a sub interface of the aggregated interface and assign a VLAN ID to it. 
* This sub interface will be a static Layer 2 bundled interface part of pseudowire that accepts packets from vlan ID associated with it. 
* MTU default 9216

#### Policy Forwarding Configuration 

* Policy-forwarding enabling EthoMPLSoGRE encapsulation of all incoming traffic:

  * The forwarding policy must allow forwarding of incoming traffic across 16 tunnels. 16 Tunnels has 16 source address and a single tunnel destination.

  * Source address must be configurable as:
    * Loopback address OR
    * 16 source addresses corresponding to a single tunnel destinations to achieve maximum entropy.

  * DSCP of the innermost IP packet header must be preserved during encapsulation

  * DSCP of the GRE/outermost IP header must be configurable (Default TOS 96) during encapsulation

  * TTL of the outer GRE must be configurable (Default TTL 64)

  * QoS Hardware queues for all traffic must be configurable (default QoS hardaware class selected is 3)

### Pseudowire (PW) Configuration 

* "Customer interface" is Aggregate 1 and Aggregate 2  pointing towards  Aggregare3
* Two unique static MPLS label for local label and remote label. 
* Enable control word

### Aggregate 3 configuration

* IPV4 and IPV6 addresses

* MTU (default 9216)

* LACP Member link configuration

* Lag id

* LACP (default: period short)

* Carrier-delay (default up:3000 down:150)

* Statistics load interval (default:30 seconds)

### Routing

* Create static route for tunnel destination pointing towards Aggregate 2. 
* Static mapping of MPLS label for encapsulation must be configurable

### MPLS Label

* Entire Label block must be reallocated for static MPLS
* Labels from start/end/mid ranges must be usable and configured corresponding to EthoMPLSoGRE encapsulation


### CFM 

CFM is cnfigured as UP MEP. The control plane is between the customer attachments on either PF.

### CFM-1.1.2: Verify PF CFM establishment over EthoMPLSoGRE encapsulate 

* Configure CFM session on DUT1 Port 1 and DUT2 Port 1 at line rate with a mix of both IPV4 and IPv6 traffic. Use 64, 128, 256, 512, 1024 MTU bytes frame size. 
* Adjacent ATE Ports must have an Active L2. 

Verify:

*  CFM session is established with default timers and All ccm PDUs are EthoCWoMPLSoGRE-encapsulated.  
*  Verify deadtimer at 3.5 (100ms --> 350ms) times the configured keepalive. 
*  
*  CCM PDUs are constant flows hence are not ECMP'd across all available tunnels.

Run the test separately for both port mode and attachment mode "customer interface" configuration. 

### CFM-1.1.3: Verify PF CFM packet integrity

* Use same configuration profile as CFM-1.1.2 (Default Timers).

Verify:
* CCM PDU Destination is Multicast.
* CFM OpCode as - Continuity check message (1).
* Increasing sequence number for consequent CCM packets.
* Verify interval field in CCM packet for following fields.
* Transmitted interval 100ms.
* Max Lifetime 350ms.
* Min lifetime 325ms.

#### CFM-1.1.3.1: Verify PF CFM Alarm 

* Use same configuration profile as CFM-1.1.2
* Configure Wrong MD level on on endpoint.
* Revert MD level and configure different CCM interval (i.e 100ms and (10ms or 1s))

Verify:

* “wrong MD  level”  alarm (when induced) on the endpoints.
* Verify “wrong interval” alarm (when induced) on the endpoints.


### CFM-1.1.4: Verify RDI bit set for CCM PDUs on a CE-PE fault 

* Use  configuration profile as descrbied in CFM-1.1.2
* turn off TX/Shutdown ATE port 1 (only on one endpoint).

Verify:

* CCM PDU is recieved on the remote endpoint with the RDI flag Set.
* A "Remote Defect Detected" alarm is raised at the remote end-point.

### CFM-1.1.5: Verify RDI bit set for CCM PDUs on a PE-PE fault 

* Use same configuration profile as descrbied in CFM-1.1.2.
* turn off  DUT 1 port 2 (only one endpoint).

Verify:

* Both DUT 1 and DUT 2 observe a CCM timeoute Event
* DUT 1 and DUT 2 emit a CCM PDU  with the RDI flag Set.


#### CFM-1.1.5.1: Verify TX actuation on a remote defect or a CCM timeout

* Use  configuration profile as describedd in CFM-1.1.2 (Port mode only).
* turn off TX/Shutdown ATE port 1 (only on one endpoint).
* revert TX on ATE port 1 and turn off DUT 1 port 2 (only one endpoint).

Verify:

* CCM PDU is recieved on the remote endpoint with the RDI flag Set.
* A "Remote Defect Detected" alarm is raised at the remote end-point.


### CFM-1.1.6: Verify CFM Loss threshold can be configrued on DUT 

* Use configuration profile as describedd in CFM-1.1.2.
* re-write defulat Loss threshold (3) knob to 6, 10 , 20, 100 and 255.
* Shutdown DUT 1 port 2 (only one endpoint).


### CFM-1.1.8: Verify CFM Delay measurement.

* Use configuration profile as described in CFM-1.1.2.
* Configure 2 way DM measurement profiles on CFM session with 60 second measurement intervals.

Verify:

* DUT 1 and DUT 2 observe a min, max and mean Delay measurements at defined.

### CFM-1.1.9: Verify CFM synthetic loss measurement).

* Use configuration profile as described in CFM-1.1.2.
* 

Verify:

* Both DUT 1 and DUT 2 observe a min, max and mean frame loss ratio for "far-end" & "near-end" with 60 second measurement intervals.

### CFM-1.1.10: Verify CFM scale.

* Use configuration profile as describedd in CFM-1.1.2.
* Configure 1000 attachments, pseudowires and Establish 1000 CFM sessions between operating at 100ms keepalive:

Verify:

* Device Health and soak for at least 6 hrs.

## Canonical OC  

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "GEO_1",
          "enabled": true,
          "mtu": 9080,
          "name": "Bundle-Ether43",
          "type": "ieee8023adLag"
        },
        "name": "Bundle-Ether43",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "description": "GEO_1",
                "enabled": true,
                "index": 1600
              },
              "index": 1600,
              "vlan": {
                "match": {
                  "single-tagged": {
                    "config": {
                      "vlan-id": 1600
                    }
                  }
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "GEO_2",
          "enabled": true,
          "mtu": 9080,
          "name": "Bundle-Ether5",
          "type": "ieee8023adLag"
        },
        "name": "Bundle-Ether5",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "description": "GEO_2",
                "enabled": true,
                "index": 4010
              },
              "index": 4010,
              "vlan": {
                "match": {
                  "single-tagged": {
                    "config": {
                      "vlan-id": 4010
                    }
                  }
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "GEO_3",
          "enabled": true,
          "mtu": 9080,
          "name": "Bundle-Ether5",
          "type": "ieee8023adLag"
        },
        "name": "Bundle-Ether5",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "description": "GEO_3",
                "enabled": true,
                "index": 4050
              },
              "index": 4050,
              "vlan": {
                "match": {
                  "single-tagged": {
                    "config": {
                      "vlan-id": 4050
                    }
                  }
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "GEO_4",
          "enabled": true,
          "mtu": 9080,
          "name": "Bundle-Ether43",
          "type": "ieee8023adLag"
        },
        "name": "Bundle-Ether43",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "description": "GEO_4",
                "enabled": true,
                "index": 1100
              },
              "index": 1100,
              "vlan": {
                "match": {
                  "single-tagged": {
                    "config": {
                      "vlan-id": 1100
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


``` json
{
  "oam": {
    "cfm": {
      "domains": {
        "maintenance-domain": [
          {
            "config": {
              "char-string": "D1",
              "level": 5,
              "md-id": "1",
              "md-name-type": "CHARACTER_STRING"
            },
            "maintenance-associations": {
              "maintenance-association": [
                {
                  "config": {
                    "ccm-interval": "300MS",
                    "group-name": "GEO_1",
                    "loss-threshold": 3,
                    "ma-id": "S1",
                    "ma-name-type": "UINT16",
                    "unsigned-int16": 1
                  },
                  "ma-id": "S1",
                  "mep-endpoints": {
                    "mep-endpoint": [
                      {
                        "config": {
                          "ccm-enabled": true,
                          "direction": "UP",
                          "interface": "Bundle-Ether43.1600",
                          "local-mep-id": 40
                        },
                        "local-mep-id": 40,
                        "pm-profiles": {
                          "pm-profile": [
                            {
                              "config": {
                                "profile-name": "cfm_delay_Bundle-Ether43_1600"
                              },
                              "profile-name": "cfm_delay_Bundle-Ether43_1600"
                            },
                            {
                              "config": {
                                "profile-name": "cfm_loss_Bundle-Ether43_1600"
                              },
                              "profile-name": "cfm_loss_Bundle-Ether43_1600"
                            }
                          ]
                        },
                        "rdi": {
                          "config": {
                            "transmit-on-defect": true
                          }
                        },
                        "remote-meps": {
                          "remote-mep": [
                            {
                              "config": {
                                "id": 39
                              },
                              "id": 39
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            },
            "md-id": "1"
          },
          {
            "config": {
              "char-string": "D2",
              "level": 5,
              "md-id": "2",
              "md-name-type": "CHARACTER_STRING"
            },
            "maintenance-associations": {
              "maintenance-association": [
                {
                  "config": {
                    "ccm-interval": "300MS",
                    "group-name": "GEO_2",
                    "loss-threshold": 3,
                    "ma-id": "S1",
                    "ma-name-type": "UINT16",
                    "unsigned-int16": 1
                  },
                  "ma-id": "S1",
                  "mep-endpoints": {
                    "mep-endpoint": [
                      {
                        "config": {
                          "ccm-enabled": true,
                          "direction": "UP",
                          "interface": "Bundle-Ether5.4010",
                          "local-mep-id": 6
                        },
                        "local-mep-id": 6,
                        "pm-profiles": {
                          "pm-profile": [
                            {
                              "config": {
                                "profile-name": "cfm_delay_Bundle-Ether5_4010"
                              },
                              "profile-name": "cfm_delay_Bundle-Ether5_4010"
                            },
                            {
                              "config": {
                                "profile-name": "cfm_loss_Bundle-Ether5_4010"
                              },
                              "profile-name": "cfm_loss_Bundle-Ether5_4010"
                            }
                          ]
                        },
                        "rdi": {
                          "config": {
                            "transmit-on-defect": true
                          }
                        },
                        "remote-meps": {
                          "remote-mep": [
                            {
                              "config": {
                                "id": 5
                              },
                              "id": 5
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            },
            "md-id": "2"
          },
          {
            "config": {
              "char-string": "D3",
              "level": 5,
              "md-id": "3",
              "md-name-type": "CHARACTER_STRING"
            },
            "maintenance-associations": {
              "maintenance-association": [
                {
                  "config": {
                    "ccm-interval": "300MS",
                    "group-name": "GEO_3",
                    "loss-threshold": 3,
                    "ma-id": "S1",
                    "ma-name-type": "UINT16",
                    "unsigned-int16": 1
                  },
                  "ma-id": "S1",
                  "mep-endpoints": {
                    "mep-endpoint": [
                      {
                        "config": {
                          "ccm-enabled": true,
                          "direction": "UP",
                          "interface": "Bundle-Ether5.4050",
                          "local-mep-id": 8
                        },
                        "local-mep-id": 8,
                        "pm-profiles": {
                          "pm-profile": [
                            {
                              "config": {
                                "profile-name": "cfm_delay_Bundle-Ether5_4050"
                              },
                              "profile-name": "cfm_delay_Bundle-Ether5_4050"
                            },
                            {
                              "config": {
                                "profile-name": "cfm_loss_Bundle-Ether5_4050"
                              },
                              "profile-name": "cfm_loss_Bundle-Ether5_4050"
                            }
                          ]
                        },
                        "rdi": {
                          "config": {
                            "transmit-on-defect": true
                          }
                        },
                        "remote-meps": {
                          "remote-mep": [
                            {
                              "config": {
                                "id": 7
                              },
                              "id": 7
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            },
            "md-id": "3"
          },
          {
            "config": {
              "char-string": "D4",
              "level": 5,
              "md-id": "4",
              "md-name-type": "CHARACTER_STRING"
            },
            "maintenance-associations": {
              "maintenance-association": [
                {
                  "config": {
                    "ccm-interval": "300MS",
                    "group-name": "GEO_4",
                    "loss-threshold": 3,
                    "ma-id": "S1",
                    "ma-name-type": "UINT16",
                    "unsigned-int16": 1
                  },
                  "ma-id": "S1",
                  "mep-endpoints": {
                    "mep-endpoint": [
                      {
                        "config": {
                          "ccm-enabled": true,
                          "direction": "UP",
                          "interface": "Bundle-Ether43.1100",
                          "local-mep-id": 36
                        },
                        "local-mep-id": 36,
                        "pm-profiles": {
                          "pm-profile": [
                            {
                              "config": {
                                "profile-name": "cfm_delay_Bundle-Ether43_1100"
                              },
                              "profile-name": "cfm_delay_Bundle-Ether43_1100"
                            },
                            {
                              "config": {
                                "profile-name": "cfm_loss_Bundle-Ether43_1100"
                              },
                              "profile-name": "cfm_loss_Bundle-Ether43_1100"
                            }
                          ]
                        },
                        "rdi": {
                          "config": {
                            "transmit-on-defect": true
                          }
                        },
                        "remote-meps": {
                          "remote-mep": [
                            {
                              "config": {
                                "id": 35
                              },
                              "id": 35
                            }
                          ]
                        }
                      }
                    ]
                  }
                }
              ]
            },
            "md-id": "4"
          }
        ]
      },
      "performance-measurement-profiles-global": {
        "performance-measurement-profile": [
          {
            "config": {
              "burst-interval": 10000,
              "intervals-archived": 5,
              "measurement-interval": 1,
              "measurement-type": "DMM",
              "packet-per-burst": 100,
              "packets-per-meaurement-period": 60,
              "profile-name": "cfm_delay_Bundle-Ether43_1100",
              "repetition-period": 1
            },
            "profile-name": "cfm_delay_Bundle-Ether43_1100"
          },
          {
            "config": {
              "burst-interval": 10000,
              "intervals-archived": 5,
              "measurement-interval": 1,
              "measurement-type": "DMM",
              "packet-per-burst": 100,
              "packets-per-meaurement-period": 60,
              "profile-name": "cfm_delay_Bundle-Ether43_1600",
              "repetition-period": 1
            },
            "profile-name": "cfm_delay_Bundle-Ether43_1600"
          },
          {
            "config": {
              "burst-interval": 10000,
              "intervals-archived": 5,
              "measurement-interval": 1,
              "measurement-type": "DMM",
              "packet-per-burst": 100,
              "packets-per-meaurement-period": 60,
              "profile-name": "cfm_delay_Bundle-Ether5_4010",
              "repetition-period": 1
            },
            "profile-name": "cfm_delay_Bundle-Ether5_4010"
          },
          {
            "config": {
              "burst-interval": 10000,
              "intervals-archived": 5,
              "measurement-interval": 1,
              "measurement-type": "DMM",
              "packet-per-burst": 100,
              "packets-per-meaurement-period": 60,
              "profile-name": "cfm_delay_Bundle-Ether5_4050",
              "repetition-period": 1
            },
            "profile-name": "cfm_delay_Bundle-Ether5_4050"
          },
          {
            "config": {
              "burst-interval": 10000,
              "intervals-archived": 5,
              "measurement-interval": 1,
              "measurement-type": "SLM",
              "packet-per-burst": 100,
              "packets-per-meaurement-period": 60,
              "profile-name": "cfm_loss_Bundle-Ether43_1100",
              "repetition-period": 1
            },
            "profile-name": "cfm_loss_Bundle-Ether43_1100"
          },
          {
            "config": {
              "burst-interval": 10000,
              "intervals-archived": 5,
              "measurement-interval": 1,
              "measurement-type": "SLM",
              "packet-per-burst": 100,
              "packets-per-meaurement-period": 60,
              "profile-name": "cfm_loss_Bundle-Ether43_1600",
              "repetition-period": 1
            },
            "profile-name": "cfm_loss_Bundle-Ether43_1600"
          },
          {
            "config": {
              "burst-interval": 10000,
              "intervals-archived": 5,
              "measurement-interval": 1,
              "measurement-type": "SLM",
              "packet-per-burst": 100,
              "packets-per-meaurement-period": 60,
              "profile-name": "cfm_loss_Bundle-Ether5_4010",
              "repetition-period": 1
            },
            "profile-name": "cfm_loss_Bundle-Ether5_4010"
          },
          {
            "config": {
              "burst-interval": 10000,
              "intervals-archived": 5,
              "measurement-interval": 1,
              "measurement-type": "SLM",
              "packet-per-burst": 100,
              "packets-per-meaurement-period": 60,
              "profile-name": "cfm_loss_Bundle-Ether5_4050",
              "repetition-period": 1
            },
            "profile-name": "cfm_loss_Bundle-Ether5_4050"
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage  

```yaml
paths:
    # interface configs
    /interfaces/interface/config/description:
    /interfaces/interface/config/enabled:
    /interfaces/interface/config/mtu:
    /interfaces/interface/config/name:
    /interfaces/interface/config/type:
    /interfaces/interface/rates/config/load-interval:
    /interfaces/interface/subinterfaces/subinterface/config/description:
    /interfaces/interface/subinterfaces/subinterface/config/enabled:
    /interfaces/interface/subinterfaces/subinterface/config/index:
    /interfaces/interface/subinterfaces/subinterface/ipv4/config/mtu:
    /interfaces/interface/subinterfaces/subinterface/ipv6/config/mtu:
    /interfaces/interface/aggregation/config/lag-type:

    #psuedowire configs
    /network-instances/network-instance/config/name:
    /network-instances/network-instance/config/type:
    /network-instances/network-instance/connection-points/connection-point/config/connection-point-id:
    /network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/config/endpoint-id:
    /network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/local/config/interface:
    /network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/local/config/subinterface:
    /network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/remote/config/virtual-circuit-identifier:

    #TODO: Add new OCs for labels and next-hop-group under connection-point
    #/network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/local/config/local-label
    #/network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/local/config/remote-label
    #/network-instances/network-instance/connection-points/connection-point/endpoints/endpoint/remote/config/next-hop-group


    #Tunnels/Next-hop group configs
    #TODO: Revisit and update as per https://github.com/openconfig/public/pull/1308
    #network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/index:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/config/next-hop:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/config/index:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/type:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dst-ip:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/src-ip:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/dscp:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/ip-ttl:
    #/network-instances/network-instance/static/next-hop-groups/next-hop-group/nexthops/nexthop/encap-headers/encap-header/gre/config/index:


    # Telemetry paths

    /oam/cfm/domains/maintenance-domain/state/md-id:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/state/ma-id:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/state/local-mep-id:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/remote-meps/remote-mep/state/id:
    /oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/state/profile-name:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/delay-measurement-state/frame-delay-two-way-average:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/delay-measurement-state/frame-delay-two-way-max:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/delay-measurement-state/frame-delay-two-way-min:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/measurement-type:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/far-end-average-frame-loss-ratio:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/far-end-max-frame-loss-ratio:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/far-end-min-frame-loss-ratio:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/near-end-average-frame-loss-ratio:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/near-end-max-frame-loss-ratio:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/near-end-min-frame-loss-ratio:


    # Config paths for GRE decap
    /network-instances/network-instance/policy-forwarding/policies/policy/rules/rule/action/config/decapsulate-gre:

    # Config Paths for CFM
    #/oam/cfm/domains/domain/config/name:
    /oam/cfm/domains/maintenance-domain/config/level:
    /oam/cfm/domains/maintenance-domain/config/md-id:
    /oam/cfm/domains/maintenance-domain/config/md-name-type:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/config/ccm-interval:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/config/loss-threshold:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/config/ma-id:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/config/ma-name-type:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/config/ccm-enabled:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/config/direction:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/config/interface:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/config/local-mep-id:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/config/profile-name:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/rdi/config/transmit-on-defect:
    /oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/remote-meps/remote-mep/config/id:
    /oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/config/burst-interval:
    /oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/config/intervals-archived:
    /oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/config/measurement-interval:
    /oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/config/measurement-type:
    /oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/config/packet-per-burst:
    /oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/config/packets-per-meaurement-period:
    /oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/config/profile-name:
    /oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/config/repetition-period:

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