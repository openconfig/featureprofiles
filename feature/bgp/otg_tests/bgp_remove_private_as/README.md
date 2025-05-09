# RT-1.11: BGP remove private AS 

## Summary

BGP remove private AS

## Procedure

*   Establish BGP sessions as follows between ATE and DUT.
*   ATE port 1 emulates two eBGP neighbors peering the DUT.
*   eBGP neighbor # 1 is injecting routes with AS_PATH modified to have private AS numbers.
*   Validate that private AS numbers are stripped before advertisement to the eBGP neighbor on ATE
    port 2.
*   Tested AS-Path Patterns:
    *   PRIV_AS1
    *   PRIV_AS1 PRIV_AS2
    *   AS1 PRIV_AS1 PRIV_AS2
    *   PRIV_AS1 AS1
    *   AS1 PRIV_AS1 AS2

## OpenConfig Path and RPC Coverage
```yaml
paths:
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/remove-private-as:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/rib/attr-sets/attr-set/as4-path/as4-segment/state/index:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Minimum DUT platform requirement

N/A