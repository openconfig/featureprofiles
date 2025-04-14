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


rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* VRX
