# RT-1.111: BGP Autonomous System Confederations (RFC 5065)

## Summary

Validate BGP Autonomous System Confederations (RFC 5065) for IPv4 and IPv6
unicast.

## Testbed type

*   [TESTBED_DUT_ATE_4LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Setup

*   Connect DUT port 1, 2 and 3 to ATE port 1, 2 and 3 respectively

    ```text
                              Confederation 64500
                  +-----------------------------------------+
                  |                                         |
    +----------+  |  +-------------+          +----------+  |
    | ATE      |  |  |     DUT     |  port2   | ATE      |  |
    | port1    |=====| Member-AS   |==========| port2    |  |
    | AS 64510 |  |  |   64501     |  iBGP    | AS 64501 |  |
    +----------+  |  +-------------+          +----------+  |
     External     |        || port3                         |
     eBGP (DUT    |        || confederation-eBGP            |
     uses 64500)  |  +-------------+                        |
                  |  | ATE port3   |                        |
                  |  | Member-AS   |                        |
                  |  |   64502     |                        |
                  |  +-------------+                        |
                  +-----------------------------------------+
    ```

*   Configure IPv4/IPv6 addresses on the interfaces
*   Configure BGP on the DUT with local Member-AS `64501`, confederation
    identifier `64500`, and confederation member-AS `[64502]`
    *   /network-instances/network-instance/protocols/protocol/bgp/global/config/as
    *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/identifier
    *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/member-as
*   Establish IPv4 and IPv6 BGP sessions between:
    *   ATE port-1 (AS `64510`) and DUT port-1 (external eBGP, peering with
        DUT confederation identifier `64500`)
    *   ATE port-2 (AS `64501`) and DUT port-2 (iBGP within Member-AS `64501`)
    *   ATE port-3 (AS `64502`) and DUT port-3 (confederation-eBGP with
        Member-AS `64502`)
*   Enable an accept-all import and export policy on all BGP sessions

### Tests

*   RT-1.111.1: Verify confederation state and session establishment

    *   Check DUT BGP global and confederation state (`as` = `64501`,
        `identifier` = `64500`, `member-as` = `[64502]`)
        *   /network-instances/network-instance/protocols/protocol/bgp/global/state/as
        *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier
        *   /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as
    *   Verify all IPv4 and IPv6 BGP sessions reach `ESTABLISHED` state
        *   /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state

*   RT-1.111.2: Verify route propagation from confederation-eBGP to iBGP

    *   Advertise IPv4/IPv6 prefixes from ATE port-3 with
        `AS_CONFED_SEQUENCE [64502]` and `LOCAL_PREF 150`
    *   Check that ATE port-2 receives the prefixes with `AS_PATH` unchanged
        (`AS_CONFED_SEQUENCE [64502]`) and `LOCAL_PREF 150` preserved
        *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type
        *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member
        *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref
    *   Send traffic from ATE port-2 to ATE port-3 and verify 0% packet loss

*   RT-1.111.3: Verify route propagation from iBGP to confederation-eBGP

    *   Advertise IPv4/IPv6 prefixes from ATE port-2 with `LOCAL_PREF 200` and
        `MED 50` (both empty `AS_PATH` and `AS_CONFED_SEQUENCE [64503]`)
    *   Check that ATE port-3 receives the prefixes with `64501` prepended in
        `AS_CONFED_SEQUENCE` (`[64501]` and `[64501, 64503]`), and `LOCAL_PREF`
        and `MED` preserved across the Member-AS boundary
        *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref
        *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med
        *   /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop
    *   Send traffic from ATE port-3 to ATE port-2 and verify 0% packet loss

*   RT-1.111.4: Verify confederation segment stripping toward external eBGP

    *   Check that ATE port-1 receives the confederation-originated prefixes
        from ATE port-2 and port-3 with all `AS_CONFED_SEQUENCE` segments
        removed, `AS_SEQ [64500]` prepended, and no Member-AS numbers (`64501`,
        `64502`, `64503`) visible
    *   Send traffic from ATE port-1 to ATE port-2 and port-3 and verify 0%
        packet loss

*   RT-1.111.5: Verify external eBGP route propagation into the confederation

    *   Advertise IPv4/IPv6 prefixes from ATE port-1 with `AS_SEQ [64510]`
    *   Check that the prefixes are installed in the DUT `loc-rib`, received on
        ATE port-3 with `AS_CONFED_SEQUENCE [64501], AS_SEQ [64510]`, and
        received on ATE port-2 with `AS_SEQ [64510]`
        *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/prefix
        *   /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/prefix
    *   Send traffic from ATE port-2 and port-3 to ATE port-1 and verify 0%
        packet loss

*   RT-1.111.6: Verify AS loop detection for Member-AS and confederation ID

    *   Advertise prefixes from ATE port-3 containing `64501` in
        `AS_CONFED_SEQUENCE` and from ATE port-1 containing `64500` in `AS_SEQ`
    *   Verify the looped prefixes are rejected (not installed in `loc-rib` or
        advertised to peers) and all BGP sessions remain `ESTABLISHED`

*   RT-1.111.7: Verify handling of malformed confederation AS_PATHs

    *   Advertise prefixes with `AS_CONFED_SEQUENCE` from external ATE port-1
        and prefixes without a leading `AS_CONFED_SEQUENCE` from confederation
        ATE port-3
    *   Verify the malformed updates are discarded (treat-as-withdraw per
        RFC 7606) and all BGP sessions remain `ESTABLISHED`

### Cleanup

*   Stop ATE traffic and protocols, and remove BGP, routing-policy, and
    interface configurations added on the DUT

## Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "config": {
          "name": "DEFAULT"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "confederation": {
                    "config": {
                      "identifier": 64500,
                      "member-as": [
                        64502
                      ]
                    }
                  },
                  "config": {
                    "as": 64501
                  }
                }
              },
              "config": {
                "identifier": "openconfig-policy-types:BGP",
                "name": "BGP"
              },
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP"
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
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/identifier:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/config/member-as:
  /network-instances/network-instance/protocols/protocol/bgp/global/state/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/identifier:
  /network-instances/network-instance/protocols/protocol/bgp/global/confederation/state/member-as:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv4-unicast/loc-rib/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/afi-safis/afi-safi/ipv6-unicast/loc-rib/routes/route/state/prefix:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/type:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as-path/as-segment/state/member:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/local-pref:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/med:
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/state/next-hop:
rpcs:
  gnmi:
    gNMI.Set:
      replace: true
      delete: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

*   vRX
