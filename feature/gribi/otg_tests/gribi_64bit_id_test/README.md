# TE-15.2: gRIBI 64-bit NextHop and NextHopGroup ID support

## Summary

The gRIBI specification defines the NextHop index and the NextHopGroup ID as
`uint64` values ([gribi_aft.proto][gribi-aft], [gribi.proto][gribi-proto]).
This test validates that the DUT accepts, stores, reports and forwards with
NextHop indices and NextHopGroup IDs across the full `uint64` range, without
rejecting them and without truncating them to a narrower width.

A device that truncates IDs internally can still acknowledge a gRIBI `Modify`
with `FIB_PROGRAMMED`, because an `AFTResult` only carries the operation ID.
The test therefore does not rely on the acknowledgement alone. It validates
each programmed ID through the gRIBI `Get` RPC and gNMI AFT telemetry, and it
detects truncation in the forwarding path by programming two entries whose IDs
differ only above bit 31 and checking that traffic for each entry egresses on
its own port.

NextHop and NextHopGroup IDs are validated independently, because a device may
apply different limits to each object.

Every category of the test runs a control case with small IDs before the
64-bit case, using identical setup. A failing control case indicates a test
setup or environment problem rather than missing 64-bit support, and stops the
remaining cases of that category.

This test is scoped to the NextHop index, the NextHopGroup ID and the
references to them from NextHopGroup members and IPv4 entries.
`backup_next_hop_group` references and IPv6 entry references are out of scope.

The gRIBIgo compliance suite ([TE-15.1][te-15-1]) does not use a traffic
generator and cannot detect truncation in the forwarding path, which is why
this is a separate test.

[gribi-aft]: https://github.com/openconfig/gribi/blob/master/v1/proto/gribi_aft/gribi_aft.proto
[gribi-proto]: https://github.com/openconfig/gribi/blob/master/v1/proto/service/gribi.proto
[te-15-1]: https://github.com/openconfig/featureprofiles/blob/main/feature/gribi/otg_tests/gribigo_compliance_test/README.md

## Testbed type

