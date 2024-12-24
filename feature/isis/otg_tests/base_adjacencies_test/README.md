# RT-2.1: Base IS-IS Process and Adjacencies

## Summary

Base IS-IS functionality and adjacency establishment.

## Testbed type

*  [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Procedure

### Test environment setup

*   DUT has an ingress port and 1 egress port.

    ```
                             |         |
        [ ATE Port 1 ] ----  |   DUT   | ---- [ ATE Port 2 ]
                             |         |
    ```

### RT-2.1.1 Basic fields test

*   Configure DUT:port1 for an IS-IS session with ATE:port1.
*   Read back the configuration to ensure that all fields are readable and
    have been set properly (or correctly have their default value).
*   Check that all relevant counters are readable and are 0 since the
    adjacency has not yet been established.
*   Push ATE configuration for the other end of the adjacency, and wait for
    the adjacency to form.
*   Check that the various state fields of the adjacency are reported
    correctly.
*   Check that error counters are still 0 and that packet counters have all
    increased.

### RT-2.1.2 Hello padding test

*   Configure IS-IS between DUT:port1 and ATE:port1 for each possible value
    of hello padding (DISABLED, STRICT, etc.)
*   Confirm in each case that that adjacency forms and the correct values
    are reported back by the device.

### RT-2.1.3 Authentication test

*   Configure IS-IS between DUT:port1 and ATE:port1 With authentication
    disabled, then enabled in TEXT mode, then enabled in MD5 mode.
*   Confirm in each case that that adjacency forms and the correct values
    are reported back by the device.

### RT-2.1.4 [TODO: https://github.com/openconfig/featureprofiles/issues/3421]

*   Configuration:
    *   Configure ISIS for ATE port-1 and DUT port-1.
    *   Configure both DUT and ATE interfaces as ISIS type point-to-point.
*   Verification:
    *   Verify that ISIS adjacency is coming up.
    *   Verify the output of streaming telemetry path displaying the interface circuit-type as point-to-point.

### RT-2.1.5 Routing test

*   Configure ISIS level authentication  and hello authentication.
*   Ensure that IPv4 and IPv6 prefixes that are advertised as attached
    prefixes within each LSP are correctly installed into the DUT
    routing table, by ensuring that packets are received to the attached
    prefix when forwarded from ATE port-1.
*   Ensure that IPv4 and IPv6 prefixes that are advertised as part of an
    (emulated) neighboring system are installed into the DUT routing
    table, and validate that packets are sent and received to them.
*   With a known LSP content, ensure that the telemetry received from the
    device for the LSP matches the expected content.

### RT-2.1.6 [TODO: https://github.com/openconfig/featureprofiles/issues/3422]

*   Baseline Configuration on the DUT:
    *   Set the hello-interval to a standard value (10 seconds).
    *   Set the hello-multiplier to its default (3).
    *   Check that the streaming telemetry values are reported correctly by the DUT.
*   Adjusting Hello-Interval configuration on the DUT:
    *   Change the hello-interval to a different value (15 seconds) in the DUT.
    *   Verify that IS-IS adjacency is coming up in the DUT.
    *   Verify that the updated Hello-Interval time is reflected in isis adjacency output in the ATE.
    *   Verify that the correct streaming telemetry values are reported correctly by the DUT.
*   Adjusting Hello-Multiplier configuration on the DUT:
    *   Change the hello-multiplier to a different value (5) the DUT.
    *   Verify that IS-IS adjacency is coming up in the DUT.
    *   Verify that the updated Hello-Multiplier is reflected in isis adjacency output in the ATE.
    *   Verify that the correct streaming telemetry values are reported correctly by the DUT.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.

```yaml
paths:
  ## Config paths
  /network-instances/network-instance/protocols/protocol/isis/global/config/authentication-check:
  /network-instances/network-instance/protocols/protocol/isis/global/config/net:
  /network-instances/network-instance/protocols/protocol/isis/global/config/level-capability:
  /network-instances/network-instance/protocols/protocol/isis/global/config/hello-padding:
  /network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/level-number:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/authentication/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/authentication/config/auth-mode:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/authentication/config/auth-password:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/authentication/config/auth-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/interface-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/timers/config/csnp-interval:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/timers/config/lsp-pacing-interval:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/config/level-number:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/timers/config/hello-interval:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/timers/config/hello-multiplier:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config/auth-mode:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config/auth-password:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config/auth-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config/enabled:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/afi-safi/af/config/afi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/afi-safi/af/config/safi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/afi-safi/af/config/enabled:


  ## State paths
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/state/circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/system-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/afi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/metric:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/safi-name:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/csnp/state/dropped:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/csnp/state/processed:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/csnp/state/received:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/csnp/state/sent:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/dropped:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/processed:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/received:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/retransmit:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/sent:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/dropped:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/processed:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/received:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/retransmit:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/sent:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/dropped:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/processed:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/received:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/retransmit:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/sent:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/timers/state/hello-interval:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/timers/state/hello-multiplier:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/adj-changes:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/adj-number:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/auth-fails:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/auth-type-fails:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/id-field-len-mismatches:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/lan-dis-changes:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/rejected-adj:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/area-address:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/dis-system-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/local-extended-circuit-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/multi-topology:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-circuit-type:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-extended-circuit-id:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv4-address:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv6-address:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-snpa:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/nlpid:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/priority:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-status:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-support:
  /network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-suppress:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/auth-fails:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/auth-type-fails:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/corrupted-lsps:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/database-overloads:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/exceed-max-seq-nums:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/id-len-mismatch:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/lsp-errors:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/max-area-address-mismatches:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/own-lsp-purges:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/seq-num-skips:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/spf-runs:
        ###For LSDB - Examples of paths
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/state/lsp-id:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/state/maximum-area-addresses:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/state/pdu-type:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/state/sequence-number:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/state/type:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/area-address/state/address:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/hostname/state/hostname:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv4-interface-addresses/state/address:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-interface-addresses/state/address:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv4-te-router-id/state/router-id:
  /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-te-router-id/state/router-id:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
```

## Minimum DUT platform requirement

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
* vRX - virtual router device
