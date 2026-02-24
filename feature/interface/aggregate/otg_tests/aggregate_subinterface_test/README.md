# RT-5.14: Aggregate Subinterface in Default and Non-default Network Instance

## Summary

This test validates the operation of aggregate subinterface ensuring the
subinterfaces come up and can take traffic successfully.

## Testbed type

  [TESTBED_DUT_ATE_4LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

#### Test Topology

```text
+-----------------------------------------------------------------------+
|                              Test Setup                               |
+-----------------------------------------------------------------------+
|  ___________ DUT ___________             ___________ ATE ___________  |
| |                         |             |                         | |
| |  Port 1 --\             |             |             /-- Port 1  | |
| |          +-- LAG 1 ---- | <---------> | ---- LAG 1 --+          | |
| |  Port 2 --/             |             |             \-- Port 2  | |
| |                         |             |                         | |
| | ----------------------- |             | ----------------------- | |
| |                         |             |                         | |
| |  Port 3 --\             |             |             /-- Port 3  | |
| |          +-- LAG 2 ---- | <---------> | ---- LAG 2 --+          | |
| |  Port 4 --/             |             |             \-- Port 4  | |
| |_________________________|             |_________________________| |
|                                                                       |
+-----------------------------------------------------------------------+

```

#### DUT and OTG Configuration Parameters

##### IPv4 Addresses (Table 1)
Device | Lag 1 vlan 10 | lag1 vlan 20 | lag2 vlan 10 | lag2 vlan20
:------| :----------| :-------- | :---------| :-------:
DUT    | 198.51.100.1/30 | 198.51.100.5/30 | 198.51.100.9/30 | 198.51.100.13/30
ATE    | 198.51.100.2/30 | 198.51.100.6/30 | 198.51.100.10/30 | 198.51.100.14/30

##### IPv6 Addresses (Table 2)
Device | Lag 1 vlan 10 | lag1 vlan 20 | lag2 vlan 10 | lag2 vlan20
:------| :----------| :-------- | :---------| :-------:
DUT    | 2001:db8::1/126 | 2001:db8::5/126 | 2001:db8::9/126 | 2001:db8::13/126
ATE    | 2001:db8::2/126 | 2001:db8::6/126 | 2001:db8::10/126 | 2001:db8::14/126


### RT-5.14.1:  Aggregate sub-interface in default Network Instance (NI)

#### Configure the DUT

  1.  Create the aggregate interfaces lag1 and lag2 with LACP
  2.  Configure the lags as the LACP active side
  3.  Add port 1 and port 2 to lag 1
  4.  Add port 3 and port 4 to lag 2
  5.  Create subinterfaces in vlan 10 and vlan 20 on both lag 1 and lag 2
  6.  Add the subinterfaces to DEFAULT NI
  7.  Configure IPV4 addresses on the subinterfaces as specified for DUT on table 1
  8.  Configure IPV6 addresses on the subinterfaces as specified for DUT on table 2

#### Configure the ATE

  1. Create two aggregate interfaces with LACP
  2. Add port 1 and port 2 to aggregate interface 1
  3. Add port 3 and port 4 to aggregate interface 2
  4. Create two emulated routers with ethernet interfaces on both routers
  5. Connect the ethernet interfaces to the aggregate interfaces
  6. Create vlan 10 and vlan 20 on both the aggregate interfaces a
  7. Configure IPV4 addresses on the subinterfaces as specified for ATE on table 1
  8. Configure IPV6 addresses on the subinterfaces as specified for ATE on table 2
  9. Create IP flows from aggregate interface with the following mapping
      *  Flow 1: 198.51.100.2 - 198.51.100.10
      *  Flow 2: 198.51.100.6 - 198.51.100.14
      *  Flow 3: 2001:db8::2 - 2001:db8::10
      *  Flow 4: 2001:db8::6 - 2001:db8::14
  10. Push Config to the OTG.
  11. Start Protocols on the OTG

####  Testing Steps

  * Wait for LAG interfaces to be UP on DUT and ATE.
  * Validate that all the 4 ports are Operationally UP on the OTG
  * Validate that the Aggregate Interfaces are UP on the OTG
  * Start Traffic flows
  * Wait for another 60 seconds
  * Stop Traffic Flow
  * Collect the receive and the transmit packet information from the
    flows and validate it against the test pass criteria

### Test Pass Criteria

  * Packet drop from the flows must be less than 2 percent.

### RT-5.14.2: Aggregate sub-interface in non-default Network Instance (NI)

#### Configure the DUT

  1.  Create the aggregate interfaces lag1 and lag2 with LACP
  2.  Configure the lags as the LACP active side
  3.  Add port 1 and port 2 to lag 1
  4.  Add port 3 and port 4 to lag 2
  5.  Create subinterfaces in vlan 10 and vlan 20 on both lag 1 and lag 2
  6.  Add the subinterfaces to test-instance NI
  7.  Configure IPV4 addresses on the subinterfaces as specified for DUT on table 1
  8.  Configure IPV6 addresses on the subinterfaces as specified for DUT on table 2

#### Configure the ATE

  1. Create two aggregate interfaces with LACP
  2. Add port 1 and port 2 to aggregate interface 1
  3. Add port 3 and port 4 to aggregate interface 2
  4. Create two emulated routers with ethernet interfaces on both routers
  5. Connect the ethernet interfaces to the aggregate interfaces
  6. Create vlan 10 and vlan 20 on both the aggregate interfaces a
  7. Configure IPV4 addresses on the subinterfaces as specified for ATE on table 1
  8. Configure IPV6 addresses on the subinterfaces as specified for ATE on table 2
  9. Create IP flows from aggregate interface with the following mapping
      *  Flow 1: 198.51.100.2 - 198.51.100.10
      *  Flow 2: 198.51.100.6 - 198.51.100.14
      *  Flow 3: 2001:db8::2 - 2001:db8::10
      *  Flow 4: 2001:db8::6 - 2001:db8::14
  10. Push Config to the OTG.
  11. Start Protocols on the OTG

####  Testing Steps

  * Wait for LAG interfaces to be UP on DUT and ATE.
  * Validate that all the 4 ports are Operationally UP on the OTG
  * Validate that the Aggregate Interfaces are UP on the OTG
  * Start Traffic flows
  * Wait for another 60 seconds
  * Stop Traffic Flow
  * Collect the receive and the transmit packet information from the
    flows and validate it against the test pass criteria

### Test Pass Criteria

  * Packet drop from the flows must be less than 2 percent.

### Canonical OC

```json
{
  "network-instances" : {
    "network-instance" : [
      {
        "config" : {
          "name" : "test-instance"
        },
        "interfaces" :{
          "interface" : [
            {
              "id" : "port-channel1.10",
              "config" : {
                "id" : "port-channel1.10",
                "interface" : "port-channel1",
                "subinterface" : 10
              }
            },
            {
              "id" : "port-channel1.20",
              "config" : {
                "id" : "port-channel1.20",
                "interface" : "port-channel1",
                "subinterface" : 20
              }
            },
            {
              "id" : "port-channel2.10",
              "config" : {
                "id" : "port-channel2.10",
                "interface" : "port-channel2",
                "subinterface" : 10
              }
            },
            {
              "id" : "port-channel2.20",
              "config" : {
                "id" : "port-channel2.20",
                "interface" : "port-channel2",
                "subinterface" : 20
              }
            }
          ]
        }
      }
    ]
  },
  "interfaces" : {
    "interface" : [
      {
        "name" : "port-channel1",
        "config": {
          "name" : "port-channel1"
        },
        "aggregation" :{
          "config": {
             "lag-type": "LACP"
          }
        },
        "subinterfaces": {
          "subinterface" : [
          {
            "index" : 10,
            "vlan" : {
              "config" : {
                "vlan-id" : 10
              }
            },
            "ipv6" : {
              "addresses" : {
                "address" : [
                    {
                    "config" : {
                      "ip" : "2001:db8::1",
                      "prefix-length" : 126
                    }
                  }
                ]
              }
            },
            "ipv4" : {
              "addresses" : {
                "address" : [
                    {
                    "config" : {
                      "ip" : "198.51.100.1",
                      "prefix-length" : 30
                    }
                  }
                ]
              }
            }
          },
          {
            "index" : 20,
            "vlan" : {
              "config" : {
                "vlan-id" : 20
              }
            },
            "ipv6" : {
              "addresses" : {
                "address" : [
                    {
                    "config" : {
                      "ip" : "2001:db8::5",
                      "prefix-length" : 126
                    }
                  }
                ]
              }
            },
            "ipv4" : {
              "addresses" : {
                "address" : [
                    {
                    "config" : {
                      "ip" : "198.51.100.5",
                      "prefix-length" : 30
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
        "name" : "port-channel2",
        "config": {
          "name" : "port-channel2"
        },
        "aggregation" :{
          "config": {
             "lag-type": "LACP"
          }
        },
        "subinterfaces": {
          "subinterface" : [
          {
            "index" : 10,
            "vlan" : {
              "config" : {
                "vlan-id" : 10
              }
            },
            "ipv6" : {
              "addresses" : {
                "address" : [
                    {
                    "config" : {
                      "ip" : "2001:db8::9",
                      "prefix-length" : 126
                    }
                  }
                ]
              }
            },
            "ipv4" : {
              "addresses" : {
                "address" : [
                    {
                    "config" : {
                      "ip" : "198.51.100.9",
                      "prefix-length" : 30
                    }
                  }
                ]
              }
            }
          },
          {
            "index" : 20,
            "vlan" : {
              "config" : {
                "vlan-id" : 20
              }
            },
            "ipv6" : {
              "addresses" : {
                "address" : [
                    {
                    "config" : {
                      "ip" : "2001:db8::13",
                      "prefix-length" : 126
                    }
                  }
                ]
              }
            },
            "ipv4" : {
              "addresses" : {
                "address" : [
                    {
                    "config" : {
                      "ip" : "198.51.100.13",
                      "prefix-length" : 30
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
        "name" : "Ethernet1/1",
        "ethernet" : {
          "config": {
              "aggregate-id": "port-channel1"
            }
        }
      },
      {
        "name" : "Ethernet1/2",
        "ethernet" : {
          "config": {
              "aggregate-id": "port-channel1"
            }
        }
      },
      {
        "name" : "Ethernet2/1",
        "ethernet" : {
          "config": {
              "aggregate-id": "port-channel2"
            }
        }
      },
      {
        "name" : "Ethernet2/2",
        "ethernet" : {
          "config": {
              "aggregate-id": "port-channel2"
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
  # Configuration coverage
  /network-instances/network-instance/interfaces/interface/id:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/interfaces/interface/config/interface:
  /network-instances/network-instance/interfaces/interface/config/subinterface:
  /interfaces/interface/subinterfaces/subinterface/vlan/config/vlan-id:
  /interfaces/interface/subinterfaces/subinterface/index:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/name:
  /interfaces/interface/config/name:
  /interfaces/interface/aggregation/config/lag-type:

  # Telemetry Parameter Coverage
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
FFF
