# RT-1.12: BGP always compare MED

## Summary

Verify BGP `always-compare-med` route selection across different neighboring
autonomous systems and validate BGP routing-policy MED operations (`SET`,
`ADD`, `SUBTRACT`, underflow saturation at `0`, and MED policy removal) for
both IPv4 and IPv6 unicast traffic.

## Testbed type

*   [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

Connect 3 links between the DUT and ATE (`DUT:port-1` to `ATE:port-1`,
`DUT:port-2` to `ATE:port-2`, and `DUT:port-3` to `ATE:port-3`) and configure
dual-stack IPv4 and IPv6 addresses:

*   Link 1 (iBGP, `BGP-PEER-GROUP1`, AS `65501`):
    *   `DUT:port-1`: `192.0.2.1/30`, `2001:db8::192:0:2:1/126`
    *   `ATE:port-1`: `192.0.2.2/30`, `2001:db8::192:0:2:2/126`
*   Link 2 (eBGP #1, `BGP-PEER-GROUP2`, ATE AS `65502`):
    *   `DUT:port-2`: `192.0.2.5/30`, `2001:db8::192:0:2:5/126`
    *   `ATE:port-2`: `192.0.2.6/30`, `2001:db8::192:0:2:6/126`
*   Link 3 (eBGP #2, `BGP-PEER-GROUP3`, ATE AS `65503`):
    *   `DUT:port-3`: `192.0.2.9/30`, `2001:db8::192:0:2:9/126`
    *   `ATE:port-3`: `192.0.2.10/30`, `2001:db8::192:0:2:10/126`

Configure the DUT BGP process in the `DEFAULT` network instance with local AS
`65501` and router ID `192.0.2.9`. Establish separate IPv4 (`IPV4_UNICAST`) and
IPv6 (`IPV6_UNICAST`) BGP sessions on each link:

*   `DUT:port-1` <-> `ATE:port-1`: iBGP (AS `65501`). Enable learned IPv4 and
    IPv6 unicast prefix inspection on `ATE:port-1` so the ATE can verify the
    winning route's MED value propagated by the DUT.
*   `DUT:port-2` <-> `ATE:port-2`: eBGP (AS `65501` <-> AS `65502`).
*   `DUT:port-3` <-> `ATE:port-3`: eBGP (AS `65501` <-> AS `65503`).

Both `ATE:port-2` and `ATE:port-3` advertise the same 254 IPv4 prefixes
(`203.0.113.1/32` to `203.0.113.254/32`) and 254 IPv6 prefixes
(`2001:db8:203:113::1/128` to `2001:db8:203:113::fe/128`) with identical
AS-path length (`1`) and origin (`IGP`). Note that `ATE:port-2` has the lower
neighbor IP address (`192.0.2.6` < `192.0.2.10` and `2001:db8::192:0:2:6` <
`2001:db8::192:0:2:10`), so whenever MED is ignored or tied, `ATE:port-2` wins
tie-breaking deterministically (when resetting sessions to verify tie-breaking,
bring up `ATE:port-2` before `ATE:port-3` to avoid oldest-route ambiguity).

Configure two IPv4 and two IPv6 traffic flows from `ATE:port-1` to the 254
advertised destination prefixes (100 pps, 512-byte frames):

*   `flowPort1toPort2` (IPv4) and `flowPort1toPort2V6` (IPv6): TX `ATE:port-1`,
    expected RX `ATE:port-2`.
*   `flowPort1toPort3` (IPv4) and `flowPort1toPort3V6` (IPv6): TX `ATE:port-1`,
    expected RX `ATE:port-3`.

Verify all DUT ports report `UP`, all IPv4 and IPv6 BGP sessions reach
`ESTABLISHED` on both DUT and ATE, and DUT neighbors report `ROUTE_REFRESH` and
`MPBGP` in `state/supported-capabilities`. Wait for ARP (IPv4) and ND (IPv6)
resolution before sending traffic.

### RT-1.12.1: Global always-compare-med enabled with MED SET

Validate that enabling global `always-compare-med` causes the DUT to compare
MED values assigned via `set-med-action: SET` across different eBGP autonomous
systems (`65502` and `65503`) for both IPv4 and IPv6.

1.  On `ATE:port-2` and `ATE:port-3`, advertise the 254 IPv4 and 254 IPv6
    prefixes with no initial MED (default `0`).
2.  On the DUT, configure routing policies `SET-MED-100` (`set-med: 100`,
    `set-med-action: SET`, `policy-result: ACCEPT_ROUTE`), `SET-MED-50`
    (`set-med: 50`, `set-med-action: SET`, `policy-result: ACCEPT_ROUTE`), and
    `ALLOW` (`policy-result: ACCEPT_ROUTE`).
3.  Attach `SET-MED-100` as the import policy on `BGP-PEER-GROUP2`
    (`ATE:port-2`) and `SET-MED-50` as the import policy on `BGP-PEER-GROUP3`
    (`ATE:port-3`) for both IPv4 and IPv6.
4.  Enable global `always-compare-med`: set
    `/network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/config/always-compare-med`
    to `true`.
5.  Verify telemetry on the DUT:
    *   `/network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/state/always-compare-med`
        reports `true`.
    *   `ATE:port-1` neighbor `prefixes/state/sent` is `254` and
        `prefixes/state/installed` is `0` for both IPv4 and IPv6.
6.  Verify control plane on `ATE:port-1`: all 254 IPv4 and 254 IPv6 learned
    prefixes carry `MED == 50` (selected from `ATE:port-3`).
7.  Send IPv4 and IPv6 traffic from `ATE:port-1` and validate forwarding:
    *   `flowPort1toPort3` and `flowPort1toPort3V6` (`ATE:port-3`): 0% loss
        (within 2% tolerance).
    *   `flowPort1toPort2` and `flowPort1toPort2V6` (`ATE:port-2`): 100% loss
        (within 2% tolerance).

#### Canonical OC

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
                  "afi-safis": {
                    "afi-safi": [
                      {
                        "afi-safi-name": "IPV4_UNICAST",
                        "config": {
                          "afi-safi-name": "IPV4_UNICAST",
                          "enabled": true
                        }
                      },
                      {
                        "afi-safi-name": "IPV6_UNICAST",
                        "config": {
                          "afi-safi-name": "IPV6_UNICAST",
                          "enabled": true
                        }
                      }
                    ]
                  },
                  "config": {
                    "as": 65501,
                    "router-id": "192.0.2.9"
                  },
                  "route-selection-options": {
                    "config": {
                      "always-compare-med": true
                    }
                  }
                },
                "peer-groups": {
                  "peer-group": [
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "import-policy": [
                                  "SET-MED-100"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST",
                              "enabled": true
                            }
                          },
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "import-policy": [
                                  "SET-MED-100"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "peer-as": 65502,
                        "peer-group-name": "BGP-PEER-GROUP2"
                      },
                      "peer-group-name": "BGP-PEER-GROUP2"
                    },
                    {
                      "afi-safis": {
                        "afi-safi": [
                          {
                            "afi-safi-name": "IPV4_UNICAST",
                            "apply-policy": {
                              "config": {
                                "import-policy": [
                                  "SET-MED-50"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV4_UNICAST",
                              "enabled": true
                            }
                          },
                          {
                            "afi-safi-name": "IPV6_UNICAST",
                            "apply-policy": {
                              "config": {
                                "import-policy": [
                                  "SET-MED-50"
                                ]
                              }
                            },
                            "config": {
                              "afi-safi-name": "IPV6_UNICAST",
                              "enabled": true
                            }
                          }
                        ]
                      },
                      "config": {
                        "peer-as": 65503,
                        "peer-group-name": "BGP-PEER-GROUP3"
                      },
                      "peer-group-name": "BGP-PEER-GROUP3"
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
  },
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "SET-MED-100"
          },
          "name": "SET-MED-100",
          "statements": {
            "statement": [
              {
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-med": 100,
                      "set-med-action": "SET"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "20"
                },
                "name": "20"
              }
            ]
          }
        },
        {
          "config": {
            "name": "SET-MED-50"
          },
          "name": "SET-MED-50",
          "statements": {
            "statement": [
              {
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-med": 50,
                      "set-med-action": "SET"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "20"
                },
                "name": "20"
              }
            ]
          }
        }
      ]
    }
  }
}
```

### RT-1.12.2: Global always-compare-med disabled vs enabled (negative check)

Validate that disabling global `always-compare-med` stops cross-AS MED
comparison between AS `65502` and AS `65503`, and that re-enabling it restores
lowest-MED selection.

1.  Keep `SET-MED-100` attached to `BGP-PEER-GROUP2` (`ATE:port-2`) and
    `SET-MED-50` attached to `BGP-PEER-GROUP3` (`ATE:port-3`).
2.  Set
    `/network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/config/always-compare-med`
    to `false`. Re-establish `ATE:port-2` before `ATE:port-3` so tie-breaking
    deterministically selects `ATE:port-2`.
3.  Verify telemetry:
    `/network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/state/always-compare-med`
    reports `false`.
4.  Verify control plane on `ATE:port-1`: all 254 IPv4 and 254 IPv6 learned
    prefixes carry `MED == 100` (`ATE:port-2` wins despite higher MED because
    neighbor ASes differ).
5.  Send IPv4 and IPv6 traffic and validate forwarding:
    *   `flowPort1toPort2` and `flowPort1toPort2V6` (`ATE:port-2`): 0% loss.
    *   `flowPort1toPort3` and `flowPort1toPort3V6` (`ATE:port-3`): 100% loss.
6.  Re-enable global `always-compare-med` (`true`), verify
    `state/always-compare-med` reports `true`, verify `ATE:port-1` receives
    `MED == 50`, and confirm traffic shifts back to `ATE:port-3` (0% loss on
    `flowPort1toPort3` / `flowPort1toPort3V6`, 100% loss on `flowPort1toPort2` /
    `flowPort1toPort2V6`).

### RT-1.12.3: MED ADD operation (`set-med-action: ADD`)

Validate that `set-med-action: ADD` increments a non-zero incoming MED attribute
rather than overwriting it, and that `always-compare-med` selects the lowest
accumulated MED across different ASes.

1.  Update `ATE:port-2` and `ATE:port-3` to advertise all 254 IPv4 and 254 IPv6
    prefixes with an initial incoming `MED = 100`.
2.  Keep global `always-compare-med` enabled (`true`).
3.  Configure import routing policies on the DUT:
    *   `ADD-MED-50`: `set-med: 50`, `set-med-action: ADD`,
        `policy-result: ACCEPT_ROUTE` (expected resulting MED:
        `100 + 50 = 150`).
    *   `ADD-MED-20`: `set-med: 20`, `set-med-action: ADD`,
        `policy-result: ACCEPT_ROUTE` (expected resulting MED:
        `100 + 20 = 120`).
4.  Attach `ADD-MED-50` to `BGP-PEER-GROUP2` (`ATE:port-2`) and `ADD-MED-20` to
    `BGP-PEER-GROUP3` (`ATE:port-3`) for both IPv4 and IPv6, and refresh the
    eBGP sessions if needed.
5.  Verify control plane on `ATE:port-1`: all 254 IPv4 and 254 IPv6 learned
    prefixes carry `MED == 120` (confirming addition on top of incoming MED
    `100`, not replacement with `20`).
6.  Send IPv4 and IPv6 traffic and validate forwarding:
    *   `flowPort1toPort3` and `flowPort1toPort3V6` (`ATE:port-3`): 0% loss.
    *   `flowPort1toPort2` and `flowPort1toPort2V6` (`ATE:port-2`): 100% loss.

#### Canonical OC

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "ADD-MED-50"
          },
          "name": "ADD-MED-50",
          "statements": {
            "statement": [
              {
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-med": 50,
                      "set-med-action": "ADD"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "20"
                },
                "name": "20"
              }
            ]
          }
        },
        {
          "config": {
            "name": "ADD-MED-20"
          },
          "name": "ADD-MED-20",
          "statements": {
            "statement": [
              {
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-med": 20,
                      "set-med-action": "ADD"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "20"
                },
                "name": "20"
              }
            ]
          }
        }
      ]
    }
  }
}
```

