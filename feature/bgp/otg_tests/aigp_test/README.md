# RT-1.36: AIGP feature support test

## Summary

This test validates the support for AIGP ensuring the following use-cases
1. The attribute can be transmitted and received between two bgp peers
2. The propagation of the attribute can be controlled both per bgp neighbor and peer-group
3. The attribute can be modified as needed with a policy on both import and export direction on a BGP peer

*   [dutdutate.testbed](https://github.com/openconfig/featureprofiles/blob/main/topologies/dutdutate.testbed)

## Procedure

### Test environment setup

#### Test Topology

```text
+---------+      Lag1[eBGP]         +---------+      LAG2 [iBGP]        +---------+
|   ATE   |=========================|  DUT 1  |=========================|  DUT 2  |
+---------+                         +---------+                         +---------+

```
#### IPV4 Addresses Lag1 (Table 1)

Device | vlan 10 | vlan 20 | vlan 30 | vlan 40
:------| :----------| :-------- | :---------| :-------:
DUT 1  | 198.51.100.1/30 | 198.51.100.5/30 | 198.51.100.9/30 | 198.51.100.13/30
ATE    | 198.51.100.2/30 | 198.51.100.6/30 | 198.51.100.10/30 | 198.51.100.14/30

#### IPV6 Addresses Lag1 (Table 2)

Device | vlan 10 | vlan 20 | vlan 30 | vlan 40
:------| :----------| :-------- | :---------| :-------:
DUT  1 | 2001:db8::1/126 | 2001:db8::5/126 | 2001:db8::9/126 | 2001:db8::13/126
ATE    | 2001:db8::2/126 | 2001:db8::6/126 | 2001:db8::10/126 | 2001:db8::14/126

#### IP Addresses LAG 2 (Table 3)

Device | vlan 10 IPV4| vlan 10 ipv6| vlan 20 IPV4 | vlan 20 ipv6
:------| :----------| :-------- | :---------| :-------------:
DUT 1  | 198.51.100.17/30 | 2001:db8::17/126 | 198.51.100.21/30 | 2001:db8::21/126
DUT 2  | 198.51.100.18/30 | 2001:db8::18/126 | 198.51.100.22/30 | 2001:db8::22/126

#### IP Addesses LAG 2( DUT 1 towards DUT2 test-originate NI ) (Table 4)

Device | vlan 30 IPV4| vlan 30 ipv6 | vlan 40 IPV4 | vlan 40 ipv6
:------| :----------| :-------- | :---------| :-------------:
DUT 1  | 198.51.100.25/30 | 2001:db8::25/126 | 198.51.100.29/30 | 2001:db8::29/126
DUT 2  | 198.51.100.26/30 | 2001:db8::26/126 | 198.51.100.30/30 | 2001:db8::30/126

#### BGP Autonomous System Number (ASN) (Table 5)

Device |  ASN
:------|:----------
ATE    | 64496
DUT 1 Default NI        | 64497
DUT 1 test-instance NI  | 64498
DUT 2 Default NI        | 64497
DUT 2 test-instance NI  | 64498
DUT 2 test-originate NI | 64499

#### ISIS Network Entity (Table 6)
Network/ Network instance |  Entity
:-------------------------|:------------------
DUT 1 Default NI          | 49.0001.1980.5110.0025.00
DUT 1 test-instance NI    | 49.0001.1980.5110.0029.00
DUT 2 Default NI          | 49.0001.1980.5110.0026.00
DUT 2 test-instance NI    | 49.0001.1980.5110.0030.00
DUT 2 test-originate NI   | 49.0001.1980.5110.0100.00

#### ATE Traffic Profile (Table 7)
Flow name | Source | Destination | flow size |  flow rate(percent) | flow vlan | Flow Src MAC | source interface | Destination interface
:------| :----------| :----------|:----------|:----------|:----------| :----------|:----------| :-------------:
Flow 1  | 198.51.210.1 | 198.55.1.1 | 512 |  5 | 10 | 02:00:03:01:02:01 | eth1.10 | eth1.20 
Flow 2  | 198.51.210.1 | 198.55.2.1 | 512 |  5 | 30 | 02:00:03:01:04:03 |  eth1.30 | eth1.40 
Flow 3  | 2001:db8:10::1 | 2001:db8:50::1 | 512 |  5 | 10 | 02:00:03:01:02:01 | eth1.10 | eth1.20 
Flow 4  | 2001:db8:10::1 | 2001:db8:60::1 | 512 |  5 | 30 | 02:00:03:01:04:03 |  eth1.30 | eth1.40 

#### ATE Interfaces MAC Addresses (Table 8)
Interface | MAC Address 
:------| :----------:
eth1.10  | 02:00:03:01:01:01
eth1.20  | 02:00:03:01:02:02
eth1.30 | 02:00:03:01:03:03
eth1.40 | 02:00:03:01:04:04

## RT-1.36.1 : AIGP modification with BGP policy, propagation control on BGP peers and next-hop self feature

### DUT 1 - Generate Configuration

#### Interface Configuration

*  Create two lags named Lag1 and lag2 with both running LACP
*  Configure both Lag1 and lag2 as the LACP active end
*  Configure port 1 as a Lag1 member
*  Configure port 2 as a Lag 2 member
*  Create a non-default network instance and name it test-instance
*  On Lag1, create subinterfaces in vlan 10 and 20
      * Add these subinterfaces to the DEFAULT network instance
      * Configure IPV4 on the subinterfaces as specified on table 1
      * Configure IPV6 on the subinterfaces as specified on Table 2
*  On Lag1, create subinterfaces in vlan 30 and 40
      * Add these subinterfaces to test-instance network instance
      * Configure IPV4 on the subinterfaces as specified on table 1
      * Configure IPV6 on the subinterfaces as specified on Table 2
*  On lag 2, create subinterface in vlan 10
      * Add the subinterface to DEFAULT network instance
      * Configure IPV4 and IPV6 addresses on the subinterfaces as specified on table 3
*  On lag 2, create subinterface in vlan 20
      * Add the subinterface to test-instance network instance
      * Configure IPV4 and IPV6 addresses on the subinterfaces as specified on table 3
* Create a Loopback interface called Loopback10
    * Configure IPV4 and IPV6 addresses on the Loopback interfaces as specified below
        * IPV4 Address: 198.55.1.1/32
        * IPV6 Address: 2001:db8:50::1/128
    * Add the Loopback interface to the DEFAULT Network instance
* Create a Loopback interface called Loopback20
    * Configure IPV4 and IPV6 addresses on the Loopback interfaces as specified below
        * IPV4 Address: 198.55.2.1/32
        * IPV6 Address: 2001:db8:60::1/128
    * Add the Loopback interface to the test-instance Network-instance

#### Create BGP import policy

*  Create a route-policy named test-import-policy_aigp_20
    *  Create a statement named test-import-statement
    *  Set an action of aigp 20 and accept the route
*  Create a route-policy named test-import-policy_aigp_150
    *  Create a statement named test-import-statement
    *  Set an action of aigp 150 and accept the route

#### Create BGP export policy

*  Create a route-policy named test-export-policy
*  Create a statement named test-export-statement
*  Create an action to set the aigp 200, next hop as self and finally accept the route

#### BGP configuration on DUT 1
DUT 1 is going to run bgp in both of its DEFAULT and test-instance network
instances. The DEFAULT network instance will run BGP on ASN 64497 and 64498 will
be the ASN on the test-instance as specified on Table 5.

##### BGP Configuration - Default Network instance

*  Configure BGP with ASN 64497 as specified on Table 5
*  Create four bgp peer-groups named uplink,uplink6 and downlink and downlink6
*  Configure peer-as on these peer-groups as specified below
      * peer-group: uplink; peer-as: 64496
      * peer-group: uplink6; peer-as: 64496
      * peer-group: downlink; peer-as: 64497
      * peer-group: downlink6; peer-as: 64497
*  Configure peers in peer-group downlink and downlink6 as route-reflector clients with cluster-id 1.1.1.1
*  Create the following bgp peers with their peer-group, these BGP peers will be
    established over vlan 10 and vlan 20 respectively on Lag1 towards the ATE
      * Peer address: 198.51.100.2; peer-group: uplink
      * Peer address: 198.51.100.6; peer-group: uplink
      * Peer address: 2001:db8::2; peer-group: uplink6
      * Peer address: 2001:db8::6; peer-group: uplink6
*  Apply test-import-policy_aigp_150 in the import direction of both peer 198.51.100.2
   and 2001:db8::2 respectively
*  Apply test-import-policy_aigp_20 in the import direction of both peer 198.51.100.6
   and 2001:db8::6 respectively
* Create the following BGP peers with their peer-group, These BGP peers will be
    established over vlan 10 on LAG 2 towards DUT 2
      * Peer address: 198.51.100.18; peer-group: downlink
      * Peer address: 2001:db8::18; peer-group: downlink6
*  Apply test-export-policy in the export direction on both 198.51.100.18 and
    2001:db8::18
*  Enable AIGP exchange on peers 198.51.100.2, 2001:db8::2, 198.51.100.18,
    2001:db8::18,198.51.100.6 and 2001:db8::6

##### BGP Configuration - test-instance Network instance

*  Configure BGP with ASN 64498 as specified on Table 5
*  Create four bgp peer-groups named uplink,uplink6 and downlink and downlink6
*  Configure peer-as on these peer-groups as specified below
      * peer-group: uplink; peer-as: 64496
      * peer-group: uplink6; peer-as: 64496
      * peer-group: downlink; peer-as: 64498
      * peer-group: downlink6; peer-as: 64498
*  Configure peers in peer-group downlink and downlink6 as route-reflector clients with cluster-id 1.1.1.1
*  Create the following bgp peers with their respective peer-groups, these BGP
   peers will be established over vlan 30 and vlan 40 respectively on Lag1
   towards the ATE
      * Peer address: 198.51.100.10; peer-group: uplink
      * Peer address: 198.51.100.14; peer-group: uplink
      * Peer address: 2001:db8::10; peer-group: uplink6
      * Peer address: 2001:db8::14; peer-group: uplink6
*  Apply test-import-policy_aigp_150 in the import direction of both peer 198.51.100.10
   and 2001:db8::10 respectively
*  Apply test-import-policy_aigp_20 in the import direction of both peer 198.51.100.14
   and 2001:db8::14 respectively
* Create the following BGP peers with their respective peer-group, These BGP
  peer will be established over vlan 20 on LAG 2 towards DUT 2
      * Peer address: 198.51.100.22; peer-group: downlink
      * Peer address: 2001:db8::22; peer-group: downlink6
*  Apply test-export-policy in the export direction on both 198.51.100.22 and
    2001:db8::22
*  Enable AIGP exchange on 198.51.100.10, 2001:db8::10, 198.51.100.22,2001:db8::22,
    198.51.100.14 and 2001:db8::14

### DUT 2 - Generate Configuration

#### Interface Configuration

*  Create a lag named lag2 running LACP
*  Configure lag2 as the LACP PASSIVE end
*  Configure port 1 as a lag2 member
*  Create a non-default network instance and name it test-instance
*  On lag 2, create a subinterface in vlan 10
      * Add the subinterfaces to the DEFAULT network instance
      * Configure IPV4 on the subinterface as specified on table 3
      * Configure IPV6 on the subinterface as specified on Table 3
*  On lag 2, create another subinterface in vlan 20
      * Add the subinterface to test-instance network instance
      * Configure IPV4 on the subinterface as specified on table 3
      * Configure IPV6 on the subinterface as specified on Table 3

#### BGP configuration on DUT 2

DUT 2 is also going to run bgp in both of its DEFAULT and test-instance network
instances. The DEFAULT network instance will run BGP on ASN 64497 and 64498 will
be the ASN on the test-instance as specified on Table 5.

##### BGP Configuration - Default Network instance

*  Configure BGP with ASN 64497 as specified on Table 5
*  Create 2 peer-groups named uplink and uplink6
*  Configure peer-as on these peer-groups as specified below
      * peer-group: uplink; peer-as: 64497
      * peer-group: uplink6; peer-as: 64497
*  Create the following bgp peers, these BGP peers will be established over
    vlan 10 towards DUT 1
      * Peer address: 198.51.100.17; peer-group: uplink
      * Peer address: 2001:db8::17; peer-group: uplink6
*  Enable AIGP exchange on both peers,that is 198.51.100.17 and 2001:db8::17

##### BGP Configuration - test-instance Network instance

*  Configure BGP with ASN 64498 as specified on Table 5
*  Create 2 peer-groups named uplink and uplink6
*  Configure peer-as on these peer-groups as specified below
      * peer-group: uplink; peer-as: 64498
      * peer-group: uplink6; peer-as: 64498
*  Create the following bgp peers with their respective peer-groups, these BGP
   peers will be established over vlan 20 towards DUT 1
      * Peer address: 198.51.100.21; peer-group: uplink
      * Peer address: 2001:db8::21; peer-group: uplink6
*  Enable AIGP exchange on both peers,that is 198.51.100.21 and 2001:db8::21

### ATE - Generate Configuration

1. Create an aggregate interface with LACP named Lag1
2. Add port 1 to Lag1
3. Create emulated router
4. Create 4 ethernet interfaces on the router, the ethernet interfaces
    named eth1.10, eth1.20, eth1.30, eth1.40 respectively
5. Connect ethernet interfaces to the aggregate interface
6. Configure Vlans 10,20,30 and 40 respectively on the ethernet interfaces
   according to the following mapping
      * eth1.10 - vlan 10
      * eth1.20 - vlan 20
      * eth1.30 - vlan 30
      * eth1.40 - vlan 40
7. Configure IPV4 addresses and gateway on the subinterfaces according to table 1
8. Configure IPV6 addresses on the subinterfaces according to table 2
9. Wait for lag interface to come up
10. Create the following ipv4 and ipv6 BGP peers of type EBGP on the emulated Router
     * Peer address: 198.51.100.1; remote AS: 64497
     * Peer address: 198.51.100.5; remote AS: 64497
     * Peer address: 198.51.100.9; remote AS: 64498
     * Peer address: 198.51.100.13; remote AS: 64498
     * Peer address: 2001:db8::1; remote AS: 64497
     * Peer address: 2001:db8::5; remote AS: 64497
     * Peer address: 2001:db8::9; remote AS: 64498
     * Peer address: 2001:db8::13; remote AS: 64498
11. Wait for the BGP peers to come up
12. Create the following ipv4 and ipv6 Routes on the emulated router
     * 198.51.210.0/24
     * 198.51.220.0/24
     * 2001:db8:10::/64
     * 2001:db8:20::/64
13. Create Traffic flow according the the specification on Table 7
14. Push config on to the OTG
15. Start Protocol on the OTG

### Testing Steps

#### Interface Testing Steps

1. Validate that all physical interfaces on the ATE, DUT1 and DUT 2 are UP
2. Validate that Aggregate interfaces on the ATE, DUT1 and DUT 2 are UP
3. On DUT1, collect the out-packet counter information on interface Lag1 subinterfaces vlan 20 and 40

#### BGP testing Steps on DUT 1

* In the default NI:
    - Fetch the BGP session-state of the peers 198.51.100.2, 2001:db8::2, 198.51.100.6 and 2001:db8::6
    - Fetch the AIGP propagation status of the peers 198.51.100.2, 2001:db8::2, 198.51.100.6 and 2001:db8::6
    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.6 and 2001:db8::6

* In the test-instance NI:
    - Fetch the BGP session-state of the peers 198.51.100.10, 2001:db8::10, 198.51.100.14 and 2001:db8::14
    - Fetch the AIGP propagation status of the peers 198.51.100.10, 2001:db8::10, 198.51.100.14 and 2001:db8::14
    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.14 and 2001:db8::14

#### BGP testing Steps on DUT 2

* In the default NI:
    - Fetch the BGP session-state of the peers 198.51.100.17 and 2001:db8::17
    - Fetch the AIGP propagation status of the peers 198.51.100.17 and 2001:db8::17
    - Fetch the BGP attributes of of the routes received from peers 198.51.100.17 and 2001:db8::17

* In the test-instance NI:
    - Fetch the BGP session-state of the peers 198.51.100.21 and 2001:db8::21
    - Fetch the AIGP propagation status of the peers 198.51.100.21 and 2001:db8::21
    - Fetch the BGP attributes of the routes received from the peers 198.51.100.21 and 2001:db8::21

#### Testng step on the ATE

* Start Traffic on the OTG
* Wait for 60 second while the traffic is flowing
* Stop traffic on the OTG

### Test Pass Criteria

#### Pass criteria on DUT 1

1. All physical interfaces must be UP
2. All aggregate interfaces must be UP
3. The out-packet counter information on interface Lag1 subinterfaces vlan 20 and 40 must be non-zero
4. All BGP peers must be in Established state
5. In the default NI, AIGP propagation must be true for the BGP peers
    198.51.100.2,2001:db8::2, 198.51.100.6 and 2001:db8::6 
6. In the default NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.2 and
    2001:db8::2 must have AIGP value of 150
7. In the default NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.6 and
    2001:db8::6 must have AIGP value of 20
8. In the default NI, Routes(IPV4 and IPV6) received from BGP peers 198.51.100.6 and
    2001:db8::6 must have the best-path state in adj-rib-in as true
9. In test-instance NI, AIGP propagation must be true for the BGP peers
    198.51.100.10,2001:db8::10, 198.51.100.14 and 2001:db8::14  
10. In test-instance NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.10
    and 2001:db8::10 must have AIGP value of 150
11. In test-instance NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.14
    and 2001:db8::14 must have AIGP value of 20
12. In test-instance NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.14
    and 2001:db8::14 must have the best-path state in adj-rib-in as true

#### Test pass criteria on DUT 2

1. All BGP peers must be in Established state
2. In the default NI, routes received from the BGP peers 198.51.100.17 and
   2001:db8::17 must have an AIGP value of 200
3. In the default NI, IPV4 route received from the peer 198.51.100.17 must have
   a next-hop address of 198.51.100.17 while ipv6 routes received from 2001:db8::17
   must have a next-hop address of 2001:db8::17
4. In the default NI, AIGP propagation must be true for bgp peers 198.51.100.17
   and 2001:db8::17
5. In test-instance NI, routes received from the BGP peers 198.51.100.21 and
   2001:db8::21 must have a AIGP value of 200
6. In test-instance NI, IPV4 route received from the peer 198.51.100.21 must have
   a next-hop address of 198.51.100.21 while ipv6 routes received from 2001:db8::21
   must have a next-hop address of 2001:db8::21
7. In test-instance NI, AIGP propagation must be true for the peers 198.51.100.21
   and 2001:db8::21

#### Test pass criteria on on the ATE

* The packet loss metric from each of the flows must be less than 2 percentage
    
### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback10.0",
                "interface": "Loopback10",
                "subinterface": 0
              }
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "DEFAULT"
              },
              "bgp": {
                "global": {
                  "config": {
                    "as": 64497
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "uplink",
                      "config": {
                        "peer-group-name": "uplink",
                        "peer-as": 64496
                      }
                    },
                    {
                      "peer-group-name": "uplink6",
                      "config": {
                        "peer-group-name": "uplink6",
                        "peer-as": 64496
                      }
                    },
                    {
                      "peer-group-name": "downlink",
                      "config": {
                        "peer-group-name": "downlink",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "1.1.1.1"
                        }
                      }
                    },
                    {
                      "peer-group-name": "downlink6",
                      "config": {
                        "peer-group-name": "downlink6",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "1.1.1.1"
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "198.51.100.2",
                        "peer-group": "uplink"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_150"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.6",
                        "peer-group": "uplink"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_20"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::2",
                        "peer-group": "uplink6"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_150"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::6",
                        "peer-group": "uplink6"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_20"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.18",
                        "peer-group": "downlink"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "test-export-policy"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::18",
                        "peer-group": "downlink6"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "test-export-policy"
                          ]
                        }
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "name": "test-instance"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback20.0",
                "interface": "Loopback20",
                "subinterface": 0
              }
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "DEFAULT"
              },
              "bgp": {
                "global": {
                  "config": {
                    "as": 64498
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "uplink",
                      "config": {
                        "peer-group-name": "uplink",
                        "peer-as": 64496
                      }
                    },
                    {
                      "peer-group-name": "uplink6",
                      "config": {
                        "peer-group-name": "uplink6",
                        "peer-as": 64496
                      }
                    },
                    {
                      "peer-group-name": "downlink",
                      "config": {
                        "peer-group-name": "downlink",
                        "peer-as": 64498
                      }
                    },
                    {
                      "peer-group-name": "downlink6",
                      "config": {
                        "peer-group-name": "downlink6",
                        "peer-as": 64498
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "198.51.100.10",
                        "peer-group": "uplink"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_150"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.14",
                        "peer-group": "uplink"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_20"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::10",
                        "peer-group": "uplink6"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_150"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::14",
                        "peer-group": "uplink6"
                      },
                      "apply-policy": {
                        "config": {
                          "import-policy": [
                            "test-import-policy_aigp_20"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.22",
                        "peer-group": "downlink"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "test-export-policy"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::22",
                        "peer-group": "downlink6"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "test-export-policy"
                          ]
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
  },
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "test-import-policy_aigp_150"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-import-statement"
                },
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        },
        {
          "config": {
            "name": "test-import-policy_aigp_20"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-import-statement"
                },
                "actions": {
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        },
        {
          "config": {
            "name": "test-export-policy"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-export-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-next-hop": "SELF"
                    }
                  },
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
  },
  "interfaces": {
    "interface": [
      {
        "name": "Loopback10",
        "config": {
          "name": "Loopback10",
          "type": "iana-if-type:softwareLoopback",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.55.1.1",
                        "prefix-length": 32
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:50::1",
                        "prefix-length": 128
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "name": "Loopback20",
        "config": {
          "name": "Loopback20",
          "type": "iana-if-type:softwareLoopback",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.55.2.1",
                        "prefix-length": 32
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:60::1",
                        "prefix-length": 128
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
## RT-1.36.2 : Validate other Attribute(AS-PATH) as tie-breaker when AIGP is the same

The following configuration steps are additions to the configuration done on
Test-case RT-1.36.1 above.

### DUT 1 - Generate Configuration

* On route-policy test-import-policy_aigp_20 statement test-import-statement, make the following modification
  * re-set the AIGP metric from 20 to 150
  * Add a new bgp action to prepend asn 64496 5 times

### Testing Steps

#### Testing Steps on DUT 1

* In the default NI:
    - Fetch the BGP session-state of the peers 198.51.100.2, 2001:db8::2, 198.51.100.6 and 2001:db8::6
    - Fetch the AIGP propagation status of the peers 198.51.100.2, 2001:db8::2, 198.51.100.6 and 2001:db8::6
    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.6 and 2001:db8::6

* In the test-instance NI:
    - Fetch the BGP session-state of the peers 198.51.100.10, 2001:db8::10, 198.51.100.14 and 2001:db8::14
    - Fetch the AIGP propagation status of the peers 198.51.100.10, 2001:db8::10, 198.51.100.14 and 2001:db8::14
    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.14 and 2001:db8::14

### Test Pass Criteria

#### Pass criteria on DUT 1

1. All BGP peers must be in Established state
2. In the default NI, AIGP propagation must be true for the BGP peers
    198.51.100.2,2001:db8::2, 198.51.100.6 and 2001:db8::6 
3. In the default NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.2 and
    2001:db8::2 must have AIGP value of 150
4. In the default NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.6 and
    2001:db8::6 must have AIGP value of 150
5. In the default NI, Routes(IPV4 and IPV6) received from BGP peers 198.51.100.2 and
    2001:db8::2 must have the best-path state in adj-rib-in as true
6. In test-instance NI, AIGP propagation must be true for the BGP peers
    198.51.100.10,2001:db8::10, 198.51.100.14 and 2001:db8::14  
7. In test-instance NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.10
    and 2001:db8::10 must have AIGP value of 150
8. In test-instance NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.14
    and 2001:db8::14 must have AIGP value of 150
9. In test-instance NI, Routes(IPV4 and IPV6) received from the BGP peers 198.51.100.10
    and 2001:db8::10 must have the best-path state in adj-rib-in as true

### Canonical OC

```
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "test-import-policy_aigp_20"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-import-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "set-as-path-prepend": {
                      "config": {
                        "repeat-n": 5,
                        "asn": 64496
                      }
                    }
                  },
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

## RT-1.36.3 : Validate AIGP propagation, propagation enabled by default on IBGP peers and next-hop IP feature

The following configuration steps are additions to the configuration done on
Test-case RT-1.36.2 above.

#### Effective Emulated Test Topology

```text
+-------------------------+ LAG2(vlan 30 and 40) [iBGP]    +-------------+          LAG2 [iBGP]          +---------+
|  DUT2(test-originate NI)|===============================|  DUT 1(RR)   |===============================|  DUT 2  |
+-------------------------+      ISIS                      +-------------+             ISIS              +---------+

```

### DUT 1 - Generate Configuration

#### Interface Configuration

* On lag 2, create subinterface in vlan 30
    * Add the subinterface to the DEFAULT Network instance
    * Configure IPV4 and IPV6 addresses on the subinterfaces as specified on table 4
* On lag 2, create subinterface in vlan 40
    * Add the subinterface to the test-instance Network-instance
    * Configure IPV4 and IPV6 addresses on the subinterfaces as specified on table 4


#### Create BGP export policy

*  Create a route-policy named default-export-v4
    * Create a statement named test-statement
    * Create the following actions
      * set-next-hop 198.55.1.1
      * accept the route

*  Create a route-policy named default-export-v6
    * Create a statement named test-statement
    * Create the following actions
      * set-next-hop 2001:db8:50::1
      * accept the route

*  Create a route-policy named non-default-export-v4
    * Create a statement named test-statement
    * Create the following actions
      * set-next-hop 198.55.2.1
      * accept the route

*  Create a route-policy named non-default-export-v6
    * Create a statement named test-statement
    * Create the following actions
      * set-next-hop 2001:db8:60::1
      * accept the route

#### Configure ISIS

##### ISIS Configuration - Default Network instance

* Create ISIS configuration instance named DEFAULT
* Configure the network entity as specified on Table 6
* Configure metric-style wide
* Add subinterfaces Lag 2 vlan 10 and lag 2 vlan 30 to the isis process and circuit type of point-to-point
* Add interface Loopback10 to the isis process
* Configure Lag 2 vlan 10 and lag 2 vlan 30 to only establish LEVEL 2 neighborship
* Configure a ISIS metric of 20 on Lag 2 vlan 10 and lag 2 vlan 30 for both IPV4 and IPV6 address-family

##### ISIS Configuration - test-instance Network instance

* Create ISIS configuration instance named DEFAULT
* Configure the network entity as specified on Table 6
* Configure metric-style wide
* Add subinterfaces Lag 2 vlan 20 and lag 2 vlan 40 to the isis process and circuit type of point-to-point
* Add interface Loopback20 to the isis process
* Configure Lag 2 vlan 20 and lag 2 vlan 40 to only establish LEVEL 2 neighborship
* Configure a ISIS metric of 20 on Lag 2 vlan 20 and lag 2 vlan 40 for both IPV4 and IPV6 address-family

#### Configure BGP

##### BGP Configuration - Default Network instance

* Remove test-export-policy in the export direction on both 198.51.100.18 and
  2001:db8::18 which is in their export direction
* Configure the route-policy default-export-v4 in the export
  direction on 198.51.100.18
* Configure the route-policy default-export-v6 in the export
  direction on 2001:db8::18
* Create the following BGP peers with their peer-group (these BGP peering will
  be established towards DUT 2 on test-originate NI)
      * Peer address: 198.51.100.26; peer-group: downlink
      * Peer address: 2001:db8::26; peer-group: downlink6

##### BGP Configuration - test-instance Network instance

* Undo the connect route to BGP redistribution that was done on test-case RT-1.36.1
* Remove test-export-policy in the export direction on both 198.51.100.22 and
  2001:db8::22 which is in their export direction
* Configure the route-policy non-default-export-v4 in the
  export direction on 198.51.100.22
* Configure the route-policy non-default-export-v6 in the
  export direction on 2001:db8::22
* Create the following BGP peers with their peer-group (these BGP peering will
  be established towards DUT 2 on test-originate NI)
      * Peer address: 198.51.100.30; peer-group: downlink
      * Peer address: 2001:db8::30; peer-group: downlink6

### DUT 2 - Generate Configuration

The following configuration steps are additions to the configuration done on
Test-case RT-1.36.1 above.

#### Interface Configuration

* Create a network instance and name it test-originate
* On lag2 create subinterfaces in vlan 30 and vlan 40 respectively
  * Add these subinterfaces to test-originate network instance
  * Configure IPV4 addresses on the subinterfaces as specified on table 4
  * Configure IPV6 addresses on the subinterfaces as specified on table 4
* Create 2 Loopback interfaces and name them Loopback10 and Loopback20 respectively
* Add these Loopback interfaces (Loopback10 and Loopback20) to test-originate network instance
* Configure IPV4 and IPV6 addresses on these subinterfaces as specified below
    * Loopback10: IPV4: 198.60.1.1/32; IPV6: 2001:db8:60::1/128
    * Loopback20: IPV4: 198.70.1.1/32; IPV6: 2001:db8:70::1/128

#### Create BGP export policy

*  Create a prefix-set named Loopback-prefix-v4 and match the following prefixes
    * ip-prefix: 198.60.1.1/32; masklength-range: exact
    * ip-prefix: 198.70.1.1/32; masklength-range: exact
*  Create a prefix-set named Loopback-prefix-v6 and match the following prefixes
    * ip-prefix: 2001:db8:60::1/128; masklength-range: exact
    * ip-prefix: 2001:db8:70::1/128; masklength-range: exact
*  Create a route-policy named test-export-policy
*  Create a statement named match-export-statement-v4
    *  Match the prefix-set Loopback-prefix-v4
    *  Create an action to set the aigp 200 and accept the route
*  Create a statement named match-export-statement-v6
    *  Match the prefix-set Loopback-prefix-v6
    *  Create an action to set the aigp 200 and accept the route

#### Configure ISIS

##### ISIS Configuration - Default Network instance

* Create ISIS configuration instance named DEFAULT
* Configure the network entity as specified on Table 6
* Configure metric-style wide
* Add subinterfaces Lag 2 vlan 10 to the ISIS process with circuit type of point-to-point
* Configure Lag 2 vlan 10 to only establish LEVEL 2 neighborship
* Configure a ISIS metric of 20 on Lag 2 vlan 10 for both IPV4 and IPV6 address-family

##### ISIS Configuration - test-instance Network instance

* Create ISIS configuration instance named DEFAULT
* Configure the network entity as specified on Table 6
* Configure metric-style wide
* Add subinterfaces Lag 2 vlan 20 to the ISIS process with circuit type of point-to-point
* Configure Lag 2 vlan 20 to only establish LEVEL 2 neighborship
* Configure a ISIS metric of 20 on Lag 2 vlan 20 for both IPV4 and IPV6 address-family

##### ISIS Configuration - test-originate Network instance

* Create ISIS configuration instance named DEFAULT
* Configure the network entity as specified on Table 6
* Configure metric-style wide
* Add subinterfaces Lag 2 vlan 30 and lag 2 vlan 40 to the isis process and circuit type of point-to-point
* Configure Lag 2 vlan 30 and lag 2 vlan 40 to only establish LEVEL 2 neighborship
* Configure a ISIS metric of 20 on lag 2 vlan 30 and vlan 40 for both IPV4 and IPV6 address-family

#### Configure BGP

##### BGP Configuration - test-originate Network instance

* Create a BGP process with ASN 64499 as specified on Table 5
* Create BGP peer-group called default-peer-group and default-peer-group6
* Configure the peer-as,local-as and export-policy on these peer-groups as specified below
      * peer-group: default-peer-group; peer-as: 64497; local-as: 64497; export-policy: test-export-policy
      * peer-group: default-peer-group6; peer-as: 64497; local-as: 64497; export-policy: test-export-policy
* Create BGP peer-group called test-instance-peer-group and test-instance-peer-group6
* Configure the peer-as,local-as and export-policy on these peer-groups as specified below
      * peer-group: test-instance-peer-group; peer-as: 64498; local-as: 64498; export-policy: test-export-policy
      * peer-group: test-instance-peer-group6; peer-as: 64498; local-as: 64498; export-policy: test-export-policy
* Create the following BGP peers with their peer-group
      * Peer address: 198.51.100.25; peer-group: default-peer-group
      * Peer address: 2001:db8::25; peer-group: default-peer-group6
      * Peer address: 198.51.100.29; peer-group: test-instance-peer-group
      * Peer address: 2001:db8::29; peer-group: test-instance-peer-group6
* Redistribute route from protocol DIRECTLY_CONNECTED to protocol BGP

### ATE Configuration

* Stop Protocol on the OTG

### Testing Steps

#### ISIS Testing

##### ISIS testing step on DUT 1

* In the default network instance, collect the level 2 adjacency state for Lag 2 vlan 10 and lag 2 vlan 30
* In test-instance Network instance, collect the level 2 adjacency state for Lag 2 vlan 20 and lag 2 vlan 40

##### ISIS testing step on DUT 2

* In the default network instance, collect the level 2 adjacency state for Lag 2 vlan 10
* In test-instance Network instance, collect the level 2 adjacency state for Lag 2 vlan 20
* In test-originate Network instance, collect the level 2 adjacency state for Lag 2 vlan 30 and Lag 2 vlan 40

#### BGP testing

#### BGP testing Steps on DUT 1

* In the default NI:
    - Fetch the BGP session-state of the peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26
    - Fetch the AIGP propagation status of the peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26
    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.26 and 2001:db8::26
    - Fetch the route-reflector state of the peer-group downlink
    - Fetch the route-reflector state of the peer-group downlink6

* In the test-instance NI:
    - Fetch the BGP session-state of the peers 198.51.100.22, 2001:db8::22, 198.51.100.30, 2001:db8::30
    - Fetch the AIGP propagation status of the peers 198.51.100.22, 2001:db8::22, 198.51.100.30, 2001:db8::30
    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.30, 2001:db8::30
    - Fetch the route-reflector state of the peer-group downlink
    - Fetch the route-reflector state of the peer-group downlink6

#### BGP testing Steps on DUT 2

* In the default NI:
    - Fetch the BGP session-state of the peers 198.51.100.17 and 2001:db8::17
    - Fetch the AIGP propagation status of the peers 198.51.100.17 and 2001:db8::17
    - Fetch the BGP rib attributes of the routes received from peers 198.51.100.17 and 2001:db8::17

* In the test-instance NI:
    - Fetch the BGP session-state of the peers 198.51.100.21 and 2001:db8::21
    - Fetch the AIGP propagation status of the peers 198.51.100.21 and 2001:db8::21
    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.21 and 2001:db8::21

* In the test-originate NI:
    - Fetch the BGP session-state of the peers 198.51.100.25, 2001:db8::25, 198.51.100.29, 2001:db8::29
    - Fetch the AIGP propagation status of the peers 198.51.100.25, 2001:db8::25, 198.51.100.29, 2001:db8::29
    - Fetch the BGP state update sent metric for the peers 198.51.100.25, 2001:db8::25, 198.51.100.29, 2001:db8::29

### Test Pass Criteria

#### Pass criteria on DUT 1

* All isis adjacency state must be up in all NI
* In the default NI, the BGP session-state must be ESTABLISHED for the peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26
* In the default NI, AIGP propagation status must be true for the peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26
* In the default NI, the routes received from the peers 198.51.100.26 and 2001:db8::26 must have a AIGP metric of 200
* In the default NI, the route-reflector-client state must be true for peer-group downlink and downlink6

* In test-instance NI, the BGP session-state must be ESTABLISHED for the peers 198.51.100.22, 2001:db8::22, 198.51.100.30 and 2001:db8::30
* In test-instance NI, AIGP propagation status must be true for the peers 198.51.100.22, 2001:db8::22, 198.51.100.30 and 2001:db8::30
* In test-instance NI, the routes received from the peers 198.51.100.30, 2001:db8::30 must have a AIGP metric of 200
* In test-instance NI, the route-reflector-client state must be true for peer-group downlink and downlink6

#### Pass criteria on DUT 2

* All isis adjacency state must be up in all NI
* In the default NI, the BGP session-state must be ESTABLISHED for the peers 198.51.100.17 and 2001:db8::17
* In the default NI, AIGP propagation status must be true for the peers 198.51.100.17 and 2001:db8::17
* In the default NI, the routes received from the peers 198.51.100.17 and 2001:db8::17 must have a AIGP metric of 220
* In the default NI, the routes received from the peer 198.51.100.17 must have a next-hop attribute of 198.55.1.1
* In the default NI, the routes received from the peer 2001:db8::17 must have a next-hop attribute of 2001:db8:50::1

* In test-instance NI, the BGP session-state must be ESTABLISHED for the peers 198.51.100.21 and 2001:db8::21
* In test-instance NI, AIGP propagation status must be true for the peers 198.51.100.21 and 2001:db8::21
* In test-instance NI, the routes received from the peers 198.51.100.21 and 2001:db8::21 must have a AIGP metric of 220
* In test-instance NI, the routes received from the peer 198.51.100.21 must have a next-hop attribute of 198.55.2.1
* In test-instance NI, the routes received from the peer 2001:db8::21 must have a next-hop attribute of 2001:db8:60::1

* In test-originate NI, the BGP session-state must be ESTABLISHED for the peers 198.51.100.25, 2001:db8::25, 198.51.100.29, 2001:db8::29
* In test-originate NI, AIGP propagation status must be true for the peers 198.51.100.25, 2001:db8::25, 198.51.100.29, 2001:db8::29
* In test-originate NI, BGP state update sent metric for the peers 198.51.100.25, 2001:db8::25, 198.51.100.29, 2001:db8::29 must be non-zero

### Canonical OC

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "default-export-v4"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-next-hop": "198.55.1.1"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        },
        {
          "config": {
            "name": "default-export-v6"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-next-hop": "2001:db8:50::1"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        },
        {
          "config": {
            "name": "non-default-export-v4"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-next-hop": "198.55.2.1"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                }
              }
            ]
          }
        },
        {
          "config": {
            "name": "non-default-export-v6"
          },
          "statements": {
            "statement": [
              {
                "config": {
                  "name": "test-statement"
                },
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-next-hop": "2001:db8:60::1"
                    }
                  },
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
  },
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback10.0",
                "interface": "Loopback10",
                "subinterface": 0
              }
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "DEFAULT"
              },
              "bgp": {
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "downlink",
                      "config": {
                        "peer-group-name": "downlink",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "1.1.1.1"
                        }
                      }
                    },
                    {
                      "peer-group-name": "downlink6",
                      "config": {
                        "peer-group-name": "downlink6",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "1.1.1.1"
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "198.51.100.18",
                        "peer-group": "downlink"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "default-export-v4"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::18",
                        "peer-group": "downlink6"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "default-export-v6"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.26",
                        "peer-group": "downlink"
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::26",
                        "peer-group": "downlink6"
                      }
                    }
                  ]
                }
              }
            },
            {
              "identifier": "DIRECTLY_CONNECTED",
              "name": "DEFAULT",
              "config": {
                "identifier": "DIRECTLY_CONNECTED",
                "name": "DEFAULT"
              }
            },
            {
              "identifier": "ISIS",
              "config": {
                "identifier": "ISIS",
                "name": "DEFAULT"
              },
              "isis": {
                "global": {
                  "config": {
                    "level-capability": "LEVEL_2",
                    "net": [
                      "49.0001.1980.5110.0025.00"
                    ]
                  }
                },
                "interfaces": {
                  "interface": [
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port-channel2.10"
                      },
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": false,
                              "level-number": 1
                            },
                            "afi-safi": {
                              "af": [
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 20
                                  }
                                },
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 20
                                  }
                                }
                              ]
                            }
                          }
                        ]
                      }
                    },
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port-channel2.30"
                      },
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": false,
                              "level-number": 1
                            },
                            "afi-safi": {
                              "af": [
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 20
                                  }
                                },
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 20
                                  }
                                }
                              ]
                            }
                          }
                        ]
                      }
                    },
                    {
                      "config": {
                        "enabled": true,
                        "interface-id": "Loopback10"
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "name": "test-instance"
        },
        "interfaces": {
          "interface": [
            {
              "config": {
                "id": "Loopback20.0",
                "interface": "Loopback20",
                "subinterface": 0
              }
            }
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "DEFAULT"
              },
              "bgp": {
                "peer-groups": {
                  "peer-group": [
                    {
                      "peer-group-name": "downlink",
                      "config": {
                        "peer-group-name": "downlink",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "1.1.1.1"
                        }
                      }
                    },
                    {
                      "peer-group-name": "downlink6",
                      "config": {
                        "peer-group-name": "downlink6",
                        "peer-as": 64497
                      },
                      "route-reflector": {
                        "config": {
                          "route-reflector-client": true,
                          "route-reflector-cluster-id": "1.1.1.1"
                        }
                      }
                    }
                  ]
                },
                "neighbors": {
                  "neighbor": [
                    {
                      "config": {
                        "neighbor-address": "198.51.100.22",
                        "peer-group": "downlink"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "non-default-export-v4"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::22",
                        "peer-group": "downlink6"
                      },
                      "apply-policy": {
                        "config": {
                          "export-policy": [
                            "non-default-export-v6"
                          ]
                        }
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "198.51.100.30",
                        "peer-group": "downlink"
                      }
                    },
                    {
                      "config": {
                        "neighbor-address": "2001:db8::30",
                        "peer-group": "downlink6"
                      }
                    }
                  ]
                }
              }
            },
            {
              "identifier": "DIRECTLY_CONNECTED",
              "name": "DEFAULT",
              "config": {
                "identifier": "DIRECTLY_CONNECTED",
                "name": "DEFAULT"
              }
            },
            {
              "identifier": "ISIS",
              "config": {
                "identifier": "ISIS",
                "name": "DEFAULT"
              },
              "isis": {
                "global": {
                  "config": {
                    "level-capability": "LEVEL_2",
                    "net": [
                      "49.0001.1980.5110.0029.00"
                    ]
                  }
                },
                "interfaces": {
                  "interface": [
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port-channel2.20"
                      },
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": false,
                              "level-number": 1
                            },
                            "afi-safi": {
                              "af": [
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 20
                                  }
                                },
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 20
                                  }
                                }
                              ]
                            }
                          }
                        ]
                      }
                    },
                    {
                      "config": {
                        "circuit-type": "POINT_TO_POINT",
                        "enabled": true,
                        "interface-id": "port-channel2.40"
                      },
                      "levels": {
                        "level": [
                          {
                            "config": {
                              "enabled": false,
                              "level-number": 1
                            },
                            "afi-safi": {
                              "af": [
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV4",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 20
                                  }
                                },
                                {
                                  "config": {
                                    "afi-name": "openconfig-isis-types:IPV6",
                                    "safi-name": "openconfig-isis-types:UNICAST",
                                    "metric": 20
                                  }
                                }
                              ]
                            }
                          }
                        ]
                      }
                    },
                    {
                      "config": {
                        "enabled": true,
                        "interface-id": "Loopback20"
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
  },
  "interfaces": {
    "interface": [
      {
        "name": "Loopback10",
        "config": {
          "name": "Loopback10",
          "type": "iana-if-type:softwareLoopback",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.55.1.1",
                        "prefix-length": 32
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:50::1",
                        "prefix-length": 128
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "name": "Loopback20",
        "config": {
          "name": "Loopback20",
          "type": "iana-if-type:softwareLoopback",
          "enabled": true
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "198.55.2.1",
                        "prefix-length": 32
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:60::1",
                        "prefix-length": 128
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

## RT-1.36.4 : Validate the Plus 1 incremental feature of AIGP when the IGP metric to original destination is zero

The following configuration steps are additions to the configuration done on
Test-case RT-1.36.3 above.

#### Effective Emulated Test Topology

```text
+-------------------------+ LAG2(vlan 30 and 40) [iBGP]    +-------------+          LAG2 [iBGP]          +---------+
|  DUT2(test-originate NI)|===============================|  DUT 1(RR)   |===============================|  DUT 2  |
+-------------------------+                                +-------------+             ISIS              +---------+

```

### DUT 1 - Generate Configuration

#### ISIS Configuration - Default Network instance

* Remove Lag 2 vlan 30 from the ISIS configuration instance named DEFAULT

#### ISIS Configuration - test-instance Network instance

* Remove Lag 2 vlan 40 from the ISIS configuration instance named DEFAULT

### DUT 2 - Generate Configuration

##### ISIS Configuration - test-originate Network instance

* Remove Lag 2 vlan 30 and vlan 40 from the ISIS configuration instance named DEFAULT
* Remove the ISIS configuration instance

### Testing Steps

###### ISIS testing step on DUT 1

* In the DEFAULT network instance, collect the ISIS level 2 adjacency state for Lag 2 vlan 10
* In test-instance Network instance, collect the ISIS level 2 adjacency state for Lag 2 vlan 20

###### ISIS testing step on DUT 2

* In the DEFAULT network instance, collect the ISIS level 2 adjacency state for Lag 2 vlan 10
* In test-instance Network instance, collect the ISIS level 2 adjacency state for Lag 2 vlan 20

##### BGP testing Steps on DUT 1

* In default NI, Fetch the BGP rib attributes of the routes received from the peers 198.51.100.26 and 2001:db8::26
* In test-instance NI, Fetch the BGP rib attributes of the routes received from the peers 198.51.100.30 and 2001:db8::30

##### BGP testing Steps on DUT 2

* In default NI,Fetch the BGP rib attributes of the routes received from peers 198.51.100.17 and 2001:db8::17
* In test-instance NI,Fetch the BGP rib attributes of the routes received from the peers 198.51.100.21 and 2001:db8::21

### Test Pass Criteria

##### Pass criteria on DUT 1

* All ISIS adjacency states must be up in DEFAULT NI  
* All ISIS adjacency states must be up in test-instance NI
* In default NI,BGP routes received from 198.51.100.26 and 2001:db8::26 must have a AIGP metric of 200
* In test-instance NI,BGP routes received from 198.51.100.30 and 2001:db8::30 must have a AIGP metric of 200

##### Pass criteria on DUT 2

* All ISIS adjacency states must be up in DEFAULT NI  
* All ISIS adjacency states must be up in test-instance NI
* In default NI,BGP routes received from 198.51.100.17 and 2001:db8::17 must have a AIGP metric of 201
* In test-instance NI,BGP routes received from 198.51.100.21 and 2001:db8::21 must have a AIGP metric of 201

## RT-1.36.5 : Validate AIGP attribute is dropped when AIGP propagation is disabled

The following configuration steps are additions to the configuration done on
Test-case RT-1.36.4 above.

### DUT 1 - Generate Configuration

* In Default NI, Disable AIGP propagation on the BGP peers 198.51.100.18 and 2001:db8::18
* In test-instance NI, Disable AIGP propagation on the BGP peers 198.51.100.22 and 2001:db8::22

### Testing Steps

##### BGP testing Steps on DUT 1

* In the default NI:
    - Fetch the BGP session-state of the peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26
    - Fetch the AIGP propagation status of the peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26
    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.26 and 2001:db8::26

* In the test-instance NI:
    - Fetch the BGP session-state of the peers 198.51.100.22, 2001:db8::22, 198.51.100.30 and 2001:db8::30
    - Fetch the AIGP propagation status of the peers 198.51.100.22, 2001:db8::22, 198.51.100.30 and 2001:db8::30
    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.30 and 2001:db8::30

##### BGP testing Steps on DUT 2

* In the default NI:
    - Fetch the BGP rib attributes of the routes received from peers 198.51.100.17 and 2001:db8::17

* In the test-instance NI:

    - Fetch the BGP rib attributes of the routes received from the peers 198.51.100.21 and 2001:db8::21

### Test Pass Criteria

##### Pass criteria on DUT 1

* In the default NI:
  - The BGP session-state on peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26 must be in ESTABLISHED state
  - AIGP propagation status must be False on the peers 198.51.100.18 and 2001:db8::18
  - AIGP propagation must be True for the peers 198.51.100.26 and 2001:db8::26
  - Routes received from peers 198.51.100.26 and 2001:db8::26 must have a AIGP metric of 200

* In test-instance NI:
  - The BGP session-state on peers 198.51.100.22, 2001:db8::22, 198.51.100.30 and 2001:db8::30 must be in ESTABLISHED state
  - AIGP propagation status must be False on the peers 198.51.100.22 and 2001:db8::22
  - AIGP propagation must be True for the peers 198.51.100.30 and 2001:db8::30
  - Route received from peers 198.51.100.30 and 2001:db8::30 must have a AIGP metric of 200

##### Pass criteria on DUT 2

* In the default NI, Routes received from peers 198.51.100.17 and 2001:db8::17 must not have AIGP attribute
* In the test-instance NI, Routes received from peers 198.51.100.21 and 2001:db8::21 must not have AIGP attribute

## RT-1.36.6 : AIGP propagation in BGP peer-group

The following configuration steps are additions to the configuration done on
Test-case RT-1.36.5 above

### DUT 1 - Generate Configuration

* In the default NI, Enable AIGP propagation for IPV4 address-family in the peer-group downlink
* In the default NI, Enable AIGP propagation for IPV6 address-family in the peer-group downlink6
* In test-instance NI, Enable AIGP propagation for IPV4 address-family in the peer-group downlink
* In test-instance NI, Enable AIGP propagation for IPV6 address-family in the peer-group downlink6

### Testing Steps

#### BGP testing Steps on DUT 1

* In the default NI:
    * Fetch the BGP session-state of the peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26
    * Fetch the AIGP propagation status of the peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26
    * Fetch the BGP rib attributes of the routes received from the peers 198.51.100.26 and 2001:db8::26
* In the test-instance NI:
    * Fetch the BGP session-state of the peers 198.51.100.22, 2001:db8::22, 198.51.100.30 and 2001:db8::30
    * Fetch the AIGP propagation status of the peers 198.51.100.22, 2001:db8::22, 198.51.100.30 and 2001:db8::30
    * Fetch the BGP rib attributes of the routes received from the peers 198.51.100.30 and 2001:db8::30

#### BGP testing Steps on DUT 2

* In the default NI:
    * Fetch the BGP session-state of the peers 198.51.100.17 and 2001:db8::17
    * Fetch the AIGP propagation status of the peers 198.51.100.17 and 2001:db8::17
    * Fetch the BGP rib attributes of the routes received from peers 198.51.100.17 and 2001:db8::17
* In the test-originate NI:
    * Fetch the BGP session-state of the peers 198.51.100.21 and 2001:db8::21
    * Fetch the AIGP propagation status of the peers 198.51.100.21 and 2001:db8::21
    * Fetch the BGP rib attributes of the routes received from peers the peers 198.51.100.21 and 2001:db8::21

### Test Pass Criteria

##### Pass criteria on DUT 1

* In the default NI,the BGP session-state of peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26 must be ESTABLISHED
* In the default NI,the AIGP propagation status of the peers 198.51.100.18, 2001:db8::18, 198.51.100.26 and 2001:db8::26 must be true
* In the default NI, the routes received from the peers 198.51.100.26 and 2001:db8::26 must have a AIGP metric of 200
* In test-instance NI,the BGP session-state of peers 198.51.100.22, 2001:db8::22, 198.51.100.30 and 2001:db8::30 must be ESTABLISHED
* In test-instance NI,the AIGP propagation status of the peers 198.51.100.22, 2001:db8::22, 198.51.100.30 and 2001:db8::30 must be true
* In test-instance NI,the routes received from the peers 198.51.100.30 and 2001:db8::30 must have a AIGP metric of 200

##### Pass criteria on DUT 2

* In the default NI,the BGP session-state of peers 198.51.100.17 and 2001:db8::17 must be ESTABLISHED
* In the default NI,the AIGP propagation status of the peers 198.51.100.17 and 2001:db8::17 must be true
* In the default NI, the routes received from the peers 198.51.100.17 and 2001:db8::17 must have a AIGP metric of 201
* In test-instance NI,the BGP session-state of peers 198.51.100.21 and 2001:db8::21 must be ESTABLISHED
* In test-instance NI,the AIGP propagation status of the peers 198.51.100.21 and 2001:db8::21 must be true
* In test-instance NI, the routes received from the peers 198.51.100.21 and 2001:db8::21 must have a AIGP metric of 201

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Configuration coverage
  /network-instances/network-instance/tables/table/config/protocol:
  /network-instances/network-instance/tables/table/config/address-family:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/repeat-n:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/set-as-path-prepend/config/asn:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-next-hop:
  /network-instances/network-instance/protocols/protocol/config/identifier:
  /network-instances/network-instance/protocols/protocol/config/name:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/route-reflector/config/route-reflector-cluster-id:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/route-reflector/config/route-reflector-client:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-group:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/isis/global/config/net:
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/interface-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/metric:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/afi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/safi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/level-number:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/enabled:
  /network-instances/network-instance/table-connections/table-connection/config/address-family:
  /network-instances/network-instance/table-connections/table-connection/config/src-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/dst-protocol:
  # TODO: Create path for enabling AIGP on neighbor and peer-group basis an also bgp action for AIGP. see [PR/1451](https://github.com/openconfig/public/pull/1451)
  # TODO: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enable-aigp
  # TODO: /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-aigp
  # TODO: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enable-aigp

  # Telemetry coverage
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/best-path:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/neighbors/neighbor/adj-rib-in-post/routes/route/state/best-path:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/sent/UPDATE:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/aigp:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/oper-status:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

*   FFF
