# Feature Profiles

Feature profiles define groups of openconfig paths that can be invoked on network devices.  A feature profile may contain configuration, telemetry, operational or any other paths that a device exposes.  Example management plane device APIs are gNMI, gNOI, and control plane APIs such as gRIBI, BGP, IS-IS.

Feature profiles also include a suite of tests for validating the network device behavior for each defined feature.

## Contributing to Feature Profiles

For information about how to contribute to OpenConfig models, please
see [Contributing to OpenConfig Feature Profiles](contributions-guide.md).

Feedback and suggestions to improve OpenConfig Feature Profiles is welcomed on the
[public mailing list](https://groups.google.com/forum/?hl=en#!forum/netopenconfig),
or by opening a GitHub [issue](https://github.com/openconfig/featureprofiles/issues).


## Example testing using [Kubernetes Network Emulation](https://github.com/google/kne)

### Arista cEOS
[Arista cEOS](https://www.arista.com/en/products/software-controlled-container-networking) images can be obtained by contacting Arista.

Setup
```
kne_cli create topologies/kne/arista_ceos.textproto
cat >topologies/kne/testbed.kne.yml << EOF
username: admin
password: admin
topology: $PWD/topologies/kne/arista_ceos.textproto
cli: $HOME/go/bin/kne_cli
EOF
```
Testing
```
go test -v feature/system/system_base/tests/*.go -config $PWD/topologies/kne/testbed.kne.yml -testbed $PWD/topologies/single_device.textproto
```

Cleanup
```
kne_cli delete topologies/kne/arista_ceos.textproto
```

### Nokia SR-Linux
SR Linux images can be found [here](https://github.com/nokia/srlinux-container-image/pkgs/container/srlinux) and will require the [SRL Controller](https://github.com/srl-labs/srl-controller) to be installed on the KNE Kubernetes cluster.

Setup
```
kne_cli create topologies/kne/nokia_srl.textproto
cat >topologies/kne/testbed.kne.yml << EOF
username: admin
password: admin
topology: $PWD/topologies/kne/nokia_srl.textproto
cli: $HOME/go/bin/kne_cli
EOF
```

Testing
```
go test -v feature/system/system_base/tests/*.go -config $PWD/topologies/kne/testbed.kne.yml -testbed $PWD/topologies/single_device.textproto
```

Cleanup
```
kne_cli delete topologies/kne/nokia_srl.textproto
```

## Path validation

The `make validate_paths` target will clone the public OpenConfig definitions and report and Feature Profiles which are not valid OpenConfig paths.
