# RT-2.1: Base IS-IS Process and Adjacencies

## Summary

Base IS-IS functionality and adjacency establishment.

## Procedure

*   Basic fields test
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
*   Hello padding test
    *   Configure IS-IS between DUT:port1 and ATE:port1 for each possible value
        of hello padding (DISABLED, STRICT, etc.)
    *   Confirm in each case that that adjacency forms and the correct values
        are reported back by the device.
*   Authentication test
    *   Configure IS-IS between DUT:port1 and ATE:port1 With authentication
        disabled, then enabled in TEXT mode, then enabled in MD5 mode.
    *   Confirm in each case that that adjacency forms and the correct values
        are reported back by the device.
*   Routing test
    *   With ISIS level authentication enabled and hello authentication enabled:
        *   Ensure that IPv4 and IPv6 prefixes that are advertised as attached
            prefixes within each LSP are correctly installed into the DUT
            routing table, by ensuring that packets are received to the attached
            prefix when forwarded from ATE port-1.
        *   Ensure that IPv4 and IPv6 prefixes that are advertised as part of an
            (emulated) neighboring system are installed into the DUT routing
            table, and validate that packets are sent and received to them.
    *   With a known LSP content, ensure that the telemetry received from the
        device for the LSP matches the expected content.

## Config Parameter coverage

*   For prefix:

    *   /network-instances/network-instance/protocols/protocol/isis/

*   Parameters:

    *   TODO: global/config/authentication-check
    *   global/config/net
    *   global/config/level-capability
    *   global/config/hello-padding
    *   global/afi-safi/af/config/enabled
    *   levels/level/config/level-number
    *   levels/level/config/enabled
    *   levels/level/authentication/config/enabled
    *   levels/level/authentication/config/auth-mode
        levels/level/authentication/config/auth-password
    *   levels/level/authentication/config/auth-type
    *   interfaces/interface/config/interface-id
    *   interfaces/interface/config/enabled
    *   interfaces/interface/config/circuit-type
    *   interfaces/interface/timers/config/csnp-interval
    *   interfaces/interface/timers/config/lsp-pacing-interval
    *   interfaces/interface/levels/level/config/level-number
    *   interfaces/interface/levels/level/timers/config/hello-interval
    *   interfaces/interface/levels/level/timers/config/hello-multiplier
    *   interfaces/interface/levels/level/hello-authentication/config/auth-mode
    *   network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/hello-authentication/config/auth-password
    *   interfaces/interface/levels/level/hello-authentication/config/auth-type
    *   interfaces/interface/levels/level/hello-authentication/config/enabled
    *   interfaces/interface/afi-safi/af/config/afi-name
    *   interfaces/interface/afi-safi/af/config/safi-name
    *   interfaces/interface/afi-safi/af/config/metric
    *   interfaces/interface/afi-safi/af/config/enabled

## Telemetry Parameter coverage

*   For prefix:

    *   /network-instances/network-instance/protocols/protocol/isis/

*   Parameters:

    *   interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv4-address
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv6-address
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/system-id
    *   interfaces/interface/levels/level/afi-safi/af/state/afi-name
    *   interfaces/interface/levels/level/afi-safi/af/state/metric
    *   interfaces/interface/levels/level/afi-safi/af/state/safi-name
    *   interfaces/interface/levels/level/afi-safis/afi-safi/state/metric
    *   interfaces/interface/levels/level/packet-counters/cnsp/dropped
    *   interfaces/interface/levels/level/packet-counters/cnsp/processed
    *   interfaces/interface/levels/level/packet-counters/cnsp/received
    *   interfaces/interface/levels/level/packet-counters/cnsp/sent
    *   interfaces/interface/levels/level/packet-counters/iih/dropped
    *   interfaces/interface/levels/level/packet-counters/iih/processed
    *   interfaces/interface/levels/level/packet-counters/iih/received
    *   interfaces/interface/levels/level/packet-counters/iih/retransmit
    *   interfaces/interface/levels/level/packet-counters/iih/sent
    *   interfaces/interface/levels/level/packet-counters/lsp/dropped
    *   interfaces/interface/levels/level/packet-counters/lsp/processed
    *   interfaces/interface/levels/level/packet-counters/lsp/received
    *   interfaces/interface/levels/level/packet-counters/lsp/retransmit
    *   interfaces/interface/levels/level/packet-counters/lsp/sent
    *   interfaces/interface/levels/level/packet-counters/psnp/dropped
    *   interfaces/interface/levels/level/packet-counters/psnp/processed
    *   interfaces/interface/levels/level/packet-counters/psnp/received
    *   interfaces/interface/levels/level/packet-counters/psnp/retransmit
    *   interfaces/interface/levels/level/packet-counters/psnp/sent
    *   interfaces/interfaces/circuit-counters/state/adj-changes
    *   interfaces/interfaces/circuit-counters/state/adj-number
    *   interfaces/interfaces/circuit-counters/state/auth-fails
    *   interfaces/interfaces/circuit-counters/state/auth-type-fails
    *   interfaces/interfaces/circuit-counters/state/id-field-len-mismatches
    *   interfaces/interfaces/circuit-counters/state/lan-dis-changes
    *   interfaces/interfaces/circuit-counters/state/max-area-address-mismatch
    *   interfaces/interfaces/circuit-counters/state/rejected-adj
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/adjacency-state
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/area-address
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/dis-system-id
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/local-extended-system-id
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/multi-topology
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/neighbor-circuit-type
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/neighbor-extended-system-id
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/neighbor-ipv4-address
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/neighbor-ipv6-address
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/neighbor-snpa
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/nlpid
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/priority
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/remaining-hold-time
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/restart-status
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/restart-support
    *   interfaces/interfaces/levels/level/adjacencies/adjacency/state/restart-suppress
    *   levels/level/system-level-counters/state/auth-fails
    *   levels/level/system-level-counters/state/auth-type-fails
    *   levels/level/system-level-counters/state/corrupted-lsps
    *   levels/level/system-level-counters/state/database-overloads
    *   levels/level/system-level-counters/state/exceeded-max-seq-nums
    *   levels/level/system-level-counters/state/id-len-mismatch
    *   levels/level/system-level-counters/state/lsp-errors
    *   levels/level/system-level-counters/state/max-area-address-mismatches
    *   levels/level/system-level-counters/state/own-lsp-purges
    *   levels/level/system-level-counters/state/seq-num-skips
    *   levels/level/system-level-counters/state/spf-runs

*   For LSDB - subpaths of

    *   /network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/...

## Protocol/RPC Parameter coverage

*   IS-IS:
    *   LSP messages
        *   TLV 1 (Area Addresses)
        *   TLV 10 (Authentication)
        *   TLV 22 (Extended IS reach)
        *   TLV 135 (Extended IP Reachability)
        *   TLV 137 (Dynamic Name)
        *   TLV 232 (IPv6 Reachability)

## Minimum DUT platform requirement

vRX
