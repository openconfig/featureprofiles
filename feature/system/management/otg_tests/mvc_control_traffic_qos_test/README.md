# MGT-2: QoS of MVC Control and Management Traffic

## Summary

Validate that DUT-originated IPv4 and IPv6 control and management plane traffic
(NTP, TACACS+, SSH, DNS client, sFlow, Syslog, P4RT, gRIBI, gNMI, and gNOI)
egresses in forwarding class `AF4` with `DSCP 32` (and never `BE1` / `DSCP 0`)
both when routed over Linecard (LC) interfaces via eBGP in the management VRF
(`mgmt`) and when failing over to the Out-of-Band (OOB) management fallback
interface via a backup static default route.

## Testbed type

* `featureprofiles/topologies/atedut_4.testbed`

## Procedure

### Test environment setup

*   **Topology & Addressing (RFC 5737 / RFC 3849)**:
    *   `<DUT_PORT1>` (`port1`, `192.0.2.1/30`, `2001:db8::1/126`) <->
        `<ATE_PORT1>` (`port1`, `192.0.2.2/30`, `2001:db8::2/126`): In-band
        Linecard (LC) management link 1 (`mgmt` VRF).
    *   `<DUT_PORT2>` (`port2`, `192.0.2.5/30`, `2001:db8::5/126`) <->
        `<ATE_PORT2>` (`port2`, `192.0.2.6/30`, `2001:db8::6/126`): In-band
        Linecard (LC) management link 2 (`mgmt` VRF).
    *   `<DUT_PORT3>` (`port3`, `192.0.2.9/30`, `2001:db8::9/126`) <->
        `<ATE_PORT3>` (`port3`, `192.0.2.10/30`, `2001:db8::a/126`):
        Out-of-Band (OOB) management fallback link (`mgmt` VRF).
    *   `<DUT_PORT4>` (`port4`, `192.0.2.13/30`, `2001:db8::d/126`) <->
        `<ATE_PORT4>` (`port4`, `192.0.2.14/30`, `2001:db8::e/126`): Data-plane
        transit link (`DEFAULT` VRF).
    *   `<DUT_LO1>` (`lo1`, `192.0.2.101/32`, `2001:db8::101/128`): Management
        loopback bound to VRF `mgmt`.
*   **Management VRF (`mgmt`) & HA Routing**:
    *   Bind `<DUT_PORT1>`, `<DUT_PORT2>`, `<DUT_PORT3>`, and `<DUT_LO1>` to
        non-default L3 VRF `mgmt`.
    *   Configure dual-stack eBGP in VRF `mgmt` on the DUT (`AS <DUT_AS>`
        `64512`, router-id `192.0.2.101`) peering with `<ATE_PORT1>` and
        `<ATE_PORT2>` (`AS <ATE_AS>` `64511`, RFC 5398). Advertise `0.0.0.0/0`,
        `::/0`, and management server prefixes `198.51.100.0/24` and
        `2001:db8:100::/64` from ATE to DUT, and redistribute `<DUT_LO1>` into
        BGP via `table-connections` and `routing-policy`.
    *   Configure backup static default routes in VRF `mgmt` over `<DUT_PORT3>`
        with administrative `preference = 220`: `0.0.0.0/0` via `192.0.2.10` and
        `::/0` via `2001:db8::a`.
    *   Host simulated management server/collector endpoints `198.51.100.1`
        (IPv4) and `2001:db8:100::1` (IPv6) behind `<ATE_PORT1>`–`<ATE_PORT3>`.
*   **QoS Configuration (`AF4` / `DSCP 32`)**:
    *   Configure queues `AF4` and `BE1`, forwarding-groups `AF4` -> `AF4` and
        `BE1` -> `BE1`, IPv4/IPv6 classifiers (`dscp_af4_v4`, `dscp_af4_v6`)
        matching `dscp-set = [32]` -> `AF4`, and egress scheduler policy
        `mgmt_qos_sched` (`AF4` `STRICT` priority over `BE1`) on `<DUT_PORT1>`,
        `<DUT_PORT2>`, and `<DUT_PORT3>`.

### Reference Protocol Suite and QoS Verification

All subtests (`MGT-2.1`–`MGT-2.7`) reference the following protocol
configuration, traffic triggers (sourced from `<DUT_LO1>` `192.0.2.101` /
`2001:db8::101` in VRF `mgmt` toward `198.51.100.1` / `2001:db8:100::1`), and
verification criteria (see **OpenConfig Path and RPC Coverage** for exact
paths):

1.  **10-Protocol Trigger Suite (IPv4 & IPv6)**:
    *   **NTP** (UDP `123`): Enable NTP client and transmit requests to
        `198.51.100.1` and `2001:db8:100::1`.
    *   **TACACS+** (TCP `49`): Enable AAA `TACACS_ALL` with group `TACACS-MGMT`
        and trigger an SSH login attempt to generate TACACS+ packets.
    *   **SSH** (TCP `22`): Enable SSH server and initiate SSH handshakes with
        `<DUT_LO1>`.
    *   **DNS Client** (UDP/TCP `53`): Invoke `gNOI.System.Ping` to unresolved
        hostname `mgmt-target.example.com` to trigger DNS queries.
    *   **sFlow** (UDP `6343`): Enable sFlow (`dscp = 32`, rate `1`, size `256`)
        on `<DUT_PORT4>` and send transit traffic from `<ATE_PORT4>` to trigger
        sFlow export.
    *   **Syslog** (UDP `514`): Configure remote servers (`ALL` /
        `INFORMATIONAL`) and trigger a config event to emit syslog messages.
    *   **P4RT** (TCP `9559`): Establish `P4Runtime.StreamChannel` sessions to
        `<DUT_LO1>` and trigger `PacketIn` / `MasterArbitrationUpdate`.
    *   **gRIBI** (TCP `9340`): Execute `gRIBI.Get` and `gRIBI.Modify` RPCs to
        `<DUT_LO1>`.
    *   **gNMI** (TCP `9339`): Execute `gNMI.Subscribe` and `gNMI.Get` RPCs to
        `<DUT_LO1>`.
    *   **gNOI** (TCP `9339`): Execute `gNOI.System.Time` and `gNOI.System.Ping`
        RPCs to `<DUT_LO1>`.
2.  **Standard Egress AF4 Verification**:
    *   **Packet Capture**: Every DUT-originated IPv4 and IPv6 packet for all 10
        protocols on the active egress port(s) carries **DSCP = `32`** (`AF4`,
        byte `0x80`), and **0** packets carry `DSCP 0` (`BE1`).
    *   **Queue Telemetry**: Output queue `AF4` `transmit-pkts` and
        `transmit-octets` increment on the active egress port(s) while `AF4`
        `dropped-pkts` remains `0`.

### MGT-2.1: QoS Marking over Linecard Interfaces

*   **Step 1:** Verify VRF `mgmt` IPv4/IPv6 eBGP sessions on `<DUT_PORT1>` and
    `<DUT_PORT2>` are `ESTABLISHED` and record baseline `AF4` queue counters.
*   **Step 2:** Start packet capture on `<ATE_PORT1>` and `<ATE_PORT2>`, run the
    **10-Protocol Trigger Suite**, then stop packet capture.
*   **Step 3:** Perform the **Standard Egress AF4 Verification** on
    `<DUT_PORT1>`/`<ATE_PORT1>` and `<DUT_PORT2>`/`<ATE_PORT2>`.

### MGT-2.2: QoS Marking on Failover to OOB Interface

*   **Step 1:** Record baseline `AF4` queue counters on `<DUT_PORT3>`, disable
    `<DUT_PORT1>` and `<DUT_PORT2>`, and verify VRF `mgmt` eBGP leaves
    `ESTABLISHED` and switches to the static default routes (`preference = 220`)
    over `<DUT_PORT3>`.
*   **Step 2:** Start packet capture on `<ATE_PORT3>`, run the **10-Protocol
    Trigger Suite**, stop packet capture, and perform the **Standard Egress AF4
    Verification** on `<DUT_PORT3>`/`<ATE_PORT3>`.
*   **Step 3:** Re-enable `<DUT_PORT1>` and `<DUT_PORT2>`, verify eBGP returns
    to `ESTABLISHED`, and verify traffic fails back to
    `<DUT_PORT1>`/`<DUT_PORT2>` with **DSCP = `32`** (`AF4`).

