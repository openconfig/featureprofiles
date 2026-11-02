# MSEC-1.1: MACsec Configuration and Verification (DUT-to-DUT)

## Summary

Ensure that network devices successfully negotiate and establish MACsec Secure Entities across two physical interfaces, validating control plane session formation, cryptographic key handling, and hardware-accelerated packet encryption/decryption.

## Testbed type

* `dut_dut.testbed`

## Procedure

### Test environment setup

* Connect two DUT devices directly via dedicated test interfaces.
* Configure test user credentials and IP addressing on both interfaces.

### MSEC-1.1.1 - VerifyStatusAndCknReported

* Step 1 - Verify that no MACsec status or CKN is reported on the interfaces of both DUTs before configuration.
* Step 2 - Configure MACsec on both DUTs with the following parameters:
    *   Enable MAC security and create a profile named `must_secure`.
    *   Set cipher to `aes256-gcm`.
    *   Set the shared key (CKN and CAK) to match on both devices.
    *   Apply IP addressing and the `must_secure` profile to the dedicated test interfaces.

#### Canonical OC

```json
{}
```

* Step 3 - Wait for the MACsec session to establish.
* Step 4 - Validate that the operational status transitions to `Secured`, the CKN matches the configured value, and control plane packet counters (`tx-pkts-ctrl` and `rx-pkts-ctrl`) are non-zero on both DUTs.

### MSEC-1.1.2 - RxUnrecognizedCkn

* Step 1 - Configure MACsec on both DUTs with mismatched Key IDs (CKNs) but identical Secret Keys (CAKs).
* Step 2 - Wait and verify that the receiving device discards the incoming control frames and increments the `rx-unrecognized-ckn` counter due to the key ID mismatch.

### MSEC-1.1.3 - RxBadIcvPkts

* Step 1 - Configure MACsec on both DUTs with matching CKNs but mismatched Secret Keys (CAKs).
* Step 2 - Generate traffic across the link by sending gNOI Pings from DUT1 to DUT2's interface IP with a packet count of 15.
* Step 3 - Verify that the receiving device fails to validate the Integrity Check Value (ICV) of the encrypted packets and increments the `rx-badicv-pkts` counter.

### MSEC-1.1.4 - VerifyRemainingCountersReported

* Step 1 - Establish a valid MACsec session between the DUTs with matching credentials.
* Step 2 - Validate that telemetry successfully exposes and reports counters for `tx-pkts-err-in`, `tx-pkts-dropped`, and `rx-pkts-dropped`.

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