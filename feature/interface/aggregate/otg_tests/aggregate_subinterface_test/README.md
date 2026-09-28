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

##### OTG Traffic Profile (Table 3)
Flow name | Source | Destination | flow size |  flow rate(percent)
:------| :----------| :--------| :---------| :-------------:
Flow 1  | 198.51.100.2 | 198.51.100.10 | 9210 |  5
Flow 2  | 198.51.100.6 | 198.51.100.14 | 9210 |  5
Flow 3  | 2001:db8::2 | 12001:db8::2   | 9210   |  5
Flow 4  | 2001:db8::6 | 12001:db8::14  | 9210  |  5

### RT-5.14.1: Aggregate interface flap using min-link

#### Configure the DUT

  1.  Create the aggregate interfaces lag1 and lag2 with LACP
  2.  Configure the lags as the LACP active side
  3.  Configure a MTU of 9216 on port 1, port 2, port 3 and port 4
  4.  Add port 1 and port 2 to lag 1
  5.  Add port 3 and port 4 to lag 2
  6.  Configure a MTU of 9216 on the aggregate interfaces lag1 and lag2
  7.  Configure a Min-link of 2 on both lag 1 and lag 2
  7.  Create subinterfaces in vlan 10 and vlan 20 on both lag 1 and lag 2
  8.  Add the subinterfaces to DEFAULT NI
  9.  Configure IPV4 addresses on the subinterfaces as specified for DUT on table 1
  10. Configure IPV6 addresses on the subinterfaces as specified for DUT on table 2

#### Configure the ATE

  1. Create two aggregate interfaces with LACP
  2. Configure a MTU of 2 on the aggregate interface
  3. Add port 1 and port 2 to aggregate interface 1
  4. Add port 3 and port 4 to aggregate interface 2
  5. Create two emulated routers with ethernet interfaces on both routers
  6. Configure MTU of 9216 on the emulated ethernet interfaces
  7. Connect the ethernet interfaces to the aggregate interfaces
  8. Create vlan 10 and vlan 20 on both the aggregate interfaces a
  9. Configure IPV4 addresses on the subinterfaces as specified for ATE on table 1
  10. Configure IPV6 addresses on the subinterfaces as specified for ATE on table 2
  11. Create IP flows from aggregate interface with according to the information on Table 3
  12. Push Config to the OTG.
  13. Start Protocols on the OTG

#### Testing Steps

  * Wait for the aggregate interface to be UP on DUT and ATE.
  * Start a loop of 10
  * Validate that the aggregate Interfaces (both lag 1 and lag 2) are UP on the DUT and the ATE
  * Disable port 1 and port 3 on DUT
  * Wait for aggregate interfaces to be Down on DUT and ATE
  * Enable port 1 and port 3 on the DUT
  * Wait for the aggregate interface to be UP on DUT and ATE
  * Start Traffic flows
  * Wait for another 60 seconds while the traffic flows
  * Stop Traffic flow
  * Collect packet loss information from the flows
  * Collect the transmit packet information from the flow
  * on the DUT,Validate that the MTU of the lags from the interface state is 9216
  * on the DUT,Validate that the MTU of the of port1, port2, port3 and port4 from the interface state is 9216
  * Collect both the in-packet and out-packet counter information on the DUT for both lag 1 and lag 2
  * Collect both the in-discards and out-discards counter information on DUT for both lag 1 and lag 2
  * For all the ports on the DUT,collect the LACP information information of the port as well as its remote 
    port on the ATE.For example here, for port 1 on the dut,belong to lag 1, the remote port on the DUT will be port 1 as
    shown in the test topology on
  * Validate that the partner-ID on from the DUT side port is the same as the SystemID of the port from the ATE and vice-versal.
  * Validate that each interface state has both collecting and distributing state as true

### Test Pass Criteria

  * The aggregate interface must go to Down state when port 1 is disabled and come back to UP when port 1 is re-enabled.
  * The packet loss metric from the flows must be less than 1 percentage
  * The transmit packet in-packet and the out-packet counter information from the DUT for both lags must be non zero
  * The in-discards and out-discards counter information from the DUT must be less than 1 percentage of the transmit packet information
    from the flows.
  * The partner ID from the Dut side must be the same value as the system ID from the ATE side and vice-versal.
  * The collecting and Distributing state for all the member links must be true.
  * MTU of interfaces and lags from the interface state must be 9216