### MGT-2.3: Control and Management Traffic Survival Under Egress Port BE1 Congestion

*   **Step 1:** Record baseline `AF4` and `BE1` queue counters on `<DUT_PORT1>`
    and `<DUT_PORT2>`.
*   **Step 2:** Inject oversubscribed background `BE1` (`DSCP 0`) IPv4/IPv6
    traffic in VRF `mgmt` (from `<ATE_PORT3>`/`<ATE_PORT2>` toward `<ATE_PORT1>`
    and `<ATE_PORT3>`/`<ATE_PORT1>` toward `<ATE_PORT2>`) to induce continuous
    `BE1` tail-drops on `<DUT_PORT1>` and `<DUT_PORT2>`.
*   **Step 3:** Capture on `<ATE_PORT1>` and `<ATE_PORT2>`, run the
    **10-Protocol Trigger Suite**, then stop background `BE1` traffic and packet
    captures.
*   **Step 4:** Verify `BE1` `dropped-pkts > 0` on `<DUT_PORT1>` and
    `<DUT_PORT2>`, and verify the **Standard Egress AF4 Verification** passes
    with **0% packet loss** (`AF4` `dropped-pkts == 0`).

### MGT-2.4: Client-Initiated BE1 (DSCP 0) Request vs. DUT Egress AF4 (DSCP 32) Response

*   **Step 1:** Record baseline `AF4`/`BE1` counters on
    `<DUT_PORT1>`–`<DUT_PORT3>`. Capture on `<ATE_PORT1>` and `<ATE_PORT2>`
    while sending inbound client requests (`gNMI`, `gRIBI`, `P4RT`, `gNOI`,
    `SSH`) from `198.51.100.1` and `2001:db8:100::1` to `<DUT_LO1>` marked
    **`DSCP 0` (`BE1`)** and **`DSCP 8` (`CS1`)**, then stop capture.
*   **Step 2:** Disable `<DUT_PORT1>` and `<DUT_PORT2>`, repeat Step 1 over OOB
    `<ATE_PORT3>`, then stop capture.
*   **Step 3:** Verify all DUT TCP ACK and RPC response packets on
    `<ATE_PORT1>`–`<ATE_PORT3>` egress with **DSCP = `32`** (`AF4`) (never
    reflecting `DSCP 0` or `DSCP 8`), then re-enable `<DUT_PORT1>` and
    `<DUT_PORT2>` and verify eBGP returns to `ESTABLISHED`.

### MGT-2.5: Partial Linecard Failure (ECMP Rehash) and Rapid Flap Stability

*   **Step 1:** Establish active flows for the **10-Protocol Trigger Suite**,
    then disable only `<DUT_PORT1>` while keeping `<DUT_PORT2>` `ESTABLISHED`.
*   **Step 2:** Verify **0** packets fail over to OOB `<DUT_PORT3>`, and all
    rehashed flows egress `<DUT_PORT2>` meeting the **Standard Egress AF4
    Verification**.
*   **Step 3:** Flap `<DUT_PORT1>` and `<DUT_PORT2>` 5 times (forcing repeated
    LC <-> OOB transitions) while deleting and re-creating the NTP and Syslog
    servers via `gNMI.Set`.
*   **Step 4:** Stop captures on `<ATE_PORT1>`–`<ATE_PORT3>`, ensure
    `<DUT_PORT1>` and `<DUT_PORT2>` are enabled with eBGP `ESTABLISHED`, and
    verify all 10 protocols retain **DSCP = `32`** (`AF4`) throughout.

### MGT-2.6: Negative Verification of Protocol-Level DSCP Override and Queue Isolation

*   **Step 1:** Set `/sampling/sflow/config/dscp = 0` (`BE1`) via `gNMI.Set`,
    capture on `<ATE_PORT1>` and `<ATE_PORT2>`, run the **10-Protocol Trigger
    Suite**, and stop capture.
