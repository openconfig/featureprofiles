# gNMI-1.7: Telemetry: NPU/ASIC Counters

## Summary

Validate the supported OC Pipeline Counters.

## Procedure

Note: New telemetry model is still being defined.

*   Connect ATE port-1 to DUT port-1. ATE port-2 to DUT port-2. ATE port-3 to DUT port-3.
*   OuterDstIP_1, OuterSrcIP_1, OuterSrcIP_2: IPinIP outer IP addresses.
*   InnerDstIP_1, InnerSrcIP_1: IPinIP inner IP addresses.
*   ATEPort2IP: testbed assigned interface IP to ATE port 2.
*   ATEPort3IP: testbed assigned interface IP to ATE port 3.
*   Connect a gRIBI client to the DUT, make it become leader and inject the following,

    NHG#1 --> NH#1 {next-hop: ATEPort2IP, network-instance:DEFAULT}
    OuterDstIP_1/32  --> NHG#1.

*   Send IPinIP traffic to OuterDstIP_1 from ATE port-1. Validate that ATE port-2 receives the IPinIP traffic.

*   Packet Counters - Interface-Block

    *   Subscribe to the following, and capture values of the 4 leaves (1).
        * /packet/interface-block/in-pkts
        * /packet/interface-block/out-pkts
        * /packet/interface-block/in-bytes
        * /packet/interfac-block/out-bytes
    *   Wait for 30 secs and capture the values of 4 leaves(2). 
    *   Calculation of step 2 - step 1 leaf values. 
        * 30*pps = in-pkts and out-pkts
        * 30*pps*300 = in-bytes and out-bytes
    * Stop traffic.

*   Packet Counters - Host-Block

    *   Subscribe to the following, and capture the values of the 4 leaves(1),
        * /packet/host-interface-block/in-pkts
        * /packet/host-interface-block/out-pkts
        * /packet/host-interface-block/in-bytes (TBD)
        * /packet/host-interface-block/in-bytes (TBD)
    *   Define variance = 0.99
    *   Send ICMP Echo traffic from  ATEPort1IP with destination DUTPort2IP. Wait for 30 secs and Stop traffic.
    *   Capture the in-pkts and out-pkts leaves(2).
    *   Calculation of step 2 - step 1 leaf values.
        * Step 2 - Step 1 >= 30 * pps
        * Step 2/Step 1 >= variance
    *   Verify in-bytes and out-bytes (TBD)

*   Packet Countes - Queueing Block

    *   Subscribe to the following, and capture the values of the 4 leaves(1),
        * /packet/queueing-block/in-pkts(TBD)
        * /packet/queueing-block/in-bytes(TBD)
        * /packet/queueing-block/out-pkts,
        * /packet/queueing-block/out-bytes
    *   Define variance = 0.98.
    *   Start traffic.
    *   Wait for 30 secs.
    *   Capture the out-pkts and out-bytes leaf values (2).
    *   Calculation of step 2 - step 1.
        *  30 * pps = out-pkts.
        *  30 * pps * 300 = out-bytes.
        *  Capture tgen rx packets, tgen rx packets/step(2) values >= variance.
    *   Stop traffic.


*   Drop Counters - Lookup-Block

    *   Subscribe to the following, and capture the leaf values(1)
        * /drop/lookup-block/no-route
        * /drop/lookup-block/no-nexthop
        * /drop/lookup-block/invalid-packet
        * /drop/lookup-block/forwarding-policy (TBD)
        * /drop/lookup-block/incorrect-software-state
        * /drop/lookup-block/rate-limit (TBD)
        * /drop/lookup-block/acl-drops
    
    *   Start traffic, verify traffic received on atePort2.
    *   Shutdown ateport2. Wait for 30 secs.
    *   Verify /drop/lookup-block/no-route counter populated (2).
        * Step 2-Step 1(no-route), calculation 30 * pps.       
    *   Unshut ateport2, verify traffic recovers on atePort2.

    *   Configure static Null0 route for outerDst_IP1.
    *   After 30 secs, Verify /drop/lookup-block/no-nexthop leaf populated(3).
        * Step 3- Step 1(no-nexthop), calcuation 30 * pps.
    *   Unconfigure static route, and verify traffic recovers on atePort2, stop traffic.

    *   Send traffic with custom checksum in ipv4 header.
    *   After 30 secs, Verify /drop/lookup-block/invalid-packet leaf populated(4).
        * Step 4 - Step 1(invalid-packet), calculation 30*pps.
    *   Stop traffic. 

    *   /drop/lookup-block/forwarding-policy <TBD>

    *   Send traffic to Outer_Dst_IP1, and verify traffic received on atePort2.
    *   Remove ipv4 address on dutport1.
    *   After 30 secs, Verify /drop/lookup-block/incorrect-software-state populated(5).
        * Step 5 - Step 1(incorrect-software-state), calculation 30*pps.
    *   Stop traffic.

    *   /drop/lookup-block/rate-limit (TBD).

    *   Configure ACL permit for source ip OuterSrcIP_2 and apply on egress DUT port-2.
    *   Start traffic and wait for 30 secs.
    *   Capture /drop/lookup-block/acl-drops(6).
    *   Calculation - Tgen drop counter = Step 6 - Step 1.
    *   Stop traffic.
    *   Remove acl and verify traffic received on ATE port2.

*   Drop Counters - Interface Block
    *   drop/interface-block/state/oversubscription (TBD)

*   Drop Counters - Queueing Block
    *   Connect a gRIBI client to the DUT, make it become leader, flush and inject the following,

        NHG#1 --> NH#1 {next-hop: ATEPort2IP}
    *   Subscribe to /drop/queueing-block/oversubscription, and capture the leaf value(1).
    *   Send traffic, 
        * Flow 1 from ATE port 1 to ATE port 2 with line rate 75%.
        * Flow 2 from ATE port 3 to ATE port 2 at 75% linerate.
    *   Wait for 30 secs.
    *   Capture the leaf value (2).
    *   Calculation - Flow 1 + Flow 2 drop counters = Step 2 - Step 1.
    *   Stop traffic.
    

## Config Parameter coverage

No configuration coverage.

## Telemetry Parameter coverage

Under the prefix of /components/component/integrated-circuit/pipeline-counters/:
   
 *   packet/interface-block/state/in-packets
 *   packet/interface-block/state/out-packets
 *   packet/interface-block/state/in-bytes 
 *	 packet/interface-block/state/out-bytes 
 *	 packet/queueing-block/state/in-packets
 *	 packet/queueing-block/state/out-packets
 *	 packet/queueing-block/state/in-bytes (TBD)
 *	 packet/queueing-block/state/out-bytes
 *	 packet/host-interface-block/state/in-packets
 *	 packet/host-interface-block/state/out-packets
 *	 packet/host-interface-block/state/in-bytes (TBD)
 *	 packet/host-interface-block/state/out-bytes (TBD)
 *	 drop/interface-block/state/oversubscription (TBD)
 *	 drop/queueing-block/state/oversubscription
 *	 drop/lookup-block/no-route
 *	 drop/lookup-block/no-nexthop
 *	 drop/lookup-block/invalid-packet
 *	 drop/lookup-block/forwarding-policy (TBD)
 *	 drop/lookup-block/incorrect-software-state
 *	 drop/lookup-block/rate-limit (TBD)
 *	 drop/lookup-block/acl-drops

The following counters are still proposed and under review
 *	packet/lookup-block/state/lookup-memory
 *	packet/lookup-block/state/lookup-memory-used
 *	packet/lookup-block/state/nexthop-memory
 *	packet/lookup-block/state/nexthop-memory-used
 *	packet/lookup-block/state/acl-memory-total-entries
 *	packet/lookup-block/state/acl-memory-used-entries
 *	packet/lookup-block/state/acl-memory-total-bytes
 *	packet/lookup-block/state/acl-memory-used-bytes

## Protocol/RPC Parameter coverage

N/A

## Minimum DUT platform requirement

N/A
