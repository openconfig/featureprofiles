# RT-5.8: IPv6 Link Local

## Summary

Configure an IPv6 address which is in link local scope. Verify the link local
IPv6 address exists by checking the state path.

## Procedure

* Subtest #1 - Configure IPv6 link local
  * Configure DUT port 1 and OTG port 1 with an IPv6 link local scope IP address
  * Configure DUT port 2 and OTG port 2 with an IPv6 link local scope IP address
  * Validate config and state paths are set

* Subtest #2 - Verify the interface will pass IPv6 traffic as expected
  * Send IPv6 traffic from OTG port 1 to OTG port 2, validate OTG port 2 does
    not receive the traffic
  * Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1
    receives the traffic

* Subtest #3 - Verify adding and removing global unicast address does not affect link local address
  * Add configuration for a global unicast address on DUT port 1 and DUT port 2
  * Validate config and state paths are set
  * Send IPv6 traffic from OTG port 1 to OTG port 2, validate OTG port 2
    receives the traffic
  * Remove configuration for the global unitcast address on DUT port 1
  * Validate that DUT port 1 link local address is still configured
  * Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1
    receives the traffic
  * Send IPv6 traffic from OTG port 1 to OTG port 2, validate OTG port 2 does
    not receive the traffic

* Subtest #4 - Verify enable/disable of DUT port 1 does not affect link local address
  * Disable/enable the port and see if the configured link-local address stays?
  * Validate that DUT port 1 link local address config and state paths continue
    to contain the address assignment
  * Send IPv6 traffic from OTG port 1 to DUT port 1, validate DUT port 1
    receives the traffic

* Subtest #5 - IPv6 Neighbor Discovery REACHABLE to STALE state transition
  * Test setup:
    * DUT port 1 connected to ATE port 1.
    * DUT port 2 connected to ATE port 2.
    * Configure DUT port 1 with IPv6 global unicast address `2001:db8:1::1/64`
      on subinterface 0
      (`/interfaces/interface/subinterfaces/subinterface/config/index`).
    * Configure DUT port 2 with IPv6 global unicast address `2001:db8:2::1/64`
      on subinterface 0.
    * Configure ATE port 1 with IPv6 address `2001:db8:1::2/64`, MAC address
      `02:00:01:01:01:01`, and gateway `2001:db8:1::1`.
    * Configure the ATE port 1 gateway MAC statically (OTG `gateway_mac`) with
      the DUT port 1 MAC address from
      `/interfaces/interface/ethernet/state/mac-address` instead of resolving
      it with Neighbor Discovery. ATE port 1 then does not send Neighbor
      Solicitations (NS) to resolve DUT port 1, while its interface stays up
      and still replies to NS from DUT port 1 with solicited Neighbor
      Advertisements (NA).
    * Configure ATE port 2 with IPv6 address `2001:db8:2::2/64`, MAC address
      `02:00:01:02:01:01`, and gateway `2001:db8:2::1`.
  * Step 1: Send ICMPv6 Echo Requests between ATE port 1 and DUT port 1
    (`2001:db8:1::1`) and between ATE port 2 and DUT port 2 (`2001:db8:2::1`).
  * Step 2: Validate via gNMI state telemetry that neighbor discovery is
    complete and neighbor state is `REACHABLE`:
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/ip`
      is `2001:db8:1::2`.
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address`
      is `02:00:01:01:01:01`.
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state`
      is `REACHABLE`.
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/origin`
      is `DYNAMIC`.
  * Step 3: Stop all traffic flows and ICMPv6 Echo Requests toward DUT port 1.
    Keep the ATE port 1 link and protocols up (do not stop protocols or flap
    the link) so that ATE port 1 sends no traffic or NS toward DUT port 1 but
    still replies to NS from DUT port 1.
  * Step 4: Wait for the IPv6 neighbor reachability timer (`BaseReachableTime`,
    default 30 seconds) to expire without positive reachability confirmation.
  * Step 5: Validate via gNMI state telemetry that the neighbor entry on DUT
    port 1 transitions from `REACHABLE` to `STALE`:
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state`
      is `STALE`.
  * Step 6: Validate via gNMI state telemetry that the cached link-layer
    address is preserved in the neighbor table while in `STALE` state:
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address`
      remains `02:00:01:01:01:01`.

* Subtest #6 - Forwarding to an IPv6 neighbor in STALE state
  * Step 1: While neighbor `2001:db8:1::2` on DUT port 1 remains in `STALE`
    state, generate an IPv6 data traffic flow from ATE port 2 destined to ATE
    port 1 (`2001:db8:1::2`) routed through DUT port 1.
  * Step 2: Validate that DUT immediately forwards data packets out of port 1
    using the cached link-layer address `02:00:01:01:01:01` without dropping
    packets (0% packet loss, confirming the switch does not blackhole or drop
    traffic to stale neighbors).

