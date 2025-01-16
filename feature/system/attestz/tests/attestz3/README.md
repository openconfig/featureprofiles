# attestz-3: Validate post-install re-attestation

## Summary

In TPM enrollment workflow the switch owner verifies device's Initial Attestation Key (IAK) and Initial DevID (IDevID) certificates (signed by the switch vendor CA) and installs/rotates owner IAK (oIAK) and owner IDevID (oIDevID) certificates (signed by switch owner CA). In TPM attestation workflow switch owner verifies that the device's end-to-end boot state (bootloader, OS, secure boot policy, etc.) matches owner's expectations.

## Procedure

Test should verify all success and failure/corner-case scenarios for TPM enrollment and attestation workflows that are specified in [attestz Readme](https://github.com/openconfig/attestz/blob/main/README.md).

TPM enrollment workflow consists of two APIs defined in openconfig/attestz/blob/main/proto/tpm_enrollz.proto: `GetIakCert` and `RotateOIakCert`.
TPM attestation workflow consists of a single API defined in openconfig/attestz/blob/main/proto/tpm_attestz.proto: `Attest`.
The tests should comprehensively cover the behavior for all three APIs when used both separately and sequentially.
Finally, the tests should cover both initial install/bootstrapping, oIAK/oIDevID rotation and post-install re-attestation workflows.

## Test Setup

1. Switch vendor provisioned the device with IAK and IDevID certs following TCG spec [Section 5.2](https://trustedcomputinggroup.org/wp-content/uploads/TPM-2p0-Keys-for-Device-Identity-and-Attestation_v1_r12_pub10082021.pdf#page=20) and [Section 6.2](https://trustedcomputinggroup.org/wp-content/uploads/TPM-2p0-Keys-for-Device-Identity-and-Attestation_v1_r12_pub10082021.pdf#page=30).
2. The device successfully completed the bootz workflow where it obtained and applied all configurations/credentials/certificates and booted into the right OS image.
3. Device is serving `enrollz` and `attestz` gRPC endpoints.

### attestz-3: Validate post-install re-attestation

The test validates that the device completes TPM attestation after initial bootstrapping when the device is already handling production traffic and has already been provisioned with oIAK cert and owner-issued mTLS credentials/certs to communicate with owner infrastructure.

| ID          | Case            | Result |
| ----------- | ----------------| ------ |
| attestz-3.1 | Successful post-install re-attestation relying an owner-issued mTLS cert | Device passed attestation for all control cards relying on the latest oIAK and mTLS certs |
| attestz-3.2 | Two re-attestations separated by a device reboot result in the same PCR values, but different PCR Quote (due a different random nonce in `AttestRequest`) | Device passed multiple re-attestations separated by a reboot for all control cards relying on the latest oIAK and mTLS certs |
| attestz-3.2 | When an active control card becomes unavailable, standby control card becomes active and can successfully complete re-attestation | Standby control card passed re-attestation after an active control card failure, relying on the latest oIAK and mTLS certs|
| attestz-3.3 | Device is unable to authenticate switch owner (e.g. no suitable TLS trust bundle) during attestation | `Attest` returns authentication failure error |

1. Execute "Initial Install" workflow.
2. Provision the device with switch owner mTLS credentials (separate key pair and cert for each control card).
3. Call `Attest` for active and standby control cards and ensure they use the new mTLS cert for TLS connection and the latest oIAK for attestation.
4. Do the same verification of attestation responses as in "Initial Install" workflow.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
rpcs:
   gnsi:
      certz.v1.Certz.Rotate:
   gnoi:
     system.System.Reboot:
     system.System.SwitchControlProcessor:
```