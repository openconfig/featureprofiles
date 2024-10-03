# RT-2.10: IS-IS change LSP lifetime

## Summary

* Changing the lsp lifetime and verifying isis lsp parameters

## Topology

* ATE:port1 <-> port1:DUT:port2 <-> ATE:port2

## Procedure

    * Configure IS-IS for ATE port-1 and DUT port-1.
    * Modify the default lifetime of the LSP PDU.
    * The default lifetime of the LSP PDU is 1200 seconds.
        This parameter can be updated using the LSP lifetime parameter.
        LSP lifetime indicates how long the LSP PDU originated by the DUT should remain in the network. 
        The DUT regenerates the LSP PDU typically ~300 seconds before its expiration.
    * Change the LSP lifetime to 500secs    
    * Verify that IS-IS adjacency for IPv4 and IPV6 address family is coming up.
    * Verify that IPv4 and IPv6 prefixes that are advertised by ATE correctly installed into DUTs route and forwarding table.
    * Verify that the updated LSP lifetime is reflected in isis database output.
    * Verify that the remaining lifetime of the lsp is remaining lifetime = configured lifetime - time passed since the LSP PDU generation.
    * Verify that once the new LSP PDU is generated the sequence number and checksum of the new LSP PDU is updated

## Config Parameter coverage

* For prefix:

    * /network-instances/network-instance/protocols/protocol/isis/

* Parameters:

    * global/timers/config/lsp-lifetime-interval 

## Telemetry Parameter coverage

* For prefix:

    * /network-instances/network-instance/protocols/protocol/isis/

* Parameters:

    * global/timers/state/lsp-lifetime-interval
    * levels/level/link-state-database/lsp/state/remaining-lifetime
