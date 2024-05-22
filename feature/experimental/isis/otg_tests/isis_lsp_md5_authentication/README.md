# RT-2.17: IS-IS LSP authentication MD5

## Summary

IS-IS LSP authentication MD5

## Procedure

* Topology: ATE-port1<—> DUT–port1
* Configure IS-IS for ATE port-1 and DUT port-1. 
* Establish basic adjacency.
* Authentication Type:
    * Select an authentication type supported by the DUT and ATE. This should be:
    MD5 Hashing
    * On both the ATE and DUT, configure matching passwords or keys for the chosen authentication type.
* Verification: Authentication in Effect:
    * Verify that LSP authentication is enabled and the configured method is active. 
    * Check relevant IS-IS interface and adjacency status.
    * Advertise routes from ATE to DUT. 
    * Verify routes are present in ISIS DB.
* Intentional Mismatch:
    * On either the ATE or DUT, change the configured password/key to create an authentication mismatch.
    * Verify that IS-IS LSPs with incorrect authentication are rejected.

## Config Parameter Coverage

* For prefix: /network-instances/network-instance/protocols/protocol/isis/

* Parameters:

    * levels/level/authentication/config/enabled
    * levels/level/authentication/config/auth-type
    * levels/level/authentication/config/auth-password

## Telemetry Parameter Coverage

* For prefix: 

    * /network-instances/network-instance/protocols/protocol/isis/

* Parameters:

    * levels/level/authentication/state/enabled
    * levels/level/link-state-database/lsp/tlvs/tlv/authentication/state/crypto-type
    * levels/level/link-state-database/lsp/tlvs/tlv/authentication/state/authentication-key

## Protocol/RPC Parameter Coverage

* IS-IS