### RT-1.12.4: MED SUBTRACT operation (`set-med-action: SUBTRACT`) and underflow floor

Validate that `set-med-action: SUBTRACT` decrements a non-zero incoming MED
attribute and saturates at `0` (without unsigned 32-bit underflow wrap-around)
when the subtracted value exceeds the incoming MED.

1.  Keep `ATE:port-2` and `ATE:port-3` advertising all 254 IPv4 and 254 IPv6
    prefixes with incoming `MED = 100`, and keep global `always-compare-med`
    enabled (`true`).
2.  **Part A (standard subtraction):**
    *   Configure import policies `SUB-MED-20` (`set-med: 20`,
        `set-med-action: SUBTRACT`, resulting MED `100 - 20 = 80`) and
        `SUB-MED-60` (`set-med: 60`, `set-med-action: SUBTRACT`, resulting MED
        `100 - 60 = 40`).
    *   Attach `SUB-MED-20` to `BGP-PEER-GROUP2` (`ATE:port-2`) and `SUB-MED-60`
        to `BGP-PEER-GROUP3` (`ATE:port-3`) for both IPv4 and IPv6, and refresh
        eBGP sessions if needed.
    *   Verify control plane on `ATE:port-1`: all 254 IPv4 and 254 IPv6 learned
        prefixes carry `MED == 40`.
    *   Send IPv4 and IPv6 traffic and validate 0% loss on `ATE:port-3`
        (`flowPort1toPort3` / `flowPort1toPort3V6`) and 100% loss on
        `ATE:port-2` (`flowPort1toPort2` / `flowPort1toPort2V6`).
