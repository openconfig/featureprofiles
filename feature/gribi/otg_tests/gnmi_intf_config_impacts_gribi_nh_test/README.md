# TE-1.7: gNMI Interface Config Change Impacting gRIBI NextHop

## Summary

Verify that configuration changes made via gNMI to an interface (e.g., MTU,
speed, enabling/disabling) used by an active gRIBI NextHop are handled
gracefully.

When an interface state changes due to gNMI configuration (like shutting down
the interface or changing its MTU), any gRIBI-programmed NextHop relying on
that interface must react gracefully. The gRIBI-routed traffic stream state
should be predictable, recovering via backup paths or restoring once the
interface is back online. The viability of the gRIBI NextHop should be
correctly reflected in gNMI telemetry, and the system must remain stable
without causing switch instability.

## Testbed type

* `TESTBED_DUT_ATE_4LINKS`

## Procedure

### Test environment setup

* Connect ATE port-1 to DUT port-1, ATE port-2 to DUT port-2, ATE port-3 to DUT port-3, and ATE port-4 to DUT port-4.
* Configure IPv4 and IPv6 addresses on the interfaces.
  * DUT port-1 (Ethernet1/1/1): `192.0.2.1/30` and `2001:db8:1::1/126`
  * ATE port-1: `192.0.2.2/30` and `2001:db8:1::2/126`
  * DUT port-2 (Ethernet1/1/2): `198.51.100.1/30` and `2001:db8:2::1/126`
  * ATE port-2: `198.51.100.2/30` and `2001:db8:2::2/126`
  * DUT port-3 (Ethernet1/1/3): `198.51.100.5/30` and `2001:db8:3::1/126`
  * ATE port-3: `198.51.100.6/30` and `2001:db8:3::2/126`
  * DUT port-4 (Ethernet1/1/4): `198.51.100.9/30` and `2001:db8:4::1/126`
  * ATE port-4: `198.51.100.10/30` and `2001:db8:4::2/126`
* Enable gNMI, gRIBI, and gNOI services on the DUT.

### TE-1.7.1 - Port Admin State Bounce Impact on gRIBI NextHop

* Step 1 - Generate DUT configuration

Configure the baseline interfaces on the DUT.

#### Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "Ethernet1/1/2",
        "config": {
          "name": "Ethernet1/1/2",
          "enabled": true,
          "mtu": 1500
        },
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "ip": "198.51.100.1",
                      "config": {
                        "ip": "198.51.100.1",
                        "prefix-length": 30
                      }
                    }
                  ]
                }
              },
              "ipv6": {
                "addresses": {
                  "address": [
                    {
                      "ip": "2001:db8:2::1",
                      "config": {
                        "ip": "2001:db8:2::1",
                        "prefix-length": 126
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

* Step 2 - Establish gRIBI client session.
* Step 3 - Program gRIBI NextHops, NextHopGroup, and 1,000 IPv4/IPv6 entries pointing to a NextHopGroup (NHG) containing DUT port-2, port-3, and port-4 for ECMP validation.
  * NextHop 1 (Index 10): DUT port-2 (Ethernet1/1/2), MAC `00:1A:11:00:1A:BC`
  * NextHop 2 (Index 11): DUT port-3 (Ethernet1/1/3), MAC `00:1A:11:00:1A:BD`
  * NextHop 3 (Index 12): DUT port-4 (Ethernet1/1/4), MAC `00:1A:11:00:1A:BE`
  * NextHopGroup (Index 100): Contains NextHops 10, 11, and 12 with equal weights.
  * IPv4 Prefixes: `203.0.113.1/32` through `203.0.116.232/32` (1,000 prefixes) pointing to NHG 100.
  * IPv6 Prefixes: `2001:db8:a::1/128` through `2001:db8:a::3e8/128` (1,000 prefixes) pointing to NHG 100.
* Step 4 - Send continuous IPv4 and IPv6 traffic at 1000 pps from ATE port-1 to the programmed prefixes. Verify traffic hashes evenly across ATE port-2, port-3, and port-4.
* Step 5 - Disable DUT port-2 via gNMI Set replacing `/interfaces/interface[name=Ethernet1/1/2]/config/enabled` to `false`.
* Step 6 - Subscribe via gNMI ON_CHANGE to `/interfaces/interface[name=Ethernet1/1/2]/state/oper-status` and verify it transitions to `DOWN`.
* Step 7 - Subscribe via gNMI ON_CHANGE to `/network-instances/network-instance[name=DEFAULT]/afts/next-hops/next-hop[index=10]/state/index` and verify it reflects the failure or viability change (if supported).
* Step 8 - Verify traffic to ATE port-2 drops to exactly 0 pps, and traffic from ATE port-1 redistributes to ATE port-3 and ATE port-4. Monitor for 30 seconds to ensure no switch instability or route flapping.
* Step 9 - Enable DUT port-2 via gNMI Set replacing `/interfaces/interface[name=Ethernet1/1/2]/config/enabled` to `true`.
* Step 10 - Subscribe via gNMI ON_CHANGE to `/interfaces/interface[name=Ethernet1/1/2]/state/oper-status` and verify it transitions to `UP`.
* Step 11 - Verify traffic resumes hashing evenly across ATE port-2, port-3, and port-4. Monitor for 30 seconds to confirm 0% loss and stable ECMP distribution.

### TE-1.7.2 - MTU Change Impact on gRIBI NextHop

* Step 1 - Continuing from TE-1.7.1 without tearing down gRIBI or gNMI sessions, ensure continuous traffic is flowing from ATE port-1 and hashing across ATE port-2, port-3, and port-4.
* Step 2 - Change the MTU of DUT port-2 via gNMI Set replacing `/interfaces/interface[name=Ethernet1/1/2]/config/mtu` to `9000`.
* Step 3 - Subscribe via gNMI ON_CHANGE to `/interfaces/interface[name=Ethernet1/1/2]/state/mtu` and verify it reflects `9000`.
* Step 4 - Verify the switch remains stable and traffic continues to hash across all three egress ports. (A brief flap on port-2 during MTU change is acceptable depending on vendor implementation, but traffic must fully recover across all ports). Monitor for 30 seconds.
* Step 5 - Revert the MTU of DUT port-2 via gNMI Set replacing `/interfaces/interface[name=Ethernet1/1/2]/config/mtu` to `1500`.
* Step 6 - Subscribe via gNMI ON_CHANGE to `/interfaces/interface[name=Ethernet1/1/2]/state/mtu` and verify it reflects `1500` and traffic fully recovers across all ports.


### TE-1.7.3 - Negative Test: Program gRIBI Route on Admin DOWN Interface

* Step 1 - Continuing from TE-1.7.2, disable DUT port-2 via gNMI Set replacing `/interfaces/interface[name=Ethernet1/1/2]/config/enabled` to `false`.
* Step 2 - Subscribe via gNMI ON_CHANGE to verify `/interfaces/interface[name=Ethernet1/1/2]/state/oper-status` is `DOWN`.
* Step 3 - Program a new gRIBI NextHop (Index 20) pointing to DUT port-2 (Ethernet1/1/2) and a new IPv4 prefix `203.0.113.255/32` pointing to it.
* Step 4 - Subscribe via gNMI ON_CHANGE to `/network-instances/network-instance[name=DEFAULT]/afts/next-hops/next-hop[index=20]/state/index` and verify it is either rejected or correctly reflects an unviable state in telemetry.
* Step 5 - Send continuous IPv4 traffic at 1000 pps from ATE port-1 to `203.0.113.255`.
* Step 6 - Verify 100% traffic drop to this prefix.
* Step 7 - Re-enable DUT port-2 via gNMI Set and verify via ON_CHANGE subscription that it transitions to `UP`.
* Step 8 - Verify traffic to `203.0.113.255` now successfully forwards to ATE port-2.

### TE-1.7.4 - Negative Test: MTU Smaller Than Packet Size

* Step 1 - Continuing from TE-1.7.3, ensure traffic is flowing steadily to all programmed prefixes.
* Step 2 - Change the MTU of DUT port-2 via gNMI Set replacing `/interfaces/interface[name=Ethernet1/1/2]/config/mtu` to `500`.
* Step 3 - Send continuous IPv4 traffic with a packet size of 1500 bytes from ATE port-1 to a prefix solely destined to DUT port-2 (e.g., `203.0.113.255`).
* Step 4 - Verify traffic drops to exactly 0 pps for this prefix.
* Step 5 - Verify via gNMI Get that the interface error counters (e.g., `/interfaces/interface[name=Ethernet1/1/2]/ethernet/state/counters/in-oversize-frames` or `in-errors`) increment correspondingly to the dropped traffic.
* Step 6 - Revert the MTU of DUT port-2 to `1500` via gNMI Set and verify traffic resumes with 0% loss.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/config/mtu:
  /interfaces/interface/state/mtu:
  /interfaces/interface/ethernet/state/counters/in-oversize-frames:
  /interfaces/interface/state/counters/in-errors:
  /network-instances/network-instance/afts/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF
