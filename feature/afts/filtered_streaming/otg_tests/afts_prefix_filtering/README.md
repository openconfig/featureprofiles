## **Testbed type**

[ATE-DUT-1](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_1.testbed)
\- A single link between an ATE and a DUT.

## **Procedure**

### **Test environment setup**

*   The `DUT` and `ATE` are connected via a single link.

*   Basic interface configuration is applied to the `DUT` and `ATE`.

*   The DUT is pre-configured with a set of static routes or BGP learned routes
    to populate the AFT. This should include a mix of IPv4 and IPv6 prefixes.

*   The DUT is pre-configured with several routing policies under
    `/routing-policy/policy-definitions/`:

    *   `POLICY-MATCH-ALL`: Matches all routes.

    *   `POLICY-MIXED`: Matches a specific list of IPv4 and IPv6 prefixes.

    *   `POLICY-PREFIX-SET-A`: Matches a specific set of IPv4 prefixes.

    *   `POLICY-PREFIX-SET-B`: Matches a specific set of IPv6 prefixes.

    *   `POLICY-SUBNET`: Provides a subnet for prefixes to match against.

### **Test Case Iteration**

The core test cases (**AFT-TBD.1.1**, …) should be iterated, substituting the
specific policy name in the gNMI subscription. Each iteration should use the
appropriate set of AFT entries and policy definitions to validate the filtering
logic for that policy type, as created at test setup.

For example, when testing with `POLICY-PREFIX-SET-B`, ensure IPv6 prefixes are
in the AFT and the subscription request in **AFT-TBD.1.1** uses
`POLICY-PREFIX-SET-B` as the policy-name key.

### **AFT-TBD.1.1 \- Validation of Subscription with Prefix-Set Policy**

Configure Routing Policy & Prefixes

*   Ensure `DUT` has `POLICY-PREFIX-SET-A` configured to match prefixes
    "`198.51.100.0/24`" and "`203.0.113.0/28`".

*   Ensure DUT's AFT contains entries for "`198.51.100.0/24`",
    "`203.0.113.0/28`", and at least one non-matching prefix (e.g.,
    "`192.0.2.0/24`").

*   Configure
    `/network-instances/network-instance/afts/global-filter/config/policy-name`
    to be `POLICY-PREFIX-SET-A`

**Subscribe** (with a long-lived subscribe request).

*   The test client establishes a gNMI subscription to the DUT.

*   The subscription should target the paths for **IPv4** / **IPv6** entries,
    **next-hop-groups**, and **next-hops**, as well as the new leaf inside the
    `global-filter` container.

```protobuf
subscribe: {
  prefix: {
    target: "target-device"
    origin: "openconfig"
    path: {
      elem: { name: "network-instances" }
      elem: { name: "network-instance" key: { key: "name" value: "DEFAULT" } }
      elem: { name: "afts" }
    }
  }
 subscription: {
    path: {
      elem: { name: "global-filter" }
      elem: { name: "state" }
      elem: { name: "policy-name" }
    }
    mode: ON_CHANGE
  }
  subscription: {
    path: {
      elem: { name: "ipv4-unicast" }
      elem: { name: "ipv4-entry" }
    }
    mode: ON_CHANGE
  }
  subscription: {
    path: {
      elem: { name: "ipv6-unicast" }
      elem: { name: "ipv6-entry" }
    }
    mode: ON_CHANGE
  }
  subscription: {
    path: {
      elem: { name: "next-hop-groups" }
      elem: { name: "next-hop-group" }
    }
    mode: ON_CHANGE
  }
  subscription: {
    path: {
      elem: { name: "next-hops" }
      elem: { name: "next-hop" }
    }
    mode: ON_CHANGE
  }
  mode: STREAM
  encoding: PROTO
}
```

Validate Initial Synced Data

*   Client waits for the initial set of gNMI Notifications, verifying `SYNC` is
    received.

*   Verify that Notifications are received **ONLY** for the **prefixes**
    *matching* `POLICY-PREFIX-SET-A` ("`198.51.100.0/24`", "`203.0.113.0/28`"),
    as well as any *necessary recursive* prefixes.

