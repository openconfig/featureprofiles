# enrollz-1: enrollz test for TPM 2.0 HMAC-based Enrollment flow

## Summary

This document outlines the functional network tests (FNTs) for the TPM 2.0
HMAC-based enrollment flow mainly for devices that lack a pre-provisioned
Initial Device Identifier (IDevID) certificate (but supports all TPM 2.0
devices). This workflow uses a Hardware-based Message Authentication Code (HMAC)
challenge-response mechanism to securely verify the device's identity. The
process establishes a chain of trust from the Endorsement Key (EK) or Platform
Primary Key (PPK), to the Initial Attestation Key (IAK), and finally to the
IDevID.

The primary objective is to test the device's ability to correctly handle the
enrollment sequence, including cryptographic operations, request validation, and
error handling.

## Procedure

The test validates that the device correctly implements the server-driven TPM
enrollment workflow. The following test cases cover both the successful
enrollment path and various failure scenarios from the device's perspective.

| ID | Case | Expected Result |
| --- | --- | --- |
| **enrollz-1.1** | Successful enrollment for a device that has an EK cert stored in RoT database. | The device successfully completes the enrollment, obtains oIAK and oIDevID certs, and updates the SSL profile to use the new oIDevID cert. |
| **enrollz-1.2** | Successful enrollment for a device has a PPK public key stored in RoT database. | The device successfully completes the enrollment, obtains oIAK and oIDevID certs, and updates the SSL profile to use the new oIDevID cert. |
| **enrollz-1.3** | Enrollz service presents an invalid client certificate. | The device rejects the mTLS connection, resulting in a gRPC connection failure. |
| **enrollz-1.4** | Invalid requests for `GetControlCardVendorIDRequest`, `ChallengeRequest`, and `GetIdevidCsrRequest` (e.g. missing `ControlCardSelection`, invalid `control_card_id`). | The device returns an invalid request error. |
| **enrollz-1.5** | `ChallengeRequest` with a malformed `HMACChallenge`. | The device's TPM fails to import the HMAC key, and the device returns an invalid request error. |
| **enrollz-1.6** | `ChallengeRequest` with correct key type `KEY_EK` but the challenge HMAC is wrapped with an invalid EK. | The device returns an invalid request error. |
| **enrollz-1.7** | `ChallengeRequest` with correct key type `KEY_PPK` but the challenge HMAC is wrapped with an invalid PPK. | The device returns an invalid request error. |
| **enrollz-1.8** | `GetIdevidCsrRequest` with an unsupported `KeyTemplate`. | The device returns a `GetIdevidCsrResponse` with `status` as `STATUS_UNSUPPORTED`. |
| **enrollz-1.9** | `RotateOIakCert` with one of the certificates being malformed. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| **enrollz-1.10** | `RotateOIakCert` with one of the certificates having a mismatched underlying IAK/IDevID public key. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |
| **enrollz-1.11** | `RotateOIakCert` with an invalid control card. | The device rejects the entire transaction, rolls back any changes, and returns an invalid request error. No certificates are updated on any control card. |

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