*   [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

*   Topology:

    ```
    [ATE port-1] ---- [DUT port-1]
    [ATE port-2] ---- [DUT port-2]
    [ATE port-3] ---- [DUT port-3]
    ```

    ATE port-1 is the traffic source. ATE port-2 and ATE port-3 are traffic
    destinations. The fourth port pair of the testbed is not used.

*   Configure the DUT and ATE interfaces in the default network instance with
    the following IPv4 addresses:

    Port   | DUT address    | ATE address
    ------ | -------------- | --------------
    port-1 | 192.0.2.1/30   | 192.0.2.2/30
    port-2 | 192.0.2.5/30   | 192.0.2.6/30
    port-3 | 192.0.2.9/30   | 192.0.2.10/30

*   Establish a gRIBI client connection to the DUT with `SINGLE_PRIMARY`
    redundancy mode, `PRESERVE` persistence and `RIB_AND_FIB_ACK` requested.
    Make the client the elected primary.

*   Configure the following ATE IPv4 flows from ATE port-1, each sending 1000
    packets per second of 512-byte packets:

    Flow   | Source      | Destination
    ------ | ----------- | ----------------
    flow-A | 192.0.2.2   | 198.51.100.0
    flow-B | 192.0.2.2   | 198.51.100.1

*   The following ID values are used throughout the test:

    Name        | Value                | Purpose
    ----------- | -------------------- | ------------------------------------
    `ID_SMALL`  | `0xA` (10)           | ID for the object not under test
    `ID_1`      | `0x1`                | Control: smallest non-zero ID
    `ID_32_MAX` | `0xFFFFFFFF`         | Control: largest 32-bit ID
    `ID_32`     | `0x100000000`        | Smallest ID that needs more than 32 bits
    `ID_32_1`   | `0x100000001`        | Truncates to `0x1` when narrowed to 32 bits
    `ID_63_MAX` | `0x7FFFFFFFFFFFFFFF` | Largest signed 64-bit value
    `ID_63`     | `0x8000000000000000` | Most significant bit set
    `ID_64_MAX` | `0xFFFFFFFFFFFFFFFF` | Largest `uint64` value

*   Validation of an installed gRIBI chain (NH, NHG, IPv4 entry), referred to as
    "validate the chain" in the subtests below:

    1.  The gRIBI client receives `FIB_PROGRAMMED` for the NH, the NHG and the
        IPv4 entry operations, each checked individually. If the DUT only
        supports RIB acknowledgements, `RIB_PROGRAMMED` is accepted instead.
    2.  A gRIBI `Get` for the default network instance returns the NH with the
        exact programmed index, the NHG with the exact programmed ID and the
        exact programmed NH index as its member, and the IPv4 entry referencing
        the exact programmed NHG ID.
    3.  gNMI AFT telemetry contains
        `/network-instances/network-instance/afts/ipv4-unicast/ipv4-entry` for
        the prefix, and the NHG it references reports
        `/network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/programmed-id`
        equal to the programmed NHG ID.

*   Each subtest starts by flushing all gRIBI entries from the default network
    instance.

### TE-15.2.1 - NextHop index boundary values

This subtest validates that the DUT stores and reports the NextHop index without
narrowing it. The NHG uses `ID_SMALL`, so that only the NH index varies.

For each NH index in `ID_1`, `ID_32_MAX` (control cases, run first), `ID_32`,
`ID_32_1`, `ID_63_MAX`, `ID_63` and `ID_64_MAX`:

1.  Flush all gRIBI entries.
2.  Using one `Modify` request, ADD:
    *   NH with the index under test, next-hop IP address 192.0.2.6 (ATE
        port-2).
    *   NHG `ID_SMALL` with the NH as its only member, weight 1.
    *   IPv4 entry 198.51.100.0/32 referencing NHG `ID_SMALL`.
3.  Validate the chain.

Pass criteria: all checks of "validate the chain" pass for every index. If a
control case fails, the subtest fails as a setup error and the remaining indices
are not run.

### TE-15.2.2 - NextHopGroup ID boundary values

This subtest validates that the DUT stores and reports the NextHopGroup ID
without narrowing it, in the NHG key and in the IPv4 entry reference. The NH
uses `ID_SMALL`, so that only the NHG ID varies.

For each NHG ID in `ID_1`, `ID_32_MAX` (control cases, run first), `ID_32`,
`ID_32_1`, `ID_63_MAX`, `ID_63` and `ID_64_MAX`:

1.  Flush all gRIBI entries.
2.  Using one `Modify` request, ADD:
    *   NH `ID_SMALL`, next-hop IP address 192.0.2.6 (ATE port-2).
    *   NHG with the ID under test, with NH `ID_SMALL` as its only member,
        weight 1.
    *   IPv4 entry 198.51.100.0/32 referencing the NHG under test.
3.  Validate the chain.

Pass criteria: all checks of "validate the chain" pass for every ID. If a
control case fails, the subtest fails as a setup error and the remaining IDs are
not run.

### TE-15.2.3 - NextHop index truncation in the forwarding path

This subtest detects a DUT that accepts a 64-bit NH index but narrows it
internally. Two NHs are programmed whose indices are equal in the lower 32 bits.
On a DUT that truncates, the second NH replaces or aliases the first one, and
traffic for the first prefix egresses on the wrong port.

Run the control case first, then the 64-bit case:

Case    | First NH index | Second NH index
------- | -------------- | ---------------
control | `ID_1`         | `0x2`
64-bit  | `ID_1`         | `ID_32_1`

For each case:

1.  Flush all gRIBI entries.
2.  ADD NH with the first index, next-hop IP address 192.0.2.6 (ATE port-2), NHG
    `0xA` with that NH as its only member, and IPv4 entry 198.51.100.0/32
    referencing NHG `0xA`. Validate the chain.
3.  ADD NH with the second index, next-hop IP address 192.0.2.10 (ATE port-3),
    NHG `0xB` with that NH as its only member, and IPv4 entry 198.51.100.1/32
    referencing NHG `0xB`. Validate the chain.
4.  Issue a gRIBI `Get` and validate that both NHs are returned, and that the NH
    with the first index still has next-hop IP address 192.0.2.6.
5.  Start flow-A and flow-B for 30 seconds, then stop them.
6.  Validate that:
    *   flow-A is received on ATE port-2 with 0% loss and no flow-A packets are
        received on ATE port-3.
    *   flow-B is received on ATE port-3 with 0% loss and no flow-B packets are
        received on ATE port-2.

Pass criteria: all validations pass in both cases. If the control case fails,
the subtest fails as a setup error and the 64-bit case is not run.

### TE-15.2.4 - NextHopGroup ID truncation in the forwarding path

This subtest applies the method of TE-15.2.3 to the NextHopGroup ID.

Run the control case first, then the 64-bit case:

Case    | First NHG ID | Second NHG ID
------- | ------------ | -------------
control | `ID_1`       | `0x2`
64-bit  | `ID_1`       | `ID_32_1`

For each case:

1.  Flush all gRIBI entries.
2.  ADD NH `0xA`, next-hop IP address 192.0.2.6 (ATE port-2), NHG with the
    first ID and NH `0xA` as its only member, and IPv4 entry 198.51.100.0/32
    referencing that NHG. Validate the chain.
3.  ADD NH `0xB`, next-hop IP address 192.0.2.10 (ATE port-3), NHG with the
    second ID and NH `0xB` as its only member, and IPv4 entry 198.51.100.1/32
    referencing that NHG. Validate the chain.
4.  Issue a gRIBI `Get` and validate that both NHGs are returned, and that the
    NHG with the first ID still has NH `0xA` as its only member.
5.  Start flow-A and flow-B for 30 seconds, then stop them.
6.  Validate that:
    *   flow-A is received on ATE port-2 with 0% loss and no flow-A packets are
        received on ATE port-3.
    *   flow-B is received on ATE port-3 with 0% loss and no flow-B packets are
        received on ATE port-2.

Pass criteria: all validations pass in both cases. If the control case fails,
the subtest fails as a setup error and the 64-bit case is not run.

### TE-15.2.5 - NextHop REPLACE and DELETE with a 64-bit index

This subtest validates that the REPLACE and DELETE operations handle a 64-bit NH
index in the same way as ADD.

Run the control case with NH index `0x64` (100) first, then the 64-bit case with
NH index `ID_64_MAX`. For each case:

1.  Flush all gRIBI entries.
2.  ADD NH with the index under test, next-hop IP address 192.0.2.6 (ATE
    port-2), NHG `0xA` with that NH as its only member, and IPv4 entry
    198.51.100.0/32 referencing NHG `0xA`. Validate the chain.
3.  Start flow-A for 30 seconds, then stop it. Validate that flow-A is received
    on ATE port-2 with 0% loss.
4.  REPLACE the NH with the same index, changing its next-hop IP address to
    192.0.2.10 (ATE port-3). Validate `FIB_PROGRAMMED` for the operation and
    validate with gRIBI `Get` that the NH with the index under test has
    next-hop IP address 192.0.2.10.
5.  Start flow-A for 30 seconds, then stop it. Validate that flow-A is received
    on ATE port-3 with 0% loss and no flow-A packets are received on ATE
    port-2.
6.  DELETE the IPv4 entry, NHG `0xA` and the NH, in that order. Validate
    `FIB_PROGRAMMED` for each operation.
7.  Validate with gRIBI `Get` that no NH, NHG or IPv4 entry is returned for the
    default network instance, and with gNMI AFT telemetry that the IPv4 entry
    198.51.100.0/32 is no longer present.
8.  Start flow-A for 30 seconds, then stop it. Validate 100% loss for flow-A.

Pass criteria: all validations pass in both cases. If the control case fails,
the subtest fails as a setup error and the 64-bit case is not run.

### TE-15.2.6 - NextHopGroup REPLACE and DELETE with a 64-bit ID

This subtest applies the method of TE-15.2.5 to the NextHopGroup ID.

Run the control case with NHG ID `0x64` (100) first, then the 64-bit case with
NHG ID `ID_64_MAX`. For each case:

1.  Flush all gRIBI entries.
2.  ADD NH `0xA`, next-hop IP address 192.0.2.6 (ATE port-2), and NH `0xB`,
    next-hop IP address 192.0.2.10 (ATE port-3). ADD the NHG with the ID under
    test with NH `0xA` as its only member, and IPv4 entry 198.51.100.0/32
    referencing that NHG. Validate the chain.
3.  Start flow-A for 30 seconds, then stop it. Validate that flow-A is received
    on ATE port-2 with 0% loss.
4.  REPLACE the NHG with the same ID, changing its only member to NH `0xB`.
    Validate `FIB_PROGRAMMED` for the operation and validate with gRIBI `Get`
    that the NHG with the ID under test has NH `0xB` as its only member.
5.  Start flow-A for 30 seconds, then stop it. Validate that flow-A is received
    on ATE port-3 with 0% loss and no flow-A packets are received on ATE
    port-2.
6.  DELETE the IPv4 entry, the NHG and both NHs, in that order. Validate
    `FIB_PROGRAMMED` for each operation.
7.  Validate with gRIBI `Get` that no NH, NHG or IPv4 entry is returned for the
    default network instance, and with gNMI AFT telemetry that the IPv4 entry
    198.51.100.0/32 is no longer present.
8.  Start flow-A for 30 seconds, then stop it. Validate 100% loss for flow-A.

Pass criteria: all validations pass in both cases. If the control case fails,
the subtest fails as a setup error and the 64-bit case is not run.

### Cleanup

*   Flush all gRIBI entries from the default network instance.
*   Remove the interface configuration applied during test environment setup,
    restoring the DUT to its baseline configuration.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "DUT port-1 to ATE port-1",
          "enabled": true,
          "name": "port1",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port1",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.1",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.1"
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
          "description": "DUT port-2 to ATE port-2",
          "enabled": true,
          "name": "port2",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port2",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.5",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.5"
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
          "description": "DUT port-3 to ATE port-3",
          "enabled": true,
          "name": "port3",
          "type": "iana-if-type:ethernetCsmacd"
        },
        "name": "port3",
        "subinterfaces": {
          "subinterface": [
            {
              "config": {
                "index": 0
              },
              "index": 0,
              "ipv4": {
                "addresses": {
                  "address": [
                    {
                      "config": {
                        "ip": "192.0.2.9",
                        "prefix-length": 30
                      },
                      "ip": "192.0.2.9"
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

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Interface setup
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  # AFT telemetry validation
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/prefix:
  /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id:
  /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/programmed-id:
  # TODO: Validate the programmed NH index through AFT telemetry once
  # /network-instances/network-instance/afts/next-hops/next-hop/state/programmed-index
  # is available in the featureprofiles Ondatra telemetry library.

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Modify:
    gRIBI.Get:
    gRIBI.Flush:
```

## Required DUT platform

*   FFF
