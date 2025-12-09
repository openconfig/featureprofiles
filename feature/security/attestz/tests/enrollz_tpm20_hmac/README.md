# enrollz-1: enrollz test for TPM 2.0 HMAC-based Enrollment flow

## Summary

This document outlines the functional network tests (FNTs) for the TPM 2.0 HMAC-based enrollment flow mainly for devices that lack a pre-provisioned Initial Device Identifier (IDevID) certificate (but supports all TPM 2.0 devices). This workflow uses a Hash-based Message Authentication Code (HMAC) challenge-response mechanism to securely verify the device's identity. The process provisions an owner Initial Attestation Key (oIAK) and an owner IDevID (oIDevID) after establishing a chain of trust from the Endorsement Key (EK) or Platform Primary Key (PPK), to the Initial Attestation Key (IAK), and finally to the IDevID.

The primary objective is to test the device's ability to correctly handle the enrollment sequence, including cryptographic operations, request validation, and error handling.


## Procedure

The test validates that the device correctly implements the server-driven TPM
enrollment workflow. The following test cases cover both the successful
enrollment path and various failure scenarios from the device's perspective.

| ID | Case | Expected Result |
| --- | --- | --- |
| enrollz-1.1 | Successful enrollment for a device that has an EK cert stored in the RoT database. | The device successfully completes the enrollment, obtains oIAK and oIDevID certs, and updates the SSL profile to use the new oIDevID cert. |
| enrollz-1.2 | Successful enrollment for a device that has a PPK public key stored in the RoT database. | The device successfully completes the enrollment, obtains oIAK and oIDevID certs, and updates the SSL profile to use the new oIDevID cert. |
| enrollz-1.3 | Enrollz service presents an invalid client certificate. | The device rejects the mTLS connection, resulting in a gRPC connection failure. |
| enrollz-1.4 | GetControlCardVendorIDRequest with missing control_card_selection. | The device returns an invalid request error. |
| enrollz-1.5 | GetControlCardVendorIDRequest with invalid control_card_selection. | The device returns an invalid request error. |
| enrollz-1.6 | ChallengeRequest with missing control_card_selection. | The device returns an invalid request error. |
| enrollz-1.7 | ChallengeRequest with invalid control_card_selection. | The device returns an invalid request error. |
| enrollz-1.8 | ChallengeRequest with invalid key type KEY_UNSPECIFIED. | The device returns an invalid request error. |
| enrollz-1.9 | ChallengeRequest with an empty hmac_pub_key in HMACChallenge. | The device returns an invalid request error. |
| enrollz-1.10 | ChallengeRequest with an empty duplicate field in HMACChallenge. | The device returns an invalid request error. |
| enrollz-1.11 | ChallengeRequest with an empty in_sym_seed field in HMACChallenge. | The device returns an invalid request error. |
| enrollz-1.12 | ChallengeRequest with a malformed hmac_pub_key in HMACChallenge. | The device's TPM fails to import the HMAC key, and the device returns an invalid request error. |
| enrollz-1.13 | ChallengeRequest with a malformed duplicate in HMACChallenge. | The device's TPM fails to import the HMAC key, and the device returns an invalid request error. |
| enrollz-1.14 | ChallengeRequest with a malformed in_sym_seed in HMACChallenge. | The device's TPM fails to import the HMAC key, and the device returns an invalid request error. |
| enrollz-1.15 | ChallengeRequest with correct key type KEY_EK but the challenge HMAC is wrapped with an invalid EK. | The device returns an invalid request error. |
| enrollz-1.16 | ChallengeRequest with correct key type KEY_PPK but the challenge HMAC is wrapped with an invalid PPK. | The device returns an invalid request error. |
| enrollz-1.17 | GetIdevidCsrRequest with missing control_card_selection. | The device returns an invalid request error. |
| enrollz-1.18 | GetIdevidCsrRequest with invalid control_card_selection. | The device returns an invalid request error. |
| enrollz-1.19 | GetIdevidCsrRequest with invalid key type KEY_UNSPECIFIED. | The device returns an invalid request error. |
| enrollz-1.20 | GetIdevidCsrRequest with an unsupported KeyTemplate. | The device returns a GetIdevidCsrResponse with status as STATUS_UNSUPPORTED. |
| enrollz-1.21 | RotateOIakCert with an invalid control card. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| enrollz-1.22 | RotateOIakCert with a populated oidevid_cert but a missing ssl_profile_id. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| enrollz-1.23 | RotateOIakCert with a malformed oIAK certificate. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| enrollz-1.24 | RotateOIakCert with a oIAK having a  mismatched underlying IAK public key. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| enrollz-1.25 | RotateOIakCert with a valid oIAK certificate chain but an invalid signature. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| enrollz-1.26 | RotateOIakCert with a malformed oIDevID certificate. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| enrollz-1.27 | RotateOIakCert with a oIDevID having a  mismatched underlying IDevID public key. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| enrollz-1.28 | RotateOIakCert with a valid oIDevID certificate chain but an invalid signature. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| enrollz-1.29 | RotateOIakCert with valid oIAK and oIDevID for active control card, and only a valid oIAK for the standby control card. | The device successfully completes the enrollment, obtains oIAK and oIDevID certs, and updates the SSL profile to use the new oIDevID ce |
| enrollz-1.30 | Reboot after successful enrollment. | The device comes up successfully and can establish an mTLS connection using the oIDevID certificate. All certificates (oIAK, oIDevID) are persistent. |
| enrollz-1.31 | Reboot after GetIdevidCsr and restart enrollment. | The device successfully completes the enrollment, obtains oIAK and oIDevID certs, and updates the SSL profile to use the new oIDevID cert. |