*   Verify that all **next-hop-groups** and **next-hops** are received and
    resolved from prefix to the expected next-hop (possibly recursively).

*   Verify that all **next-hop-groups** are referenced by some **prefix**
    received.

*   Verify that all **next-hops** are referenced by some **next-hop-group**.

*   Verify all notifications have the `atomic` flag set.

Validate Dynamic Updates

*   Add a new **prefix** to the DUT that matches `POLICY-PREFIX-SET-A`. This
    prefix should have a forwarding entry not yet present in AFT.

*   Verify the receipt of an update for the **prefix**.

*   Remove the **prefix** from the DUT that matches `POLICY-PREFIX-SET-A` that
    we added.

*   Verify the receipt of a delete for the **prefix**, as well as its
    **next-hop-groups** / **next-hops**.

*   Add a new prefix to the DUT that does **NOT** match the routing policy.

*   Verify no gNMI Updates are received.

Remove the filtered view

*   Delete configuration for the `global-filter`

*   Verify that we receive Delete for
    `/network-instances/network-instance/afts/global-filter/state/policy-name`

*   Verify some prefixes outside the policy are received, which were not present
    before.

### **AFT-TBD.1.2 \- Validation with Non-Existent Policy**

*   Attempt to configure the AFT global filter policy-name with a policy that is
    not configured on the device, `POLICY-DOES-NOT-YET-EXIST`.
    *   Verify configuration error `FAILED_PRECONDITION`
*   Apply a configuration to the DUT defining "`POLICY-DOES-NOT-YET-EXIST`" with
    specific match criteria (e.g., matching prefix "`192.168.100.0/24`").

*   Again, attempt to configure the device with the policy. No error should be
    returned.

*   Subscribe to AFT as in **AFT-TBD.1.1** and verify that all expected leaves
    are present.

### **AFT-TBD.1.3 \- Validation Policy Deletion**

*   Configure the device to filter AFT using a policy present on the device,
    e.g. `POLICY-PREFIX-SET-A`.

*   Form a gNMI Subscribe session for the data, waiting for a sync response.

*   Delete `POLICY-PREFIX-SET-A` from the DUT's configuration.

    *   Verify an error of `FAILED_PRECONDITION` error is returned, as AFT
        configuration needs to be deleted, as well.

*   Delete both the global filter and the policy

    *   Verify no errors are returned

*   Verify some new prefix outside of the configured policy is returned

*   Configure `POLICY-PREFIX-SET-A` along with the global filter

*   Verify the data as in **AFT-TBD.1.1**

### **AFT-TBD.1.4 \- Validation After Device Reboot**

Establish Subscription

*   Subscribe successfully to a filtered AFT view as in AFT-TBD.1.1, after
    configuring a filtered view for `POLICY-PREFIX-SET-A`.

*   Verify that the initial set of matching AFT entries is received.

Reboot DUT

*   While the subscription is active, issue a reboot command to the DUT (via
    `gNOI.System/Reboot`).

*   The test client should observe the gNMI stream terminating.

Await DUT Readiness

*   Wait for the DUT to become reachable and gNMI to be available after reboot.

Re-establish Subscription

*   Attempt to establish the same gNMI subscription as in the first step.

*   Verify that the subscription is successfully established.

    *   We should only ever see an error that the endpoint is not responding, no
        internal errors should be returned

*   Verify that the DUT streams the correct set of filtered AFT entries matching
    `POLICY-PREFIX-SET-A`.

### **AFT-TBD.1.5 \- Scale Test**

*   Populate AFT with **X** IPv4 routes and **Y** IPv6 routes by configuring the
    routes and advertising them from the connected IXIA.

*   Configure three routing policies, which match (1%, 5%, and 20%) of routes.

*   Subscribe to AFT data with two telemetry collectors and wait until all
    expected leaves are received in both instances.

*   Measure the time taken for the initial synchronization for each policy
    scenario.

*   Validate correct operation and confirm in all cases, we get a sync within
    **K** minutes.

### **AFT-TBD.1.6 \- Per Network-Instance Filtering with Multiple Collectors**

To validate that AFT filters are applied independently per **network instance**
and that multiple collectors can subscribe to different network instances with
their respective filters.

**Setup**