*   **Step 2:** Verify the 9 remaining protocols still meet the **Standard
    Egress AF4 Verification** (and if VRF `mgmt` enforces egress remarking,
    `sFlow` is also remarked to `DSCP = 32`); verify negatively that any packet
    egressing with `DSCP 0` (`BE1`) increments only queue `BE1` and **not**
    queue `AF4`.
*   **Step 3:** Restore `/sampling/sflow/config/dscp = 32` via `gNMI.Set` and
    verify `/sampling/sflow/state/dscp == 32`.

### MGT-2.7: Negative Verification of Total Management Path Failure (VRF Isolation)

*   **Step 1:** Configure temporary default routes (`0.0.0.0/0` via
    `192.0.2.14`, `::/0` via `2001:db8::e`) in the `DEFAULT` VRF over
    `<DUT_PORT4>`.
*   **Step 2:** Disable `<DUT_PORT1>`, `<DUT_PORT2>`, and `<DUT_PORT3>`, capture
    on `<ATE_PORT1>`–`<ATE_PORT4>`, and trigger the **10-Protocol Trigger
    Suite**.
*   **Step 3:** Verify negatively on `<ATE_PORT4>` that **0** VRF `mgmt` packets
    leak into the `DEFAULT` VRF.
*   **Step 4:** Re-enable `<DUT_PORT3>` then `<DUT_PORT1>` and `<DUT_PORT2>`,
    stop captures, verify recovery on `<DUT_PORT3>` and failback to
    `<DUT_PORT1>`/`<DUT_PORT2>` meet the **Standard Egress AF4 Verification**,
    and remove the temporary `DEFAULT` VRF default routes.

### Cleanup

*   Stop all active OTG flows and packet captures on
    `<ATE_PORT1>`–`<ATE_PORT4>`, ensure `<DUT_PORT1>`–`<DUT_PORT4>` are enabled
    (`oper-status = UP`), and revert all test configurations via `t.Cleanup()`.

## Canonical OC

