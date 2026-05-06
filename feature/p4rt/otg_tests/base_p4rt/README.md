# P4RT-1.1: Base P4RT Functionality


## Summary

Validate that the P4RT server can accept basic configuration and Read/Write RPCs.


## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Configure P4RT id and node-id with two different interfaces on different FAPs.

*   Send the WBB P4Info via the SetForwardingConfigPipeline.

*   Send RPC Write to install the WBBIngressaclTable for GDP, LLDP and traceroute with meters attached.

*   Verify if the RPC write is success.

*   Send RPC Read to read back the installed table entries.

*   Validate the read Response for each of table entries GDP, LLDP and traceroute.

*   Repeat the same steps for another FAP and verify the Table entries. 

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /components/component/integrated-circuit/config/node-id:
    platform_type: ["INTEGRATED_CIRCUIT"]
  /interfaces/interface/config/id:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

