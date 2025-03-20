# Optical Transport Network (OTN) Transceiver Test Suite

## Overview
This project contains a test suite for validating the functionality of Optical Transport Network (OTN) transceivers. The tests are designed to ensure that the OTN channels and interfaces operate correctly under various conditions. The suite leverages the OnDatra testing framework and OpenConfig models to configure and verify the state of the device under test (DUT).

OTN tests here are done to subscribe and stream OTN leaves, for this the tests configure OTN on two back to back ports. Sample config is given below.

```
RP/0/RP1/CPU0:sfd18#show running-config terminal-device 
Wed Mar 19 12:25:56.808 UTC
terminal-device
 logical-channel 4000
  admin-state enable
  description Coherent Logical Channel
  loopback-mode terminal
  logical-channel-type Otn
  assignment-id 1
   allocation 400
   assignment-type optical
   description OTN to Optical Channel
   assigned-optical-channel OpticalChannel0_0_0_32
  !
 !
 logical-channel 4001
  admin-state enable
  description Coherent Logical Channel
  loopback-mode terminal
  logical-channel-type Otn
  assignment-id 1
   allocation 400
   assignment-type optical
   description OTN to Optical Channel
   assigned-optical-channel OpticalChannel0_0_0_35
  !
 !
 logical-channel 40000
  rate-class 400G
  admin-state enable
  description ETH Logical Channel
  loopback-mode terminal
  trib-protocol 400GE
  logical-channel-type Ethernet
  assignment-id 1
   allocation 400
   assignment-type logical
   description ETH to Coherent assignment
   assigned-logical-channel 4000
  !
 !
 logical-channel 40001
  rate-class 400G
  admin-state enable
  description ETH Logical Channel
  loopback-mode terminal
  trib-protocol 400GE
  logical-channel-type Ethernet
  assignment-id 1
   allocation 400
   assignment-type logical
   description ETH to Coherent assignment
   assigned-logical-channel 4001
  !
 !
 optical-channel OpticalChannel0_0_0_32
  power -900
  frequency 193100000
  operational-mode 5003
 !
 optical-channel OpticalChannel0_0_0_35
  power -900
  frequency 193100000
  operational-mode 5003
 !
!

RP/0/RP1/CPU0:sfd18#

```


## Files
- `transceiver_otn_test.go`: This file includes the test functions such as `TestZRShutPort`, `TestZRRPFO`,`TestZRProcessRestart`, and `TestZRLCReload`. Each function validates the operational status of OTN channels and interfaces.


## Running the Tests

To execute the test suite, run the following command in the terminal:
Running each testcase:
```bash
go test -timeout 0 -run TestZRLCReload . -binding=$BINDING -testbed=$TESTBED -alsologtostderr -v 5 > /Users/gsrungav/Desktop/output.log 
```

Running all tests
```bash
go test -timeout 0 . -binding=$BINDING -testbed=$TESTBED -alsologtostderr -v 5 > /Users/gsrungav/Desktop/output.log 
```

## FEAT-ID
https://miggbo.atlassian.net/browse/XR-45985