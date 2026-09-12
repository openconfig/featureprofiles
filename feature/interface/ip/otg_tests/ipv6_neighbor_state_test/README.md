# RT-5.16: IPv6 Dynamic Neighbor Discovery (NDP) State Transitions

## Summary

Verify the dynamic IPv6 Neighbor Discovery (NDP) state transitions on the DUT.
This includes verifying neighbor resolution, transitions to STALE state on inactivity, and recovery back to REACHABLE when traffic resumes.

## Procedure

*   Configure DUT port-1 with IPv6 address `2001:db8:1::1/64` and port-2 with `2001:db8:2::1/64`.
*   Configure ATE port-1 with IPv6 address `2001:db8:1::2/64` and ATE port-2 with `2001:db8:2::2/64`.
*   Enable IPv6 on both interfaces.

### TC1: Initial Neighbor Discovery Resolution (REACHABLE State)
*   Start continuous traffic flow from ATE port-2 to ATE port-1 (forwarded by the DUT).
*   Verify that the neighbor entry for ATE port-1 (`2001:db8:1::2`) is resolved on the DUT.
*   Verify that the neighbor attributes under `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state` match:
    *   `ip`: `2001:db8:1::2`
    *   `link-layer-address`: ATE port-1 MAC address
    *   `origin`: `DYNAMIC`
    *   `neighbor-state`: `REACHABLE`

### TC2: Neighbor Inactivity and Transition to STALE
*   Stop all traffic forwarding and interface packets.
*   Wait for the neighbor entry to transition to `STALE` state (usually ~30s-60s on typical implementations).
*   Query `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state` until it shows `STALE`.
*   Verify the neighbor attributes:
    *   `neighbor-state`: `STALE`
    *   `link-layer-address`: remains populated with ATE port-1 MAC address

### TC3: Stale Neighbor Traffic Recovery (Transition to REACHABLE)
*   Resume continuous traffic flow from ATE port-2 to ATE port-1.
*   Ensure that the DUT sends packets to the stale neighbor `2001:db8:1::2`.
*   Verify that the neighbor state transitions back to `REACHABLE` (may transiently go through `DELAY` or `PROBE` state).
*   Verify that the packet loss at the ATE receiver is sub-second (minimal packet loss during NDP probe phase).
*   Verify that the neighbor attributes are updated back to:
    *   `neighbor-state`: `REACHABLE`

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Ethernet1",
        "config": {
          "name": "Ethernet1"
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {
                "index": 0
              },
              "ipv6": {
                "config": {
                  "enabled": true
                },
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8:1::1",
                      "config": {
                        "ip": "2001:db8:1::1",
                        "prefix-length": 64
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

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPC intended to be covered by this test.

```yaml
paths:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/origin:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT Platform Requirement

FFF
