# Feature Profiles

Feature profiles defines groups of OpenConfig paths that can be invoked on
network devices. A feature profile may contain configuration, telemetry,
operational or any other paths that a device exposes. Example management plane
device APIs are gNMI, and gNOI. Example control plane APIs are gRIBI, and
protocols such as BGP, IS-IS.

Feature profiles also includes a suite of
[Ondatra](https://github.com/openconfig/ondatra) tests for validating the
network device behavior for each defined feature. If you are new to Ondatra,
please start by reading the Ondata
[README](https://github.com/openconfig/ondatra#readme) and taking the [Ondatra
tour](https://docs.google.com/viewer?url=https://raw.githubusercontent.com/openconfig/ondatra/main/internal/tour/tour.pdf).

# Contributing

For information about how to contribute to OpenConfig Feature Profiles, please
see [Contributing to OpenConfig Feature Profiles](CONTRIBUTING.md).

Feedback and suggestions to improve OpenConfig Feature Profiles is welcomed on
the
[public mailing list](https://groups.google.com/forum/?hl=en#!forum/netopenconfig),
or by opening a GitHub
[issue](https://github.com/openconfig/featureprofiles/issues).

# Running Tests on Virtual Devices

Tests may be run on virtual devices using the
[Kubernetes Network Emulation](https://github.com/openconfig/kne) binding.

First, follow the
[steps for deploying a KNE cluster](https://github.com/openconfig/kne/blob/main/docs/create_topology.md#deploy-a-cluster).
Then follow the per-vendor instructions below for creating a KNE topology and
running a test on it.

## Arista

### cEOS

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

## Cisco

### 8000e

> NOTE: `8000e` images require the host supports nested virtualization.

Cisco `8000e` images can be obtained by contacting Cisco.

1. Create the topology:

```
kne create topologies/kne/cisco/8000e/topology.textproto
```

2. Run a sample test:

```
go test ./feature/example/tests/... -kne-topo $PWD/topologies/kne/cisco/8000e/topology.textproto -vendor_creds CISCO/cisco/cisco123
```

3. Cleanup:

```
kne delete topologies/kne/cisco/8000e/topology.textproto
```

### XRD

Cisco `XRD` images can be obtained by contacting Cisco.

1. Create the topology:

```
kne create topologies/kne/cisco/xrd/topology.textproto
```

2. Run a sample test:

```
go test ./feature/example/tests/... -kne-topo $PWD/topologies/kne/cisco/xrd/topology.textproto -vendor_creds CISCO/cisco/cisco123
```

3. Cleanup:

```
kne delete topologies/kne/cisco/xrd/topology.textproto
```

## Juniper

### cPTX

> NOTE: `cPTX` images require the host supports nested virtualization.

Juniper `cPTX` images can be obtained by contacting Juniper.

1. Create the topology:

```
kne create topologies/kne/juniper/cptx/topology.textproto
```

2. Run a sample test:

```
go test ./feature/example/tests/... -kne-topo $PWD/topologies/kne/juniper/cptx/topology.textproto -vendor_creds JUNIPER/root/Google123
```

3. Cleanup:

```
kne delete topologies/kne/juniper/cptx/topology.textproto
```

### ncPTX

Juniper `ncPTX` images can be obtained by contacting Juniper.

1. Create the topology:

```
kne create topologies/kne/juniper/ncptx/topology.textproto
```

2. Run a sample test:

```
go test ./feature/example/tests/... -kne-topo $PWD/topologies/kne/juniper/ncptx/topology.textproto -vendor_creds JUNIPER/root/Google123
```

3. Cleanup:

```
kne delete topologies/kne/juniper/ncptx/topology.textproto
```

## Nokia

### SR Linux

SR Linux images can be found
[here](https://github.com/nokia/srlinux-container-image/pkgs/container/srlinux).

1. Create the topology:

```
kne create topologies/kne/nokia/srlinux/topology.textproto
```

2. Run a sample test:

```
go test ./feature/example/tests/... -kne-topo $PWD/topologies/kne/nokia/srlinux/topology.textproto -vendor_creds NOKIA/admin/NokiaSrl1!
```

3. Cleanup:

```
kne delete topologies/kne/nokia/srlinux/topology.textproto
```

# Running Tests on Real Hardware

Tests may be run on real hardware devices using the static binding.

The static binding supports the testbeds in the `topologies/*.testbed` files.
The mapping between the IDs in the testbed file and the physical devices are
provided by the corresponding `topologies/*.binding` files. To try it out, edit
`otgdut_4.binding` to specify the mapping from testbed IDs to actual hardware
devices, as well as the desired protocol dial options. Then test it by running:

```
go test ./feature/example/tests/topology_test -binding $PWD/topologies/otgdut_4.binding
```

# Path validation

The `make validate_paths` target will clone the public OpenConfig definitions
and report Feature Profiles that have invalid OpenConfig paths.
