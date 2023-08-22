# attestz: General enrollz and attestz tests

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

### Successful Initial Install

1. Call `GetIakCert` for an active control card with correct `ControlCardSelection`.
2. Verify that correct IDevID cert was used for establishing TLS session:
   - Cert structure matches TCG specification [Section 8](https://trustedcomputinggroup.org/wp-content/uploads/TPM-2p0-Keys-for-Device-Identity-and-Attestation_v1_r12_pub10082021.pdf#page=55).
   - Cert is not expired.
   - Cert is signed by switch vendor CA.
   - Cert is tied to the active control card.
3. Verify IAK cert:
   - Cert structure matches TCG spec (similar to IDevID above).
   - Cert is not expired.
   - Cert is signed by switch vendor CA.
   - Cert is tied to the active control card.
   - IAK and IDevID cert contain the same device serial number field.
4. Verify that the device returned the correct `ControlCardVendorId` with all fields populated.
5. Issue owner IAK (oIAK) and owner IDevID (oIDevID) certs, which are based on the same underlying public keys, have the same structure and fields, but are signed by a different - owner - CA.
6. Call `RotateOIakCert` to store newly issued oIAK and oIDevID certs and verify successful response.
7. Call `GetIakCert` for a standby control card with correct `ControlCardSelection`.
8. Repeat step (2) (TLS session will be secured by active control card's IDevID) and verify IDevID cert of standby control card was specified in the response payload.
9. Repeat steps (3-6) for the standby control card.
10. Call `Attest` for active control card with correct `ControlCardSelection`, random nonce, hash algo of choice (all should be supported and tested) and all PCR indices.
11. Verify that the correct oIDevID cert was used for establishing TLS session.
12. Verify that the device returned the correct `ControlCardVendorId` with all fields populated.
13. Verify oIAK cert is the same as the one installed earlier.
14. Verify all `pcr_values` match expectations.
15. Verify `quote_signature` signature with oIAK cert.
16. Use `pcr_values` and `tpms_quote_info` to recompute PCR Quote digest and verify that it matches the one used in `quote_signature`.
17. Call `Attest` for standby control card with correct `ControlCardSelection`, random nonce, hash algo of choice (all should be supported and tested) and all PCR indices.
18. Verify that the oIDevID cert of active control card was used for establishing TLS session and verify that oIDevID cert of standby control card was specified in the response payload.
19. Repeat steps (12-16) for the standby control card.

### Successful oIAK/oIDevID rotation

1. Execute "Initial Install" workflow.
2. Issue new oIAK and oIDevID certs for active control card, call `RotateOIakCert` to store those on the right card and verify successful response.
3. Issue new oIAK and oIDevID certs for standby control card, call `RotateOIakCert` to store those on the right card and verify successful response.
4. Call `Attest` for active and standby control cards and ensure they use the latest oIAK for attestation and, if there is no owner-provisioned TLS cert installed, use latest oIDevID for TLS session (both oIDevID-based and mTLS-cert-based scenarios should be tested).
5. Do the same verification of attestation responses as in "Initial Install" workflow.

### Successful post-install re-attestation

1. Execute "Initial Install" workflow.
2. Provision the device with switch owner mTLS credentials.
3. Call `Attest` for active and standby control cards and ensure they use the new mTLS cert for TLS connection and the latest oIAK for attestation.
4. Do the same verification of attestation responses as in "Initial Install" workflow.

### Failure Scenarios

1. IAK or IDevID are not present on the device.
2. Bad request for `GetIakCertRequest`, `RotateOIakCertRequest` and  `AttestRequest`. Examples: `ControlCardSelection control_card_selection` is not specified or `control_card_id.role = 0`. Invalid `control_card_id.serial` or `control_card_id.slot`.
3. Store oIAK/oIDevId certs that have different underlying IAK/IDevID pub keys or intended for other control card.
4. Ensure that reboots do not wipe out oIAK and oIDevID certs.
5. Ensure that full factory reset deletes oIAK and oIDevID certs, but does not affect IAK and IDevID certs.
6. Ensure that reboots do not change PCR values, but random nonce affects PCR Quote.
7. Out of bound or repeated `pcr_indices` in `AttestRequest`.
8. Ensure that post-install `Attest` RPCs are secured by owner-installed mTLS cert if that is available on the device (as opposed to oIDevID).
9. Various RMA scenarios and cases where one of the control cards is unavailable.