```json
{
  "interfaces": {
    "interface": [
      {
        "name": "lo1",
        "config": {"enabled": true, "name": "lo1", "type": "iana-if-type:softwareLoopback"},
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {"index": 0},
              "ipv4": {"addresses": {"address": [{"ip": "192.0.2.101", "config": {"ip": "192.0.2.101", "prefix-length": 32}}]}},
              "ipv6": {"addresses": {"address": [{"ip": "2001:db8::101", "config": {"ip": "2001:db8::101", "prefix-length": 128}}]}}
            }
          ]
        }
      },
      {
        "name": "port1",
        "config": {"enabled": true, "name": "port1", "type": "iana-if-type:ethernetCsmacd"},
        "subinterfaces": {
          "subinterface": [
            {
              "index": 0,
              "config": {"index": 0},
              "ipv4": {"addresses": {"address": [{"ip": "192.0.2.1", "config": {"ip": "192.0.2.1", "prefix-length": 30}}]}},
              "ipv6": {"addresses": {"address": [{"ip": "2001:db8::1", "config": {"ip": "2001:db8::1", "prefix-length": 126}}]}}
            }
          ]
        }
      },
      {"name": "port4", "config": {"enabled": true, "name": "port4", "type": "iana-if-type:ethernetCsmacd"}}
    ]
  },
  "network-instances": {
    "network-instance": [
      {
        "name": "mgmt",
        "config": {"name": "mgmt", "type": "openconfig-network-instance-types:L3VRF"},
        "interfaces": {
          "interface": [
            {"id": "lo1", "config": {"id": "lo1", "interface": "lo1"}},
            {"id": "port1", "config": {"id": "port1", "interface": "port1"}}
          ]
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP",
              "config": {"identifier": "openconfig-policy-types:BGP", "name": "BGP"},
              "bgp": {
                "global": {"config": {"as": 64512, "router-id": "192.0.2.101"}},
                "neighbors": {
                  "neighbor": [
                    {"neighbor-address": "192.0.2.2", "config": {"neighbor-address": "192.0.2.2", "peer-as": 64511}},
                    {"neighbor-address": "2001:db8::2", "config": {"neighbor-address": "2001:db8::2", "peer-as": 64511}}
                  ]
                }
              }
            },
            {
              "identifier": "openconfig-policy-types:STATIC",
              "name": "STATIC",
              "config": {"identifier": "openconfig-policy-types:STATIC", "name": "STATIC"},
              "static-routes": {
                "static": [
                  {
                    "prefix": "0.0.0.0/0",
                    "config": {"prefix": "0.0.0.0/0"},
                    "next-hops": {"next-hop": [{"index": "1", "config": {"index": "1", "next-hop": "192.0.2.10", "preference": 220}}]}
                  },
                  {
                    "prefix": "::/0",
                    "config": {"prefix": "::/0"},
                    "next-hops": {"next-hop": [{"index": "1", "config": {"index": "1", "next-hop": "2001:db8::a", "preference": 220}}]}
                  }
                ]
              }
            }
          ]
        }
      }
    ]
  },
  "qos": {
    "classifiers": {
      "classifier": [
        {
          "name": "dscp_af4_v4",
          "config": {"name": "dscp_af4_v4", "type": "IPV4"},
          "terms": {"term": [{"id": "0", "config": {"id": "0"}, "conditions": {"ipv4": {"config": {"dscp-set": [32]}}}, "actions": {"config": {"target-group": "AF4"}}}]}
        },
        {
          "name": "dscp_af4_v6",
          "config": {"name": "dscp_af4_v6", "type": "IPV6"},
          "terms": {"term": [{"id": "0", "config": {"id": "0"}, "conditions": {"ipv6": {"config": {"dscp-set": [32]}}}, "actions": {"config": {"target-group": "AF4"}}}]}
        }
      ]
    },
    "forwarding-groups": {
      "forwarding-group": [
        {"name": "AF4", "config": {"name": "AF4", "output-queue": "AF4"}},
        {"name": "BE1", "config": {"name": "BE1", "output-queue": "BE1"}}
      ]
    },
    "interfaces": {
      "interface": [
        {
          "interface-id": "port1",
          "config": {"interface-id": "port1"},
          "output": {
            "queues": {"queue": [{"name": "AF4", "config": {"name": "AF4"}}, {"name": "BE1", "config": {"name": "BE1"}}]},
            "scheduler-policy": {"config": {"name": "mgmt_qos_sched"}}
          }
        }
      ]
    },
    "queues": {
      "queue": [
        {"name": "AF4", "config": {"name": "AF4"}},
        {"name": "BE1", "config": {"name": "BE1"}}
      ]
    },
    "scheduler-policies": {
      "scheduler-policy": [
        {
          "name": "mgmt_qos_sched",
          "config": {"name": "mgmt_qos_sched"},
          "schedulers": {
            "scheduler": [
              {
                "sequence": 1,
                "config": {"priority": "STRICT", "sequence": 1, "type": "openconfig-qos-types:TWO_RATE_THREE_COLOR"},
                "inputs": {"input": [{"id": "AF4", "config": {"id": "AF4", "input-type": "QUEUE", "queue": "AF4", "weight": 100}}]}
              },
              {
                "sequence": 2,
                "config": {"sequence": 2, "type": "openconfig-qos-types:TWO_RATE_THREE_COLOR"},
                "inputs": {"input": [{"id": "BE1", "config": {"id": "BE1", "input-type": "QUEUE", "queue": "BE1", "weight": 1}}]}
              }
            ]
          }
        }
      ]
    }
  },
  "routing-policy": {
    "defined-sets": {
      "prefix-sets": {
        "prefix-set": [
          {
            "name": "lo1_v4_prefix_set",
            "config": {"mode": "IPV4", "name": "lo1_v4_prefix_set"},
            "prefixes": {"prefix": [{"ip-prefix": "192.0.2.101/32", "masklength-range": "exact", "config": {"ip-prefix": "192.0.2.101/32", "masklength-range": "exact"}}]}
          }
        ]
      }
    },
    "policy-definitions": {
      "policy-definition": [
        {
          "name": "redist_lo1_v4",
          "config": {"name": "redist_lo1_v4"},
          "statements": {
            "statement": [
              {
                "name": "allow_lo1_v4",
                "config": {"name": "allow_lo1_v4"},
                "conditions": {"match-prefix-set": {"config": {"match-set-options": "ANY", "prefix-set": "lo1_v4_prefix_set"}}},
                "actions": {"config": {"policy-result": "ACCEPT_ROUTE"}}
              }
            ]
          }
        }
      ]
    }
  },
  "sampling": {
    "sflow": {
      "config": {"dscp": 32, "enabled": true, "ingress-sampling-rate": 1, "sample-size": 256},
      "collectors": {
        "collector": [
          {"address": "198.51.100.1", "port": 6343, "config": {"address": "198.51.100.1", "network-instance": "mgmt", "port": 6343, "source-address": "192.0.2.101"}}
        ]
      },
      "interfaces": {"interface": [{"name": "port4", "config": {"enabled": true, "name": "port4"}}]}
    }
  },
  "system": {
    "aaa": {
      "authentication": {"config": {"authentication-method": ["openconfig-aaa-types:TACACS_ALL"]}},
      "server-groups": {
        "server-group": [
          {
            "name": "TACACS-MGMT",
            "config": {"name": "TACACS-MGMT", "type": "openconfig-aaa:TACACS"},
            "servers": {
              "server": [
                {
                  "address": "198.51.100.1",
                  "config": {"address": "198.51.100.1", "timeout": 5},
                  "tacacs": {"config": {"port": 49, "secret-key": "tacacs-secret", "source-address": "192.0.2.101"}}
                }
              ]
            }
          }
        ]
      }
    },
    "dns": {
      "servers": {
        "server": [
          {"address": "198.51.100.1", "config": {"address": "198.51.100.1", "port": 53}}
        ]
      }
    },
    "grpc-servers": {
      "grpc-server": [
        {
          "name": "mgmt-grpc",
          "config": {
            "enable": true,
            "listen-addresses": ["192.0.2.101", "2001:db8::101"],
            "name": "mgmt-grpc",
            "network-instance": "mgmt",
            "port": 9339,
            "services": ["GNMI"]
          }
        }
      ]
    },
    "logging": {
      "remote-servers": {
        "remote-server": [
          {
            "host": "198.51.100.1",
            "config": {"host": "198.51.100.1", "network-instance": "mgmt", "remote-port": 514, "source-address": "192.0.2.101"},
            "selectors": {"selector": [{"facility": "openconfig-system-logging:ALL", "severity": "INFORMATIONAL", "config": {"facility": "openconfig-system-logging:ALL", "severity": "INFORMATIONAL"}}]}
          }
        ]
      }
    },
    "ntp": {
      "config": {"enabled": true},
      "servers": {
        "server": [
          {"address": "198.51.100.1", "config": {"address": "198.51.100.1", "network-instance": "mgmt", "source-address": "192.0.2.101"}}
        ]
      }
    },
    "ssh-server": {"config": {"enable": true}}
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Paths ##
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /network-instances/network-instance/config/name:
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/interface:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /network-instances/network-instance/protocols/protocol/static-routes/static/config/prefix:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/index:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/next-hop:
  /network-instances/network-instance/protocols/protocol/static-routes/static/next-hops/next-hop/config/preference:
  /network-instances/network-instance/table-connections/table-connection/config/address-family:
  /network-instances/network-instance/table-connections/table-connection/config/default-import-policy:
  /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation:
  /network-instances/network-instance/table-connections/table-connection/config/dst-protocol:
  /network-instances/network-instance/table-connections/table-connection/config/import-policy:
  /network-instances/network-instance/table-connections/table-connection/config/src-protocol:
  /qos/classifiers/classifier/config/name:
  /qos/classifiers/classifier/config/type:
  /qos/classifiers/classifier/terms/term/actions/config/target-group:
  /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set:
  /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set:
  /qos/classifiers/classifier/terms/term/config/id:
  /qos/forwarding-groups/forwarding-group/config/name:
  /qos/forwarding-groups/forwarding-group/config/output-queue:
  /qos/interfaces/interface/config/interface-id:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/output/scheduler-policy/config/name:
  /qos/queues/queue/config/name:
  /qos/scheduler-policies/scheduler-policy/config/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:
  /routing-policy/policy-definitions/policy-definition/config/name:
  /routing-policy/policy-definitions/policy-definition/statements/statement/actions/config/policy-result:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/match-set-options:
  /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-prefix-set/config/prefix-set:
  /routing-policy/policy-definitions/policy-definition/statements/statement/config/name:
  /sampling/sflow/collectors/collector/config/address:
  /sampling/sflow/collectors/collector/config/network-instance:
  /sampling/sflow/collectors/collector/config/port:
  /sampling/sflow/collectors/collector/config/source-address:
  /sampling/sflow/config/dscp:
  /sampling/sflow/config/enabled:
  /sampling/sflow/config/ingress-sampling-rate:
  /sampling/sflow/config/sample-size:
  /sampling/sflow/interfaces/interface/config/enabled:
  /sampling/sflow/interfaces/interface/config/name:
  /system/aaa/authentication/config/authentication-method:
  /system/aaa/server-groups/server-group/config/name:
  /system/aaa/server-groups/server-group/config/type:
  /system/aaa/server-groups/server-group/servers/server/config/address:
  /system/aaa/server-groups/server-group/servers/server/config/timeout:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/port:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/secret-key:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/source-address:
  /system/dns/servers/server/config/address:
  /system/dns/servers/server/config/port:
  /system/grpc-servers/grpc-server/config/enable:
  /system/grpc-servers/grpc-server/config/listen-addresses:
  /system/grpc-servers/grpc-server/config/name:
  /system/grpc-servers/grpc-server/config/network-instance:
  /system/grpc-servers/grpc-server/config/port:
  /system/grpc-servers/grpc-server/config/services:
  /system/logging/remote-servers/remote-server/config/host:
  /system/logging/remote-servers/remote-server/config/network-instance:
  /system/logging/remote-servers/remote-server/config/remote-port:
  /system/logging/remote-servers/remote-server/config/source-address:
  /system/logging/remote-servers/remote-server/selectors/selector/config/facility:
  /system/logging/remote-servers/remote-server/selectors/selector/config/severity:
  /system/ntp/config/enabled:
  /system/ntp/servers/server/config/address:
  /system/ntp/servers/server/config/network-instance:
  /system/ntp/servers/server/config/source-address:
  /system/ssh-server/config/enable:

  ## State Paths ##
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
  /qos/interfaces/interface/output/queues/queue/state/name:
  /qos/interfaces/interface/output/queues/queue/state/transmit-octets:
  /qos/interfaces/interface/output/queues/queue/state/transmit-pkts:
  /qos/interfaces/interface/output/scheduler-policy/state/name:
  /qos/scheduler-policies/scheduler-policy/state/name:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/state/priority:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/state/sequence:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/state/type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/id:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/input-type:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/queue:
  /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/state/weight:
  /sampling/sflow/collectors/collector/state/address:
  /sampling/sflow/collectors/collector/state/network-instance:
  /sampling/sflow/collectors/collector/state/port:
  /sampling/sflow/collectors/collector/state/source-address:
  /sampling/sflow/state/dscp:
  /sampling/sflow/state/enabled:
  /system/aaa/authentication/state/authentication-method:
  /system/aaa/server-groups/server-group/servers/server/state/address:
  /system/aaa/server-groups/server-group/servers/server/tacacs/state/port:
  /system/aaa/server-groups/server-group/servers/server/tacacs/state/source-address:
  /system/dns/servers/server/state/address:
  /system/dns/servers/server/state/port:
  /system/grpc-servers/grpc-server/state/enable:
  /system/grpc-servers/grpc-server/state/listen-addresses:
  /system/grpc-servers/grpc-server/state/name:
  /system/grpc-servers/grpc-server/state/network-instance:
  /system/grpc-servers/grpc-server/state/port:
  /system/grpc-servers/grpc-server/state/services:
  /system/logging/remote-servers/remote-server/state/host:
  /system/logging/remote-servers/remote-server/state/network-instance:
  /system/logging/remote-servers/remote-server/state/remote-port:
  /system/logging/remote-servers/remote-server/state/source-address:
  /system/ntp/state/enabled:
  /system/ntp/servers/server/state/address:
  /system/ntp/servers/server/state/network-instance:
  /system/ntp/servers/server/state/source-address:
  /system/ssh-server/state/enable:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    system.System.Ping:
    system.System.Time:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
```

## Required DUT platform

*   **MFF**
