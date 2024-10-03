## Test Module System Container

#### 1. Update system container

Test       | **Update system container**
-|-
Description| This test verifies updating the system container
Path       | /system/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the system container with the hostname `tempHost1`<br>2. Verify the updated system container
Expected Result | The system container should be updated correctly
Comments | 

### Test Module: gRPC State

#### 2. Subscribe to system grpc-servers grpc-server state port

Test       | **Subscribe to system grpc-servers grpc-server state port**
-|-
Description| This test verifies the gRPC server state port by subscribing to it
Path       | /system/grpc-servers/grpc-server/state/port
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/grpc-servers/grpc-server/state/port`<br>2. Verify the gRPC server state port value
Expected Result | The gRPC server state port value should be `57777`
Comments | 

#### 3. Subscribe to system grpc-servers grpc-server state name

Test       | **Subscribe to system grpc-servers grpc-server state name**
-|-
Description| This test verifies the gRPC server state name by subscribing to it
Path       | /system/grpc-servers/grpc-server/state/name
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/grpc-servers/grpc-server/state/name`<br>2. Verify the gRPC server state name value
Expected Result | The gRPC server state name value should be `DEFAULT`
Comments | 

#### 4. Subscribe to system grpc-servers grpc-server state enable

Test       | **Subscribe to system grpc-servers grpc-server state enable**
-|-
Description| This test verifies the gRPC server state enable by subscribing to it
Path       | /system/grpc-servers/grpc-server/state/enable
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/grpc-servers/grpc-server/state/enable`<br>2. Verify the gRPC server state enable value
Expected Result | The gRPC server state enable value should be `true`
Comments | 

#### 5. Subscribe to system grpc-servers grpc-server state transport-security

Test       | **Subscribe to system grpc-servers grpc-server state transport-security**
-|-
Description| This test verifies the gRPC server state transport-security by subscribing to it
Path       | /system/grpc-servers/grpc-server/state/transport-security
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/grpc-servers/grpc-server/state/transport-security`<br>2. Verify the gRPC server state transport-security value
Expected Result | The gRPC server state transport-security value should be either `true` or `false`
Comments | 

#### 6. Subscribe to system state

Test       | **Subscribe to system state**
-|-
Description| This test verifies the system state by subscribing to it
Path       | /system/state
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/state`<br>2. Verify the system state values
Expected Result | The system state values should be consistent with the expected values
Comments | 

#### 7. Subscribe to system grpc-servers grpc-server['DEFAULT']

Test       | **Subscribe to system grpc-servers grpc-server['DEFAULT']**
-|-
Description| This test verifies the gRPC server state for 'DEFAULT' by subscribing to it
Path       | /system/grpc-servers/grpc-server['DEFAULT']/state
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/grpc-servers/grpc-server['DEFAULT']/state`<br>2. Verify the gRPC server state values
Expected Result | The gRPC server state values should be consistent with the expected values
Comments | 

#### 8. Subscribe to system grpc-servers

