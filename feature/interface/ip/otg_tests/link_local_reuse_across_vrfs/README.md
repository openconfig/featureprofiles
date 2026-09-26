# RT-5.16: Link-Local Subnet Reuse Across VRFs

## Summary

Verify that the device correctly supports assigning identical IPv4/IPv6 link-local addresses to multiple interfaces, provided they are isolated within different Virtual Routing and Forwarding (VRF) instances. This is a fundamental networking requirement for physical loopback prober designs to establish Layer 3 adjacency without consuming multiple public subnets.

## Testbed type

* dut.testbed: Single DUT with a physical fiber loop connecting two ports (e.g., Port 1 and Port 2).

## Procedure

### Test environment setup

* Configure the DUT with two independent network-instances (type L3VRF) named vrf-primary and vrf-secondary.
* Ensure physical loopback is present between two ports.

### RT-5.16.1 - Positive: Overlapping Subnets in Different VRFs

Verify that the device successfully applies overlapping link-local IPs when the target interfaces reside in different VRFs.

#### Step 1 - Generate DUT configuration

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": { "name": "vrf-primary", "type": "L3VRF" },
        "name": "vrf-primary",
        "interfaces": {
          "interface": [
            {
              "config": { "id": "Port-Channel6.100", "interface": "Port-Channel6.100", "subinterface": 100 },
              "id": "Port-Channel6.100"
            }
          ]
        }
      },
      {
        "config": { "name": "vrf-secondary", "type": "L3VRF" },
        "name": "vrf-secondary",
        "interfaces": {
          "interface": [
            {
              "config": { "id": "Port-Channel7.100", "interface": "Port-Channel7.100", "subinterface": 100 },
              "id": "Port-Channel7.100"
            }
          ]
        }
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": { "name": "Port-Channel6.100" },
        "name": "Port-Channel6.100",
        "subinterfaces": {
          "subinterface": [
            {
              "index": 100,
              "ipv4": {
                "addresses": {
                  "address": [
                    { "config": { "ip": "169.254.0.1", "prefix-length": 30 }, "ip": "169.254.0.1" }
                  ]
                }
              }
            }
          ]
        }
      },
      {
        "config": { "name": "Port-Channel7.100" },
        "name": "Port-Channel7.100",
        "subinterfaces": {
          "subinterface": [
            {
              "index": 100,
              "ipv4": {
                "addresses": {
                  "address": [
                    { "config": { "ip": "169.254.0.1", "prefix-length": 30 }, "ip": "169.254.0.1" }
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

* Step 2 - Push configuration to DUT using gnmi.Set.
* Step 3 - Verify control-plane reachability (e.g., ping between subinterfaces).
* Step 4 - Validation:
    * Interface State: Both interfaces report OPER_UP.
    * IP State: The identical IP address is confirmed present in telemetry for both independent VRFs.
    * Connectivity: The ping initiated in Step 3 reports 0% packet loss.
    * Adjacency: The device's neighbor table (ARP/ND) confirms that the MAC address for the peer IP matches the secondary port, proving the packet physically traversed the fiber loop.

### RT-5.16.2 - Negative: Collision Within the Same VRF

Verify that the system rejects identical link-local subnets if they reside in the same VRF or Global context.

* Step 1 - Configure Interface 1 in vrf-shared with 169.254.0.1/30.
* Step 2 - Attempt to configure Interface 2 in the same vrf-shared with 169.254.0.2/30.
* Step 3 - Validation: The system MUST reject the second configuration. The gNMI Set request must return a gRPC error indicating a configuration conflict.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/state/oper-status:
  /network-instances/network-instance/ipv4/neighbors/neighbor/state/link-layer-address:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF (Fixed Form Factor) or MFF (Modular Form Factor) with support for multiple VRFs.
