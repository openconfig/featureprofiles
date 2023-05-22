# SEC-3.1: Authentication

## Summary

Ensure that user-name/password based authentication is performed at SSH/console
and gRPC connections.

## Procedure

*   For SSH/console
    *   Configure user login/password through CLI, and ensure they are honored
        for SSH over management port as discussed in the previous section.
    *   Ensure that the same login/password credentials are also honored from
        the console as discussed in the previous section.
*   For gRPC layer
    *   TODO: Set the user and password using AAA CLI on the device, configured
        to authenticate using TACACS+ with a fallback of the local database.
    *   Use the username/password in the metadata of all gNMI requests as
        explained in Openconfig gNMI authentication.
        *   TODO: if SPIFFE-id comes in before PVT, test with SPIFFE-id.
            SPIFFE-id will carry the username in X.509 certificate as opposed to
            metadata.
        *   Ensure gNMI set/get requests are accepted only with the correct
            username/password.
        *   Ensure gNMI set/get requests are denied with incorrect login or
            incorrect password.

## Config Parameter coverage

N/A

## Telemetry Parameter coverage

N/A

## Protocol/RPC Parameter coverage

N/A

## Minimum DUT platform requirement

FFF