### RT-5.14.2: Aggregate sub-interface in default Network Instance (NI)

#### Configure the DUT

  1.  Create the aggregate interfaces lag1 and lag2 with LACP
  2.  Configure the lags as the LACP active side
  3.  Configure a min-link of 1 on tha lags
  4.  Configure a MTU of 9216 on port 1, port 2, port 3 and port 4
  5.  Add port 1 and port 2 to lag 1
  6.  Add port 3 and port 4 to lag 2
  7.  Configure a MTU of 9216 on the aggregate interfaces lag1 and lag2
  8.  Create subinterfaces in vlan 10 and vlan 20 on both lag 1 and lag 2
  8.  Add the subinterfaces to DEFAULT NI
  10. Configure IPV4 addresses on the subinterfaces as specified for DUT on table 1
  11. Configure IPV6 addresses on the subinterfaces as specified for DUT on table 2

#### Configure the ATE

  1. Create two aggregate interfaces with LACP
  2. Add port 1 and port 2 to aggregate interface 1
  3. Add port 3 and port 4 to aggregate interface 2
  4. Create two emulated routers with ethernet interfaces on both routers
  5. Configure MTU of 9216 on the emulated ethernet interfaces
  6. Connect the ethernet interfaces to the aggregate interfaces
  7. Create vlan 10 and vlan 20 on both the aggregate interfaces a
  8. Configure IPV4 addresses on the subinterfaces as specified for ATE on table 1
  9. Configure IPV6 addresses on the subinterfaces as specified for ATE on table 2
  10. Create IP flows from aggregate interface with according to the information on Table 3
  11. Push Config to the OTG.
  12. Start Protocols on the OTG

#### Testing Steps

  * Wait for LAG interfaces to be UP on DUT and ATE.
  * Validate that all the 4 ports are Operationally UP on the OTG
  * Validate that the Aggregate Interfaces are UP on the OTG
  * Start Traffic flows
  * Wait for another 60 seconds while the traffic flows
  * Disable port 1 and port 3 on DUT
  * Wait for another 60 seconds while the traffic flows
  * Stop Traffic Flow
  * Validate that the Aggregate Interfaces are UP on the OTG
  * on the DUT,Validate that the MTU of the lags from the interface state is 9216
  * on the DUT,Validate that the MTU of the of port1, port2, port3 and port4 from the interface state is 9216
  * Collect packet loss information from the flows
  * Collect the transmit packet information from the flow
  * Collect both the in-packet and out-packet counter information on the DUT for both lag 1 and lag 2
  * Collect both the in-discards and out-discards counter information on DUT for both lag 1 and lag 2
  * For all the ports on the DUT,collect the LACP information information of the port as well as its remote 
    port on the ATE.For example here, for port 1 on the dut,belong to lag 1, the remote port on the DUT will be port 1 as
    shown in the test topology on
  * Validate that the partner-ID on from the DUT side port is the same as the SystemID of the port from the ATE and vice-versal.
  * Validate that each interface state has both collecting and distributing state as true

### Test Pass Criteria

  * The packet loss metric from the flows must be less than 1 percentage
  * The lags must remain UP even when member is disabled
  * The transmit packet in-packet and the out-packet counter information from the DUT for both lags must be non zero
  * The in-discards and out-discards counter information from the DUT must be less than 1 percentage of the transmit packet information
    from the flows.
  * The partner ID from the Dut side must be the same value as the system ID from the ATE side and vice-versal.
  * The collecting and Distributing state for all the member links must be true.
  * MTU of interfaces and lags from the interface state must be 9216

### RT-5.14.3: Aggregate sub-interface in non-default Network Instance (NI)

#### Configure the DUT

  1.  Create the aggregate interfaces lag1 and lag2 with LACP
  2.  Configure the lags as the LACP active side
  3.  Configure a min-link of 1 on tha lags
  4.  Configure a MTU of 9216 on port 1, port 2, port 3 and port 4
  5.  Add port 1 and port 2 to lag 1
  6.  Add port 3 and port 4 to lag 2
  7.  Configure a MTU of 9216 on the aggregate interfaces lag1 and lag2
  8.  Create subinterfaces in vlan 10 and vlan 20 on both lag 1 and lag 2
  8.  Add the subinterfaces to test-instance NI
  10. Configure IPV4 addresses on the subinterfaces as specified for DUT on table 1
  11. Configure IPV6 addresses on the subinterfaces as specified for DUT on table 2

