# RT-7.12: BGP Drain using Route Policy

## Summary

This test validates the capability to drain a BGP peering session by applying an
outbound (export) routing policy to stop prefix advertisements. The objective is
to verify that applying a "deny all" policy to a BGP neighbor correctly
withdraws the advertised prefixes. Traffic to those prefixes from the peer's
perspective now uses alternative paths or stops. Removing
the policy restores the advertisements and traffic forwarding. The test
explicitly covers both IPv4 and IPv6 counterparts.

## Testbed type

* [`TESTBED_DUT_ATE_2LINKS`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

* Configure ATE port 1 and DUT port 1 to establish an eBGP session (IPv4 and
  IPv6).
* Configure ATE port 2 and DUT port 2 to establish another eBGP session (IPv4
  and IPv6).
* ATE port 1 announces 10,000 IPv4 prefixes and 10,000 IPv6 prefixes.
* DUT learns these prefixes and advertises them to ATE port 2.
* Validate BGP session establishment by checking
  `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state`
  equals `ESTABLISHED` for all peers.
* Wait for BGP routing tables to converge on both DUT and ATE.

### RT-7.12.1 - Verify baseline Prefix Advertisement and Traffic

* Step 1 - Start continuous IPv4 traffic from ATE port 2 to the 10,000 IPv4 prefixes
  announced by ATE port 1.
* Step 2 - Start continuous IPv6 traffic from ATE port 2 to the 10,000 IPv6 prefixes
  announced by ATE port 1.
* Step 3 - Validate that both IPv4 and IPv6 traffic is successfully forwarded
  with 0% packet loss (Tx == Rx). Keep the traffic streams running for subsequent subtests.
* Step 4 - Validate telemetry that the DUT has received prefixes from ATE port 1
  and transmitted them to ATE port 2:
  * Check `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received`
    equals 10,000 for IPv4 and IPv6 from ATE port 1.
  * Check `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent`
    equals 10,000 for IPv4 and IPv6 to ATE port 2.

### RT-7.12.2 - Apply "Deny All" Outbound Route Policy

*Note: This subtest builds on the state from RT-7.12.1. Traffic streams should remain active.*

* Step 1 - Generate DUT configuration to create a "deny all" route policy and
  apply it as an export policy to the eBGP neighbor connected to ATE port 2.

#### Canonical OC

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "DENY_ALL"
          },
          "name": "DENY_ALL",
          "statements": {
            "statement": [
              {
                "actions": {
                  "config": {
                    "policy-result": "REJECT_ROUTE"
                  }
                },
                "config": {
                  "name": "deny-all-statement"
                },
                "name": "deny-all-statement"
              }
            ]
          }
        }
      ]
    }
  },
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
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.6",
                      "config": {
                        "neighbor-address": "192.0.2.6"
                      },
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "DENY_ALL"
                                ]
                              }
                            }
                          },
                          {
                            "afi-safi-name": "openconfig-bgp-types:IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "export-policy": [
                                  "DENY_ALL"
                                ]
                              }
                            }
                          }
                        ]
                      }
                    }
                  ]
                }
              },
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "identifier": "BGP",
              "name": "BGP"
            }
          ]
        }
      }
    ]
  }
}
```

* Step 2 - Push configuration to DUT using gNMI Set with REPLACE option on the

  `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy`
  paths for the neighbor facing ATE port 2.
* Step 3 - Validate that the DUT withdraws the prefixes from the peer:
  * Check `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent`
    equals 0 for both IPv4 and IPv6 to ATE port 2.
  * Verify if the ATE captures BGP UPDATE (Withdraw) messages for the drained prefixes.
* Step 4 - Validate that both IPv4 and IPv6 continuous traffic streams experience
  a drop in Rx rate to 0 (100% loss) at ATE port 2 following the policy application.

### RT-7.12.3 - Remove Policy and Restore Prefix Advertisement

*Note: This subtest builds on the state from RT-7.12.2. Traffic streams should remain active.*

* Step 1 - Remove the export policy from the eBGP neighbor configuration by
  using gNMI Delete on the path:
  `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy`
  for both IPv4 and IPv6.
* Step 2 - Wait for BGP routing to converge.
* Step 3 - Validate telemetry that prefixes are re-advertised:

  * Check `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent`
    equals 10,000 for both IPv4 and IPv6 to ATE port 2.
* Step 4 - Validate that both IPv4 and IPv6 continuous traffic streams recover,
  and are successfully forwarded with 0% packet loss (Tx == Rx).

### RT-7.12.4 - Apply AFI/SAFI Specific Policy

*Note: This subtest builds on the state from RT-7.12.3, assuming prefixes are currently advertised. Traffic streams should remain active.*

* Step 1 - Apply the "DENY_ALL" export policy only to the `IPV4_UNICAST` afi-safi on the eBGP neighbor connected to ATE port 2 using gNMI Set.
* Step 2 - Wait for BGP routing to converge.
* Step 3 - Validate that the DUT withdraws ONLY IPv4 prefixes from the peer:
  * Check `/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent`
    equals 0 for IPv4 and 10,000 for IPv6 to ATE port 2.
* Step 4 - Validate that the IPv4 continuous traffic stream drops to 0 Rx rate (100% loss), while the IPv6 traffic stream continues with 0% packet loss (Tx == Rx).
* Step 5 - Remove the export policy using gNMI Delete to restore the environment for the next test.

### RT-7.12.5 - Application of Non-Existent Policy

*Note: This subtest builds on the baseline established in RT-7.12.1 or restored in RT-7.12.4.*

* Step 1 - Attempt to apply an export policy named "NON_EXISTENT_POLICY" (which has not been defined in `/routing-policy/policy-definitions`) to the eBGP neighbor connected to ATE port 2 using gNMI Set.
* Step 2 - Verify that the gNMI Set operation fails with an appropriate error (e.g., Invalid Argument).
* Step 3 - Validate that the BGP session remains `ESTABLISHED` and prefix advertisements (`prefixes/sent` equals 10,000) are unaffected.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # Routing policy definitions
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  # BGP export policy application
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/apply-policy/config/export-policy:
  # Telemetry checks
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/received:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent:

rpcs:
  gnmi:
    gNMI.Set:
      delete: true
    gNMI.Get:
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

* vRX
