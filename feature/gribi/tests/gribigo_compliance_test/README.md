# TE-15.1: gRIBI Compliance

## Summary

Execute the [gRIBIgo compliance tests][1] against a DUT.

[1]: https://github.com/openconfig/gribigo/tree/main/compliance

## Procedure

For each compliance test case in the test suite:

1.  Set the network instance names for the default VRF and non-default VRF.
2.  Create gRIBI-A as the first client.
3.  Create gRIBI-B as the second client.
4.  Call the test case function with the two gRIBI clients.
    *   If the case expects a t.Fatal result, use testt.ExpectFatal.
    *   If the case expects a t.Error result, use testt.ExpectError.
    *   Otherwise, call the test case function directly.
