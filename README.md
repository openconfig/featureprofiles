# Feature Profiles

Feature profiles define groups of OpenConfig paths that can be invoked on network 
devices.  A feature profile may contain configuration, telemetry, operational or 
any other paths that a device exposes.  Example management plane device APIs are 
gNMI, and gNOI.  Example control plane APIs are gRIBI, and protocols such as BGP, 
IS-IS.

Feature profiles also include a suite of tests for validating the network device
behavior for each defined feature.

# Contributing

For information about how to contribute to OpenConfig Feature Profiles, please
see [Contributing to OpenConfig Feature Profiles](CONTRIBUTING.md).

Feedback and suggestions to improve OpenConfig Feature Profiles is welcomed on the
[public mailing list](https://groups.google.com/forum/?hl=en#!forum/netopenconfig),
or by opening a GitHub [issue](https://github.com/openconfig/featureprofiles/issues).

# Examples
Tests below are implemented using the [ONDATRA](https://github.com/openconfig/ondatra)
test framework with the [Kubernetes Network Emulation](https://github.com/google/kne) 
binding.

To set up a local installation on Ubuntu using [kind](https://kind.sigs.k8s.io/),
you can follow these steps:

1. Install [Docker](https://docs.docker.com/engine/install/ubuntu/) &
[golang](https://go.dev/doc/install).
2. Install kind and kubectl.
```
go install sigs.k8s.io/kind@latest
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
```
3. Install KNE.
```
git clone https://github.com/google/kne.git
cd kne/kne_cli
go install -v
kne_cli deploy ../deploy/kne/kind.yaml
```
4. Install any docker images as needed.
```
docker import ceos.tar.xz ceos:latest
kind load docker-image --name=kne ceos:latest
```

### Arista cEOS
[Arista cEOS](https://www.arista.com/en/products/software-controlled-container-networking) images can be obtained by contacting Arista.

```
make kne_arista_setup
make kne_tests
make kne_arista_cleanup
```

### Nokia SR-Linux
SR Linux images can be found [here](https://github.com/nokia/srlinux-container-image/pkgs/container/srlinux) and will require the [SRL Controller](https://github.com/srl-labs/srl-controller) to be installed on the KNE Kubernetes cluster.

```
make kne_nokia_setup
make kne_tests
make kne_nokia_cleanup
```

## Path validation

The `make validate_paths` target will clone the public OpenConfig definitions and report and Feature Profiles which are not valid OpenConfig paths.
