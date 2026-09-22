# RT-14.3: Soft Drain vs. Hard Drain Convergence Verification

## Summary

This test compares the packet loss duration of a soft drain (IS-IS metric
increase) against a hard drain (admin down) to verify the operational benefit
of soft-draining. The test ensures that soft draining via metric modification
provides a near hitless traffic transition compared to an interface hard
shutdown. Dual-stack (IPv4 and IPv6) traffic is used to evaluate convergence.

## Testbed type

* [`TESTBED_DUT_ATE_4LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

* Configure the DUT with 4 ports connected to ATE Port 1, 2, 3, and 4
  respectively.
* Configure IS-IS (Level-2 only) adjacencies on all four ports between DUT and
  ATE.
* For ATE Port 1, configure IPv4 address `192.0.2.2/30` and IPv6
  `2001:db8:1::2/126`. (DUT is `192.0.2.1/30`, `2001:db8:1::1/126`).
* For ATE Port 2, configure IPv4 address `192.0.2.6/30` and IPv6
  `2001:db8:2::2/126`. (DUT is `192.0.2.5/30`, `2001:db8:2::1/126`).
* For ATE Port 3, configure IPv4 address `192.0.2.10/30` and IPv6
  `2001:db8:3::2/126`. (DUT is `192.0.2.9/30`, `2001:db8:3::1/126`).
* For ATE Port 4, configure IPv4 address `192.0.2.14/30` and IPv6
  `2001:db8:4::2/126`. (DUT is `192.0.2.13/30`, `2001:db8:4::1/126`).
* ATE Port 3 and 4 advertise a block of 10,000 IPv4 routes and 10,000 IPv6
  routes via IS-IS.
* Establish iBGP peering between the DUT and ATEs (using loopbacks reachable via
  IS-IS).
* ATE Port 3 and 4 advertise 100,000 BGP routes over the iBGP sessions.
* Ensure ECMP is established on DUT to both ATE Port 3 and 4 for the
  destination networks.
* Ensure all IS-IS and BGP adjacencies are strictly established and stable.

### RT-14.3.1 - Hard Drain (Admin Down) Convergence

* Step 1 - Generate DUT configuration.
* Step 2 - Push configuration to DUT using `gNMI.Set` with `REPLACE` option. Wait
  for IS-IS and BGP neighbor adjacencies to be fully UP.
* Step 3 - Use `gNMI.Get` or `gNMI.Subscribe` to verify the control plane has
  converged before beginning the data-plane soak. Validate that the BGP routes are
  programmed in the hardware forwarding table (FIB) by ensuring the number of installed
  prefixes equals the expected 100,000 for the corresponding AFI/SAFI:
  * `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed`
* Step 4 - Start continuous background IPv4 and IPv6 traffic from ATE Port 1
  and Port 2 to the destination networks. Set a fixed, high transmission rate
  (e.g., 100,000 packets per second per flow) to accurately measure convergence
  in milliseconds.
* Step 5 - Validate 0% packet loss for a soak period of 10 seconds to ensure a
  stable baseline.
* Step 6 - Disable the active interface to ATE Port 3 using `gNMI.Set` (UPDATE)
  by setting `/interfaces/interface[name=<port3>]/config/enabled` to `false`.
* Step 7 - Measure packet loss duration until BGP/ISIS reconverges on the
  alternate path (ATE Port 4). Calculate the loss duration:
  `loss_duration_ms = (tx_frames - rx_frames) / (tx_rate_pps) * 1000`. Record
  the loss duration.
* Step 8 - Re-enable the interface on DUT Port 3 using `gNMI.Set` (UPDATE) by
  setting `/interfaces/interface[name=<port3>]/config/enabled` to `true`.
* Step 9 - Wait for the IS-IS adjacency to recover, BGP to reconverge, and
  verify traffic load balances again across Port 3 and Port 4 with 0% packet
  loss.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "DUT_PORT_3",
        "config": {
          "name": "DUT_PORT_3",
          "enabled": false
        }
      }
    ]
  }
}
```

### RT-14.3.2 - Soft Drain (IS-IS Metric Increase) Convergence

* Step 1 - While traffic is load balancing across ATE Port 3 and 4, validate 0%
  packet loss for a soak period of 10 seconds.
* Step 2 - Use `gNMI.Set` (UPDATE) to update the IS-IS metric for the interface
  facing ATE Port 3 to the maximum value (16777215 for wide metrics) to soft-drain it:
  * IPv4: `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=ISIS]/isis/interfaces/interface[interface-id=<port3>]/levels/level[level-number=2]/afi-safi/af[afi-name=IPV4][safi-name=UNICAST]/config/metric = 16777215`
  * IPv6: `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=ISIS]/isis/interfaces/interface[interface-id=<port3>]/levels/level[level-number=2]/afi-safi/af[afi-name=IPV6][safi-name=UNICAST]/config/metric = 16777215`

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
                "interfaces": {
                  "interface": [
                    {
                      "interface-id": "DUT_PORT_3",
                      "levels": {
                        "level": [
                          {
                            "level-number": 2,
                            "afi-safi": {
                              "af": [
                                {
                                  "afi-name": "IPV4",
                                  "safi-name": "UNICAST",
                                  "config": {
                                    "afi-name": "IPV4",
                                    "safi-name": "UNICAST",
                                    "metric": 16777215
                                  }
                                },
                                {
                                  "afi-name": "IPV6",
                                  "safi-name": "UNICAST",
                                  "config": {
                                    "afi-name": "IPV6",
                                    "safi-name": "UNICAST",
                                    "metric": 16777215
                                  }
                                }
                              ]
                            }
                          }
                        ]
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

* Step 3 - Use `gNMI.Get` or `gNMI.Subscribe` to verify the route shift via
  telemetry on the DUT, accounting for potential BGP Next-Hop tracking delays due
  to scanner timers. Verify that the active next-hop group for the advertised prefixes
  has updated to the alternate path (ATE Port 4) in the Abstract Forwarding Table (AFT):
  * IPv4: `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry[prefix=<prefix>]/state/next-hop-group`
  * IPv6: `/network-instances/network-instance/afts/ipv6-unicast/ipv6-entry[prefix=<prefix>]/state/next-hop-group`
* Step 4 - Wait for traffic to entirely shift to the alternate path (ATE Port
  4). Measure the packet loss during the metric change process.
* Step 5 - Once traffic has completely shifted, disable the interface to ATE
  Port 3 using `gNMI.Set` (UPDATE) by setting `/interfaces/interface[name=<port3>]/config/enabled`
  to `false`.
* Step 6 - Measure the packet loss during this subsequent link shutdown.
* Step 7 - Validation with pass/fail criteria:
  * **Pass**: The packet loss duration during the metric change in RT-14.3.2
    must be near zero, and the packet loss during the subsequent link shutdown
    must be zero. The total packet loss duration for RT-14.3.2 must be at least
    an order of magnitude lower than the hard link down packet loss measured
    in RT-14.3.1.
  * **Fail**: Traffic loss duration during soft drain is equivalent to hard
    drain, or traffic does not smoothly shift away from the soft-drained
    interface.
* Step 8 - Restoration: Re-enable the interface on DUT Port 3 using `gNMI.Set`
  (UPDATE) by setting `/interfaces/interface[name=<port3>]/config/enabled` to
  `true`. Wait for the IS-IS adjacency to recover. Then restore the IS-IS metric
  to its original value and verify traffic load balances again across Port 3 and
  Port 4 with 0% packet loss.

### RT-14.3.3 - Soft Drain with No Alternate Path

* Step 1 - While traffic is load balancing across ATE Port 3 and 4, disable ATE
  Port 4 and wait for traffic to fully shift to ATE Port 3.
* Step 2 - Apply the soft drain by increasing the IS-IS metric to the maximum
  value (16777215) on the interface facing ATE Port 3.
* Step 3 - Validation with pass/fail criteria:
  * **Pass**: Traffic must continue to flow over Port 3 with zero loss despite
    the high metric (since it is the only viable path).
  * **Fail**: Traffic drops unexpectedly upon metric increase.
* Step 4 - Shut down ATE Port 3 administratively and measure loss (will be 100%
  since no alternate path).
* Step 5 - Restoration: Re-enable both ATE Port 3 and ATE Port 4, restore
  original metrics, and verify ECMP load balancing.

### RT-14.3.4 - Soft Undrain (Graceful Insertion)

* Step 1 - Start with ATE Port 3 administratively disabled and traffic flowing
  completely over ATE Port 4.
* Step 2 - Ensure the IS-IS metric for the interface facing ATE Port 3 is already
  pre-configured to the maximum value (16777215).
* Step 3 - Administratively enable ATE Port 3.
* Step 4 - Validation with pass/fail criteria (Insertion):
  * **Pass**: Verify the IS-IS adjacency forms, but no traffic shifts (since the
    metric is max). Packet loss must be zero.
  * **Fail**: Traffic shifts prematurely, causing loss.
* Step 5 - Lower the IS-IS metric for ATE Port 3 to the normal baseline value.
* Step 6 - Validation with pass/fail criteria (Undrain):
  * **Pass**: Measure the hitless transition of traffic back to ECMP across Port
    3 and Port 4. Loss must be zero or near zero.
  * **Fail**: Substantial packet loss occurs during the ECMP restoration.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Interface configuration
  /interfaces/interface/config/enabled:
  # ISIS interface configuration
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/config/metric:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF - fixed form factor
