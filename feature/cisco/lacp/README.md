# LACP Tests

## Summary

This package contains tests for configuring and verifying Link Aggregation Control Protocol (LACP) settings on network devices using the OpenConfig (OC) model. It includes tests for LACP configuration, state verification, telemetry, and member management.

## Topology

(DUT) <-------> (peer/dut2)
```
Device name: dut
Interface under test: Bundle-Ether120
Members of the bundle interface are dynamically configured from the input file.
```


## Procedure

- LACP Configuration Tests
  -  Configure LACP settings on the DUT.
  -  Verify that configuration updates are properly applied using gNMI.

- LACP State Tests
  - Validate LACP state data such as operational key, system ID, and port number.

- LACP Counters State Tests
  - Check LACP counters for errors and packet statistics.

- LACP Telemetry Tests
  - Subscribe to LACP telemetry data and verify updates for configuration changes over time.

- LACP Member Management Tests
  - Add, delete, shut, and un-shut members of a bundle interface.
  - Verify that updates are received by the gNMI client.
  - Test re-parenting of member interfaces to different bundle interfaces.

- LACP Member Edit Tests
  - Perform operations like reloading line cards and verify telemetry data updates.


## File Organization

```
.
├── lacp_test.go        <- Contains the LACP test cases
├── README.md           <- This README file
└── testdata
    └── interface.yaml  <- Initial interface configuration for the DUT
```

## OpenConfig Path and RPC Coverage

The following OpenConfig paths and RPCs are covered by these tests:

```
RPCs

    gNMI
        Update:
            /lacp/interfaces/interface/config/interval
            /lacp/interfaces/interface/config/system-priority
            /lacp/interfaces/interface/config/system-id-mac
            /lacp/interfaces/interface/config/lacp-mode
        Replace:
            /interfaces/interface/ethernet/config/aggregate-id
        Delete:
            /lacp/interfaces/interface/config
        Subscribe:
            /lacp/interfaces/interface/state/system-id-mac
            /lacp/interfaces/interface/state/system-priority
            /lacp/interfaces/interface/members/member/state/oper-key
            /lacp/interfaces/interface/members/member/state/system-id
            /lacp/interfaces/interface/members/member/state/port-num
            /lacp/interfaces/interface/members/member/state/partner-id
            /lacp/interfaces/interface/members/member/state/counters/lacp-errors
            /lacp/interfaces/interface/members/member/state/counters/lacp-in-pkts
            /lacp/interfaces/interface/members/member/state/counters/lacp-out-pkts
            /lacp/interfaces/interface/members/member/state/counters/lacp-unknown-errors
            /lacp/interfaces/interface/members/member/state/counters/lacp-rx-errors
            /lacp/interfaces/interface/members/member/state/counters/lacp-timeout-transitions
```

## Notes

The tests use a sample input file interface.yaml for initial configuration.

Ensure the device under test is correctly set up and accessible for gNMI operations before running these tests.

The test cases assume the existence of certain interfaces and configurations as defined in the interface.yaml. Adjust the file for different setups if necessary.


