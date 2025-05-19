# Gribi Scale

### Topology
```
+-------------------------+                 +-------------------------+
|          DUT            |                 |          ATE            |
|      CISCO-8818         |                 |                         |
|-------------------------|                 |-------------------------|
| port1 [100GB]           |-----------------| port1 [100GB]           |
| port2 [100GB]           |-----------------| port2 [100GB]           |
| port3 [100GB]           |-----------------| port3 [100GB]           |
| port4 [100GB]           |-----------------| port4 [100GB]           |
| port5 [100GB]           |-----------------| port5 [100GB]           |
| port6 [100GB]           |-----------------| port6 [100GB]           |
| port7 [100GB]           |-----------------| port7 [100GB]           |
| port8 [100GB]           |-----------------| port8 [100GB]           |
| port9 [100GB]           |-----------------| port9 [100GB]           |
| port10 [100GB]          |-----------------| port10 [100GB]          |
| port11 [100GB]          |-----------------| port11 [100GB]          |
| port12 [100GB]          |-----------------| port12 [100GB]          |
| port13 [100GB]          |-----------------| port13 [100GB]          |
| port14 [100GB]          |-----------------| port14 [100GB]          |
| port15 [100GB]          |-----------------| port15 [100GB]          |
+-------------------------+                 |                         |
          |                                 |                         |
          |                                 |                         |
          | ( Dynamic                       |                         |
          |   Bundle                        |                         |
          |   Creation )                    |                         |
          |                                 |                         |
          |                                 |                         |
+-------------------------+                 |                         |
|          PEER           |                 |                         |
|      CISCO-8808         |                 |                         |
|-------------------------|                 |                         |
| port16 [100GB]          |-----------------| port16 [100GB]          |
+-------------------------+                 |          ATE            |
                                            +-------------------------+
```

# TestTrigger Suite

This document provides an overview of the `TestTrigger` suite, which is designed to test various network process triggers and configurations on a Device Under Test (DUT). The test suite is implemented using Go's testing framework.

## Overview

The `TestTrigger` function is a Go test designed to execute a series of network process triggers on a DUT. Each trigger is accompanied by a defined duration, and the suite also includes mechanisms for log collection and traffic verification.

## Test Resources

The test suite begins by initializing necessary test resources using the `initializeTestResources` function. This function sets up the required context and resources for testing.

##### Processes

A set of processes is defined for restarting during the tests, which include:

| Category| Processes|
|-|-|
| Control Plane  | `emsd`,`db_writer` |
| Routing  | `bgp`,`ipv6_rib`,`fib_mgr`,`ifmgr`,`isis`,`ipv4_rib` |

## Test Triggers

The test suite defines various triggers, each with a specific function, duration, and a flag indicating whether reprogramming is required:

- **RPFO**: Utilizes `utils.Dorpfo` for Resource Path Failover testing.
- **LC Reload**: Executes `utils.DoAllAvailableLcParallelOir` to test Line Card Online Insertion and Removal.
- **ProcessRestartParllel**: Uses `utils.DoProcessesRestart` to restart processes in parallel.
- **ProcessRestartSequential**: Sequential restart of processes using `utils.DoProcessesRestart`.
- **gNOI Reboot**: Reboots the DUT using `utils.GnoiReboot`.
- **LC Shut/Unshut**: Shuts and unshuts line cards in parallel using `utils.DoShutUnshutAllAvailableLcParallel`.
- **GrpcConfigChange**: Tests gRPC configuration changes with `utils.GrpcConfigChange`.

## Log Collection and Verification

After each trigger, the suite performs log collection and traffic verification:

1. **Log Collection After Trigger**: Collects router logs using `log_collector.CollectRouterLogs`.
2. **Traffic Verification**: Placeholder function to verify traffic.
3. **Log Collection After Traffic Validation**: Collects router logs post traffic validation.

## Reprogramming

If a trigger requires reprogramming, the suite includes a subtest to handle this using a placeholder `dummy` function.


