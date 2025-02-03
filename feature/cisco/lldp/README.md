# OC LLDP Internal

## Summary

Configure lldp globally and at interface level. Subscribe and verify if we get correct lldp state.

## Topology

*   2DUT
    DUT1 <---> DUT2

Following hardcoding expectation for Device name
DUT1 name is `dut`
DUT2 name is `peer`
Connection **Port 1** has to connect two devices

8808-OC SIM is ideal choice

## Procedure

- Test Update/Replace/Delete on global lldp config (OC.lldp.config.enabled)
- (Behaviour change Needed from 25.1.1) Add Interface to lldp and enable the interface for dut and peer
- Modify the global lldp config and verify the lldp interface config follows that global config.
- Subscribe and verify the following lldp interface neighbout leaves
    - system-name
    - chassis-id-type
    - port-id
    - port-id-type
    - system-description
    - chassis-id

## File organisation

```shell
┌─[feature/cisco/lldp]
└──> tree
.
├── lldp_base_test.go   <- defines test Main and parses test data
├── lldp_test.go        <- contains the lldp test cases
├── README.md           <- this README file
└── testdata
    └── lldp.yaml       <- initial interface config for the duts

1 directory, 4 files
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

# RPCs
```yaml
rpcs:
  gnmi:
    oc.Update:
      paths:
        OC.lldp.config.enabled
        OC.lldp.interfaces.interface.config.enable
    oc.Replace:
      paths:
        OC.lldp.config.enabled
    oc.Delete:
      paths:
        OC.lldp.config.enabled
    oc.Subscribe:
      paths:
        Oc.lldp.interfaces.interface.state.enabled
        OC.lldp.interfaces.interface.neighbors.neighbor.state.system-name
        OC.lldp.interfaces.interface.neighbors.neighbor.state.chassis-id-type
        OC.lldp.interfaces.interface.neighbors.neighbor.state.port-id
        OC.lldp.interfaces.interface.neighbors.neighbor.state.port-id-type
        OC.lldp.interfaces.interface.neighbors.neighbor.state.system-description
        OC.lldp.interfaces.interface.neighbors.neighbor.state.chassis-id
```