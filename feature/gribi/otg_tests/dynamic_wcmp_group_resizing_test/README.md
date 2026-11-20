# TE-3.10: gRIBI WCMP Group Resizing and Weight Changes

## Summary

This test validates the switch's ability to handle dynamic changes to Weighted
Cost Multi-Path (WCMP) groups programmed via gRIBI. It simulates a controller
adjusting tunnel allocations. Specifically, it validates `gRIBI` `Modify`
operations (such as `REPLACE`) to adjust weights of NextHops within an active
NextHopGroup, as well as adding and removing NextHops dynamically. It ensures
that traffic distribution across WCMP members matches the configured weights,
changes are hitless for unaffected flows, and telemetry via
`/network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/`
correctly reflects the changes.

## Testbed type

* `TESTBED_DUT_ATE_4LINKS`

## Procedure

### Test environment setup

* Configure 4 interfaces on both the DUT and the ATE.
    * Port 1 (Ingress): Connect DUT port 1 to ATE port 1.
      * Configure IPv4: ATE `192.0.2.2/30`, DUT `192.0.2.1/30`
      * Configure IPv6: ATE `2001:db8:1::2/126`, DUT `2001:db8:1::1/126`
    * Port 2 (Egress 1): Connect DUT port 2 to ATE port 2.
      * Configure IPv4: ATE `192.0.2.6/30`, DUT `192.0.2.5/30`
      * Configure IPv6: ATE `2001:db8:2::2/126`, DUT `2001:db8:2::1/126`
    * Port 3 (Egress 2): Connect DUT port 3 to ATE port 3.
      * Configure IPv4: ATE `192.0.2.10/30`, DUT `192.0.2.9/30`
      * Configure IPv6: ATE `2001:db8:3::2/126`, DUT `2001:db8:3::1/126`
    * Port 4 (Egress 3): Connect DUT port 4 to ATE port 4.
      * Configure IPv4: ATE `192.0.2.14/30`, DUT `192.0.2.13/30`
      * Configure IPv6: ATE `2001:db8:4::2/126`, DUT `2001:db8:4::1/126`
* Establish a gRIBI client connection with the DUT with persistence `PRESERVE`, elect the client as leader, and flush all existing entries.

