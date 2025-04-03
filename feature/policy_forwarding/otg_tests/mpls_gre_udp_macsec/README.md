# PF-1.17 - MPLSoGRE and MPLSoGUE MACsec 

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
    * Define first Policy(1) to cover must-secure scenario  
    * Define second Policy(2) to cover should-secure scenario
    * Define 5 pre-shared keys (with overlapping time of 15 minutes) each for both Policy(1) and Policy(2)
    * Each pre-shared key mush have a unique Connectivity Association Key Name(CKN) and Connectivity Association Key(CAK)
    * Set CKN as encrypted/hidden in the running configuration
    * Use 256 bit cipher GCM-AES-256-XPN and an associated 64 char CAK-CKN pair
    * Set Key server priority: 15
    * Set Security association key rekey interval: 28800 seconds
    * Set MACsec confidentiality offset: 0
    * Set Replay Protection Window size: 64
    * Set ICV enabled:True
    * Set SCI enabled:True
    * Set Out of sequence protection window size:64
    * Set maximum value of Association Number: 3 (NOTE: This is currently not configurable)

## PF-1.17.2: Verify PF MPLSoGRE and MPLSoGUE traffic forwarding with MACSec must-secure policy
* Generate bidirectional traffic as highlighted in the test environment setup section:
    * MPLSoGRE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * MPLSoGUE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * IPV4 and IPV6 traffic from ATE ports 1,2
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.
* Enable must secure policy (Policy(1)) on both interfaces

Verify:
* Verify that MACsec sessions are up 
* No packet loss while forwarding at line rate
* Traffic equally load-balanced across bundle interfaces in both directions
* Header fields are as expected in both directions
* Traffic is dropped (100 percent) when the must-secure MACSec sessions are down by disabling MACsec on ATE ports


## PF-1.17.3: Verify PF MPLSoGRE and MPLSoGUE traffic forwarding with MACSec should-secure policy
* Generate bidirectional traffic as highlighted in the test environment setup section:
    * MPLSoGRE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * MPLSoGUE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * IPV4 and IPV6 traffic from ATE ports 1,2
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.
* Enable should secure policy (Policy(2)) on both interfaces

Verify:
* Verify that MACsec sessions are up 
* No packet loss while forwarding at line rate
* Traffic equally load-balanced across bundle interfaces in both directions
* Header fields are as expected in both directions
* Traffic is not dropped when the should-secure MACSec sessions are down by disabling MACsec on ATE ports

## PF-1.10.3: Verify MACSec key rotation
* Generate bidirectional traffic as highlighted in the test environment setup section:
    * MPLSoGRE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * MPLSoGUE traffic with IPV4 and IPV6 payloads from ATE ports 3,4,5,6
    * IPV4 and IPV6 traffic from ATE ports 1,2
* Use 64, 128, 256, 512, 1024.. MTU bytes frame size.
* Enable must secure policy (Policy(1)) on both interfaces

Verify:
* Verify that MACsec sessions are up 
* No packet loss while forwarding at line rate
* Traffic equally load-balanced across bundle interfaces in both directions
* Header fields are as expected in both directions
* No packet loss when keys one through five expires as configured
* 100 percent packet loss after all the keys configured expires

## Canonical OpenConfig for MACsec configuration
TODO: Finalize and update the below paths after the review and testing on any vendor device.
 
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
              "key-chain": "ci_2114275276238",
              "mka-policy": "ci_aesxpn256"
            }
          },
          "name": "Ethernet12/1"
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
              "name": "ci_aesxpn256",
              "sak-rekey-interval": 28800,
              "security-policy": "MUST_SECURE"
            },
            "name": "ci_aesxpn256"
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
              "name": "ci_aesxpn256_fo",
              "sak-rekey-interval": 28800,
              "security-policy": "SHOULD_SECURE"
            },
            "name": "ci_aesxpn256_fo"
          }
        ]
      }
    }
  },
  "keychains": {
    "keychain": {
      "key": [
        {
          "config": {
            "secret-key": "sercret password/CAK",
            "key-id": "key-id/CKN",
            "crypto-algorithm": "AES_256_CMAC"
            "send-lifetime": {
              "config": {
                "start-time": "my_start_time",
                "end-time": "my_end_time"
              }
            },
            "receive-lifetime: {
              "config": {
                "start-time": "my_start_time",
                "end-time": "my_end_time"
              },
            }
          }
        }
      ]
    }
   }
} 
  ```

## OpenConfig Path and RPC Coverage
TODO: Finalize and update the below paths after the review and testing on any vendor device.

```yaml
paths:
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
#/macsec/interfaces/interface/[Physical Interface]/state/status:
#/macsec/interfaces/interface/[Physical Interface]/state/ckn:
#/macsec/mka/policies/policy/config/security-policy:
#/macsec/interfaces/interface/[Physical Interface]/state/counters/rx-pkts-ctrl:
#/macsec/interfaces/interface/[Physical Interface]/state/counters/rx-pkts-data:
#/macsec/interfaces/interface/[Physical Interface]/state/counters/rx-pkts-dropped:
#/macsec/interfaces/interface/[Physical Interface]/state/counters/rx-pkts-err-in:
#/macsec/interfaces/interface/[Physical Interface]/state/counters/tx-pkts-ctrl:
#/macsec/interfaces/interface/[Physical Interface]/state/counters/tx-pkts-data:
#/macsec/interfaces/interface/[Physical Interface]/state/counters/tx-pkts-dropped:
#/macsec/interfaces/interface/[Physical Interface]/state/counters/tx-pkts-err-in:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

FFF
