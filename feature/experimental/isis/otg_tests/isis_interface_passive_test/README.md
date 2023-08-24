# RT-2.7: IS-IS Passive is enabled at interface level

## Summary

*  Base IS-IS functionality and adjacency establishment.
*  Ensure that IS-IS adjacency is not coming up on the passive interface

## Procedure

*   TestIsisInterfacePassive

    *	Configure IS-IS for ATE port-1 and DUT port-1.
    *	Configure DUT interface as ISIS passive interface.
    *	Ensure that IS-IS adjacency is not coming up on the passive interface.
    *	TODO-Verify the output of ST path displaying the interface as passive in ISIS database/adj table

## Config Parameter coverage

*   For prefix:

    *   /network-instances/network-instance/protocols/protocol/isis/

*   Parameters:

    *   global/config/authentication-check
    *   global/config/net
    *   global/config/level-capability
    *   global/config/hello-padding
    *   global/afi-safi/af/config/enabled
    *   levels/level/config/level-number
    *   levels/level/config/enabled
    *   levels/level/authentication/config/enabled
    *   levels/level/authentication/config/auth-mode
    *   levels/level/authentication/config/auth-password
    *   levels/level/authentication/config/auth-type
    *   interfaces/interface/config/interface-id
    *   interfaces/interface/config/enabled
    *   interfaces/interface/config/circuit-type
    *   interfaces/interface/config/passive
    *   interfaces/interface/timers/config/csnp-interval
    *   interfaces/interface/timers/config/lsp-pacing-interval
    *   interfaces/interface/levels/level/config/level-number
    *   interfaces/interface/levels/level/config/passive
    *   interfaces/interface/levels/level/timers/config/hello-interval
    *   interfaces/interface/levels/level/timers/config/hello-multiplier
    *   interfaces/interface/levels/level/hello-authentication/config/auth-mode
    *   interfaces/interface/levels/level/hello-authentication/config/auth-password
    *   interfaces/interface/levels/level/hello-authentication/config/auth-type
    *   interfaces/interface/levels/level/hello-authentication/config/enabled
    *   interfaces/interface/afi-safi/af/config/afi-name
    *   interfaces/interface/afi-safi/af/config/safi-name
    *   interfaces/interface/afi-safi/af/config/enabled

## Telemetry Parameter coverage

*   For prefix:

    *   /network-instances/network-instance/protocols/protocol/isis/

*   Parameters:

    *   interfaces/interface/state/passive
    *   interfaces/interface/levels/level/state/passive	
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv4-address
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv6-address
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/system-id
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/area-address
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/dis-system-id
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/local-extended-circuit-id
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/multi-topology
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-circuit-type
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-extended-circuit-id
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-snpa
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/nlpid
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/priority
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/restart-status
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/restart-support
    *   interfaces/interface/levels/level/adjacencies/adjacency/state/restart-suppress
    *   interfaces/interface/levels/level/afi-safi/af/state/afi-name
    *   interfaces/interface/levels/level/afi-safi/af/state/metric
    *   interfaces/interface/levels/level/afi-safi/af/state/safi-name
    *   interfaces/interface/levels/level/afi-safi/af/state/metric
    *   levels/level/system-level-counters/state/auth-fails
    *   levels/level/system-level-counters/state/auth-type-fails
    *   levels/level/system-level-counters/state/corrupted-lsps
    *   levels/level/system-level-counters/state/database-overloads
    *   levels/level/system-level-counters/state/exceed-max-seq-nums
    *   levels/level/system-level-counters/state/id-len-mismatch
    *   levels/level/system-level-counters/state/lsp-errors
    *   levels/level/system-level-counters/state/manual-address-drop-from-area 
    *   levels/level/system-level-counters/state/max-area-address-mismatches
    *   levels/level/system-level-counters/state/own-lsp-purges
    *   levels/level/system-level-counters/state/part-changes 
    *   levels/level/system-level-counters/state/seq-num-skips
    *   levels/level/system-level-counters/state/spf-runs
