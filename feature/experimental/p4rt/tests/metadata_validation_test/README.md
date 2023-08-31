# P4RT-2.2: P4RT Metadata Validation

## Summary

Validate the P4RT server handles Metadata set in Table Entry correctly.

## Procedure

*   Enable P4RT on a single FAP by configuring an ID on the device and one or
    more interfaces.
*   Instantiate a `primary` P4RT client and execute Client Arbitration and Set
    the Forwarding Pipeline using the `wbb.p4info.pb.txt` file.
*   Write a `TableEntry` with `Metadata` field set and validate correct
    `Metadata` is retrieved in `ReadRequest` for the below scenarios:
    *   Using Update Type as `INSERT`, Write a TableEntry with the `Metadata`
        field set, and then validate correct `Metadata` is returned using Read.
    *   Using Update Type as `MODIFY`, update the existing TableEntry with a
        change in the `Metadata` field, and then validate correct `Metadata` is
        returned using Read.
    *   Using Update Type as `DELETE`, delete the existing TableEntry and then
        validate the deletion of the TableEntry using Read.

## Notes

*   [P4RT Proto](https://github.com/p4lang/p4runtime/blob/main/proto/p4/v1/p4runtime.proto)
