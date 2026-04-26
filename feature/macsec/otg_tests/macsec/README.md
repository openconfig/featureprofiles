# MSEC-1.1: MACsec Configuration and Verification (DUT-to-DUT)

## Summary

Ensure that network devices successfully negotiate and establish MACsec Secure Entities across two physical interfaces configured back-to-back, validating control plane session formation, cryptographic key handling, and hardware-accelerated packet encryption/decryption.

## Testbed type

* `dut_dut.testbed`

## Procedure

### Test environment setup

* Connect two DUT devices back-to-back via dedicated test interfaces.
* Configure test user credentials and IP addressing on both interfaces.

### MSEC-1.1.1 - VerifyStatusAndCknReported

* Step 1 - Generate DUT configuration

#### Canonical OC

```json
{}
```

* Step 2 - Push configuration to DUT and configure MACsec profiles (CKN / CAK).
* Step 3 - Send Traffic.
* Step 4 - Validate that status transitions to `Secured`, CKN exactly matches, and `tx-pkts-ctrl` / `rx-pkts-ctrl` are non-zero.

### MSEC-1.1.2 - RxUnrecognizedCkn

* Step 1 - Generate MACsec configuration with mismatched Key IDs (CKNs).
* Step 2 - Push configuration.
* Step 3 - Send Traffic.
* Step 4 - Verify that the receiving device discards the incoming control frames and increments the `rx-unrecognized-ckn` counter.

### MSEC-1.1.3 - RxBadIcvPkts

* Step 1 - Configure MACsec profiles with mismatched Secret Keys (CAKs).
* Step 2 - Push configuration.
* Step 3 - Send ICMP traffic unconditionally using a static ARP entry.
* Step 4 - Verify that the device fails to validate ICV and increments the `rx-badicv-pkts` counter.

### MSEC-1.1.4 - VerifyRemainingCountersReported

* Step 1 - Establish valid MACsec profiles across DUT interfaces.
* Step 2 - Push configuration.
* Step 3 - Send Traffic.
* Step 4 - Validate that telemetry successfully exposes and reports `tx-pkts-err-in`, `tx-pkts-dropped`, and `rx-pkts-dropped`.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # TODO: Migrate this test to use standard OpenConfig paths once they are widely supported across vendors.
  # Non-standard paths presently covered via Functional Translators:
  # /openconfig/macsec/interfaces/interface/state/status
  # /openconfig/macsec/interfaces/interface/state/ckn
  # /openconfig/macsec/interfaces/interface/state/counters/rx-unrecognized-ckn
  # /openconfig/macsec/interfaces/interface/state/counters/rx-badicv-pkts
  # /openconfig/macsec/interfaces/interface/state/counters/tx-pkts-ctrl
  # /openconfig/macsec/interfaces/interface/state/counters/rx-pkts-ctrl
  # /openconfig/macsec/interfaces/interface/state/counters/tx-pkts-err-in
  # /openconfig/macsec/interfaces/interface/state/counters/tx-pkts-dropped
  # /openconfig/macsec/interfaces/interface/state/counters/rx-pkts-dropped

rpcs:
  gnmi:
    gNMI.Get:
```

## Required DUT platform

* FFF - fixed form factor
* MFF - modular form factor