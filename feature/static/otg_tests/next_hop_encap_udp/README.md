# RT-3.53: Static route based GUE Encapsulation to IPv6 tunnel

## Summary

This test verifies using static route to encapsulate packets in an
IPv6 GUE tunnel. The encapsulation is based on a statically configured GUE
tunnel.

The GUE header ((UDPoIP)) used in this test refers to GUE Variant 1 as specified
in
[draft-ietf-intarea-gue-09](https://datatracker.ietf.org/doc/html/draft-ietf-intarea-gue-09).

The following behavioral properties are called out for awareness:

*   When the DSCP for the tunnel is not explicitly set, it is copied from the
    inner to the outer encapsulating header.
*   When the TTL for the tunnel is not explicitly set, it is decremented by 1
    and copied from inner to the outer encapsulating header. Except TTL=1, where
    Time Exceeded is sent back to the inner source IP.

## Testbed Type

*  [`featureprofiles/topologies/atedut_5.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_5.testbed)

## Procedure

### Test environment setup

    ```
                              -------
                             |       | ==== LAG1 ==== [ ATE:Port2, ATE:Port3 ]
          [ ATE:Port1 ] ---- |  DUT  |
                             |       | ==== LAG2 ==== [ ATE:Port4, ATE:Port5 ]
                              -------
    ```

*   Routes are advertised from ATE:Port1
*   Traffic is generated from ATE:Port1
*   ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5 are used as the destination port for GUE encapsulated traffic
*   PNH-IPv6 is fc00:10::1
*   IPv6-DST-GUE is 2001:DB8:C0FE:AFFE::1
*   IPv4-DST-NET is 10.1.1.1
*   IPv4-DST-NET is 2001:DB8::10:1:1::1


#### Configuration

1.  DUT:Port1 is configured as singleton interface towards ATE:Port1.

    -   DUT:Port1:IPv6 is 2001:DB8::192:168:10:0/127
    -   ATE:Port1:IPv6 is 2001:DB8::192:168:10:1/127

2.  DUT:Port2 and DUT:Port3 are configured as LAG1 interface towards
    ATE:Port2 and ATE:Port3 respectively.

    -   DUT:LAG1:IPv6 is 2001:DB8:20::/127
    -   ATE:LAG1:IPv6 is 2001:DB8:20::1/127

3.  DUT:Port4 and DUT:Port5 are configured as LAG2 interface towards
    ATE:Port4 and ATE:Port5 respectively.
    -   DUT:LAG2:IPv6 is 2001:DB8:30::/127
    -   ATE:LAG2:IPv6 is 2001:DB8:30::1/127

4.  DUT:Port1 is configured to form an iBGP session with ATE:Port1 using
    [RFC5549](https://datatracker.ietf.org/doc/html/rfc5549).
    Peering is done with the directly connected interface IP.

5.  ATE:Port1 is configured to advertise destination networks
    IPv4-DST-NET/32 and IPv6-DST-NET/128 via BGP to DUT. The protocol
    next hop of both the IPv4 and IPv6 networks should be PNH-IPv6.

6.  DUT is configured with an IPv6 GUE tunnel with destination
    IPv6-DST-GUE without any explicit tunnel Type of Service (ToS) or
    Time to Live (TTL) values. The next hop group will be used for
    GUE encapsulation.

7.  DUT is configured with the following static routes:

    -   To PNH-IPv6, next hop is the statically configured IPv6 GUE tunnel.
    -   To IPv6-DST-GUE, next hop is ATE:LAG1:IPv6.
    -   To IPv6-DST-GUE, next hop is ATE:LAG2:IPv6.


### RT-3.53.1: IPv4 traffic GUE encapsulation without explicit ToS/TTL configuration on tunnel

ATE action:

*   Generate 12000000 **IPv4 packets** from ATE:Port1 to random IP addresses in
    IPv4-DST-NET ensuring that there are at least 65000 different 5-tuple flows.
    *   Use 512 bytes frame size.
    *   Set ToS value of *0x80* for all packets.
    *   Set TTL of all packets to *10*.

Verify:

*   The packet count of traffic sent from ATE:Port1 should be equal to the sum
    of all packets:
    *   Egressing DUT:Port2, DUT:Port3, DUT:Port4 and DUT:Port5
        by checking `out-unicast-pkts` counter.
    *   Received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5.
*   All packets received on ATE:Port2, ATE:Port3, ATE:Port4 and ATE:Port5 are
    GUE encapsulated.
*   ECMP hashing works over the two LAG interfaces with a tolerance of <6%.
*   LAG hashing works over the two singleton ports within LAG1 and LAG2 with a
    tolerance of <6%.
*   ToS for all GUE encapsulated packets received at ATE ports:
    *   GUE header ToS is **0x80**.
    *   Inner header ToS is **0x80**.
*   TTL for all GUE encapsulated packets received at ATE ports:
    *   GUE header TTL is **9**.
    *   Inner header TTL is **9**.

### RT-3.53.2: IPv6 traffic GUE encapsulation without explicit ToS/TTL configuration on tunnel

*   Modify the flows in `RT-3.53.1` to use IPv6 destination IPv6-DST-NET and
    repeat the traffic generation and validation.

### RT-3.53.3: IPv4 traffic GUE encapsulation with explicit ToS configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv6 GUE tunnel on the DUT with ToS value *0x60*.
*   Generate the same flows in `RT-3.53.1`.

Verify:

*   Repeat same verifications in `RT-3.53.1` but with the following differences
    *   ToS for all GUE encapsulated packets:
        *   GUE header ToS is **0x60**.

### RT-3.53.4: IPv6 traffic GUE encapsulation with explicit ToS configuration on tunnel

*   Modify the flows in `RT-3.53.3` to use IPv6 destination IPv6-DST-NET and
    repeat the traffic generation and validation.

### RT-3.53.5: IPv4 traffic GUE encapsulation with explicit TTL configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv6 GUE tunnel on the DUT without an explicit ToS value.
*   Re-configure the IPv6 GUE tunnel on the DUT with TTL value of *20*.
*   Generate the same flows in `RT-3.53.1`.

Verify:

*   Repeat same verifications in `RT-3.53.1` but with the following differences
    *   TTL for all GUE encapsulated packets:
        *   GUE header TTL is **20**.

### RT-3.53.6: IPv6 traffic GUE encapsulation with explicit TTL configuration on tunnel

*   Modify the flows in `RT-3.53.5` to use IPv6 destination IPv6-DST-NET and
    repeat the traffic generation and validation.

### RT-3.53.7: IPv4 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel

DUT and ATE actions:

*   Re-configure the IPv6 GUE tunnel on the DUT with ToS value *0x60*.
*   Re-configure the IPv6 GUE tunnel on the DUT with TTL value of *20*.
*   Generate the same flows in `RT-3.53.1`.

Verify:

*   Repeat same verifications in `RT-3.53.1` but with the following differences
    *   ToS for all GUE encapsulated packets:
        *   GUE header ToS is **0x60**.
    *   TTL for all GUE encapsulated packets:
        *   GUE header TTL is **20**.

### RT-3.53.8: IPv6 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel

*   Modify the flows in `RT-3.53.7` to use IPv6 destination IPv6-DST-NET and
    repeat the traffic generation and validation.

### RT-3.53.9: IPv4 traffic GUE encapsulation to a single 5-tuple tunnel

DUT and ATE actions:

*   Re-configure DUT without explicit ToS/TTL configuration on tunnel
*   Modify flows in `RT-3.53.1` to be a single flow only; use fixed source and
    destination IPs as well as fixed source and destination UDP ports.

Verify:

Verify:

*   Repeat same verifications in `RT-3.53.1` but with the following differences
    *   All traffic is hashed to a only one LAG and only one singleton port in the
        LAG.

### RT-3.53.10: IPv4 traffic GUE encapsulation to a single tunnel

*   Modify the flows in `RT-3.53.9` to use IPv6 destination IPv6-DST-NET
    and repeat the traffic generation and validation.

### RT-3.53.11: IPv4 traffic that should be GUE encapsulated but TTL=1

ATE action:

*   Modify flows in `RT-3.53.1` and set TTL of all packets to *1*.

Verify:

*   All packets should have TTL decremented to 0, dropped and ICMPv6 Time
    Exceeded (Type 3) / Time to Live exceeded in Transit (Code 0) sent back to
    source address (ATE:Port1).

### RT-3.53.12: IPv6 traffic that should be GUE encapsulated but Hop Limit=1

*   Modify the flows in `RT-3.53.11` to use IPv6 destination IPv6-DST-NET 
    and repeat the traffic generation and validation.

### Canonical OC
```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "DEFAULT",
        "config": {
          "name": "DEFAULT"
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "STATIC",
              "name": "STATIC",
              "config": {
                "identifier": "STATIC",
                "name": "STATIC"
              },
              "static-routes": {
                "static": [
                  {
                    "prefix": "fc00:10::1/128",
                    "config": {
                      "prefix": "fc00:10::1/128"
                    },
                    "next-hop-group": {
                      "config": {
                        "name": "ENCAP-NHG-1"
                      }
                    }
                  }
                ]
              }
            }
          ]
        },
        "static": {
          "next-hop-groups": {
            "next-hop-group": [
              {
                "name": "ENCAP-NHG-1",
                "config": {
                  "name": "ENCAP-NHG-1"
                },
                "next-hops": {
                  "next-hop": [
                    {
                      "index": "0",
                      "config": {
                        "index": "0"
                      }
                    }
                  ]
                }
              }
            ]
          },
          "next-hops": {
            "next-hop": [
              {
                "index": "0",
                "config": {
                  "index": "0"
                },
                "encap-headers": {"encap-header": [
                      {
                        "config": {
                          "index": 0,
                          "type": "UDPV4"
                        },
                        "index": 0,
                        "udp-v4": {
                          "config": {
                            "dscp": 32,
                            "dst-ip": "10.50.50.1",
                            "dst-udp-port": 6080,
                            "ip-ttl": 20,
                            "src-ip": "10.5.5.5",
                            "src-udp-port": 49152
                          }
                        }
                      }
                    ]
                }
              }
            ]
          }
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
    # config
    /network-instances/network-instance/static/next-hop-groups/next-hop-group/config/name:
    /network-instances/network-instance/static/next-hop-groups/next-hop-group/next-hops/next-hop/config/index:
    /network-instances/network-instance/static/next-hops/next-hop/config/index:
    /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/config/index:
    /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/config/type:
    /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/udp-v4/config/dscp:
    /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/udp-v4/config/dst-ip:
    /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/udp-v4/config/dst-udp-port:
    /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/udp-v4/config/ip-ttl:
    /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/udp-v4/config/src-ip:
    /network-instances/network-instance/static/next-hops/next-hop/encap-headers/encap-header/udp-v4/config/src-udp-port:
    /network-instances/network-instance/protocols/protocol/static-routes/static/next-hop-group/config/name:

    # telemetry
    /interfaces/interface/state/counters/out-unicast-pkts:


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
```