3.  **Part B (underflow saturation at `0`):**
    *   Configure import policy `SUB-MED-150` (`set-med: 150`,
        `set-med-action: SUBTRACT`, resulting MED `max(0, 100 - 150) = 0`).
    *   Keep `SUB-MED-20` on `BGP-PEER-GROUP2` (`MED = 80`) and attach
        `SUB-MED-150` to `BGP-PEER-GROUP3` (`MED = 0`) for both IPv4 and IPv6.
    *   Verify control plane on `ATE:port-1`: all 254 IPv4 and 254 IPv6 learned
        prefixes carry `MED == 0` (no uint32 wrap-around).
    *   Send IPv4 and IPv6 traffic and validate 0% loss on `ATE:port-3` and 100%
        loss on `ATE:port-2`.

#### Canonical OC

```json
{
  "routing-policy": {
    "policy-definitions": {
      "policy-definition": [
        {
          "config": {
            "name": "SUB-MED-60"
          },
          "name": "SUB-MED-60",
          "statements": {
            "statement": [
              {
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-med": 60,
                      "set-med-action": "SUBTRACT"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "20"
                },
                "name": "20"
              }
            ]
          }
        },
        {
          "config": {
            "name": "SUB-MED-150"
          },
          "name": "SUB-MED-150",
          "statements": {
            "statement": [
              {
                "actions": {
                  "bgp-actions": {
                    "config": {
                      "set-med": 150,
                      "set-med-action": "SUBTRACT"
                    }
                  },
                  "config": {
                    "policy-result": "ACCEPT_ROUTE"
                  }
                },
                "config": {
                  "name": "20"
                },
                "name": "20"
              }
            ]
          }
        }
      ]
    }
  }
}
```

