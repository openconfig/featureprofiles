# INT-1.1: Interface Performance
## Summary

Test performance of interfaces using a "snake style topology" and IPv4 and IPv6 packet flows.

## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)
* ATE port1 - Used for traffic Source
* ATE port2 - Used for traffic Destination

## Topology

VLAN1            |  VLAN2    | VLAN3     | VLAN4     | VLAN5      | VLAN6     |
:----------------| :---------| :-------- | :---------| :----------| :---------|
ATE1, DUT1, DUT2 | DUT3-DUT4 | DUT5-DUT6 | DUT7-DUT8 | DUT9-DUT10 | DUT11-DUT12

VLAN12       |  VLAN13     | VLAN14      | VLAN15    | VLAN16       | VLAN17         |
:------------| :-----------| :---------- | :---------| :------------| :--------------|
DUT23, DUT24 | DUT25-DUT26 | DUT27-DUT28 | DUT29-DUT30 | DUT31-DUT32 | DUT33-DUT34-ATE2

ATE-PORT1, DUT-PORT1 & DUT-PORT2 ---> VLAN1\
DUT-PORT3 & DUT-PORT4 ---> VLAN2\
DUT-PORT5 & DUT-PORT6 ---> VLAN3\
DUT-PORT7 & DUT-PORT8 ---> VLAN4\
DUT-PORT9 & DUT-PORT10 ---> VLAN5\
DUT-PORT11 & DUT-PORT12 ---> VLAN6\
DUT-PORT13 & DUT-PORT14 ---> VLAN7\
DUT-PORT15 & DUT-PORT16 ---> VLAN8\
DUT-PORT17 & DUT-PORT18 ---> VLAN9\
DUT-PORT19 & DUT-PORT20 ---> VLAN10\
DUT-PORT21 & DUT-PORT22 ---> VLAN11\
DUT-PORT23 & DUT-PORT24 ---> VLAN12\
DUT-PORT25 & DUT-PORT26 ---> VLAN13\
DUT-PORT27 & DUT-PORT28 ---> VLAN14\
DUT-PORT29 & DUT-PORT30 ---> VLAN15\
DUT-PORT31 & DUT-PORT32 ---> VLAN16\
DUT-PORT33 & DUT-PORT34 & ATE-PORT2 ---> VLAN17

## Procedure

### Testbed setup - Generate configuration for ATE and DUT

#### DUT Configuration
  * Create 18 VLAN's on the DUT from VLAN1 to VLAN18 and are all configured as Access VLAN
  * Assign IPv4, IPv6 addresses to ATE-PORT1, ATE-PORT2
      ATE-PORT1 - IPv4 address 192.168.1.1/24; IPv6 address 2000:1:1:1::1/64
      ATE-PORT1 - IPv4 address 193.168.1.1/24; IPv6 address 2001:1:1:1::1/64
  * Traffic enters from ATE-PORT1 to DUT:PORT1 tagged/untagged and it is forwarded to 
    DUT:PORT2 part of VLAN1. Then traffic is forwarded to DUT:PORT3 because it is 
    back-back connected. Once it enters DUT:PORT3, traffic is forwarded to DUT:PORT4
    and similar to the earlier scenario traffic enters into the DUT:PORT5 and follows
    same till we reach the DUT-PORT36 of VLAN18 which is connected to ATE-PORT2
  * Traffic entering from ATE-PORT2 follows similar forwarding mechanism as shown
    in the previous step

#### Traffic profile
  * Create 6 traffic profiles as below 
  
##### Traffic-ipv4-framesize-64bytes
  * Create ipv4 traffic profile which has following properties
    - Frame size 64 bytes
    - Line rate traffic (100%) / 595millon packets per second (pps)

##### Traffic-ipv4-framesize-mixed
  * Create ipv4 traffic profile which has following properties
    - Frame size mixed (64-1518) with an average packet size of 760bytes
    - Line rate traffic (100%) / ~74million packets per second (pps)

##### Traffic-ipv4-framesize-jumbo-9000bytes
  * Create ipv4 traffic profile which has following properties
    - Frame size 9000 bytes
    - Line rate traffic (100%) / 5.5million packets per second (pps)

##### Traffic-ipv6-framesize-64bytes
  * Create ipv6 traffic profile which has following properties
    - Frame size 64 bytes
    - Line rate traffic (100%) / 595millon packets per second (pps)

