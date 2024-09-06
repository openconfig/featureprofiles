# System - Test Cases

## Overview

This repository folder `featureprofiles/feature/cisco/system` contains Go test script designed to validate various system features of Cisco devices. The test cases  cover a wide range of functionalities, including system boot time, gRPC configurations, IANA Ports, CPU metrics, memory states, hostname, NTP and SSH.

## Table of Contents

- [System - Test Cases](#system---test-cases)
  - [Overview](#overview)
  - [Table of Contents](#table-of-contents)
  - [Prerequisites](#prerequisites)
  - [Topology](#topology)
  - [Test Case Structure](#test-case-structure)
  - [Test Cases](#test-cases)
    - [Test System GRPC](#test-system-grpc)
    - [Test System Hostname](#test-system-hostname)
    - [Test System CPU](#test-system-cpu)
    - [Test IANA Ports](#test-iana-ports)
    - [Test Module Time](#test-module-time)
      - [1. Subscribe to system state boot-time](#1-subscribe-to-system-state-boot-time)
    - [Test System Memory](#test-system-memory)
      - [1. Testing system memory state reserved](#1-testing-system-memory-state-reserved)
      - [2. Testing system memory state physical](#2-testing-system-memory-state-physical)
    - [Test Module SSH](#test-module-ssh)
      - [1. Replace system ssh-server config enable](#1-replace-system-ssh-server-config-enable)
      - [2. Update system ssh-server config enable](#2-update-system-ssh-server-config-enable)
      - [3. Delete system ssh-server config enable](#3-delete-system-ssh-server-config-enable)
      - [4. Subscribe system ssh-server config enable](#4-subscribe-system-ssh-server-config-enable)


## Prerequisites

Before using the test cases, ensure you have the following:

- Basic understanding of the system under test
- 8000 Series router as DUT ( SIM or HW set up )

## Topology

graph TD;
    Router---Switch1;
    Router---Switch2;
    Switch1---Server;
    Switch2---Server;

## Test Case Structure

Each test case in this list follows a standardized structure for consistency and ease of use. Below is the structure of a typical test case:

1. **Test Case ID**: Unique identifier for the test case
2. **Title**: Brief description of the test case
3. **Description**: Detailed explanation of the test case
4. **Preconditions**: Any prerequisites or setup required before executing the test case
5. **Steps to Execute**: Step-by-step instructions to perform the test
6. **Expected Result**: The expected outcome of the test
7. **Comments**: Any additional notes or observations

## Test Cases

Below is a list of test seperated module wise

### Test System GRPC

Refer [system.md](./system.md)

### Test System Hostname

Refer [hostname.md](./hostname.md)

### Test System CPU

Refer [cpu.md](./cpu.md)

### Test IANA Ports

Refer [ianaports.md](./ianaports.md)

### Test Module Time

#### 1. Subscribe to system state boot-time

Test       | **Subscribe to system state boot-time**
-|-
Description| This test verifies the timestamp that the system was last restarted can be read and is not an unreasonable value
Path       | /system/state/boot-time
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/state/boot-time`<br>2. Verify the boot-time value
Expected Result | The boot-time value should be after Dec 22, 2021 00:00:00 GMT in nanoseconds (1640131200000000000)
Comments | Boot time should be after Dec 22, 2021 00:00:00 GMT in nanoseconds

### Test System Memory

#### 1. Testing system memory state reserved

Test       | **Testing system memory state reserved**
-|-
Description| This test verifies the reserved memory state by subscribing to it
Path       | /system/memory/state/reserved
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/memory/state/reserved`<br>2. Verify the reserved memory value
Expected Result | The reserved memory value should be `0`
Comments |

#### 2. Testing system memory state physical

Test       | **Testing system memory state physical**
-|-
Description| This test verifies the physical memory state by subscribing to it
Path       | /system/memory/state/physical
Preconditions | DUT should be up and running
Steps to Execute | 1. Subscribe to `/system/memory/state/physical`<br>2. Verify the physical memory value
Expected Result | The physical memory value should be greater than `0`
Comments | 

</details>

### Test Module SSH

#### 1. Replace system ssh-server config enable

Test       | **Replace system ssh-server config enable**
-|-
Description| This test verifies replacing the SSH server enable configuration
Path       | /system/ssh-server/config/enable
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace `/system/ssh-server/config/enable` with `true`<br>2. Verify the SSH server enable configuration<br>3. Replace `/system/ssh-server/config/enable` with `false`<br>4. Verify the SSH server enable configuration
Expected Result | The SSH server enable configuration should be replaced correctly
Comments | 

#### 2. Update system ssh-server config enable

Test       | **Update system ssh-server config enable**
-|-
Description| This test verifies updating the SSH server enable configuration
Path       | /system/ssh-server/config/enable
Preconditions | DUT should be up and running
Steps to Execute | 1. Update `/system/ssh-server/config/enable` with `true`<br>2. Verify the SSH server enable configuration<br>3. Update `/system/ssh-server/config/enable` with `false`<br>4. Verify the SSH server enable configuration
Expected Result | The SSH server enable configuration should be updated correctly
Comments | 

#### 3. Delete system ssh-server config enable

Test       | **Delete system ssh-server config enable**
-|-
Description| This test verifies deleting the SSH server enable configuration
Path       | /system/ssh-server/config/enable
Preconditions | DUT should be up and running
Steps to Execute | 1. Update `/system/ssh-server/config/enable` with `true`<br>2. Delete `/system/ssh-server/config/enable`<br>3. Verify the SSH server enable configuration is removed
Expected Result | The SSH server enable configuration should be removed correctly
Comments | 

#### 4. Subscribe system ssh-server config enable

Test       | **Subscribe system ssh-server config enable**
-|-
Description| This test verifies subscribing to the SSH server enable configuration
Path       | /system/ssh-server/config/enable
Preconditions | DUT should be up and running
Steps to Execute | 1. Replace `/system/ssh-server/config/enable` with `true`<br>2. Subscribe to `/system/ssh-server/config/enable`<br>3. Verify the SSH server enable configuration
Expected Result | The SSH server enable configuration should be `true`
Comments | 