### TE-3.10.1 - Initial WCMP Group Configuration

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {
          "name": "DEFAULT"
        }
      }
    ]
  }
}
```

* **Step 1 - Push configuration to DUT using gnmi.Set with REPLACE option**
  * Push the Canonical OC configuration above.

* **Step 2 - Initial WCMP Group Configuration (gRIBI)**
  * Via gRIBI, install three IPv4 NextHops pointing to ATE ports 2, 3, and 4 (e.g., NextHop IDs `101`, `102`, `103`).
  * Via gRIBI, install three IPv6 NextHops pointing to ATE ports 2, 3, and 4 (e.g., NextHop IDs `201`, `202`, `203`).
  * Install an IPv4 NextHopGroup (e.g., ID `10`). Add NextHops `101` and `102` to Group `10`, each with a weight of `1`.
  * Install an IPv6 NextHopGroup (e.g., ID `20`). Add NextHops `201` and `202` to Group `20`, each with a weight of `1`.
  * Install a separate "background" IPv4 NextHopGroup (e.g., ID `11`) and add NextHop `103` to it.
  * Install a separate "background" IPv6 NextHopGroup (e.g., ID `21`) and add NextHop `203` to it.
  * Install 1,000 `IPv4Entry` routes (`10.0.0.0/16` split into `/32` routes) pointing to NextHopGroup `10`.
  * Install 1,000 `IPv6Entry` routes (`2001:db8:1000::/48` split into `/128` routes) pointing to NextHopGroup `20`.
  * Install a background `IPv4Entry` route (`198.51.100.0/24`) pointing to NextHopGroup `11`.
  * Install a background `IPv6Entry` route (`2001:db8:9999::/64`) pointing to NextHopGroup `21`.
  * Use `gNMI.Get` to validate that the AFT telemetry paths `/network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight` accurately reflect the configured NextHops and weights.

* **Step 3 - Initial Traffic Validation**
  * Generate 10,000 high-entropy UDP flows of IPv4 and IPv6 traffic from ATE port 1 destined to the 1,000 routes within `10.0.0.0/16` and `2001:db8:1000::/48`, varying source and destination UDP ports to ensure hashing.
  * Start the background traffic flows from ATE port 1 destined to `198.51.100.0/24` and `2001:db8:9999::/64`.
  * Validate that the WCMP traffic is balanced 50/50 (±2% tolerance) across ATE port 2 and ATE port 3.
  * Ensure ATE port 4 receives 100% of the background traffic and 0% of the WCMP traffic.
  * Validate 0% packet loss for all flows.

### TE-3.10.2 - Dynamic Weight Modification

* **Step 1 - Modify Weights**
  * Use a gRIBI `Modify` operation to update the weights of NextHops within the active NextHopGroup.
    * For Group `10` and `20`, change the weight of the first NextHop (`101`, `201`) to `3`, and keep the second NextHop (`102`, `202`) at weight `1`.
  * Use `gNMI.Subscribe` (ON_CHANGE) or `gNMI.Get` on `/network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight` to verify the DUT's telemetry reflects the new weights before proceeding.

* **Step 2 - Traffic Validation**
  * Validate that the WCMP traffic distribution shifts to 75% on ATE port 2 and 25% on ATE port 3 (±2% tolerance).
  * Validate that the background traffic on ATE port 4 experiences 0% packet loss during the WCMP group modification.

### TE-3.10.3 - Dynamic NextHop Addition (Resizing)

* **Step 1 - Add NextHop to Group**
  * Use a gRIBI `Modify` operation to add the third NextHop (`103` for IPv4, `203` for IPv6) to the active WCMP NextHopGroups (`10` and `20`), with a weight of `4`. 
  * *Note: The NextHops 103 and 203 are now shared between the WCMP groups (10, 20) and the background groups (11, 21).*
  * Use `gNMI.Subscribe` (ON_CHANGE) or `gNMI.Get` to verify the DUT's AFT telemetry reflects the addition of the new NextHops and their weights.

* **Step 2 - Traffic Validation**
  * Validate that the WCMP traffic distribution shifts to 37.5% on ATE port 2, 12.5% on ATE port 3, and 50% on ATE port 4 (±2% tolerance).
  * Validate that the background traffic destined to `198.51.100.0/24` and `2001:db8:9999::/64` remains unaffected (0% packet loss).

### TE-3.10.4 - Dynamic NextHop Removal

* **Step 1 - Remove NextHop from Group**
  * Use a gRIBI `Modify` operation to remove the first NextHop (`101` for IPv4, `201` for IPv6) from the active WCMP NextHopGroups (`10` and `20`).
  * Use `gNMI.Subscribe` (ON_CHANGE) or `gNMI.Get` to verify the DUT's AFT telemetry reflects the removal of the NextHops.

* **Step 2 - Traffic Validation**
  * Validate that the WCMP traffic distribution shifts to 20% on ATE port 3 and 80% on ATE port 4 (±2% tolerance).
  * Validate that the background traffic remains unaffected (0% packet loss).

### TE-3.10.5 - Negative Test Cases

* **Step 1 - Invalid NextHop Weight Modification**
  * Use a gRIBI `Modify` operation to attempt to update the weight of a NextHop that is **not** currently a member of the active NextHopGroup (e.g., attempt to modify weight of NextHop `101` in Group `10` after it was removed in TE-3.10.4).
  * Verify that the DUT rejects the operation and returns a gRIBI error.
  * Verify via gNMI that the group state remains unchanged.

* **Step 2 - Invalid NextHop Removal**
  * Use a gRIBI `Modify` operation to attempt to remove a NextHop from a NextHopGroup that it does not belong to.
  * Verify that the DUT rejects the operation and returns a gRIBI error.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Required DUT platform

* FFF