*   Configure at least two network instances on the DUT:

    *   `DEFAULT`

    *   `VRF-A`

*   Populate the routing tables across both network instances with distinct sets
    of static or BGP learned routes. Ensure some prefix overlap between the
    instances to verify filter independence.

    *   `DEFAULT`:

    *   **`198.51.100.0/24`**,

    *   `203.0.113.0/28`,

    *   `192.0.2.0/24`

    *   `VRF-A`:

    *   **`198.51.100.0/24`**,

    *   `10.0.0.0/8`,

    *   `203.0.113.128/28`

*   Ensure the following routing policies are configured:

    *   `POLICY-A`: Matches `198.51.100.0/24` and `203.0.113.0/28`.

    *   `POLICY-B`: Matches `10.0.0.0/8`.

    *   `POLICY-MATCH-ALL`: Matches **all** routes.

*   Configure the AFT filters for each network instance:

    *   For `DEFAULT` Network Instance: **Set**
        `/network-instances/network-instance[name=DEFAULT]/afts/global-filter/config/policy-name`
        to `POLICY-A`.

    *   For `VRF-A` Network Instance: **Set**
        `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/policy-name`
        to `POLICY-B`.

**Validation**

*   Collector 1: Establishes a gNMI subscription to the AFT paths (ipv4-unicast,
    next-hop-groups, etc.) within the `DEFAULT` network instance.

*   Collector 2: Establishes a gNMI subscription to the AFT paths within the
    `VRF-A` network instance.

*   Collector 1: Verify it receives a `SYNC` and only the AFT entries matching
    `POLICY-A` from the `DEFAULT` instance (i.e., `198.51.100.0/24`,
    `203.0.113.0/28`) and their associated next-hops/groups.

*   Collector 2: Verify it receives a `SYNC` and only the AFT entries matching
    `POLICY-B` from the `VRF-A` instance (i.e., `10.0.0.0/8`) and its associated
    next-hops/groups.

*   Add a route `10.1.1.0/24` to the `DEFAULT` network instance. Verify neither
    collector receives an update.

*   Add a route `203.0.113.128/28` to the DEFAULT network instance. Verify
    neither collector receives an update.

*   Add a route `198.51.100.1/32` (matching `POLICY-A` via longest prefix match)
    to `DEFAULT`. Verify **Collector 1** receives the update, and **Collector
    2** does not.

*   Add a route `10.2.2.0/24` (matching `POLICY-B`) to `VRF-A`. Verify
    **Collector 2** receives the update, and **Collector 1** does not.

*   Change the filter for `VRF-A`: Set
    `/network-instances/network-instance[name=VRF-A]/afts/global-filter/config/policy-name`
    to `POLICY-MATCH-ALL`.

*   **Collector 1**: Verify its received AFT set for `DEFAULT` remains
    unchanged; the connection should remain stable throughout.

*   **Collector 2**: Expect the stream to be terminated. Upon resubscription,
    verify **Collector 2** now receives all AFT entries from the `VRF-A`
    instance (`198.51.100.0/24`, `10.0.0.0/8`, `203.0.113.128/28`, and the
    dynamically added `10.2.2.0/24`)

## **OpenConfig Path and RPC Coverage**

```
paths:
  # Paths for the new filter mechanism
  /network-instances/network-instance/afts/global-filter/config/policy-name:
  /network-instances/network-instance/afts/global-filter/state/policy-name:

  # Standard AFT paths being filtered
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry:
  /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group:
  /network-instances/network-instance/afts/next-hops/next-hop:

  # Paths for configuring policies and prefixes (used in setup)
  /routing-policy/policy-definitions/policy-definition:
  /routing-policy/defined-sets/prefix-sets/prefix-set:
  /network-instances/network-instance/protocols/protocol/static-routes/static:
  /network-instances/network-instance/protocols/protocol/bgp:

rpcs:
  gnmi:
    gNMI.Subscribe:
      STREAM: true
      ON_CHANGE: true
    gNMI.Set:
      REPLACE: true
      UPDATE: true
  gnoi:
    System:
      Reboot: true
```

## **Required DUT platform**

FFF (Fixed Form Factor) or MFF (Modular Form Factor).
