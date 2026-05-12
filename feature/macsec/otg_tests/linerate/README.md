# MSEC-1.2: MACsec Line Rate Performance Verification

## Summary

This test verifies that the DUT can maintain line-rate performance for MACsec-encrypted traffic. It ensures that the encryption and decryption processes do not introduce packet loss, excessive latency, or throughput degradation across a range of frame sizes (64B to Jumbo) and cipher suites (AES-256-GCM).

## Testbed type

* `topologies/atedutdutate.testbed`

```
                                                                                              
         ┌──────────┐          ┌──────────┐           ┌──────────┐           ┌──────────┐     
         │          │          │          │           │          │           │          │     
         │          │   100G   │          │   100G    │          │   100G    │          │     
         │         1├──────────│1        2├───────────┤2        1├───────────┤2         │     
         │          │          │          │           │          │           │          │     
         │   ATE1   │   400G   │   DUT11  │   400G    │   DUT2   │   400G    │   ATE    │     
         │         3├──────────│3        4├───────────┤4        3├───────────┤4         │     
         │          │          │          │           │          │           │          │     
         │          │          │          │           │          │           │          │     
         └──────────┘          └──────────┘           └──────────┘           └──────────┘     
                                                                                              
 
```

## Procedure

### Test environment setup

* Connect the ATE to the DUT1 and DUT2,respectively, as per the testbed layout using 1x100G and 1x400G interfaces. Also connect DUT1 and DUT2 using 1x100G and 1x400G interface. All the below tests need to send traffic between a pair of 100G ports (ate:port1->dut1:port1->dut2:port2->ate:port2) or a pair of 400G ports (ate:port3->dut1:port3->dut2:port4->ate:port4). No oversubscription being tested as part of these tests.
* All links between DUT1 and DUT2 will run MACSec. DUT1 will receive unencrypted packets from ATE1 and transmit MACSec encrypted traffic towards DUT2.

### MACsec
* Configure MACsec Static Connectivity Association Key (CAK) Mode on both ends of the physical links connecting DUT1 and DUT2:
    * Define the Policy(1) to cover must-secure scenario, as defined below
    * Define 5 pre-shared keys (with overlapping time of 1 minute and lifetime of 2 minutes) for Policy(1)
    * Each pre-shared key must have a unique Connectivity Association Key Name(CKN) and Connectivity Association Key(CAK)
    * Set CAK as encrypted/hidden in the running configuration
    * Use 256 bit cipher GCM-AES-256-XPN and an associated 64 char CAK-CKN pair
    * Set Key server priority: 15
    * Set Security association key rekey interval: 30 seconds (test only)
    * Set MACsec confidentiality offset: 0
    * Set Replay Protection Window (out-of-sequence protection) size: 64
    * Include ICV indicator: True
    * Include SCI: True
    * Set maximum value of Association Number: 3 (NOTE: This is currently not configurable and is not included in the test cases)

### MSEC-1.2.1 - Line Rate Performance with 64B Frames

* Step 1 - Configure MACsec on the DUT with `GCM_AES_256` cipher suite and a valid keychain.
* Step 2 - Generate 100% line-rate traffic from the ATE with fixed 64-byte frames.
* Step 3 - Verify that no packet loss occurs over a 10-minute duration.
* Step 4 - Validate that throughput matches the expected line rate for 64B frames (accounting for MACsec overhead).

#### Canonical OC