* Subtest #7 - Verify IPv6 neighbor state transition from STALE back to REACHABLE state
  * Step 1: Stop traffic and allow neighbor `2001:db8:1::2` on DUT port 1 to
    transition back to `STALE` state after reachability timer expiry (repeat
    Subtest #5 Steps 3 to 5).
  * Step 2: Start the IPv6 data traffic flow from ATE port 2 to ATE port 1
    (`2001:db8:1::2`) routed through DUT port 1.
  * Step 3: Validate that sending data traffic to the stale neighbor triggers
    the DUT neighbor state machine to initiate reachability verification per
    RFC 4861:
    * The neighbor state transitions from `STALE` to `DELAY` and then `PROBE`.
    * DUT transmits unicast Neighbor Solicitation (NS) probes out of port 1
      toward ATE port 1.
  * Step 4: ATE port 1 replies to the NS probe with a solicited Neighbor
    Advertisement (NA). The ATE emulated interface does this by default, so no
    additional ATE configuration is needed.
  * Step 5: Validate via gNMI state telemetry that upon receiving the NA, the
    neighbor state on DUT port 1 transitions back to `REACHABLE`:
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state`
      is `REACHABLE`.
  * Step 6: Validate continuous traffic forwarding across DUT port 2 to port 1
    achieves 100% packet delivery with 0% loss during and after the transition.

* Subtest #8 - Neighbor cache update via unsolicited Neighbor Advertisement and control plane stability under load
  * Step 1: Halt traffic and allow neighbor `2001:db8:1::2` on DUT port 1 to
    transition back to `STALE` state after reachability timer expiry.
  * Step 2: Validate via gNMI state telemetry that neighbor state is `STALE`:
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state`
      is `STALE`.
  * Step 3: Transmit an unsolicited Neighbor Advertisement (NA) from ATE port 1
    (source `2001:db8:1::2`) to the all-nodes multicast address `ff02::1`,
    with target IPv6 address `2001:db8:1::2`, a new target link-layer address
    `02:00:01:01:01:02`, the `Override` flag set, and the `Solicited` and
    `Router` flags clear. OTG has no native NA header, so send the NA as an
    ATE raw packet flow on ATE port 1: Ethernet and IPv6 headers (Next Header
    `58`, Hop Limit `255`) followed by an OTG `custom` header that carries the
    ICMPv6 NA bytes, with the ICMPv6 checksum precomputed by the test.
  * Step 4: Validate via gNMI state telemetry that the DUT applies the
    unsolicited NA per RFC 4861 (sections 7.2.5 and 7.2.6): it updates the
    cached link-layer address but does not treat the NA as reachability
    confirmation:
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address`
      is `02:00:01:01:01:02`.
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state`
      is `STALE` (an unsolicited NA must not move the entry to `REACHABLE`).
  * Step 5: Transmit a second unsolicited NA, identical to the one in Step 3
    except that it carries the original target link-layer address
    `02:00:01:01:01:01`, and validate via gNMI state telemetry that the
    neighbor entry is restored:
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address`
      is `02:00:01:01:01:01`.
    * `/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state`
      is `STALE`.
  * Step 6: Generate continuous bidirectional data traffic between ATE port 1
    and ATE port 2 at line rate across the DUT.
  * Step 7: While data traffic is actively flowing, transmit periodic ICMPv6
    Echo Requests from ATE port 1 directed to the DUT interface IP
    (`2001:db8:1::1`).
  * Step 8: Validate that the DUT control plane responds to 100% of ICMPv6 Echo
    Requests without packet timeout or unreachability.
  * Step 9: Validate via gNMI state telemetry that the IPv6 neighbor table
    remains healthy, entries do not get stuck in `STALE`, and the switch
    remains consistently reachable throughout the duration of the test.

### Cleanup

* Stop all ATE traffic flows, including the raw Neighbor Advertisement flows,
  and stop ATE protocols.
* Register and execute `t.Cleanup()` routines that ensure DUT port 1 and DUT
  port 2 are administratively enabled (`/interfaces/interface/config/enabled`
  is `true`) and remove all IPv6 link-local and global unicast addresses
  configured on them during the test, so the DUT is returned to its exact
  pre-test baseline state even if the test fails.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT Port 1 to ATE Port 1",
          "enabled": true,
          "name": "port1"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
                "index": 0
              },
              "index": 0,
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:1::1",
                        "prefix-length": 64
                      },
                      "ip": "2001:db8:1::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
                }
              }
            }
          ]
        }
      },
      {
        "config": {
          "description": "DUT Port 2 to ATE Port 2",
          "enabled": true,
          "name": "port2"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "enabled": true,
                "index": 0
              },
              "index": 0,
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "2001:db8:2::1",
                        "prefix-length": 64
                      },
                      "ip": "2001:db8:2::1"
                    }
                  ]
                },
                "config": {
                  "enabled": true
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

## Config Parameter Coverage

```
/interfaces/interface/subinterfaces/subinterface/config/index
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/type
```

## Telemetry Parameter Coverage

```
/interfaces/interface/subinterfaces/subinterface/state/index
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/prefix-length
/interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/type
/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/ip
/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/is-router
/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address
/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state
/interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/origin
```

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/enabled:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/config/index:
  /interfaces/interface/subinterfaces/subinterface/state/admin-status:
  /interfaces/interface/subinterfaces/subinterface/state/counters/in-pkts:
  /interfaces/interface/subinterfaces/subinterface/state/counters/out-pkts:
  /interfaces/interface/subinterfaces/subinterface/state/enabled:
  /interfaces/interface/subinterfaces/subinterface/state/index:
  /interfaces/interface/subinterfaces/subinterface/state/oper-status:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/state/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/is-router:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state:
  /interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/origin:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* FFF - fixed form factor

