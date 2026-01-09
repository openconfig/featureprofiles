# PF-1.17: MPLSoGRE and MPLSoGUE MACsec 

## Summary
This test verifies MACSec with MPLSoGRE and MPLSoGUE IP encap and decap traffic on the test device.

## Testbed type
* [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure
### Test environment setup

```
DUT has 3 aggregate interfaces.

                         |         | --eBGP-- | ATE Ports 3,4 |
    [ ATE Ports 1,2 ]----|   DUT   |          |               |
                         |         | --eBGP-- | ATE Port 5,6  |
```

Test uses aggregate 802.3ad bundled interfaces (Aggregate).

* Send bidirectional traffic:
  * IP to Encap Traffic: The IP to Encap traffic is from ATE Ports [1,2] to ATE Ports [3,4,5,6]. 

  * Encap to IP Traffic: The Encap traffic to IP traffic is from ATE Ports [3,4,5,6] to ATE Ports [1,2].

Please refer to the MPLSoGRE [encapsulation PF-1.14](feature/policy_forwarding/otg_tests/mpls_gre_ipv4_encap_test/README.md) and [decapsulation PF-1.12](feature/policy_forwarding/otg_tests/mpls_gre_ipv4_decap_test/README.md) READMEs for additional information on the test traffic environment setup.

## PF-1.17.1: Generate DUT Configuration
### MACsec
* Configure MACsec Static Connectivity Association Key (CAK) Mode on both ends of the aggregate bundle links connecting ATE ports 1,2 and DUT:
    * Define first Policy(1) to cover must-secure scenario, as defined below
    * Define second Policy(2) to cover should-secure scenario, as defined below
    * Define 5 pre-shared keys (with overlapping time of 1 minute and lifetime of 2 minutes) for both Policy(1) and Policy(2)
    * Each pre-shared key must have a unique Connectivity Association Key Name(CKN) and Connectivity Association Key(CAK)
    * Set CAK as encrypted/hidden in the running configuration
    * Use 256 bit cipher GCM-AES-256-XPN and an associated 64 char CAK-CKN pair
    * Set Key server priority: 15
    * Set Security association key rekey interval: 30 seconds (test only)
    * Set MACsec confidentiality offset: 0
    * Set Replay Protection Window (out-of-sequence protection) size: 64
    * Include ICV indicator:True
    * Include SCI:True
    * Set maximum value of Association Number: 3 (NOTE: This is currently not configurable and is not included in the test cases)

## PF-1.17.2: Verify PF MPLSoGRE and MPLSoGUE traffic forwarding with MACSec must-secure policy
* Generate bidirectional traffic as highlighted in the test environment setup section:
    * MPLSoGRE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * MPLSoGUE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * IPV4 and IPV6 traffic from ATE ports 1,2
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.
* Generate config to attach must secure policy (Policy(1)) on both interfaces ATE ports 1,2 and DUT

Verify:
* Verify that MACsec sessions are up 
* No packet loss while forwarding at line rate
* Traffic equally load-balanced across bundle interfaces in both directions
* Header fields are as expected in both directions
* Traffic is dropped (100 percent) when the must-secure MACSec sessions are down by changing a key on one side to a mismatch & forcing renegotiation on ATE ports

## PF-1.17.3: Verify PF MPLSoGRE and MPLSoGUE traffic forwarding with MACSec should-secure policy
* Generate bidirectional traffic as highlighted in the test environment setup section:
    * MPLSoGRE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * MPLSoGUE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * IPV4 and IPV6 traffic from ATE ports 1,2
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.
* Generate config to attach should secure policy (Policy(2)) on both interfaces ATE ports 1,2 and DUT

Verify:
* Verify that MACsec sessions are up 
* No packet loss while forwarding at line rate
* Traffic equally load-balanced across bundle interfaces in both directions
* Header fields are as expected in both directions
* Traffic is not dropped when the should-secure MACSec sessions are down by changing a key on one side to a mismatch & forcing renegotiation on ATE ports

## PF-1.17.4: Verify MACSec key rotation
* Generate bidirectional traffic as highlighted in the test environment setup section:
    * MPLSoGRE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * MPLSoGUE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * IPV4 and IPV6 traffic from ATE ports 1,2
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.
* Enable must secure policy (Policy(1)) on both interfaces ATE ports 1,2 and DUT

Verify:
* Verify that MACsec sessions are up 
* No packet loss while forwarding at line rate
* Traffic equally load-balanced across bundle interfaces in both directions
* Header fields are as expected in both directions
* No packet loss when keys one through five expires as configured
* 100 percent packet loss after all the keys configured expires

## PF-1.17.5: Verify standard Security-Association timer
* Generate bidirectional traffic as highlighted in the test environment setup section:
    * MPLSoGRE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * MPLSoGUE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * IPV4 and IPV6 traffic from ATE ports 1,2
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.
* Enable must secure policy (Policy(1)) on both interfaces ATE ports 1,2 and DUT
* Set the security association key rekey interval to 28800 seconds

Verify:
* Verify the SAK key value is accepted by the DUT
* Verify that MACsec sessions are up
* No packet loss while forwarding at line rate

## Definitions
  * *must-secure:* All non-macsec-control packets must be encrypted. On transmit (tx), packets are dropped if encryption is not used or if keys have expired. On receive (rx), unencrypted packets that should be secure or encrypted with expired keys are dropped.
  * *should-secure:* Unencrypted packets are permitted. On receive (rx), it's recommended but not required to drop unencrypted packets if a macsec session is active. On transmit (tx), it's recommended but not required to send unencrypted packets if macsec session negotiation has failed.

## Canonical OC
 
```json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "name": "Ethernet1/1"
        },
        "name": "Ethernet1/1"
      },
      {
        "config": {
          "name": "Ethernet1/2"
        },
        "name": "Ethernet1/2"
      }
    ]
  },
  "keychains": {
    "keychain": [
      {
        "config": {
          "name": "keychain1"
        },
        "keys": {
          "key": [
            {
              "config": {
                "crypto-algorithm": "AES_256_CMAC",
                "key-id": "0xabcd111122223333444455556666777788889999000011112222333344445555",
                "secret-key": "ad4rf10kn85fc0adk5dfcsnr1or4cm08q"
              },
              "key-id": "0xabcd111122223333444455556666777788889999000011112222333344445555"
            }
          ]
        },
        "name": "keychain1"
      }
    ]
  },
  "macsec": {
    "interfaces": {
      "interface": [
        {
          "config": {
            "enable": true,
            "name": "Ethernet1/1",
            "replay-protection": 64
          },
          "mka": {
            "config": {
              "key-chain": "keychain1",
              "mka-policy": "must_secure"
            }
          },
          "name": "Ethernet1/1"
        },
        {
          "config": {
            "enable": true,
            "name": "Ethernet1/2",
            "replay-protection": 64
          },
          "mka": {
            "config": {
              "key-chain": "keychain1",
              "mka-policy": "must_secure"
            }
          },
          "name": "Ethernet1/2"
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
              "name": "must_secure",
              "sak-rekey-interval": 30
            },
            "name": "must_secure"
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage
TODO: Finalize and update the below paths after the review and testing on any vendor device.

```yaml
paths:
 # TODO:  /macsec/mka/config/security-policy  MUST_SECURE,SHOULD_SECURE
 /macsec/interfaces/interface/state/name:
 /macsec/interfaces/interface/state/enable:
 /macsec/interfaces/interface/state/replay-protection:
 /macsec/interfaces/interface/state/counters/tx-untagged-pkts:
 /macsec/interfaces/interface/state/counters/rx-untagged-pkts:
 /macsec/interfaces/interface/state/counters/rx-badtag-pkts:
 /macsec/interfaces/interface/state/counters/rx-unknownsci-pkts:
 /macsec/interfaces/interface/state/counters/rx-nosci-pkts:
 /macsec/interfaces/interface/state/counters/rx-late-pkts:
 /macsec/interfaces/interface/scsa-tx/scsa-tx/state/sci-tx:
 /macsec/interfaces/interface/scsa-tx/scsa-tx/state/counters/sc-auth-only:
 /macsec/mka/state/counters/out-mkpdu-errors:
 /macsec/mka/state/counters/in-mkpdu-icv-verification-errors:
 /macsec/mka/state/counters/in-mkpdu-validation-errors:
 /macsec/mka/state/counters/in-mkpdu-bad-peer-errors:
 /macsec/mka/state/counters/in-mkpdu-peer-list-errors:
 /macsec/mka/state/counters/sak-generation-errors:
 /macsec/mka/state/counters/sak-hash-errors:
 /macsec/mka/state/counters/sak-encryption-errors:
 /macsec/mka/state/counters/sak-decryption-errors:
 /macsec/mka/state/counters/sak-cipher-mismatch-errors:
 /macsec/interfaces/interface/name:
 /macsec/interfaces/interface/config/name:
 /macsec/interfaces/interface/config/enable:
 /macsec/interfaces/interface/config/replay-protection:
 /macsec/mka/policies/policy/config/name:
 /macsec/mka/policies/policy/config/key-server-priority:
 /macsec/mka/policies/policy/config/confidentiality-offset:
 /macsec/mka/policies/policy/config/delay-protection:
 /macsec/mka/policies/policy/config/include-icv-indicator:
 /macsec/mka/policies/policy/config/include-sci:
 /macsec/mka/policies/policy/config/sak-rekey-interval:
 /macsec/mka/policies/policy/config/sak-rekey-on-live-peer-loss:
 /macsec/mka/policies/policy/config/use-updated-eth-header:
 /macsec/mka/policies/policy/config/macsec-cipher-suite:
 /keychains/keychain/config/name:
 /keychains/keychain/keys/key/config/key-id:
 /keychains/keychain/keys/key/config/secret-key:
 /keychains/keychain/keys/key/config/crypto-algorithm:
 /keychains/keychain/keys/key/send-lifetime/config/start-time:
 /keychains/keychain/keys/key/send-lifetime/config/end-time:
 /keychains/keychain/keys/key/send-lifetime/config/send-and-receive:
 /keychains/keychain/keys/key/receive-lifetime/config/start-time:
 /keychains/keychain/keys/key/receive-lifetime/config/end-time:

#TODO: Add following OC paths
#/macsec/interfaces/interface/state/status:
#/macsec/interfaces/interface/state/ckn:
#/macsec/mka/policies/policy/config/security-policy:
#/macsec/interfaces/interface/state/counters/rx-pkts-ctrl:
#/macsec/interfaces/interface/state/counters/rx-pkts-data:
#/macsec/interfaces/interface/state/counters/rx-pkts-dropped:
#/macsec/interfaces/interface/state/counters/rx-pkts-err-in:
#/macsec/interfaces/interface/state/counters/tx-pkts-ctrl:
#/macsec/interfaces/interface/state/counters/tx-pkts-data:
#/macsec/interfaces/interface/state/counters/tx-pkts-dropped:
#/macsec/interfaces/interface/state/counters/tx-pkts-err-in:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

FFF