##### Traffic-ipv6-framesize-mixed
  * Create ipv6 traffic profile which has following properties
    - Frame size mixed (64-1518) with an average frame size of 760
    - Line rate traffic (100%) / ~74million packets per second (pps)

##### Traffic-ipv6-framesize-jumbo-9000bytes
  * Create ipv6 traffic profile which has following properties
    - Frame size 9000 bytes
    - Line rate traffic (100%) / 5.5million packets per second (pps)

    
## Canonical OC
```json
{
  "interfaces": {
    "interface": [
      {
        "aggregation": {
          "config": {
            "lag-type": "LACP",
            "min-links": 1
          }
        },
        "config": {
          "name": "ae0"
        },
        "name": "ae0"
      },
      {
        "config": {
          "loopback-mode": "FACILITY",
          "name": "eth0"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "ae0",
            "duplex-mode": "FULL",
            "port-speed": "SPEED_10GB"
          }
        },
        "name": "eth0"
      }
    ]
  }
}
```

## TestCase-1:
### INT-1.1.1 Test IPv4 traffic 400G throughput
#### Start test
  *   Start traffic profile "Traffic-ipv4-framesize-64bytes" described above

#### Verification
  *   Verify that each port on the device shows 400G in and out traffic statistics
  *   Verify that the traffic sent from ATE:PORT1 as source is recieved on ATE:PORT2
      as destination
  *   Make sure there is 0 drop in packets
  *   Verify CPU utilization, Power utilization and it should be normal

### INT-1.1.2 Test IPv4 traffic 400G throughput
#### Start test
  *   Start traffic profile "Traffic-ipv4-framesize-mixed" described above

#### Verification
  *   Verify that each port on the device shows 400G in and out traffic statistics
  *   Verify that the traffic sent from ATE:PORT1 as source is recieved on ATE:PORT2
      as destination
  *   Make sure there is 0 drop in packets
  *   Verify CPU utilization, Power utilization and it should be normal


### INT-1.1.3 Test IPv4 traffic 400G throughput
#### Start test
  *   Start traffic profile "Traffic-ipv4-framesize-jumbo-9000bytes" described above

#### Verification
  *   Verify that each port on the device shows 400G in and out traffic statistics
  *   Verify that the traffic sent from ATE:PORT1 as source is recieved on ATE:PORT2
      as destination
  *   Make sure there is 0 drop in packets
  *   Verify CPU utilization, Power utilization and it should be normal

### INT-1.2.1 Test IPv6 traffic 400G throughput
#### Start test
  *   Start traffic profile "Traffic-ipv6-framesize-64bytes" described above

#### Verification
  *   Verify that each port on the device shows 400G in and out traffic statistics
  *   Verify that the traffic sent from ATE:PORT1 as source is recieved on ATE:PORT2
      as destination
  *   Make sure there is 0 drop in packets
  *   Verify CPU utilization, Power utilization and it should be normal

### INT-1.2.2 Test IPv6 traffic 400G throughput
#### Start test
  *   Start traffic profile "Traffic-ipv6-framesize-mixed" described above

#### Verification
  *   Verify that each port on the device shows 400G in and out traffic statistics
  *   Verify that the traffic sent from ATE:PORT1 as source is recieved on ATE:PORT2
      as destination
  *   Make sure there is 0 drop in packets
  *   Verify CPU utilization, Power utilization and it should be normal

### INT-1.2.3 Test IPv6 traffic 400G throughput
#### Start test
  *   Start traffic profile "Traffic-ipv6-framesize-jumbo-9000bytes" described above

#### Verification
  *   Verify that each port on the device shows 400G in and out traffic statistics
  *   Verify that the traffic sent from ATE:PORT1 as source is recieved on ATE:PORT2
      as destination
  *   Make sure there is 0 drop in packets
  *   Verify CPU utilization, Power utilization and it should be normal

## OpenConfig Path and RPC Coverage

The below YAML defines the OC paths intended to be covered by this test. 
OC paths used for test setup are not listed here.

```yaml
openconfig_paths:
  ## Config paths
  /interfaces/interface/config/name:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:

  ## Telemetry paths
  /interfaces/interface/state/loopback-mode:
  /interfaces/interface/state/counters/in-discards:
  /interfaces/interface/state/counters/in-errors:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/in-unicast-pkts:
  /interfaces/interface/state/counters/out-discards:
  /interfaces/interface/state/counters/out-errors:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/counters/out-pkts:
  /interfaces/interface/state/counters/out-unicast-pkts:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement
* FFF
* 32 PORTS CONNECTED B2B wired


