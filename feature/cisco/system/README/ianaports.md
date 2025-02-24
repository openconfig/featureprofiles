### Test Module IANA Ports

#### 1. Process restart EMSD and get updated listen-address
Test       | **Process restart EMSD and get updated listen-address**
-|-
Description| This test verifies the listen-address after restarting EMSD
Path       | /system/gnmi/config/listen-address
Preconditions | DUT should be up and running
Steps to Execute | 1. Restart the EMSD process<br>2. Verify the updated listen-address
Expected Result | The listen-address should be updated correctly
Comments | 

#### 2. Reload Router and check gRPC before and after
Test       | **Reload Router and check gRPC before and after**
-|-
Description| This test verifies the gRPC configuration before and after reloading the router
Path       | /system/grpc-servers/grpc-server/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Check the gRPC configuration before reloading the router<br>2. Reload the router<br>3. Check the gRPC configuration after reloading the router
Expected Result | The gRPC configuration should be consistent before and after reloading the router
Comments | 

#### 3. Assign a GNMI/GRIBI/P4RT Default Ports
Test       | **Assign a GNMI/GRIBI/P4RT Default Ports**
-|-
Description| This test assigns default ports for GNMI, GRIBI, and P4RT
Path       | /system/gnmi/config/default-ports
Preconditions | DUT should be up and running
Steps to Execute | 1. Assign default ports for GNMI, GRIBI, and P4RT<br>2. Verify the port assignments
Expected Result | The default ports should be assigned correctly
Comments | 

#### 4. GRPC Server Update Test
Test       | **GRPC Server Update Test**
-|-
Description| This test verifies the gRPC server update functionality
Path       | /system/grpc-servers/grpc-server/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the gRPC server configuration<br>2. Verify the updated configuration
Expected Result | The gRPC server configuration should be updated correctly
Comments | 

#### 5. GRPC Server Replace Test
Test       | **GRPC Server Replace Test**
-|-
Description| This test verifies the gRPC server replace functionality
Path       | /system/grpc-servers/grpc-server/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace the gRPC server configuration<br>2. Verify the replaced configuration
Expected Result | The gRPC server configuration should be replaced correctly
Comments | 

#### 6. GRPC Server Port Update Test
Test       | **GRPC Server Port Update Test**
-|-
Description| This test verifies the gRPC server port update functionality
Path       | /system/grpc-servers/grpc-server/config/port
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the gRPC server port<br>2. Verify the updated port
Expected Result | The gRPC server port should be updated correctly
Comments | 

#### 7. GRPC Server Port Replace Test
Test       | **GRPC Server Port Replace Test**
-|-
Description| This test verifies the gRPC server port replace functionality
Path       | /system/grpc-servers/grpc-server/config/port
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace the gRPC server port<br>2. Verify the replaced port
Expected Result | The gRPC server port should be replaced correctly
Comments | 

#### 8. GRPC Config Update Test
Test       | **GRPC Config Update Test**
-|-
Description| This test verifies the gRPC configuration update functionality
Path       | /system/grpc-servers/grpc-server/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the gRPC configuration<br>2. Verify the updated configuration
Expected Result | The gRPC configuration should be updated correctly
Comments | 

#### 9. GRPC Config Replace Test
Test       | **GRPC Config Replace Test**
-|-
Description| This test verifies the gRPC configuration replace functionality
Path       | /system/grpc-servers/grpc-server/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace the gRPC configuration<br>2. Verify the replaced configuration
Expected Result | The gRPC configuration should be replaced correctly
Comments | 

#### 10. TLS Update Test
Test       | **TLS Update Test**
-|-
Description| This test verifies the TLS update functionality
Path       | /system/grpc-servers/grpc-server/config/transport-security
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the TLS configuration<br>2. Verify the updated configuration
Expected Result | The TLS configuration should be updated correctly
Comments | 

#### 11. TLS Replace Test
Test       | **TLS Replace Test**
-|-
Description| This test verifies the TLS replace functionality
Path       | /system/grpc-servers/grpc-server/config/transport-security
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace the TLS configuration<br>2. Verify the replaced configuration
Expected Result | The TLS configuration should be replaced correctly
Comments | 

#### 12. GRPC Name Update Test
Test       | **GRPC Name Update Test**
-|-
Description| This test verifies the gRPC name update functionality
Path       | /system/grpc-servers/grpc-server/config/name
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the gRPC server name<br>2. Verify the updated name
Expected Result | The gRPC server name should be updated correctly
Comments | 

#### 13. GRPC Name Replace Test
Test       | **GRPC Name Replace Test**
-|-
Description| This test verifies the gRPC name replace functionality
Path       | /system/grpc-servers/grpc-server/config/name
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace the gRPC server name<br>2. Verify the replaced name
Expected Result | The gRPC server name should be replaced correctly
Comments | 

#### 14. Assign a Non-Default GNMI/GRIBI/P4RT Default Ports
Test       | **Assign a Non-Default GNMI/GRIBI/P4RT Default Ports**
-|-
Description| This test assigns non-default ports for GNMI, GRIBI, and P4RT
Path       | /system/gnmi/config/default-ports
Preconditions | DUT should be up and running
Steps to Execute | 1. Assign non-default ports for GNMI, GRIBI, and P4RT<br>2. Verify the port assignments
Expected Result | The non-default ports should be assigned correctly
Comments | 

#### 15. Rollback to IANA Default Ports
Test       | **Rollback to IANA Default Ports**
-|-
Description| This test rolls back to the IANA default ports
Path       | /system/gnmi/config/default-ports
Preconditions | DUT should be up and running
Steps to Execute | 1. Rollback to the IANA default ports<br>2. Verify the port assignments
Expected Result | The ports should be rolled back to the IANA defaults correctly
Comments | 