### Enrollment and Validation Steps

1.  Initiate an mTLS connection from the test client (acting as Enrollz) to the device.
2.  Call `GetControlCardVendorID` for the active control card with a valid `ControlCardSelection`.
3.  Verify the `GetControlCardVendorIDResponse` contains the correct `ControlCardVendorId` for the selected card.
4.  Using the serial number from the response, fetch the corresponding EK or PPK public key from a test database (simulating the RoT database).
5.  Generate a restricted HMAC key, wrap it using the EK/PPK public key, and send it as a challenge to the device in a `Challenge` RPC.
6.  Upon receiving the `ChallengeResponse` from the device, verify the HMAC signature (`iak_certify_info_signature`) over the IAK certify info (`iak_certify_info`) using the generated HMAC key.
7.  Call `VerifyIAKKey` to validate the received IAK public key. This includes:
    *   Verify that the IAK public area (`iak_pub`) has attributes specified in [Template H-0](https://trustedcomputinggroup.org/wp-content/uploads/TPM-2p0-Keys-for-Device-Identity-and-Attestation_v1_r12_pub10082021.pdf#page=43) for AKs.
    *   Verify that the `magic` field in `iak_certify_info` is the expected TPM Generated Value (0xff544347).
    *   Compute the IAK [name](https://trustedcomputinggroup.org/wp-content/uploads/Trusted-Platform-Module-2.0-Library-Part-1-Version-184_pub.pdf#page=87) and verify that it matches the `name` field in `iak_certify_info`.
    *   Verify that in the `iak_certify_info`, the `name` and `qualifiedName` fields are not identical.
8.  Call `GetIdevidCsr` with a supported `KeyTemplate` (currently only [KEY_TEMPLATE_ECC_NIST_P384](https://trustedcomputinggroup.org/wp-content/uploads/TPM-2p0-Keys-for-Device-Identity-and-Attestation_v1_r12_pub10082021.pdf#page=44)) .
9.  Upon receiving the `GetIdevidCsrResponse`, call `VerifyIdevidKey` to validate the IDevID and CSR. This includes:
    *   Parsing the [TCG-CSR-IDEVID contents](https://trustedcomputinggroup.org/wp-content/uploads/TPM-2p0-Keys-for-Device-Identity-and-Attestation_v1_r12_pub10082021.pdf#page=67) `csr_contents`.
    *   Verifying the signature (`signCertifyInfoSignature`) of IDevID certify info (`signCertifyInfo`) using IAK pub key.
    *   Verify the `signCertifyInfo` contains the following:
        *   The `magic` field is the expected TPM Generated Value.
        *   The computed IDevID name matches the `name` field.
        *   The `name` and `qualifiedName` fields are not identical.
    *   Verifying the IDevID public key attributes against the template provided.
    *   Verifying the CSR's signature (`idevid_signature_csr`) using the IDevID public key.
10. Repeat steps 2-9 for the standby control card.
11. Generate owner-signed oIAK and oIDevID certificates for both control cards.
12. Call `RotateOIakCert` to install the newly generated owner-signed oIAK and
oIDevID certificates for both control cards. The request must use the `updates`
field to specify the certificates. Verify the device returns a successful
response.

## OpenConfig Path and RPC Coverage

```yaml
paths:
rpcs:
  gnmi:
    gNMI.Subscribe:
```

## Canonical OC

```json
{}
```