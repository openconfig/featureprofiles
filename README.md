# Feature Profiles

Feature profiles define independent features that can be invoked on network devices.  A feature profile can be a combination or subset of configuration, telemetry, operational commands, or any other interface that the device exposes.  Example management plane device APIs are gNMI, gNOI, and control plane APIs such as gRIBI, BGP, IS-IS.

Feature profiles include a suite of tests for validating each defined feature.

## Virtualized Testing

### Arista cEOS
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
go test -v featureprofiles/system/system_base/tests/system_base_test.go -config $PWD/topologies/kne/testbed.kne.yml -testbed $PWD/topologies/testbed.textproto
```

Cleanup
```
kne_cli delete topologies/kne/arista_ceos.textproto
```

### Nokia SR-Linux
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
go test -v featureprofiles/system/system_base/tests/system_base_test.go -config $PWD/topologies/kne/testbed.kne.yml -testbed $PWD/topologies/testbed.textproto
```

Cleanup
```
kne_cli delete topologies/kne/nokia_srl.textproto
```
