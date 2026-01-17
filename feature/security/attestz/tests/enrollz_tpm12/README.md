# enrollz-2: enrollz test for TPM 1.2 Enrollment flow

## Summary

This document details the RotateAIK enrollment flow for devices equipped with TPM 1.2. It describes the interaction between the EnrollZ Service and the device, the cryptographic structures involved, and the mapping to the `RotateAIKCertRequest` and `RotateAIKCertResponse` Protocol Buffers.

## Overview

The RotateAIK flow for TPM 1.2 is used to generate an Attestation Identity Key (AIK) pair on the device, certify it using the device's Endorsement Key (EK), and install the resulting AIK Certificate. 
This flow requires the service to have prior knowledge of the device's public Endorsement Key (EK), which is typically retrieved from the device's Endorsement Certificate. TPM 1.2 utilizes the `MakeIdentity` protocol. This process involves the device generating a `TPM_IDENTITY_REQ`, and the service responding with an encrypted credential that only the device's EK can decrypt.

## Protocol Buffer Definitions

The flow relies on the `RotateAIKCertRequest` and `RotateAIKCertResponse` messages.

```protobuf
// The RotateAIKCertRequest handles the workflow for enrollment of TPM 1.2
// devices.
message RotateAIKCertRequest {
  message IssuerCertPayload {
    // This field contains the TPM_ASYM_CA_CONTENTS encrypted with the EK.
    // Despite the name 'symmetric_key_blob', this contains the asymmetric
    // encryption of the session key.
    bytes symmetric_key_blob = 1;

    // This field contains the AIK Certificate encrypted with the session key
    // defined in the symmetric_key_blob.
    bytes aik_cert_blob = 2;
  }

  oneof value {
    // Step 1: Service provides the Privacy CA (Issuer) Public Key.
    bytes issuer_public_key = 1;

    // Step 3: Service provides the encrypted credentials.
    IssuerCertPayload issuer_cert_payload = 2;

    // Step 5: Service confirms finalization.
    bool finalize = 3;
  }

  // Switch control card selected identifier.
  ControlCardSelection control_card_selection = 4;
}

message RotateAIKCertResponse {
  oneof value {
    // Step 2: Device sends the TPM_IDENTITY_REQ blob.
    bytes application_identity_request = 1;

    // Step 4: Device returns the decrypted PEM cert to prove possession of EK.
    string aik_cert = 2;
  }

  // Vendor identity fields of the selected control card.
  ControlCardVendorId control_card_id = 3;
}
```

## TPM 1.2 Structures (from `tpm12_utils.go`)

The workflow relies on the correct formation, serialization, and parsing of several TPM 1.2 structures which are defined in the TPM 1.2 TCG specifications, including:

*   `TPM_IDENTITY_REQ`
*   `TPM_IDENTITY_PROOF`
*   `TPM_IDENTITY_CONTENTS`
*   `TPM_PUBKEY`
*   `TPM_SYMMETRIC_KEY`
*   `TPM_ASYM_CA_CONTENTS`
*   `TPM_SYM_CA_ATTESTATION`

---

## Test Cases (enrollz-2)

This section validates that the device correctly implements the service-driven TPM 1.2 RotateAIK enrollment workflow. The following test cases cover both the successful enrollment path and various failure scenarios from the device's perspective.

