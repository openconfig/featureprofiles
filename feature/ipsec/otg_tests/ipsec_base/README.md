# IPSEC-1.1: IPSec with MACSec over aggregated links.

## Summary

This test verifies the IPSec tunneling between a pair of devices. A pair of DUTs establish an IPsec tunnel. Traffic on ingress to the DUT is then encrypted and forwarded over the tunnel to the egress DUT, which then decrypts the packets and forwards to the final destination.

## Testbed Type

The ate-dut testbed configuration would be used, as described below.

*  [`featureprofiles/topologies/atedut4dutate.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut4dutate.testbed)

TODO: when OTG API supports IPSec, refactor the topology to be: `atedut8` where the ATE serves as the endpoints of the ipsec tunnel


## Procedure

### Test Environment Setup

DUT has an ingress and 2 egress aggregate interfaces.

``` 
[ ATE Port 1 ]----| Port5  :DUT1:  Port1|2|3|4 | ---- |Port1|2|3|4 :DUT2: Port5 | ---- |ATE Port 2 |

```

Four ports are required between the DUT devices to verify hashing.

All physical interfaces are expected to be the same physical speed unless specified otherwise.

* DUT \<\> DUT will contain the IPsec tunnel
* ATE Port1 \<\> DUT1 Port5 are configured with MACsec; ATE Port2 \<\> DUT2 Port 5 do not have MACsec.
* DUT1 and DUT2 are the endpoints of the IPSec tunnel;
* Traffic on the DUT-DUT links is encrypted using native-ipsec tunnel-mode. There are no additional GUE / MPLSoGUE/ MPLSoGRE / GRE / MPLS encapsulation layers in/above/below the ipsec traffic. 

ATE generated traffic flow definition:
* IPv4 and IPv6 addressing with 200 flows each.
* The sum of the IPv4 and IPv6 traffic should equal line rate for the ATE to DUT interface. 
* Mix of 64, 128, 256, 512, 1024, 1500, 4500, and MAX MTU bytes frame size.
* Send traffic both directions; from ATE port1 to ATE port 2 and vice versa.

For the DUT \<\> ATE configuration (on both sides), see the gre\_ipv4\_encap setup for the “customer interface” setup in [policy\_forwarding/otg\_tests/mpls\_gre\_ipv4\_encap\_test](https://github.com/openconfig/featureprofiles/tree/main/feature/policy_forwarding/otg_tests/mpls_gre_ipv4_encap_test)

1.  A single customer attachment is required, with both IPv4 and IPv6 addresses
2.  The “customer interface” must be a vlan, and in a (non-default / non-management) VRF
3.  DUT1 \<\> ATE configured with MACsec, as described in the MACSec OC section below.
4.  MTU of 9080 (including L2 header)
    1.  The macsec MTU may be adjusted as necessary to account for any macsec overhead not included in the MTU value
5.  Interfaces
    1.  1 link, as port-channel with 1 member
    2.  IPv4 and IPv6 address on the subnet

For the DUT interface configuration:

1.  Interfaces
    1.  4 links, as 2 Port-Channels, each with 2 members to verify both LAG and ECMP compatibility.
    2.  Loopback interface, per tunnel-pair
    3.  IPv6 (IPv4 routing not necessary between DUTs)
    4.  MTU 9216
    5.  LACP Member Link configuration
    6.  LAG ID
    7.  LACP (default: period short)
    8.  Carrier-delay (default up: 3000 down: 150)
    9.  Statistics load interval (default: 30 seconds)

### DUT Routing Configuration

1.  Static routes to reach loopback of each DUT, in the default VRF
2.  VRF routing to send all traffic received from ATE-1 to ATE-2 and vice-versa for both ipv4 and ipv6 within the VRF
    1.  A VRF for the ATE interface, with a v4/v6 route for all traffic arriving on the ATE interface to route into the VRF for the tunnels, ECMP’ng across all tunnels
    2.  A VRF for the tunnels, with a v4/v6 route for all traffic arriving on the tunnels to route into the VRF directed at the ATE interface

### IPSEC-1.1.1: Verify Base IPv4 & IPv6 traffic forwarding with ipsec

1.  Configure the security association / IKE between DUT1 \<\> DUT2 with the following parameters:
    1.  For IKE negotiation:
        1.  IKE version: 2
        2.  Diffie-hellman group: 24
        3.  IKE lifetime: 10 hours
    2.  For the Security Association (SA) negotiation:
        1.  Encryption Algorithm (ESP): aes-256 gcm-128
        2.  ESP Integrity: (not needed, aes256gcm128 provides integrity)
        3.  SA Lifetime: 8 hours
        4.  PFS DH-Group: 14
        5.  Anti-replay detection: disabled
2.  For the Tunnel Profile:
    1.  Tunnel mode (not transport mode)
    2.  IKE & SA profiles as described above
    3.  Ability to initiate the connection (not passive)
    4.  Dead-Peer Detection for 10 second / 30 second
    5.  Static shared-key
3.  Configure the IPSec tunnel between the DUT1 \<\> DUT2 with the following parameters:
    1.  The tunnel itself is in a separate VRF as the ATE interface, as described in the DUT-DUT VRF-routing section above
    2.  Tunnel is ipsec (exclusively), with the ipsec profile as described above
    3.  Tunnel internal addressing (inner-ip) supporting both IPv4 + IPv6
    4.  MTU at max (9066? TODO: double check)
    5.  Tunnel outer addressing (outer-ip) between the assigned v6 loopback of DUT1 and v6 loopback of DUT2
    6.  Tunnel outer addressing reachable in the default VRF.
    7.  Outer IPv6 flow label & packet-next-hop selection is generated based on the hash of the inner (unencrypted) packet

* Generate bidirectional traffic as highlighted in the test environment.
    * Traffic with IPV4 and IPv6 Payloads from ATE ports 1,2
* Send packets from ATE1 to ATE2, verifying packets traverse the ipsec tunnel.

Verify:

* Verify the IPSEC sessions are established. TODO: provide telemetry paths+values that are to be used on the DUT
* Packets are received at the remote end ATE, with no loss.
* Verify all flows have no packet loss.
* Packet as received by the remote-end ATE is preserved and not altered outside of changes expected with routing (TTL), validating:
  * A test sending all packets with DSCP=10 results in all packets received with DSCP=10; repeat with DSCP 20, 30.
  * A test sending all packets with flow-label=10 results in all packets received with flow-label=10; repeat with flow label 1000.

### IPSEC-1.1.2: Verify Line-Rate IPv4 Connectivity over a Single Tunnel

* Generate traffic on ATE-\>DUT1 Ports having a random combination of 1000 source addresses to ATE-2 destination address(es) at line rate IPv4 traffic. Use MTU-RANGE frame-size.

Verify:

* All traffic received from ATE (other than any local traffic) gets forwarded as ipsec
* No packet loss
* Traffic equally load balanced / hashed across DUT \<\> DUT ports.

### IPSEC-1.1.3: Verify Line-Rate IPv6 Connectivity over a Single Tunnel

* Generate traffic on ATE-\>DUT1 Ports having a random combination of 1000 source addresses to ATE-2 destination address(es) at line rate IPv6 traffic. Use MTU-RANGE frame-size.

Verify:

* All traffic received from ATE (other than any local traffic) gets forwarded as ipsec
* No packet loss
* Traffic equally load balanced / hashed across DUT \<\> DUT ports.

### IPSEC-1.1.4: Verify Hitless SA Renegotation (New Key Generation)

Requires single-tunnel setup, with the following modifications:

* SA renegotiation: Minimum time (O(seconds or few minutes) if available), or use external method to manually trigger an SA renegotiation

Establish tunnel, send traffic.

Verify no tunnel traffic loss during SA renegotiation event.

Option to do this test with the maximum number of device tunnels.

### IPSEC-1.1.5: Verify Hitless IKE Renegotation

Requires single-tunnel setup, with the following modifications:

* IKE lifetime: Minimum time (O(seconds or few minutes) if available), or use external method to manually trigger an IKE lifetime renegotiation

Establish a tunnel, send traffic (single protocol, packet-size acceptable).

Verify no tunnel traffic loss during IKE renegotiation event.

Option to do this test with the maximum number of device tunnels.

### IPSEC-1.1.6: Verify DPD / dead-peer detection

Change in the base setup:

* Two parallel ipsec tunnels (loopback1 \<\> loopback1 + loopback2 \<\> loopback2)
* DPD timers lower (2 second keepalive, 10 sec hold-time)

Establish tunnel, send traffic.

* Verify traffic is ECMP’ng across both tunnels, no traffic loss

Filter IKE traffic directed at loopback on DUT2, which is expected to cause DPD keepalives over tunnel \#2 to fail.

* Verify encrypted traffic (non-IKE) traffic continues to flow, traffic continues to ECMP across both tunnels until DPD timers fire.
* Verify traffic moves exclusively to Tunnel-1 after the DPD timers fire with no loss of traffic

### IPSEC-1.1.7: Invalid Tunnel - Mismatch on Key

Change in base setup:

* Two tunnels
* Tunnel 1: Standard setup
* Tunnel 2: Standard setup, except:
    * Non-matching shared-key between DUT1/DUT2

Verify:

* Packets sent between ATE1 & ATE2 do not see loss
* All packets go over Tunnel1
* Tunnel2 is “down” per monitoring

### IPSEC-1.1.8: Invalid Tunnel - Mismatch on Cipher Algorithm

Change in base setup:

* Two tunnels
* Tunnel 1: Standard setup
* Tunnel 2: Standard setup, except:
    * Different encryption algorithm for the SA on DUT2 (where DUT2 tunnel2 does not match the encryption algorithm configured on DUT1 tunnel1)

Verify:

* Packets sent between ATE1 & ATE2 do not see loss
* All packets go over Tunnel1
* Tunnel2 is “down” per monitoring

### IPSEC-1.1.9: Verify IPSec shared-key key rotation

Use the standard ATE - DUT - DUT - ATE topology with 2 DUTs that have an ipsec tunnel established between them

* Generate bidirectional traffic as highlighted in the test environment.
    * IPSEC traffic with IPV4 Payloads from ATE ports 1,2
* Send packets from ATE1 to ATE2, for packets that traverse the ipsec tunnel.
* Verify that the packets succeed
* Change the shared key on both ends
    * Option to shut down / unshut the tunnel
    * Wait sufficient time for the rekey event (X seconds), or validate through polling that the changes have taken place & tunnel is operational (again)
* Send packets from ATE1 to ATE2, for packets that traverse the ipsec tunnel.
* Verify that the packets succeed

Verify

* Validate tunnel comes back up operational

### IPSEC-1.1.10: Verify Flow-Label/Hash-Disablement

Change in base setup:

* Disable the outer packet’s ipv6-flow-label generation from the inner-packet / keep the ipv6 flow label constant for the outer packets

Test:

* Generate traffic on ATE-\>DUT1 Ports having a random combination of 1000 source addresses to ATE-2 destination address(es) at 50%-of-line rate IPv6 traffic.

Verify:

* No packet loss
* Tunneled traffic is mapped/hashed to one DUT1-DUT2 link, instead of being load-balanced across all DUT1-DUT2 links

### IPSEC-1.1.11: Verify Tunnel Re-Pathing upon Failure

Test:

* Generate traffic on ATE-\>DUT1 Ports with a large number of source addresses
* Validate traffic is hashed across all 4 DUT1-DUT2 links
* Fail one of the DUT1-DUT2 links
* Validate traffic is now hashed across remaining links

Verify:

* All links that are available (not failed) are in-use
* No more than Low/Minor packet loss during the link failure, with dataplane automatically recovering after failure
* No loss of the tunnel

### IPSEC-1.1.12: Verify QoS: Control Plane survives with Dataplane Congestion

1.  Using the setup described in [feature/qos/otg\_tests/egress\_strict\_priority\_scheduler\_test](https://github.com/openconfig/featureprofiles/blob/main/feature/qos/otg_tests/egress_strict_priority_scheduler_test/README.md)
2.  Classify/schedule traffic on the DUT \<\> DUT connection as:
    1.  IPSec dataplane (tunneled) traffic as AF3
    2.  IPSec control plane (IKE) as AF4
    3.  All other dataplane traffic as BE1

These are the default settings; each individual test MAY change these values

Additional changes from the default base setup:

* DUT1 \<\> DUT2 is connected with 1 active link, giving it the same speed/bandwidth as the DUT \<\> ATE links
* Short SA times (SA to renew during test)
* Short IKE renegotiation times (IKE to renew during test)
* Short DPD times (2 second keepalive, 5 second hold-time)

Test:

* Generate traffic on ATE-\>DUT1 Ports at line-rate, which will saturate the DUT1 \<\> DUT2 link with traffic
* Run for a few minutes, with SA and IKE renewing at least once

Verify:

* Tunnel packet loss consistent with loss expected due to congestion (from adding ipsec headers)
* No loss of the tunnel; SA & IKE renewals not impacted; DPD not detecting any problem


## Flow Definitions

  * *MTU-RANGE:* Use 64, 128, 256, 512, 1024, 1500, 4500, and MAX MTU bytes frame size.

## Canonical OC for MACsec configuration
 
```json
{
    "macsec": {
        "interfaces": {
            "interface": [
                {
                    "config": {
                        "enable": true,
                        "name": "Ethernet12/1",
                        "replay-protection": 64
                    },
                    "mka": {
                        "config": {
                            "key-chain": "my_macsec_keychain",
                            "mka-policy": "must_secure_policy"
                        }
                    },
                    "name": "Ethernet12/1"
                },
                {
                    "config": {
                        "enable": true,
                        "name": "Ethernet11/1",
                        "replay-protection": 64
                    },
                    "mka": {
                        "config": {
                            "key-chain": "my_macsec_keychain",
                            "mka-policy": "must_secure_policy"
                        }
                    },
                    "name": "Ethernet11/1"
                }
            ]
        },
        "mka": {
            "policies": {
                "policy": [
                    {
                        "config": {
                            "confidentiality-offset": "0_BYTES",
                            "include-icv-indicator": true,
                            "include-sci": true,
                            "key-server-priority": 15,
                            "macsec-cipher-suite": [
                                "GCM_AES_XPN_256"
                            ],
                            "name": "must_secure_policy",
                            "sak-rekey-interval": 28800,
                            "security-policy": "MUST_SECURE"
                        },
                        "name": "must_secure_policy"
                    },
                    {
                        "config": {
                            "confidentiality-offset": "0_BYTES",
                            "include-icv-indicator": true,
                            "include-sci": true,
                            "key-server-priority": 15,
                            "macsec-cipher-suite": [
                                "GCM_AES_XPN_256"
                            ],
                            "name": "should_secure_policy",
                            "sak-rekey-interval": 28800,
                            "security-policy": "SHOULD_SECURE"
                        },
                        "name": "should_secure_policy"
                    }
                ]
            }
        }
    },
    "keychains": {
        "keychain": {
            "config": {
                "name": "my_macsec_keychain"
            },
            "keys": {
                "key": [
                    {
                        "config": {
                            "secret-key": "secret password/CAK",
                            "key-id": "key-id/CKN",
                            "crypto-algorithm": "AES_256_CMAC",
                            "send-lifetime": {
                                "config": {
                                    "start-time": "my_start_time",
                                    "end-time": "my_end_time"
                                }
                            },
                            "receive-lifetime": {
                                "config": {
                                    "start-time": "my_start_time",
                                    "end-time": "my_end_time"
                                }
                            }
                        }
                    }
                ]
            }
        }
    }
}
  ```

## Canonical OC

```json
{
}
  ```

## OpenConfig Path and RPC Coverage

Monitoring for these tunnels must be available via OC and/or standard native YANG modeling.

```yaml
paths:
  # TODO
  # /security/ipsec/profiles/profile/config/mode:
  # /security/ipsec/profiles/profile/config/ike-policy:
  # /security/ipsec/profiles/profile/config/security-association:
  # /security/ipsec/profiles/profile/config/connection-type:
  # /security/ipsec/profiles/profile/config/shared-key:
  # /security/ipsec/profiles/profile/dpd:
  # /security/ipsec/profiles/profile/dpd/config/enabled:
  # /security/ipsec/profiles/profile/dpd/config/keepalive:
  # /security/ipsec/profiles/profile/dpd/config/hold-time:
  # /security/ipsec/profiles/profile/dpd/config/action:
  # /security/ike/policies/policy/config/ike-lifetime:
  # /security/ike/policies/policy/config/integrity:
  # /security/ike/policies/policy/config/dh-group:
  # /security/ike/policies/policy/config/encryption:
  # /security/ike/policies/policy/config/version:
  # /interfaces/interface/tunnel/config/mode:
  # /interfaces/interface/tunnel/config/ipsec-profile:
  # /security/ike/security-associations/security-association/config/sa-lifetime:
  # /security/ike/security-associations/security-association/config/esp-encryption:
  # /security/ike/security-associations/security-association/config/esp-integrity:
  # /security/ike/security-associations/security-association/config/pfs-dh-group:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  ```

## Required DUT platform

FFF
