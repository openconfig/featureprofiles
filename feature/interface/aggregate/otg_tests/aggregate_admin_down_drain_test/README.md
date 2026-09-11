# RT-5.17: Aggregate Interface (LAG) Drain via Admin Down

## Summary

* Verify that administratively disabling a LAG interface (e.g., Port-Channel) via gNMI brings down associated protocols and stops all traffic on the aggregate.

## Testbed type

* `atedut_4.testbed`

## Procedure

### Test environment setup

* Configure a 4-port ATE topology connected to the DUT (ATE Port-1, Port-2, Port-3, Port-4).
* Configure DUT Port-1 through Port-4 as part of a LAG interface (e.g., `Port-Channel1`) via `/interfaces/interface[name=<DUT Port-1>]/ethernet/config/aggregate-id` up to `<DUT Port-4>` with value `Port-Channel1`.
* Configure IPv4 (`192.0.2.1/30`) and IPv6 (`2001:db8::1/126`) addresses on the DUT LAG interface via `/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=0]/ipv4/addresses/address[ip=192.0.2.1]/config/ip` and `/interfaces/interface[name=Port-Channel1]/subinterfaces/subinterface[index=0]/ipv6/addresses/address[ip=2001:db8::1]/config/ip`.
* Configure matching IPv4 (`192.0.2.2/30`) and IPv6 (`2001:db8::2/126`) addresses on the ATE LAG interface.
* Establish IS-IS adjacencies (IPv4 and IPv6) over the LAG in the default network instance.
* Establish eBGP sessions (IPv4 and IPv6) with 4 neighbors (e.g., `192.0.2.2`, `192.0.2.6`, `2001:db8::2`, `2001:db8::6`) over the LAG in the default network instance.
* Advertise 10,000 IPv4 and 10,000 IPv6 prefixes from the ATE over BGP and IS-IS.

### RT-5.17.1 - Admin Down Drain - Disable

* Step 1 - Verify BGP `installed` prefixes reach `10000` for IPv4 and IPv6 via gNMI telemetry by checking `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/neighbors/neighbor[neighbor-address=<ATE IP>]/afi-safis/afi-safi[afi-safi-name=<IPV4_UNICAST|IPV6_UNICAST>]/state/prefixes/installed`.
* Step 2 - Send continuous bidirectional IPv4 and IPv6 traffic flows between ATE ports via the advertised prefixes at a non-congesting rate (e.g., 10% line rate).
* Step 3 - Verify 0% steady-state traffic loss (no arbitrary sleep wait time).
* Step 4 - Administratively disable the LAG interface by setting `/interfaces/interface[name=Port-Channel1]/config/enabled` to `false` using `gNMI.Set` with `REPLACE` option.

#### Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "Port-Channel1",
        "config": {
          "enabled": false
        }
      }
    ]
  }
}
```

* Step 5 - Verify via gNMI Get/Subscribe that `/interfaces/interface[name=Port-Channel1]/state/admin-status` is `DOWN` and `/interfaces/interface[name=Port-Channel1]/state/oper-status` is `DOWN`.
* Step 6 - Verify that the associated BGP sessions transition to `ACTIVE` or `IDLE` (not `ESTABLISHED`) via `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=BGP][name=BGP]/neighbors/neighbor[neighbor-address=<ATE IP>]/state/session-state`.
* Step 7 - Verify that the IS-IS adjacencies transition to `DOWN` via `/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=ISIS]/interfaces/interface[interface-id=Port-Channel1]/levels/level[level-number=2]/adjacencies/adjacency/state/adjacency-state`.
* Step 8 - Verify that traffic on the aggregate stops completely (loss is 100%).

### RT-5.17.2 - Admin Down Drain - Re-enable

* Step 1 - Re-enable the LAG interface by setting `/interfaces/interface[name=Port-Channel1]/config/enabled` to `true` using `gNMI.Set` with `REPLACE` option.
* Step 2 - Verify via gNMI Get/Subscribe that `/interfaces/interface[name=Port-Channel1]/state/admin-status` is `UP` and `/interfaces/interface[name=Port-Channel1]/state/oper-status` is `UP`.
* Step 3 - Verify that BGP sessions re-establish by checking `session-state` is `ESTABLISHED` at the paths from RT-5.17.1 Step 6.
* Step 4 - Verify that IS-IS adjacencies re-establish by checking `adjacency-state` is `UP` at the paths from RT-5.17.1 Step 7.
* Step 5 - Verify via gNMI that the BGP `installed` prefix count returns to `10000` at the paths from RT-5.17.1 Step 1.
* Step 6 - Once gNMI confirms convergence, verify that steady-state traffic loss returns to 0%.

### RT-5.17.3 - Negative Test - Admin Down on Individual Member Links

* Step 1 - Verify BGP `installed` prefixes are at `10000` for IPv4 and IPv6 via gNMI telemetry at the paths from RT-5.17.1 Step 1.
* Step 2 - Send continuous bidirectional IPv4 and IPv6 traffic flows between ATE ports via the advertised prefixes at 10% line rate.
* Step 3 - Administratively disable 2 out of the 4 individual member links (e.g., `<DUT Port-1>` and `<DUT Port-2>`) by setting their `/interfaces/interface[name=<DUT Port>]/config/enabled` to `false` using `gNMI.Set` with `REPLACE` option.
* Step 4 - Verify via gNMI that the LAG interface `/interfaces/interface[name=Port-Channel1]/state/oper-status` remains `UP`.
* Step 5 - Verify that BGP and IS-IS sessions remain `ESTABLISHED` and `UP` at their respective paths.
* Step 6 - Verify that traffic continues to flow (loss is 0%) over the remaining active member links.
* Step 7 - Re-enable the member links by setting their `/interfaces/interface[name=<DUT Port>]/config/enabled` to `true` and verify traffic remains at 0% loss.

### RT-5.17.4 - Negative Test - Admin Down on Already Disabled LAG

* Step 1 - Disable the LAG via `/interfaces/interface[name=Port-Channel1]/config/enabled = false`.
* Step 2 - Verify LAG `/interfaces/interface[name=Port-Channel1]/state/oper-status` is `DOWN` and traffic loss is 100%.
* Step 3 - Send another `gNMI.Set` with `/interfaces/interface[name=Port-Channel1]/config/enabled = false` to the already disabled LAG.
* Step 4 - Verify the DUT accepts the configuration without errors.
* Step 5 - Verify the LAG `/interfaces/interface[name=Port-Channel1]/state/oper-status` remains `DOWN` and traffic remains at 100% loss.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/state/admin-status:
  /interfaces/interface/state/oper-status:
  /interfaces/interface/aggregation/state/lag-type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* vRX
