# SYS-3.1: AAA and TACACS+ Configuration Verification Test Suite


## Summary

This test suite aims to thoroughly validate the correct implementation  of the AAA (Authentication, Authorization, and Accounting) framework with TACACS+ and local authentication on a device. 

## Testbed type

*  [`featureprofiles/topologies/dut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Procedure

### Test environment setup

#### Configuration

*   Configure the loopback interface with IPv4 and IPv6 address and netmasks of /32, /64 respectively

### SYS-3.1.1 User Configuration Test

*   Create a user on the DUT with the following parameters:
    ```yaml
    system:  
    aaa:  
        authentication:  
        users:  
            - user:  
            config:  
                username: "testuser" 
                password: "password"
                role: SYSTEM_ROLE_ADMIN
    ```
*   Verification:
    *   Get  the device configuration via gNMI and verify the successful creation of the "testuser" account with the specified parameters.

### SYS-3.1.2 TACACS+ Server Configuration Test

*   Configuration:   
    *   Configure a TACACS server on the DUT as below:
        *   Host IP: 192.168.1.1
        *   Port: 49
        *   Key: tacacs_password
        *   Timeout: 4
    *   Configure the source IP address of a selected interface for all outgoing TACACS+ packets as the loopback interface.
    ```yaml
    system:  
    aaa:  
        server-groups:  
        server-group:  
            config:  
            name: "my_server_group"  
            servers:  
            - server:  
                config:  
                name: "tacacs_server_1"  
                address: 192.168.1.1 
                timeout: 4  
                tacacs:  
                config:  
                    port: 49  
                    source-address: dut_loopback_address  
                    secret-key: acacs_password
    ```
*   Verification:
    *   Get the device configuration via gNMI and verify that the TACACS+ server has been successfully created with the correct parameters.

### SYS-3.1.3 AAA Authentication Configuration Test

*   Configuration:
    *   Configure the DUT to use TACACS+ as the primary authentication method and local authentication as a fallback option.
    ```yaml
    system:  
    aaa:  
        authentication:  
        config:  
            authentication-method: [TACACS_ALL, LOCAL] 
    ```
*   Verification:
    *   Use gNMI to get the device's current configuration and verify that the authentication settings match the intended design.

### SYS-3.1.4 AAA Authorization Configuration Test

*   Configuration:
    *   Configure command authorization to exclusively utilize the TACACS+ server.
    *   Configure authorization for configuration mode to primarily use the TACACS+ server and fall back to local authorization if the TACACS+ server is unavailable or does not respond.
    ```yaml
    system:  
    aaa:  
        authorization:  
        config:  
            authorization-method: [TACACS_ALL, LOCAL]  
        events:  
            config:  
                - event-type: AAA_AUTHORIZATION_EVENT_COMMAND  
                - event-type: AAA_AUTHORIZATION_EVENT_CONFIG
    ```
*   Verification:
    *   Use gNMI to Get the device's current configuration and verify its alignment with the intended authorization settings..

### SYS-3.1.5 AAA Accounting Configuration Test

*   Configuration:
    *   Activate accounting for command line interface (CLI) commands on the device under test (DUT).
    *   Activate accounting for login sessions on the DUT.
    *   Activate accounting for system event logs on the DUT.
    ```yaml
    system:  
    aaa:  
        accounting:  
        config:  
            name: "my_server_group"  
            accounting-method: TACACS_ALL  
        events:  
            config:  
            - event-type: AAA_ACCOUNTING_EVENT_COMMAND  
                record: START_STOP 
            - event-type: AAA_ACCOUNTING_EVENT_LOGIN  
                record: START_STOP 
    ```
*   Verification:
    *   Use gNMI to get the device's configuration and validate that the accounting settings are correctly implemented as intended.

### SYS-3.1.6 Local Fallback Authentication on Unexpected TACACS+ Server EOF

SYS-3.1.3 configures TACACS+ as the primary authentication method and local
authentication as the fallback, but only verifies that configuration through
gNMI. This subtest exercises the runtime behavior of that fallback chain when
the TACACS+ server terminates the TCP session abruptly (EOF) in the middle of an
authentication exchange, without returning a TACACS+ REPLY packet. A transient
TACACS+ connection reset or EOF must be handled gracefully by the DUT: the
authentication attempt toward the server must fail cleanly, the DUT must fall
through to the next configured method (LOCAL), the SSH login of a valid local
user must succeed, and the SSH server process on the DUT must remain stable
(no crash, abort, restart, or core dump).

#### Topology and EOF server emulation

*   The test uses the DUT-only topology defined above. No ATE ports are needed.
*   The test harness hosts a scripted TCP listener that emulates a misbehaving
    TACACS+ server. The listener:
    *   Listens on TCP port 49 on `192.0.2.10` (IPv4) and `2001:db8::10` (IPv6).
        These are documentation addresses used as placeholders; the actual
        harness addresses must be reachable from the DUT management interface.
    *   Accepts each incoming TCP connection from the DUT.
    *   Reads the TACACS+ authentication START packet sent by the DUT.
    *   Immediately closes the socket (TCP FIN) without sending any TACACS+
        REPLY, producing an "unexpected EOF from server" condition on the DUT.
    *   Counts accepted connections so the test can confirm that the DUT
        actually attempted TACACS+ authentication.

#### Configuration

*   Ensure the local user `testuser` with role `SYSTEM_ROLE_ADMIN` from
    SYS-3.1.1 is present on the DUT. This user is the local fallback identity
    used to log in after the TACACS+ attempt fails.
*   Configure a TACACS+ server-group named `tacacs_eof_group` with a single
    server pointing to the IPv4 address of the EOF listener:
    *   Server name: `tacacs_eof_server`
    *   Address: `192.0.2.10`
    *   Port: `49`
    *   Secret key: `tacacs_password`
    *   Timeout: `5` seconds
*   Configure the authentication method list so that the TACACS+ server-group
    is tried first and local authentication is the fallback:
    `authentication-method: ["tacacs_eof_group", LOCAL]`.
*   Push the configuration using gNMI.Set with UPDATE (or REPLACE at the
    `/system/aaa` subtree) so that unrelated baseline AAA configuration is
    preserved.

#### Canonical OC

```json
{
  "system": {
    "aaa": {
      "authentication": {
        "config": {
          "authentication-method": [
            "tacacs_eof_group",
            "LOCAL"
          ]
        },
        "users": {
          "user": [
            {
              "config": {
                "password": "password",
                "role": "SYSTEM_ROLE_ADMIN",
                "username": "testuser"
              },
              "username": "testuser"
            }
          ]
        }
      },
      "server-groups": {
        "server-group": [
          {
            "config": {
              "name": "tacacs_eof_group",
              "type": "TACACS"
            },
            "name": "tacacs_eof_group",
            "servers": {
              "server": [
                {
                  "address": "192.0.2.10",
                  "config": {
                    "address": "192.0.2.10",
                    "name": "tacacs_eof_server",
                    "timeout": 5
                  },
                  "tacacs": {
                    "config": {
                      "port": 49,
                      "secret-key": "tacacs_password"
                    }
                  }
                }
              ]
            }
          }
        ]
      }
    }
  }
}
```

#### Procedure (IPv4 TACACS+ server)

*   Step 1 - Verify configuration
    *   Use gNMI Get on
        `/system/aaa/server-groups/server-group/servers/server/state/address`,
        `/system/aaa/server-groups/server-group/servers/server/state/timeout`,
        and
        `/system/aaa/server-groups/server-group/servers/server/tacacs/state/port`
        to verify that the server `192.0.2.10`, port `49`, and timeout `5` are
        applied.
    *   Use gNMI Get on
        `/system/aaa/authentication/state/authentication-method` to verify the
        method list is exactly `["tacacs_eof_group", LOCAL]` in that order.
*   Step 2 - Record baseline state
    *   Record `/system/state/boot-time`.
    *   Record a snapshot of all processes using
        `/system/processes/process/state/name`,
        `/system/processes/process/state/pid`, and
        `/system/processes/process/state/start-time`.
    *   Record `/system/ssh-server/state/counters/access-accepts` and
        `/system/ssh-server/state/counters/access-rejects`.
    *   Record the TACACS+ server counters
        `/system/aaa/server-groups/server-group/servers/server/state/connection-opens`,
        `/system/aaa/server-groups/server-group/servers/server/state/connection-aborts`,
        and
        `/system/aaa/server-groups/server-group/servers/server/state/errors-received`
        where supported.
    *   Record the baseline list of core dump files on the DUT using
        `gnoi.file.File.Stat` on the platform-specific core dump directory
        (the same per-vendor directory and file name pattern used by the
        featureprofiles core file validator). The baseline is needed so that
        pre-existing core files on the DUT are not attributed to this test.
*   Step 3 - Trigger the TACACS+ EOF condition
    *   Start the EOF listener on `192.0.2.10:49`.
    *   Sequentially open 5 SSH sessions to the DUT management IPv4 address as
        `testuser` with password `password`. For each session, open a session
        channel, wait for the session to be established, and then close it.
    *   Each SSH login triggers a TACACS+ authentication START toward the EOF
        listener, which closes the connection without a REPLY. The DUT is
        expected to treat this as a TACACS+ failure and fall back to LOCAL.
*   Step 4 - Validate behavior
    *   Each of the 5 SSH logins completes successfully within 30 seconds.
    *   The EOF listener reports at least 5 accepted connections from the DUT,
        confirming the TACACS+ EOF path was exercised and not bypassed.
    *   Use `gnmi.Watch` on `/system/ssh-server/state/counters/access-accepts`
        until it increases by at least 5 relative to the baseline;
        `/system/ssh-server/state/counters/access-rejects` must not increase.
    *   Where supported, the TACACS+ server counters reflect the aborted
        sessions (`connection-opens` increases, and `connection-aborts` or
        `errors-received` increases).
    *   The DUT remains responsive to gNMI, and `/system/state/boot-time` is
        unchanged.
    *   Compare the process snapshot against the baseline: no long-running
        process (in particular the SSH server process, matched in a
        vendor-neutral way by a process name containing `ssh`) has disappeared
        or has a changed `start-time`, apart from short-lived per-session child
        processes. This confirms the SSH server did not crash, abort, or
        restart while handling the TACACS+ EOF.
    *   Use `gnoi.file.File.Stat` on the same platform-specific core dump
        directory and compare the result against the baseline recorded in
        Step 2. No new core dump file may be present. An abnormal termination
        of the SSH server (or any other process) while handling the TACACS+
        EOF typically leaves a core file even if the process is restarted
        quickly enough to escape the process snapshot comparison, so this check
        is the authoritative crash signal.

#### Procedure (IPv6 TACACS+ server)

*   Replace the server in `tacacs_eof_group` with one whose address is
    `2001:db8::10` (port `49`, timeout `5`, same secret key) and verify it via
    `/system/aaa/server-groups/server-group/servers/server/state/address`.
*   Start the EOF listener on `[2001:db8::10]:49`.
*   Repeat Steps 2 through 4, opening the 5 SSH sessions to the DUT management
    IPv6 address. All pass criteria are identical to the IPv4 case.

#### Cleanup

*   Stop the EOF listener on the test harness.
*   Restore `/system/aaa/authentication/config/authentication-method` to the
    value recorded before this subtest.
*   Delete the `tacacs_eof_group` server-group.
*   Remove `testuser` if it was created by this test, so the DUT is returned to
    its baseline AAA configuration regardless of pass or fail.

#### Pass/Fail criteria

*   **Pass**: All SSH logins for both the IPv4 and IPv6 TACACS+ server cases
    succeed via LOCAL fallback, `access-accepts` increases by at least 5 per
    address family with no increase in `access-rejects`, the EOF listener
    observes the DUT connections, `boot-time` is unchanged, no SSH server
    process restart or crash is observed, and `gnoi.file.File.Stat` shows no
    new core dump files relative to the baseline.
*   **Fail**: Any SSH login is rejected, hangs, or times out; the SSH server
    process crashes, aborts, or restarts; `gnoi.file.File.Stat` reports any
    new core dump file in the platform core dump directory; the DUT
    reboots or stops responding to gNMI; or the EOF listener observes no
    connection from the DUT (TACACS+ path not exercised).

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  /system/aaa/authentication/users/user/config/username:
  /system/aaa/authentication/users/user/config/password-hashed:
  /system/aaa/authentication/users/user/config/role:
  /system/aaa/authorization/config/authorization-method:
  /system/aaa/accounting/config/accounting-method:
  /system/aaa/accounting/events/event/config/event-type:
  /system/aaa/accounting/events/event/config/record:
  /system/aaa/server-groups/server-group/servers/server/config/address:
  /system/aaa/server-groups/server-group/servers/server/config/timeout:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/port:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/secret-key:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/secret-key-hashed:
  /system/aaa/server-groups/server-group/servers/server/tacacs/config/source-address:
  /system/aaa/authentication/config/authentication-method:
  /system/aaa/server-groups/server-group/config/name:
  /system/aaa/server-groups/server-group/config/type:
  /system/aaa/server-groups/server-group/servers/server/config/name:

  ## State paths
  /system/aaa/authentication/users/user/state/username:
  /system/aaa/authentication/users/user/state/password-hashed:
  /system/aaa/authentication/users/user/state/role:
  /system/aaa/authorization/state/authorization-method:
  /system/aaa/accounting/state/accounting-method:
  /system/aaa/accounting/events/event/state/event-type:
  /system/aaa/accounting/events/event/state/record:
  /system/aaa/server-groups/server-group/servers/server/state/address:
  /system/aaa/server-groups/server-group/servers/server/state/timeout:
  /system/aaa/server-groups/server-group/servers/server/tacacs/state/port:
  /system/aaa/server-groups/server-group/servers/server/tacacs/state/secret-key:
  /system/aaa/server-groups/server-group/servers/server/tacacs/state/secret-key-hashed:
  /system/aaa/server-groups/server-group/servers/server/tacacs/state/source-address:
  /system/aaa/authentication/state/authentication-method:
  /system/aaa/server-groups/server-group/servers/server/state/connection-opens:
  /system/aaa/server-groups/server-group/servers/server/state/connection-aborts:
  /system/aaa/server-groups/server-group/servers/server/state/errors-received:
  /system/ssh-server/state/counters/access-accepts:
  /system/ssh-server/state/counters/access-rejects:
  /system/processes/process/state/name:
  /system/processes/process/state/pid:
  /system/processes/process/state/start-time:
  /system/state/boot-time:


rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    file.File.Stat:
```

## Minimum DUT platform requirement

* VRX
