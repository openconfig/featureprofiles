# P4RT-2.1: P4RT Election

## Summary

Validate the P4RT server handles primary election and failover.

## Procedure

*   Enable P4RT on a single FAP by configuring an ID on the device and one or
    more interfaces.
*   Verify that the right clients become primary. Verify that primary can read &
    write and that non-primary can only read through the following scenarios:
    *   Become Primary
        1.  Connect two P4RT clients with different election IDs.
        2.  Verify client with the higher election ID (primary) receives a
            successful MasterArbitrationUpdate.
        3.  Verify primary client can read as well as write.
    *   Fail to become Primary
        1.  Connect two P4RT clients with different election IDs.
        2.  Verify client with the lower election ID (secondary) receives a
            successful MasterArbitrationUpdate.
        3.  Verify secondary client can read but not write.
    *   Replace Primary
        1.  Connect two P4RT clients with different election IDs.
        2.  Verify client with the lower election ID (secondary) receives a
            successful MasterArbitrationUpdate.
        3.  Verify secondary client can read but not write.
        4.  TODO: Trigger MasterArbitrationUpdate using the secondary client
            with an election ID higher than that of primary client.
        5.  TODO: Verify that the old secondary client now becomes primary and
            able to read and write.
        6.  TODO: Verify that `status` field of `new primary` client's
            MasterArbitrationUpdate response is set to `google.rpc.OK`.
        7.  TODO: Verify that `election_id` field of `new primary` client's
            MasterArbitrationUpdate response is set to the highest election_id.
        8.  TODO: Verify that old primary is now only able to read and not
            write.
        9.  TODO: Verify that `status` field of `old primary` client's
            MasterArbitrationUpdate response is set to
            `google.rpc.ALREADY_EXISTS`.
        10. TODO: Verify that `election_id` field of `old primary` client's
            MasterArbitrationUpdate response is set to `new primary` client's
            election_id.
    *   Replace Primary after Failure
        1.  Connect two P4RT clients with different election IDs.
        2.  Verify primary client can read and write.
        3.  Stop primary client by closing the stream.
        4.  Trigger MasterArbitrationUpdate using the secondary client with an
            election ID equal to that of primary client.
        5.  Verify that old secondary client now becomes primary and able to
            read and write.
    *   TODO: Fail To become Primary after Primary Disconnect
        1.  Connect two P4RT clients with different election IDs.
        2.  Verify primary client can read and write.
        3.  Stop primary client by closing the Stream.
        4.  Verify that the secondary client can only read and not write.
        5.  Verify that `status` field of `secondary` client's
            MasterArbitrationUpdate response is set to `google.rpc.NOT_FOUND`.
    *   Reconnect Primary
        1.  Connect two P4RT clients with different election IDs.
        2.  Verify primary client can read and write.
        3.  Stop primary client by closing the stream.
        4.  Connect a new P4RT client with election ID higher that old primary
            election ID.
        5.  verify that new primary client is able to read and write.
    *   Double Primary
        1.  Connect two P4RT clients with different election IDs.
        2.  Verify primary client can read and write.
        3.  TODO: Trigger MasterArbitrationUpdate using the secondary client
            with an election ID equal to that of primary client.
        4.  TODO: Verify secondary client stream terminates with
            `google.rpc.INVALID_ARGUMENT`.
        5.  Connect a new P4RT client with election ID equal to that of primary
            client.
        6.  Verify new client's stream terminates with
            `google.rpc.INVALID_ARGUMENT`.
    *   Unset Election ID
        1.  Connect two P4RT clients with an `unset` election ID and no other
            active P4RT clients for the corresponding device_id. (`unset` and
            `zero` electionIDs are two different scenarios and a `zero`
            electionID is considered as being Set)
        2.  Verify that the clients are able to read and not write using Get and
            Set ForwardingPipelineConfig requests.
    *   TODO: Long Evolution
        1.  Connect five P4RT clients to the same device_id with election_id's
            1,2,3,4,5
        2.  Verify primary client is able to read and write.
        3.  Trigger MasterArbitrationUpdate from client with `election_id=1` and
            make it primary using `election_id=6`.
        4.  Verify that client with `election_id=6` is able to read and write.
        5.  Verify that client with `election_id=5` is able to read and not
            write.
        6.  Repeat steps `c`, `d`, `e` for the below client and election_id
            combinations:
            *   MasterArbitrationUpdate from client with `election_id=2` and
                make it primary using `election_id=7` and verify correct read &
                writes for clients with `election_id=6` & `election_id=7`.
            *   MasterArbitrationUpdate from client with `election_id=3` and
                make it primary using `election_id=8` and verify correct read &
                writes for clients with `election_id=7` & `election_id=8`.
            *   MasterArbitrationUpdate from client with `election_id=4` and
                make it primary using `election_id=9` and verify correct read &
                writes for clients with `election_id=8` & `election_id=9`.
            *   MasterArbitrationUpdate from client with `election_id=5` and
                make it primary using `election_id=10` and verify correct read &
                writes for clients with `election_id=9` & `election_id=10`.
*   TODO: Enable P4RT on an additional FAP and verify that the same set of
    scenarios work independently of the first FAP
