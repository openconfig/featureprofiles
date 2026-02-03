# ACL-1.2: ACL Update (Make-before-break)

## Summary

Test configuration of an IP ACL.
Test changing the ACL configuration to ensure no packets are dropped due to
the configuration change, when the rule added or removed is not intended to
affect the traffic (make before break).


## Testbed type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## ACL-1 Layer 3 terms

* IP src
* IP dst
* TCP src port
* TCP src port range
* TCP dst port
* TCP dst port range
* UDP src port
* UDP src port range
* ICMP proto
* ICMP type

* IPv4 initial fragment
* IPv4 non-initial fragment
* IPv6 fragmentation (1st next-header)
* MatchAll

## Procedure

### Sub Test 1

* Configure DUT with input and output interfaces and static routing.
* Configure IPv4 and IPv6 ACLs with terms specified in the table.
  * All terms should have Deny action.
  * “Match all” term should have Accept and Count actions.
* Apply these ACLs in ingress direction on the DUT input interface.
* Start IP traffic flows matching these terms.
* Verify received packets and ACL term counters on DUT.

### Sub Test 2

* Inverse filtering logic: permit traffic on all terms, deny traffic on MatchAll terms.
* Perform ACL update by adding a single matching condition to all terms (additional address or port).
* Verify that the device is running an updated ACL version.
  * No config error
  * No difference between committed ACL and intended config ACL
* Verify traffic drops for sent flows on ATE ingress interface (no more than 50ms of traffic should be dropped).

### Sub test 3

* Repeat the same test by moving ACLs to the DUT egress interface.

## Canonical OC

```json
{
  "acl": {
    "acl-sets": {
      "acl-set": [
        {
          "config": {
            "name": "ACL-1.2-IPV4",
            "type": "openconfig-acl:ACL_IPV4"
          },
          "name": "ACL-1.2-IPV4",
          "type": "openconfig-acl:ACL_IPV4",
          "acl-entries": {
            "acl-entry": [
              {
                "actions": {
                  "config": {
                    "forwarding-action": "openconfig-acl:DROP",
                    "log-action": "openconfig-acl:LOG_SYSLOG"
                  }
                },
                "config": {
                  "sequence-id": 10
                },
                "ipv4": {
                  "config": {
                    "destination-address": "192.168.200.2/32",
                    "source-address": "192.168.100.1/32"
                  }
                },
                "sequence-id": 10
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "openconfig-acl:DROP",
                    "log-action": "openconfig-acl:LOG_SYSLOG"
                  }
                },
                "config": {
                  "sequence-id": 20
                },
                "ipv4": {
                  "config": {
                    "destination-address": "192.168.200.2/32",
                    "protocol": 6,
                    "source-address": "192.168.100.1/32"
                  }
                },
                "sequence-id": 20,
                "transport": {
                  "config": {
                    "destination-port": 2345,
                    "source-port": 1234
                  }
                }
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "openconfig-acl:DROP",
                    "log-action": "openconfig-acl:LOG_SYSLOG"
                  }
                },
                "config": {
                  "sequence-id": 30
                },
                "ipv4": {
                  "config": {
                    "destination-address": "192.168.200.2/32",
                    "protocol": 17,
                    "source-address": "192.168.100.1/32"
                  }
                },
                "sequence-id": 30,
                "transport": {
                  "config": {
                    "destination-port": 2345,
                    "source-port": 1234
                  }
                }
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "openconfig-acl:DROP",
                    "log-action": "openconfig-acl:LOG_SYSLOG"
                  }
                },
                "config": {
                  "sequence-id": 40
                },
                "ipv4": {
                  "config": {
                    "destination-address": "192.168.200.2/32",
                    "protocol": 1,
                    "source-address": "192.168.100.1/32"
                  }
                },
                "sequence-id": 40
              },
              {
                "actions": {
                  "config": {
                    "forwarding-action": "openconfig-acl:ACCEPT",
                    "log-action": "openconfig-acl:LOG_SYSLOG"
                  }
                },
                "config": {
                  "sequence-id": 990
                },
                "ipv4": {
                  "config": {
                    "destination-address": "0.0.0.0/0",
                    "source-address": "0.0.0.0/0"
                  }
                },
                "sequence-id": 990
              }
            ]
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # base acl paths
  /acl/acl-sets/acl-set/config/name:
  /acl/acl-sets/acl-set/config/type:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/config/sequence-id:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/config/description:
  
  # ipv4 address match
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address-prefix-set:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address-prefix-set:

  # icmpv4 match
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/icmpv4/config/type:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/icmpv4/config/code:

  # ipv6 address match
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address-prefix-set:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address-prefix-set:

  # paths for tcp/udp port and port-range
  /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/source-port-set:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/destination-port-set:

  # paths needed to match IP fragments
  /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/detail-mode:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/explicit-detail-match-mode:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/explicit-tcp-flags:
  /acl/acl-sets/acl-set/acl-entries/acl-entry/transport/config/builtin-detail:

  # state paths for ACL counters
  /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/acl-entries/acl-entry/state/matched-packets:
  /acl/interfaces/interface/egress-acl-sets/egress-acl-set/acl-entries/acl-entry/state/matched-packets:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Minimum DUT platform requirement

MFF
