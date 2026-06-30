# RT-1.73: Static Route Resilience Test

## Summary

This test validates the functionality and resilience of static routes configured
with next-hops pointing to complex logical interfaces (VLANs and LAGs).
Specifically, it verifies:

* DUT can forward IPv4/IPv6 flows over Switched VLAN interfaces and Link Aggregation Groups (LAGs).
* The next-hop of the route can point to a VLAN interface or an aggregate interface recursively.
* **Resilience & Control Plane Integrity**: When the underlying
  member links of a LAG or VLAN go down, the aggregate/VLAN operational status
  correctly transitions to `DOWN`, and subsequently unrelated gNMI Set
  operations succeed without failing due to "unreachable next-hop" or
  translation errors.
* **FIB Reprogramming & Scale**: Ensuring that the FIB correctly reprograms when static routes are updated with new next-hops, and correctly scales with multiple subinterfaces.
* **Route Persistence**: Validating routes persist across Linecard OIR / port disable events.

## Testbed Type

* [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure

### Test environment setup

* Connect DUT Ports `1` through `8` to ATE Ports `1` through `8`.
* **VLAN Interface:** Configure VLAN 10 on the DUT. Assign DUT Ports `1` and `2` as access ports to VLAN 10 via `/interfaces/interface/ethernet/switched-vlan/config/access-vlan`. Configure a Routed VLAN Interface (SVI) for VLAN 10 with IPv4 `198.51.100.1/24` and IPv6 `2001:db8:100::1/64` using `/interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip` (and `ipv6` equivalent). Configure ATE Ports `1` and `2` with IPs `198.51.100.2`, `198.51.100.3` and IPv6 `2001:db8:100::2`, `2001:db8:100::3`.
* **LAG 1 (Resilience):** Configure DUT Ports `3` and `4` as a Layer 3 Link Aggregation Group (LAG 1) by setting `/interfaces/interface/config/type` to `iana-if-type:ieee8023adLag` and setting the member ports' `/interfaces/interface/ethernet/config/aggregate-id`. Configure IPv4 `198.51.101.1/24` and IPv6 `2001:db8:101::1/64` on LAG 1. Configure a matching LAG on ATE Ports `3` and `4` with IP `198.51.101.2` and IPv6 `2001:db8:101::2`.
* **LAG 2 (Scale & FIB):** Configure DUT Ports `5` and `6` as Layer 3 LAG 2. Configure a matching LAG on ATE Ports `5` and `6`. Create 10 subinterfaces (VLAN tags 101-110). Assign IPv4 networks `198.51.111.0/24` through `198.51.120.0/24` and IPv6 networks `2001:db8:111::/64` through `2001:db8:120::/64` across the subinterfaces on both DUT and ATE.
* **Standalone Port:** Configure DUT Port `7` as a standalone routed interface with IPv4 `198.51.102.1/24` and IPv6 `2001:db8:102::1/64`. Configure ATE Port `7` with IP `198.51.102.2` and IPv6 `2001:db8:102::2`.
* **Traffic Source Port:** Configure DUT Port `8` as a standalone routed interface with IPv4 `10.0.0.1/24` and IPv6 `2001:db8:a::1/64`. Configure ATE Port `8` with IP `10.0.0.2` and IPv6 `2001:db8:a::2`.

### RT-1.73.1 - Validate Static Route with VLAN Interface (SVI)

* Step 1 - Configure static routes for destination networks `203.0.113.0/24` and `2001:db8:213::/64` via `/network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix`. Set the next-hops to `198.51.100.2` and `2001:db8:100::2` (ATE Port 1's IPs, reachable via the VLAN 10 SVI) using `/network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop`.
* Step 2 - Push configuration to DUT via gNMI Set Replace.
* Step 3 - Start IPv4 and IPv6 traffic from ATE Port `8` (Source) destined to `203.0.113.1` and `2001:db8:213::1`.
* Step 4 - Verify the DUT correctly routes the traffic out of DUT Port `1` (or `2`) towards the ATE.
* Step 5 - Stop traffic.

### RT-1.73.2 - Validate Static Route over LAG Interface

* Step 1 - Configure static routes for destination networks `203.0.114.0/24` and `2001:db8:214::/64` with next-hops set to `198.51.101.2` and `2001:db8:101::2` (ATE LAG 1's IPs).
* Step 2 - Push configuration to DUT.
* Step 3 - Start IPv4 and IPv6 traffic from ATE Port `8` (Source) destined to `203.0.114.1` and `2001:db8:214::1`.
* Step 4 - Verify the DUT routes the traffic out of DUT Ports `3` and `4`, load-balancing across the LAG 1 member links.
* *(Note: Do not stop traffic; leave it running for the next test)*

### RT-1.73.3 - Control Plane Resilience on LAG Failure

*(Builds directly on 1.73.2 with traffic actively flowing)*
* Step 1 - Simulate a LAG failure by disabling ATE Port `3` and ATE Port `4`.
* Step 2 - Verify via gNMI state paths (`/interfaces/interface/state/oper-status`) that DUT LAG 1 transitions to `DOWN`. Verify traffic drops.
* Step 3 - Perform an unrelated gNMI Set operation (e.g., updating the description on DUT Port `7`). Verify this succeeds and does not throw an "unreachable next-hop" error.
* Step 4 - Re-enable ATE Ports `3` and `4`. Verify via `/interfaces/interface/state/oper-status` that LAG 1 transitions to `UP` and traffic forwarding resumes automatically.
* Step 5 - Stop traffic.

### RT-1.73.4 - Scale, Dynamic FIB Re-programming, and Route Persistence

* Step 1 (Scale Routes) - Configure 100 IPv4 static routes (`10.1.0.0/24` through `10.1.99.0/24`) and 100 IPv6 static routes (`2001:db8:1001::/64` through `2001:db8:1099::/64`). Distribute the next-hops for these 200 routes evenly across the 10 ATE LAG 2 subinterface IPs (configured in the setup phase).
* Step 2 (Verify Scale) - Push configuration to DUT. Start 200 concurrent traffic flows from ATE Port `8` to the 200 route destinations. Verify traffic load-balances successfully out of DUT Ports `5` and `6`.
* Step 3 (FIB Update - Add Next-Hop) - While traffic is flowing, update all 200 static routes via a single gNMI Set `Replace` operation to add an *additional* next-hop pointing to `198.51.102.2` and `2001:db8:102::2` (ATE Port `7`'s standalone IPs).
* Step 4 (Verify Add) - Verify the DUT seamlessly load-balances traffic across DUT Ports `5`, `6` (LAG 2) **AND** DUT Port `7` without traffic loss.
* Step 5 (FIB Update - Remove Next-Hop) - Update all 200 static routes via gNMI Set `Replace` to remove the LAG 2 subinterface next-hops, leaving *only* ATE Port `7` as the next-hop.
* Step 6 (Verify Remove) - Verify 100% of the traffic now flows strictly out of DUT Port `7`.
* Step 7 (Linecard OIR / Disable) - Simulate Linecard OIR by disabling the linecard hosting DUT Port `7` (or simply admin-disable DUT Port `7` via `/interfaces/interface/config/enabled` if OIR is unsupported on the platform/testbed). Verify traffic drops.
* Step 8 (Verify Persistence) - Enable the linecard (or DUT Port `7`). Verify the static routes persist and traffic forwarding resumes automatically over DUT Port `7`.
* Step 9 (Stop & Delete) - Stop traffic. Delete all 200 static routes using a gNMI `Delete` operation.
* Step 10 (Verify Flush) - Start traffic briefly to verify all traffic is dropped by the DUT, confirming the FIB was fully flushed, then stop traffic.

## Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "Vlan10",
        "config": {
          "name": "Vlan10"
        }
      }
    ]
  },
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "name": "default",
        "config": {
          "name": "default"
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "openconfig-policy-types:STATIC",
              "name": "DEFAULT",
              "config": {
                "identifier": "openconfig-policy-types:STATIC",
                "name": "DEFAULT"
              },
              "static-routes": {
                "static": [
                  {
                    "prefix": "198.51.100.160/28",
                    "config": {
                      "prefix": "198.51.100.160/28"
                    },
                    "next-hops": {
                      "next-hop": [
                        {
                          "index": "0",
                          "config": {
                            "index": "0",
                            "next-hop": "198.51.100.161"
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
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Paths ##
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/ethernet/switched-vlan/config/interface-mode:
  /interfaces/interface/ethernet/switched-vlan/config/access-vlan:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:

  ## State Paths ##
  /interfaces/interface/state/description:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/ethernet/switched-vlan/state/interface-mode:
  /interfaces/interface/ethernet/switched-vlan/state/access-vlan:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/state/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/prefix-length:
  /network-instances/network-instance/protocols/protocol/static-routes/static/state/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/state/next-hop:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
      delete: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* FFF
