# KNE Topology Configuration

This directory contains the KNE topology configuration of the following kinds:

-   The `*.textproto` files describe the topology elements. These may contain
    one or more DUTs with ports connected to other DUTs or OTG ports.

-   The `*.cfg` files contain the DUT configuration, which are used by
    Ondatra reservation to perform a device reset when a test starts.

OTG releases can be found here:
https://github.com/open-traffic-generator/ixia-c/releases

Each release contains an `ixiatg-configmap.yaml` which describes the docker
image versions required for that release. The docker images should be locally
available, and the "operator" pods should be brought up.

> :exclamation: **IMPORTANT**: change the version in the `*.textproto` file
> from `"0.0.1-9999"` to the one used by `ixiatg-configmap.yaml` before
> creating the actual KNE topology (e.g change `"0.0.1-9999"` to
> `"0.0.1-3662"`).

A How-To guide for creating KNE topologies can be found at:
https://github.com/openconfig/kne/blob/main/docs/README.md