### RT-1.12.5: MED policy removal

Validate that removing MED modification policies reverts received routes to
their unmodified incoming MED and normal BGP tie-breaking.

1.  Keep `ATE:port-2` and `ATE:port-3` advertising all 254 IPv4 and 254 IPv6
    prefixes with incoming `MED = 100`, and keep global `always-compare-med`
    enabled (`true`).
2.  Replace import policies on both `BGP-PEER-GROUP2` and `BGP-PEER-GROUP3`
    with `ALLOW` (no MED modification) for both IPv4 and IPv6.
3.  Bounce eBGP neighbors (`ATE:port-2` disabled/re-enabled before
    `ATE:port-3`) to force a clean route refresh and deterministic
    tie-breaking.
4.  Verify control plane on `ATE:port-1`: all 254 IPv4 and 254 IPv6 learned
    prefixes carry the unmodified incoming `MED == 100`.
5.  Send IPv4 and IPv6 traffic and validate that forwarding flips back to
    `ATE:port-2` (0% loss on `flowPort1toPort2` / `flowPort1toPort2V6`, 100%
    loss on `flowPort1toPort3` / `flowPort1toPort3V6`).

### Cleanup

*   DUT: Re-enable any BGP neighbors modified during session-refresh steps,
    restore `always-compare-med` settings and routing policies, and delete test
    BGP protocol and routing-policy configuration upon test completion (via
    `t.Cleanup()`).

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/config/always-compare-med:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/config/set-med-action:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/state/always-compare-med:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/supported-capabilities:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/installed:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/prefixes/sent:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-med:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/bgp-actions/state/set-med-action:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Required DUT platform

*   FFF
