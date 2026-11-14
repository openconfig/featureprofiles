# RT-1.21: BGP PMTUD

## Summary

> [!NOTE]
> The TCP MSS test moved to
> [RT-1.22: BGP TCP MSS](https://github.com/openconfig/featureprofiles/blob/main/feature/bgp/otg_tests/bgp_tcp_mss/README.md).
> The MSS steps below are kept until this test is updated to cover PMTUD only.

*   Validate changes in TCP MSS value is allowed and takes effect.
*   Validate DUT's PMTUD compliance.

## Topology

*   ATE:port1 <-> port1:DUT1:Port2 <-> port1:DUT2

## Procedure

*   Establish BGP sessions as follows:
    *   ATE:port1 --- eBGP-IPv4/IPv6 ---- DUT1:port1.
    *   ATE:port1 ---- iBGP IPv4 ---- DUT2:port1.
*   Verify that the default TCP MSS value is set below the default interface MTU value.
*   Change the Interface MTU on the DUT1:port1 port as well as ATE:port1 to 5040B
*   Configure IP TCP MSS value of 4096 bytes on the DUT1:port1.
*   Re-establish the EBGP sessions by tcp reset.
*   Verify that the TCP MSS value is set to 4096 bytes for the IPv4 and IPv6 EBGP sessions.
*   Establish iBGP session with MD5 enabled between ATE:port1 and DUT2:port1.
*   Ensure that the MTU on the DUT1:port1 towards ATE1:port1 is left at default (1500B) while the ATE1:port1 interface towards DUT1:port1 is set at 5040B. Please also make sure that the DUT2:port1 MTU is set at 5040B as well.
*   Enable PMTUD on DUT2:port1.
*   Re-establish the IBGP sessions by tcp reset.
*   Validate that the min MSS value has been adjusted to be below 1500 bytes on the tcp session.

## OpenConfig Path and RPC Coverage
```yaml
paths:
  ## Config Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/tcp-mss:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/config/mtu-discovery:

  ## Telemetry Parameter Coverage
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/tcp-mss:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/transport/state/mtu-discovery:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
```

## Canonical OC
```json
{}
```

## Minimum DUT platform requirement

N/A
