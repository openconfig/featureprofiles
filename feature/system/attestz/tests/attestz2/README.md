# attestz-2: Validate oIAK and oIDevID rotation

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

### attestz-2: Validate oIAK and oIDevID rotation

The test validates that the device can rotate oIAK and oIDevID certificates post-install.

| ID          | Case            | Result |
| ----------- | ----------------| ------ |
| attestz-2.1 | Successful oIAK and oIDevID cert rotation when no owner-issued mTLS cert is available on the device | Device obtained newly-rotated oIAK and oIDevID certs and passed attestation for all control cards relying on the new oIAK and oIDevID certs |
| attestz-2.2 | Successful oIAK and oIDevID cert rotation when owner-issued mTLS cert is available on the device | Device obtained newly-rotated oIAK and oIDevID certs and passed attestation for all control cards relying on the new oIAK and previously owner-issued mTLS cert |
| attestz-2.3 | Device is unable to authenticate switch owner (e.g. no suitable TLS trust bundle) during oIAK/oIDevID rotation | Both `GetIakCert` and `RotateOIakCert` return authentication failure error |

1. Execute "Initial Install" workflow.
2. Issue new oIAK and oIDevID certs for active control card, call `RotateOIakCert` to store those on the right card and verify successful response.
3. Issue new oIAK and oIDevID certs for standby control card, call `RotateOIakCert` to store those on the right card and verify successful response.
4. Call `Attest` for active and standby control cards and ensure they use the latest oIAK for attestation and, if there is no owner-provisioned TLS cert installed, use latest oIDevID for TLS session.
5. Do the same verification of attestation responses as in "Initial Install" workflow.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC paths used for test setup are not listed here.

```yaml
rpcs:
   gnsi:
      certz.v1.Certz.Rotate:
```