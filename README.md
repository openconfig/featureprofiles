# Feature Profiles

## Virtualized Testing

### Arista cEOS
Setup
```
kne_cli create topologies/kne/arista_ceos.textproto
cat >topologies/kne/testbed.kne.yml << EOF
username: admin
password: admin
topology: $PWD/topologies/kne/arista_ceos.textproto
cli: /usr/local/google/home/bstoll/go/bin/kne_cli
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
cli: /usr/local/google/home/bstoll/go/bin/kne_cli
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