```json
{
  "keychains": {
    "keychain": [
      {
        "config": {
          "name": "macsec_keychain"
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
        "name": "macsec_keychain"
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
              "key-chain": "macsec_keychain",
              "mka-policy": "line_rate_policy"
            }
          },
          "name": "Ethernet1/1"
        },
        {
          "config": {
            "enable": true,
            "name": "Ethernet1/3",
            "replay-protection": 64
          },
          "mka": {
            "config": {
              "key-chain": "macsec_keychain",
              "mka-policy": "line_rate_policy"
            }
          },
          "name": "Ethernet1/3"
        },
        {
          "config": {
            "enable": true,
            "name": "Ethernet2/1",
            "replay-protection": 64
          },
          "mka": {
            "config": {
              "key-chain": "macsec_keychain",
              "mka-policy": "line_rate_policy"
            }
          },
          "name": "Ethernet2/1"
        },
        {
          "config": {
            "enable": true,
            "name": "Ethernet2/2",
            "replay-protection": 64
          },
          "mka": {
            "config": {
              "key-chain": "macsec_keychain",
              "mka-policy": "line_rate_policy"
            }
          },
          "name": "Ethernet2/2"
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
              "name": "line_rate_policy",
              "sak-rekey-interval": 30
            },
            "name": "line_rate_policy"
          }
        ]
      }
    }
  }
}
```

### MSEC-1.2.2 - Line Rate Performance with IMIX Traffic
* Step 1 - Maintain the MACsec configuration from MSEC-1.2.1.
* Step 2 - Generate line-rate traffic using an IMIX profile (e.g., a mix of 64B, 570B, and 1518B).
* Step 3 - Verify zero packet loss and consistent throughput.

### MSEC-1.2.3 - Line Rate Performance with Jumbo Frames
* Step 1 - Configure the DUT1<->DUT2 interfaces to support a MTU of 9216 bytes.
* Step 2 - Generate line-rate traffic with 9000-byte frames.
* Step 3 - Verify that the hardware correctly handles large encrypted payloads without fragmentation or loss.

## OpenConfig Path and RPC Coverage

```json
paths:
  config:
  /macsec/interfaces/interface/config/enable:
  /macsec/interfaces/interface/config/replay-protection:
  /macsec/mka/policies/policy/config/name
  /macsec/mka/policies/policy/config/macsec-cipher-suite:
  /macsec/mka/policies/policy/config/confidentiality-offset:
  /macsec/mka/policies/policy/config/key-server-priority
  /macsec/mka/policies/policy/config/sak-rekey-interval
  
  /keychains/keychain/keys/key/config/secret-key:
  /keychains/keychain/keys/key/config/crypto-algorithm:
  
  state:
  /macsec/interfaces/interface/state/counters/rx-badtag-pkts
  /macsec/interfaces/interface/state/counters/rx-late-pkts
  /macsec/interfaces/interface/state/counters/rx-nosci-pkts
  /macsec/interfaces/interface/state/counters/rx-unknownsci-pkts
  /macsec/interfaces/interface/state/counters/rx-untagged-pkts
  /macsec/interfaces/interface/state/counters/tx-untagged-pkts
  
  /macsec/interfaces/interface/mka/state/counters/in-cak-mkpdu
  /macsec/interfaces/interface/mka/state/counters/in-mkpdu
  /macsec/interfaces/interface/mka/state/counters/in-sak-mkpdu
  /macsec/interfaces/interface/mka/state/counters/out-cak-mkpdu
  /macsec/interfaces/interface/mka/state/counters/out-mkpdu
  /macsec/interfaces/interface/mka/state/counters/out-sak-mkpdu

  /macsec/mka/state/counters/in-mkpdu-bad-peer-errors	
  /macsec/mka/state/counters/in-mkpdu-icv-verification-errors	
  /macsec/mka/state/counters/in-mkpdu-peer-list-errors	
  /macsec/mka/state/counters/in-mkpdu-validation-errors	
  /macsec/mka/state/counters/out-mkpdu-errors	
  /macsec/mka/state/counters/sak-cipher-mismatch-errors	
  /macsec/mka/state/counters/sak-decryption-errors	
  /macsec/mka/state/counters/sak-encryption-errors	
  /macsec/mka/state/counters/sak-generation-errors	
  /macsec/mka/state/counters/sak-hash-errors


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
      sampled: true
```

### Required DUT platform
* FFF - Fixed Form Factor
* MFF - Modular Form Factor
