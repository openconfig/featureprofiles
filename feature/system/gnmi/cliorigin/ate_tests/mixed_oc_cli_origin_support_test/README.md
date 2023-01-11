# gNMI-1.12: Mixed OpenConfig/CLI Origin

## Summary

Ensure that both CLI and OC configuration can be pushed to the device at the
same time.

## Common Case

Note: this test is intended to cover only the case of pushing some configuration
along with OC paths - since it is unknown what CLI configuration would be
required in the emergency case that is covered by this requirement.

*   Push non-overlapping mixed SetRequest specifying CLI for DUT port-1 and
    OpenConfig for DUT port-2.

    *   `origin: "cli"` containing vendor configuration.

        ```
        interface <DUT port-1>
          description foo1
        ```

    *   `origin: ""` (openconfig, default origin) setting the DUT port-2 string
        value at `/interfaces/interface/config/description` to `"foo2"`.

*   Validate the DUT port-1 and DUT port-2 descriptions through telemetry.

*   Push overlapping mixed SetRequest specifying CLI before OpenConfig for DUT
    port-1.

    *   `origin: "cli"` containing vendor configuration.

        ```
        interface <DUT port-1>
          description from cli
        ```

    *   `origin: ""` (openconfig, default origin) setting the DUT port-1 string
        value at `/interfaces/interface/config/description` to `"from oc"`.

*   Validate that DUT port-1 description is `"from cli"` as a CLI value should take precedence over an OpenConfig value if both are defined as per [Mixing Schemas in gNMI](https://github.com/openconfig/reference/blob/master/rpc/gnmi/mixed-schema.md).

*   Push overlapping mixed SetRequest specifying OpenConfig before CLI for
    DUT port-1.

    *   `origin: ""` (openconfig, default origin) setting the DUT port-1 string
        value at `/interfaces/interface/config/description` to `"from oc"`.

    *   `origin: "cli"` containing vendor configuration.

        ```
        interface <DUT port-1>
          description from cli
        ```

*   Validate that DUT port-1 description is `"from cli"`.

## Interdependent Case

There latter two test cases cover setting interdependent CLI and OC configuration in the same request (OC requires CLI to be applied first in order to make sense).

* First, we provide CLI update + OC update (in this order) in the same Set() request.

* Second, we provide OC update + CLI update (in this order) in the same Set() request.

In both cases, CLI is ARISTA-specific and a test will skip if the DUT is from another vendor.

The second case is not a requirement at this point and will skip if failed. However, DUTs from ARISTA are known to pass it.
