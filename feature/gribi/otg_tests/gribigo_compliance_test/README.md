# TE-15.1: gRIBI Compliance

## Summary

Execute the [gRIBIgo compliance tests][1] against a DUT.

[1]: https://github.com/openconfig/gribigo/tree/main/compliance

## Procedure

For each compliance test case in the test suite:

1.  Connect DUT port-1 to ATE port-1, DUT port-2 to ATE port-2, DUT port-3 to
    ATE port-3. Assign 192.0.2.0/31, 192.0.2.2/31, and 192.0.2.4/31 to DUT and
    ATE as defined in the
    [gribigo complaince test topology](https://github.com/openconfig/gribigo/blob/98b0f3bbf1f750542fc505f9a3e24d6d9ce67b3d/compliance/compliance.go#L1129-L1140).
2.  Set the network instance names for the default VRF and non-default VRF.
3.  Create gRIBI-A as the first client.
4.  Create gRIBI-B as the second client.
5.  Call the test case function with the two gRIBI clients.
    *   If the case expects a t.Fatal result, use testt.ExpectFatal.
    *   If the case expects a t.Error result, use testt.ExpectError.
    *   Otherwise, call the test case function directly.
