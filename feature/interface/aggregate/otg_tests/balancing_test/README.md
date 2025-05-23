# RT-5.3: Aggregate Balancing

## Summary

Load balancing across members of a LACP-controlled LAG

## Testbed type

*   [`featureprofiles/topologies/atedut_9.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_9.testbed)

## Topology

```mermaid
graph LR;
A[ATE] <-- (Port 1) --> B[DUT];
B[DUT] <-- LACP LAG (Port 2-9) --> C[ATE];
```

## Procedure

*   Connect ATE port-1 to DUT port-1
*   Connect ATE ports 2 through 9 to DUT ports 2-9
*   Configure ATE and DUT ports 2-9 to be part of a LACP-controlled LAG
*   Configure a default static route on DUT with the next hop to ATE LAG
*   Send at least 1000 flows from ATE port-1 towards the DUT with the following
    parameters
    *   IP/TCP (Protocol number 0x06):
        *   IP header: IPv4/v6 packets with different source and destination
            address pairs
        *   TCP header: IPv4/IPv6 packets with varying TCP source port and
            destination ports
        *   Flow Label: IPv6 packets with varying flow labels
    *   IPinIP (Protocol number 0x04):
        *   TCP header: IPinIP containing IPv4/v6 payload with different source
            and destination addresse pairs
        *   Flow Label: IPinIP containing IPv4/v6 payload with varying flow
            labels
    *   Ensure that traffic is seen across all the LAG members

NOTE: Due to the random nature of the test you may not see a balanced
distribution of traffic, but all the links should get traffic

## Canonical OC

```json
{
  "openconfig-lacp:config": {
    "name": "Port-Channel1"
  },
  "openconfig-lacp:members": {
    "member": [
      {
        "config": {
          "interface": "Ethernet29/1"
        },
        "interface": "Ethernet29/1",
        "state": {
          "activity": "ACTIVE",
          "aggregatable": true,
          "collecting": true,
          "counters": {
            "lacp-errors": "0",
            "lacp-in-pkts": "4393",
            "lacp-out-pkts": "130415",
            "lacp-rx-errors": "0",
            "lacp-tx-errors": "0",
            "lacp-unknown-errors": "0"
          },
          "distributing": true,
          "interface": "Ethernet29/1",
          "oper-key": 1,
          "partner-id": "ac:78:d1:1e:ad:c8",
          "partner-key": 55,
          "partner-port-num": 44,
          "port-num": 175,
          "arista-lacp-augments:selected": "selected",
          "synchronization": "IN_SYNC",
          "system-id": "38:38:a6:a2:f7:30",
          "timeout": "LONG"
        }
      }
    ]
  },
  "openconfig-lacp:name": "Port-Channel1",
  "openconfig-lacp:state": {
    "name": "Port-Channel1",
    "system-id-mac": "38:38:a6:a2:f7:30"
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/aggregation/config/lag-type:
  /lacp/config/system-priority:
  /lacp/interfaces/interface/config/name:
  /lacp/interfaces/interface/config/interval:
  /lacp/interfaces/interface/config/lacp-mode:
  /lacp/interfaces/interface/config/system-id-mac:
  /lacp/interfaces/interface/config/system-priority:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: false
    gNMI.Subscribe:
      on_change: false
```

## Minimum DUT platform requirement

*   vRX