| ID | Case | Expected Result |
| --- | --- | --- |
| enrollz-2.1 | Successful enrollment flow. | The device successfully completes the enrollment and obtains AIK cert. |
| enrollz-2.2 | RotateAIKCertRequest with a missing control_card_selection. | The device returns an invalid request error and closes the stream. |
| enrollz-2.3 | RotateAIKCertRequest with an invalid control_card_selection. | The device returns an invalid request error and closes the stream. |
| enrollz-2.4 | Service sends an initial RotateAIKCertRequest with a missing issuer_public_key. | The device returns an invalid request error and closes the stream. |
| enrollz-2.5 | Service sends an initial RotateAIKCertRequest with a malformed issuer_public_key. | The device returns an invalid request error and closes the stream. |
| enrollz-2.6 | Service sends RotateAIKCertRequest with issuer_cert_payload but the symmetric_key_blob is missing. | The device returns an invalid request error and closes the stream. |
| enrollz-2.7 | Service sends RotateAIKCertRequest with issuer_cert_payload where the aik_cert_blob is missing. | The device returns an invalid request error and closes the stream. |
| enrollz-2.8 | Service sends RotateAIKCertRequest with issuer_cert_payload but the symmetric_key_blob is not decryptable with the device's EK. | The device cannot decrypt the session key and returns an error, closing the stream. |
| enrollz-2.9 | Service sends RotateAIKCertRequest with issuer_cert_payload where symmetric_key_blob is malformed (e.g., not valid RSAES-OAEP format). | The device fails to decrypt the symmetric_key_blob and returns an error. |
| enrollz-2.10| Service sends RotateAIKCertRequest with issuer_cert_payload where symmetric_key_blob decrypts, but the TPM_ASYM_CA_CONTENTS digest does not match the AIK. | The device's TPM fails the TPM_ActivateIdentity step, and the device returns an error. |
| enrollz-2.11| Service sends RotateAIKCertRequest with issuer_cert_payload where the aik_cert_blob is not decryptable with the recovered session key. | The device cannot decrypt the AIK certificate and returns an error. |
| enrollz-2.12| Service sends RotateAIKCertRequest with issuer_cert_payload where aik_cert_blob is a malformed TPM_SYM_CA_ATTESTATION structure. | The device fails to parse the structure and returns an error. |
| enrollz-2.13| Service sends RotateAIKCertRequest with issuer_cert_payload where the decrypted aik_cert_blob contains a malformed PEM certificate. | The device fails to install the certificate and returns an error. |
| enrollz-2.14| Service sends an aik_cert_blob which, when decrypted, contains a certificate for a different AIK public key than the one generated by the device in Phase 1. | The device should detect the mismatch and return an error, refusing to install the certificate. |
| enrollz-2.15| Service sends a RotateAIKCertRequest with finalize = true before the device has returned its aik_cert in Phase 4. | The device should return an error indicating the state is unexpected. |
| enrollz-2.16| Service closes the stream prematurely at any stage. | The device enrollment process is aborted. No new AIK certificate is persisted. |
| enrollz-2.17| Reboot at any time during enrollment (before finalize message) and restart enrollment. | The device successfully completes the enrollment and obtains AIK cert. |
| enrollz-2.18| Reboot after successful enrollment (after finalize message) | The device comes up successfully and has AIK cert still present on the device.|                                                                                                               |
---

## Workflow Steps

### Phase 1: Initialization & Key Generation

1.  **Service**: Generates an RSA 2048-bit Issuer key pair.
2.  **Service**: Sends the `RotateAIKCertRequest` containing the `issuer_public_key` to the device.
3.  **Device**: Calls `TPM_MakeIdentity` (via `Tspi_TPM_collateIdentityRequest` or similar).
    *   This generates the new AIK key pair.
    *   This creates a `TPM_IDENTITY_REQ` structure containing the `TPM_IDENTITY_PROOF`.
    *   **Note**: The `TPM_IDENTITY_REQ` involves a symmetric key (`symBlob`) and an asymmetric encryption of that key (`asymBlob`) using the provided `issuer_public_key`.
4.  **Device**: Sends `RotateAIKCertResponse` containing the `application_identity_request` (the raw `TPM_IDENTITY_REQ` bytes).

### Phase 2: Verification (Service Side)

Upon receiving the `application_identity_request`, the EnrollZ Service performs the following:

1.  **Parse Request**: Parses the `TPM_IDENTITY_REQ` structure.
2.  **Decrypt Session Key**: Uses the Issuer Private Key to decrypt the `asymBlob`, retrieving the `TPM_SYMMETRIC_KEY`.
3.  **Decrypt Proof**: Uses the `TPM_SYMMETRIC_KEY` to decrypt the `symBlob`, retrieving the `TPM_IDENTITY_PROOF`.
4.  **Reconstruct Contents**: Constructs the `TPM_IDENTITY_CONTENTS` structure to verify the signature.
    *   `Ver`: `0x01010000`
    *   `Ordinal`: `0x00000079`
    *   `labelPrivCADigest`: `SHA1(identityLabel || privacyCA)`
    *   `identityLabel`: `"Identity"` (UTF-8: `0x4964656e74697479`)
    *   `privacyCA`: The `issuer_public_key` (`TPM_PUBKEY`)
    *   `identityPubKey`: The AIK Public Key extracted from the `TPM_IDENTITY_PROOF`.
5.  **Verify Signature**: Verifies the `identityBinding` signature (from the proof) against the hash of the reconstructed `TPM_IDENTITY_CONTENTS` using the `identityPubKey` (AIK Public Key).

### Phase 3: Certification & Encryption (Service Side)

Once the AIK is verified:

1.  **Generate Cert**: The Service sends the AIK Public Data to the CA and receives the AIK Certificate (PEM).
2.  **Create Session Key**: Generates a new AES-256 symmetric key.
3.  **Encapsulate Key (`TPM_ASYM_CA_CONTENTS`)**:
    *   Creates a `TPM_ASYM_CA_CONTENTS` structure.
    *   `Digest`: `SHA1(TPM_PUBKEY)` (Hash of the AIK Public Key).
    *   **Encryption**: Encrypts this structure using the device's Endorsement Key (EK) public key.
        *   `Algorithm`: RSAES-OAEP with SHA-1 and MGF1.
        *   `OAEP Parameter`: `"TCPA"` (Required for TPM 1.2 compatibility).
    *   **Output**: This forms the `symmetric_key_blob` field in `IssuerCertPayload`.
4.  **Encrypt Certificate**:
    *   Encrypts the PEM-encoded AIK Certificate using the AES-256 session key generated in Step 2 (AES CBC).
    *   A unique Initialization Vector (IV) must be used and is typically prepended to the ciphertext.
    *   **Output**: This forms the `aik_cert_blob` field in `IssuerCertPayload` (e.g., `iv || ciphertext`).
5.  **Send**: The Service sends the `RotateAIKCertRequest` containing the `IssuerCertPayload` to the device.

### Phase 4: Activation (Device Side)

1.  **Device**: Receives the `IssuerCertPayload`.
2.  **Decrypt (Activate Identity)**:
    *   The device uses its EK Private Key to decrypt the `symmetric_key_blob`. This recovers the AES-256 session key.
    *   **Note**: In standard TSS, this maps to `TPM_ActivateIdentity`, which validates the `TPM_ASYM_CA_CONTENTS` digest against the loaded AIK.
3.  **Decrypt Certificate**: The device uses the recovered session key to decrypt the `aik_cert_blob`.
4.  **Install**: The device installs/stores the plaintext AIK Certificate.
5.  **Prove Possession**: The device sends a `RotateAIKCertResponse` containing the plaintext `aik_cert` back to the Service.

### Phase 5: Finalization

1.  **Service**: Compares the received `aik_cert` with the original certificate it generated.
2.  **Service**: If they match, sends a `RotateAIKCertRequest` with `finalize = true`.
3.  **Device**: Marks the enrollment as complete.

## Cryptographic Parameters Summary

| Parameter | Value |
|---|---|
| TPM Version | 1.2 |
| Identity Label | `"Identity"` (`0x4964656e74697479`) |
| Structure Version | `0x01010000` |
| OAEP Parameter | `"TCPA"` |
| Hash Algorithm | SHA-1 |
| Symmetric Key | AES-256 (for Cert encryption) |
| Issuer Key | RSA 2048 |

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