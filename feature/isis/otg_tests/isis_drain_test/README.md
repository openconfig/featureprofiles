# RT-2.14: IS-IS Drain Test

## Summary

Verify that traffic can be drained out of DUT trunk interfaces by changing ISIS metric and using overload bit.

## Testbed type

* [`atedut_3.testbed`](https://github.com/openconfig/featureprofiles/tree/main/topologies)

## Procedure

### Test environment setup

* Configure a 3-port ATE topology connected to the DUT (ATE Port-1, Port-2, and Port-3).
* Configure DUT with trunk interfaces (e.g., trunk-2 connected to Port-2, and trunk-3 connected to Port-3) and a single interface connected to Port-1.
* Establish IS-IS adjacencies (IPv4 and IPv6) on all links between the DUT and ATE ports.
* Configure a DUT Loopback IP and advertise it into IS-IS.

### RT-2.14.1 - IS-IS Metric Drain (Trunk Interfaces)

* Step 1 - Advertise 1,000 IPv4 and 1,000 IPv6 prefixes from the ATE connected to trunk-2 and trunk-3.
* Step 2 - Send continuous IPv4 and IPv6 traffic flows from ATE Port-1 to the 2,000 advertised prefixes.
* Step 3 - Wait for IS-IS convergence (up to 30s). Validate that steady-state traffic loss is 0% and traffic is load-balanced or transits via trunk-2 and trunk-3.
* Step 4 - Change the ISIS metric of trunk-2 to `1000` via gNMI Set on path `/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/metric`.
* Step 5 - Validate that 100% of the traffic transits via trunk-3 only. Verify steady-state traffic loss is 0% after a brief convergence window.
* Step 6 - Revert the ISIS metric on trunk-2 back to its original value. Validate that traffic recovers and is again load-balanced/transiting via both trunk-2 and trunk-3 with 0% steady-state loss.

### RT-2.14.2 - IS-IS Overload Bit Drain

* Step 1 - Continuing from the previous subtest, ensure ATE Port-2 advertises transit networks.
* Step 2 - Establish an iBGP session from ATE Port-1 to the DUT's loopback interface to simulate control plane traffic.
* Step 3 - Send transit traffic (ATE Port-1 -> DUT -> ATE Port-2) and local traffic (ATE Port-1 -> DUT Loopback).
* Step 4 - Set the ISIS Overload bit to `true` via gNMI Set on path `/network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/config/set-bit`.
* Step 5 - Use gNMI Get to verify the telemetry state `/network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/state/set-bit` reflects the `true` configuration.
* Step 6 - Wait for convergence. Verify that transit traffic to ATE Port-2 drops to 0 (routed around the DUT) with no false positives.
* Step 7 - Verify that local traffic (and the iBGP session) to the DUT loopback experiences 0% loss (Control Plane Preservation).
* Step 8 - Verify that toggling the overload-bit config does not flap or reset the existing IS-IS adjacencies by checking that the state path `/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state` remains `UP` (Adjacency Stability).
* Step 9 - Clear the ISIS Overload bit by setting it to `false` via gNMI Set on path `/network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/config/set-bit`.
* Step 10 - Verify the telemetry state reflects the change to `false`.
* Step 11 - Verify that transit IPv4 and IPv6 traffic recovers and is once again successfully forwarded through the DUT to ATE Port-2 with 0% steady-state loss.

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
              "identifier": "ISIS",
              "name": "ISIS",
              "isis": {
                "global": {
                  "lsp-bit": {
                    "overload-bit": {
                      "config": {
                        "set-bit": true
                      }
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

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /interfaces/interface/aggregation/config/lag-type:
  /interfaces/interface/aggregation/config/min-links:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/protocols/protocol/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/global/config/instance:
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/global/config/max-ecmp-paths:
  /network-instances/network-instance/protocols/protocol/isis/global/config/net:
  /network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/config/set-bit:
  /network-instances/network-instance/protocols/protocol/isis/global/lsp-bit/overload-bit/state/set-bit:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/afi-safi/af/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/interface-ref/config/interface:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/interface-ref/config/subinterface:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/metric:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/metric-style:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* vRX