Test       | **Subscribe to system grpc-servers**
-|-
Description| This test verifies the gRPC servers state by subscribing to it
Path       | /system/grpc-servers
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/grpc-servers`<br>2. Verify the gRPC servers state values
Expected Result | The gRPC servers state values should be consistent with the expected values
Comments | 

### Test Module: gRPC Config

#### 9. Update system grpc-servers grpc-server config name

Test       | **Update system grpc-servers grpc-server config name**
-|-
Description| This test verifies updating the gRPC server config name
Path       | /system/grpc-servers/grpc-server/config/name
Preconditions | DUT should be up and running
Steps to Execute | 1. Update `/system/grpc-servers/grpc-server/config/name` with `DEFAULT`<br>2. Verify the updated gRPC server config name
Expected Result | The gRPC server config name should be updated correctly
Comments | 

#### 10. Replace system grpc-servers grpc-server config name

Test       | **Replace system grpc-servers grpc-server config name**
-|-
Description| This test verifies replacing the gRPC server config name
Path       | /system/grpc-servers/grpc-server/config/name
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace `/system/grpc-servers/grpc-server/config/name` with `DEFAULT`<br>2. Verify the replaced gRPC server config name
Expected Result | The gRPC server config name should be replaced correctly
Comments | 

#### 11. Update system grpc-servers grpc-server config port

Test       | **Update system grpc-servers grpc-server config port**
-|-
Description| This test verifies updating the gRPC server config port
Path       | /system/grpc-servers/grpc-server/config/port
Preconditions | DUT should be up and running
Steps to Execute | 1. Update `/system/grpc-servers/grpc-server/config/port` with `57777`<br>2. Verify the updated gRPC server config port
Expected Result | The gRPC server config port should be updated correctly
Comments | 

#### 12. Replace system grpc-servers grpc-server config port

Test       | **Replace system grpc-servers grpc-server config port**
-|-
Description| This test verifies replacing the gRPC server config port
Path       | /system/grpc-servers/grpc-server/config/port
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace `/system/grpc-servers/grpc-server/config/port` with `57777`<br>2. Verify the replaced gRPC server config port
Expected Result | The gRPC server config port should be replaced correctly
Comments | 

#### 13. Update system grpc-servers grpc-server config enable

Test       | **Update system grpc-servers grpc-server config enable**
-|-
Description| This test verifies updating the gRPC server config enable
Path       | /system/grpc-servers/grpc-server/config/enable
Preconditions | DUT should be up and running
Steps to Execute | 1. Update `/system/grpc-servers/grpc-server/config/enable` with `true`<br>2. Verify the updated gRPC server config enable
Expected Result | The gRPC server config enable should be updated correctly
Comments | 

#### 14. Replace system grpc-servers grpc-server config enable

Test       | **Replace system grpc-servers grpc-server config enable**
-|-
Description| This test verifies replacing the gRPC server config enable
Path       | /system/grpc-servers/grpc-server/config/enable
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace `/system/grpc-servers/grpc-server/config/enable` with `true`<br>2. Verify the replaced gRPC server config enable
Expected Result | The gRPC server config enable should be replaced correctly
Comments | 

#### 15. Update system grpc-servers grpc-server config transport-security

Test       | **Update system grpc-servers grpc-server config transport-security**
-|-
Description| This test verifies updating the gRPC server config transport-security
Path       | /system/grpc-servers/grpc-server/config/transport-security
Preconditions | DUT should be up and running
Steps to Execute | 1. Update `/system/grpc-servers/grpc-server/config/transport-security` with `false`<br>2. Verify the updated gRPC server config transport-security
Expected Result | The gRPC server config transport-security should be updated correctly
Comments | 

#### 16. Replace system grpc-servers grpc-server config transport-security

Test       | **Replace system grpc-servers grpc-server config transport-security**
-|-
Description| This test verifies replacing the gRPC server config transport-security
Path       | /system/grpc-servers/grpc-server/config/transport-security
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace `/system/grpc-servers/grpc-server/config/transport-security` with `false`<br>2. Verify the replaced gRPC server config transport-security
Expected Result | The gRPC server config transport-security should be replaced correctly
Comments | 

#### 17. Update system grpc-servers grpc-server config name to TEST

Test       | **Update system grpc-servers grpc-server config name to TEST**
-|-
Description| This test verifies updating the gRPC server config name to `TEST`
Path       | /system/grpc-servers/grpc-server/config/name
Preconditions | DUT should be up and running
Steps to Execute | 1. Update `/system/grpc-servers/grpc-server/config/name` with `TEST`<br>2. Verify the updated gRPC server config name
Expected Result | The gRPC server config name should be updated correctly
Comments | 

#### 18. Replace system grpc-servers grpc-server config name to TEST

Test       | **Replace system grpc-servers grpc-server config name to TEST**
-|-
Description| This test verifies replacing the gRPC server config name to `TEST`
Path       | /system/grpc-servers/grpc-server/config/name
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace `/system/grpc-servers/grpc-server/config/name` with `TEST`<br>2. Verify the replaced gRPC server config name
Expected Result | The gRPC server config name should be replaced correctly
Comments | 

### Test Module: gRPC Listen Address

#### 19. Update listen address on System container level

Test       | **Update listen address on System container level**
-|-
Description| This test verifies updating the listen address on the System container level
Path       | /system/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the listen address on the System container level<br>2. Verify the updated listen address
Expected Result | The listen address should be updated correctly
Comments | 

#### 20. Subscribe listen address on System Config container level

Test       | **Subscribe listen address on System Config container level**
-|-
Description| This test verifies subscribing to the listen address on the System Config container level
Path       | /system/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to the listen address on the System Config container level<br>2. Verify the listen address
Expected Result | The listen address should be consistent with the expected value
Comments | 

#### 21. Subscribe listen address on System State container level

Test       | **Subscribe listen address on System State container level**
-|-
Description| This test verifies subscribing to the listen address on the System State container level
Path       | /system/state
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to the listen address on the System State container level<br>2. Verify the listen address
Expected Result | The listen address should be consistent with the expected value
Comments | 

#### 22. Update listen address on GrpcServer container level

Test       | **Update listen address on GrpcServer container level**
-|-
Description| This test verifies updating the listen address on the GrpcServer container level
Path       | /system/grpc-servers/grpc-server/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the listen address on the GrpcServer container level<br>2. Verify the updated listen address
Expected Result | The listen address should be updated correctly
Comments | 

#### 23. Subscribe listen address on GrpcServer Config container level

Test       | **Subscribe listen address on GrpcServer Config container level**
-|-
Description| This test verifies subscribing to the listen address on the GrpcServer Config container level
Path       | /system/grpc-servers/grpc-server/config
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to the listen address on the GrpcServer Config container level<br>2. Verify the listen address
Expected Result | The listen address should be consistent with the expected value
Comments | 

#### 24. Subscribe listen address on GrpcServer State container level

Test       | **Subscribe listen address on GrpcServer State container level**
-|-
Description| This test verifies subscribing to the listen address on the GrpcServer State container level
Path       | /system/grpc-servers/grpc-server/state
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to the listen address on the GrpcServer State container level<br>2. Verify the listen address
Expected Result | The listen address should be consistent with the expected value
Comments | 

#### 25. Update listen address on listen-address leaf level

Test       | **Update listen address on listen-address leaf level**
-|-
Description| This test verifies updating the listen address on the listen-address leaf level
Path       | /system/grpc-servers/grpc-server/config/listen-addresses
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the listen address on the listen-address leaf level<br>2. Verify the updated listen address
Expected Result | The listen address should be updated correctly
Comments | 

#### 26. Subscribe listen address on listen-address config leaf level

Test       | **Subscribe listen address on listen-address config leaf level**
-|-
Description| This test verifies subscribing to the listen address on the listen-address config leaf level
Path       | /system/grpc-servers/grpc-server/config/listen-addresses
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to the listen address on the listen-address config leaf level<br>2. Verify the listen address
Expected Result | The listen address should be consistent with the expected value
Comments | 

#### 27. Subscribe listen address on listen-address state leaf level

Test       | **Subscribe listen address on listen-address state leaf level**
-|-
Description| This test verifies subscribing to the listen address on the listen-address state leaf level
Path       | /system/grpc-servers/grpc-server/state/listen-addresses
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to the listen address on the listen-address state leaf level<br>2. Verify the listen address
Expected Result | The listen address should be consistent with the expected value
Comments | 

#### 28. Modify leaf-list

Test       | **Modify leaf-list**
-|-
Description| This test verifies modifying the leaf-list
Path       | /system/grpc-servers/grpc-server/config/listen-addresses
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the leaf-list with a new address<br>2. Verify the updated leaf-list<br>3. Append a second address to the leaf-list<br>4. Verify the updated leaf-list<br>5. Remove the second address from the leaf-list<br>6. Verify the updated leaf-list
Expected Result | The leaf-list should be modified correctly
Comments | 

#### 29. Process restart emsd and get updated listen-address

Test       | **Process restart emsd and get updated listen-address**
-|-
Description| This test verifies the listen-address after restarting EMSD
Path       | /system/grpc-servers/grpc-server/config/listen-addresses
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the listen-address<br>2. Restart the EMSD process<br>3. Verify the updated listen-address
Expected Result | The listen-address should be updated correctly after restarting EMSD
Comments | 

#### 30. Reload router and check grpc before and after

Test       | **Reload router and check grpc before and after**
-|-
Description| This test verifies the gRPC configuration before and after reloading the router
Path       | /system/grpc-servers/grpc-server/config/listen-addresses
Preconditions | DUT should be up and running
Steps to Execute | 1. Update the listen-address<br>2. Check the gRPC configuration before reloading the router<br>3. Reload the router<br>4. Check the gRPC configuration after reloading the router
Expected Result | The gRPC configuration should be consistent before and after reloading the router
Comments | 
