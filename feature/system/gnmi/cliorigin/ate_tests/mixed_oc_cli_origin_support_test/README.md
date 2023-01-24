# gNMI-1.12: Mixed OpenConfig/CLI Origin

## Summary

Ensure that both CLI and OC configuration can be pushed to the device within the
same `SetRequest`.

## Interdependent Case

`TestQoSDependentCLIThenOC` and `TestQoSDependentOCThenCLI` cases cover setting interdependent CLI and OC configuration in the same request (OC requires CLI to be applied first in order to make sense).

* First, we provide CLI update + OC update (in this order) in the same Set() request.

* Second, we provide OC update + CLI update (in this order) in the same Set() request.

In both cases, CLI is ARISTA-specific and a test will skip if the DUT is from another vendor.

The second case is not a requirement at this point and will skip if failed. However, DUTs from ARISTA are known to pass it.
