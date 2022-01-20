# Feature Profiles

Feature profiles define independent features that can be invoked on network devices.  A feature profile can be a combination or subset of configuration, telemetry, operational commands, or any other interface that the device exposes.  Example management plane device APIs are gNMI, gNOI, and control plane APIs such as gRIBI, BGP, IS-IS.

Feature profiles include a suite of tests for validating each defined feature.

## Virtualized Testing

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
