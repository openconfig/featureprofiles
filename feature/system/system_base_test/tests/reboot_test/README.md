# System-2.1: DUT Reboot Test

## Summary

- Configure BGP, IS-IS and redistribution
- Validate control-plane and data-plane post reboot
- Validate P4RT, gNMI, SHH connectivity post reboot

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed

## Procedure

#### Initial Setup:

*   Connect DUT port-1, 2 to ATE port-1, 2
*   Configure IPv4/IPv6 addresses on the ports
*   Configure IPv4 and IPv6 IS-IS L2 adjacency between ATE port-1 and DUT port-1
*   Configure IPv4 and IPv6 eBGP between DUT Port-2 and ATE Port-2
*   Advertise a few IPv4/v6 BGP routes from ATE to DUT through the BGP session
*   Configure redistribution of the BGP prefixes to IS-IS on the DUT

### System-2.1.1
#### Pre Reboot Validation
---

##### Validate SSH connectivity
*   Verify SSH connectivity to the device works

##### Validate P4RT connectivity
*   Configure P4RT id and node-id (device_id) with port-1 interface
    *  Linecard `device_id = 111`
*   Using P4RT Read RPC send a ReadRequest for port-1 counters
*   Ensure a ReadResponse is received

##### Validate Traffic Forwarding
*   Initiate traffic from ATE port-1 to the DUT and destined towards the BGP redistributed routes
*   Validate that the traffic is received on ATE port-2

#### Canonical OC
```json
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
                      "config": {
                        "neighbor-address": "192.0.2.6"
                      },
                      "neighbor-address": "192.0.2.6",
                      "timers": {
                        "config": {
                          "hold-time": 30,
                          "keepalive-interval": 10
                        }
                      }
                    }
                  ]
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "config": {
                        "peer-group-name": "peer_group"
                      },
                      "peer-group-name": "peer_group",
                      "timers": {
                        "config": {
                          "hold-time": 30,
                          "keepalive-interval": 10
                        }
                      }
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
            }
          ]
        }
      }
    ]
  }
}
```

### System-2.1.2
#### Post Reboot Validation
---
##### Reboot the DUT
*   Reboot the DUT and wait for it to come back up

##### Validate SSH connectivity
*   Verify SSH connectivity to the device works

##### Validate P4RT connectivity
*   Configure P4RT id and node-id (device_id) with port-1 interface
    *  Linecard `device_id = 111`
*   Using P4RT Read RPC send a ReadRequest for port-1 counters
*   Ensure a ReadResponse is received

##### Validate Traffic Forwarding
*   Initiate traffic from ATE port-1 to the DUT and destined towards the BGP redistributed routes
*   Validate that the traffic is received on ATE port-2


## Protocol/RPC Parameter Coverage

* gNMI
  * Get
  * Set

## Required DUT platform

* FFF

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml 
paths:
  /network-instances/network-instance/table-connections/table-connection/config/address-family:
  /network-instances/network-instance/table-connections/table-connection/config/src-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/dst-protocol:


rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```