#### Configure the ATE

  1. Create two aggregate interfaces with LACP
  2. Add port 1 and port 2 to aggregate interface 1
  3. Add port 3 and port 4 to aggregate interface 2
  4. Create two emulated routers with ethernet interfaces on both routers
  5. Configure MTU of 9216 on the emulated ethernet interfaces
  6. Connect the ethernet interfaces to the aggregate interfaces
  7. Create vlan 10 and vlan 20 on both the aggregate interfaces a
  8. Configure IPV4 addresses on the subinterfaces as specified for ATE on table 1
  9. Configure IPV6 addresses on the subinterfaces as specified for ATE on table 2
  10. Create IP flows from aggregate interface with according to the information on Table 3
  11. Push Config to the OTG.
  12. Start Protocols on the OTG

#### Testing Steps

  * Wait for LAG interfaces to be UP on DUT and ATE.
  * Validate that all the 4 ports are Operationally UP on the OTG
  * Validate that the Aggregate Interfaces are UP on the OTG
  * Start Traffic flows
  * Wait for another 60 seconds while the traffic flows
  * Disable port 1 and port 3 on DUT
  * Wait for another 60 seconds while the traffic flows
  * Stop Traffic Flow
  * Validate that the Aggregate Interfaces are UP on the OTG
  * on the DUT,Validate that the MTU of the lags from the interface state is 9216
  * on the DUT,Validate that the MTU of the of port1, port2, port3 and port4 from the interface state is 9216
  * Collect packet loss information from the flows
  * Collect the transmit packet information from the flow
  * Collect both the in-packet and out-packet counter information on the DUT for both lag 1 and lag 2
  * Collect both the in-discards and out-discards counter information on DUT for both lag 1 and lag 2
  * For all the ports on the DUT,collect the LACP information information of the port as well as its remote 
    port on the ATE.For example here, for port 1 on the dut,belong to lag 1, the remote port on the DUT will be port 1 as
    shown in the test topology on
  * Validate that the partner-ID on from the DUT side port is the same as the SystemID of the port from the ATE and vice-versal.
  * Validate that each interface state has both collecting and distributing state as true

### Test Pass Criteria

  * The packet loss metric from the flows must be less than 1 percentage
  * The lags must remain UP even when member is disabled
  * The transmit packet in-packet and the out-packet counter information from the DUT for both lags must be non zero
  * The in-discards and out-discards counter information from the DUT must be less than 1 percentage of the transmit packet information
    from the flows.
  * The partner ID from the Dut side must be the same value as the system ID from the ATE side and vice-versal.
  * The collecting and Distributing state for all the member links must be true.
  * MTU of interfaces and lags from the interface state must be 9216

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
          "name" : "port-channel1",
          "mtu": 9216
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
          "name" : "port-channel2",
          "mtu": 9216
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
        "config": {
          "name" : "Ethernet1/1",
          "mtu": 9216
        },
        "ethernet" : {
          "config": {
              "aggregate-id": "port-channel1"
            }
        }
      },
      {
        "name" : "Ethernet1/2",
        "config": {
          "name" : "Ethernet1/2",
          "mtu": 9216
        },
        "ethernet" : {
          "config": {
              "aggregate-id": "port-channel1"
            }
        }
      },
      {
        "name" : "Ethernet2/1",
        "config": {
          "name" : "Ethernet2/1",
          "mtu": 9216
        },
        "ethernet" : {
          "config": {
              "aggregate-id": "port-channel2"
            }
        }
      },
      {
        "name" : "Ethernet2/2",
        "config": {
          "name" : "Ethernet2/2",
          "mtu": 9216
        },
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
  /interfaces/interface/config/mtu:
  /interfaces/interface/config/enabled:

  # Telemetry Parameter Coverage
  /lacp/interfaces/interface/members/member/state/collecting:
  /lacp/interfaces/interface/members/member/state/distributing:
  /lacp/interfaces/interface/members/member/state/partner-id:
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/enabled:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform
FFF
