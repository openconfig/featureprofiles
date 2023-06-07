# Feature Profiles

Feature profiles define groups of OpenConfig paths that can be invoked on
network devices. A feature profile may contain configuration, telemetry,
operational or any other paths that a device exposes. Example management plane
device APIs are gNMI, and gNOI. Example control plane APIs are gRIBI, and
protocols such as BGP, IS-IS.

Feature profiles also include a suite of tests for validating the network device
behavior for each defined feature.

# Contributing

For information about how to contribute to OpenConfig Feature Profiles, please
see [Contributing to OpenConfig Feature Profiles](CONTRIBUTING.md).

Feedback and suggestions to improve OpenConfig Feature Profiles is welcomed on
the
[public mailing list](https://groups.google.com/forum/?hl=en#!forum/netopenconfig),
or by opening a GitHub
[issue](https://github.com/openconfig/featureprofiles/issues).

# Examples

Tests below are implemented using the
[ONDATRA](https://github.com/openconfig/ondatra) test framework with the
[Kubernetes Network Emulation](https://github.com/openconfig/kne) binding.

Before creating a topology, follow the
[steps for deploying a cluster](https://github.com/openconfig/kne/blob/main/docs/create_topology.md#deploy-a-cluster).

## Arista cEOS

[Arista cEOS](https://www.arista.com/en/products/software-controlled-container-networking)
images can be obtained by contacting Arista.

1. Create the topology:

```
kne create topologies/kne/arista/ceos/topology.textproto
```

2. Run a sample test:

```
go test ./feature/system/tests/... -kne-topo $PWD/topologies/kne/arista/ceos/topology.textproto -vendor_creds ARISTA/admin/admin
```

3. Cleanup:

```
kne delete topologies/kne/arista/ceos/topology.textproto
```

## Cisco 8000e

> NOTE: `8000e` images require the host supports nested virtualization.

Cisco `8000e` images can be obtained by contacting Cisco.

1. Create the topology:

```
kne create topologies/kne/cisco/8000e/topology.textproto
```

2. Run a sample test:

```
go test ./feature/system/tests/... -kne-topo $PWD/topologies/kne/cisco/8000e/topology.textproto -vendor_creds CISCO/cisco/cisco123
```

3. Cleanup:

```
kne delete topologies/kne/cisco/8000e/topology.textproto
```

## Cisco XRD

Cisco `XRD` images can be obtained by contacting Cisco.

1. Create the topology:

```
kne create topologies/kne/cisco/xrd/topology.textproto
```

2. Run a sample test:

```
go test ./feature/system/tests/... -kne-topo $PWD/topologies/kne/cisco/xrd/topology.textproto -vendor_creds CISCO/cisco/cisco123
```

3. Cleanup:

```
kne delete topologies/kne/cisco/xrd/topology.textproto
```

## Juniper CPTX

> NOTE: `CPTX` images require the host supports nested virtualization.

Juniper `CPTX` images can be obtained by contacting Juniper.

1. Create the topology:

```
kne create topologies/kne/juniper/cptx/topology.textproto
```

2. Run a sample test:

```
go test ./feature/system/tests/... -kne-topo $PWD/topologies/kne/juniper/cptx/topology.textproto -vendor_creds JUNIPER/root/Google123
```

3. Cleanup:

```
kne delete topologies/kne/juniper/cptx/topology.textproto
```

## Nokia SR Linux

SR Linux images can be found
[here](https://github.com/nokia/srlinux-container-image/pkgs/container/srlinux).

1. Create the topology:

```
kne create topologies/kne/nokia/srlinux/topology.textproto
```

2. Run a sample test:

```
go test ./feature/system/tests/... -kne-topo $PWD/topologies/kne/nokia/srlinux/topology.textproto -vendor_creds NOKIA/admin/NokiaSrl1!
```

3. Cleanup:

```
kne delete topologies/kne/nokia/srlinux/topology.textproto
```

## Static Binding (Experimental)

The static binding supports ATE based testing with a real hardware device. It
assumes that there is one ATE hooked up to one DUT in the testbed, and their
ports are connected pairwise. They are defined in `topologies/atedut_*.testbed`
with three variants: 2 ports, 4 ports, and 12 ports.

*   The 2 port variant is able to run the vast majority of the control plane
    tests.
*   The 4 port variant is required by some VRF based or data plane tests.
*   The 12 port variant is required by the aggregate interface (static LAG and
    LACP) tests.

Setup: edit `topologies/atedut_12.binding` to specify the mapping from testbed
topology to the actual hardware as well as the dial options.

Testing:

```
cd ./topologies/ate_tests/topology_test
go test . -testbed ../../atedut_12.testbed -binding ../../atedut_12.binding
```

> :exclamation: **NOTE**: when `go test` runs a test, the current working
> directory is set to the path of the test package, so the testbed and binding
> files are relative to the test package and not to the source root. It is
> recommended to just `cd` to the test package to be consistent.

> :warning: **WARNING**: the topology\_test is derived from a similar test used
> at Google. The test code compiles but is not tested because we have not hooked
> up Google's testing environment to the open-sourced static binding. This is an
> early preview meant to demonstrate Ondatra API usage.

## Path validation

The `make validate_paths` target will clone the public OpenConfig definitions
and report Feature Profiles that have invalid OpenConfig paths.
