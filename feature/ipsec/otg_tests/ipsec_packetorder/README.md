# IPSEC-1.3: IPSec Packet-Order with MACSec over aggregated links.

## Summary

This test verifies proper IPSec packet out-of-order processing. A pair of DUTs establish an IPsec tunnel. Traffic on ingress to the DUT is then encrypted and forwarded over the tunnel to the egress DUT, with the packets arriving out-of-order, which then decrypts the packets and forwards to the final destination.

## Testbed Type

The ate-dut testbed configuration would be used, as described below.

*  [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

TODO: when OTG API supports IPSec, refactor the topology to be: `atedut8` where the ATE serves as the endpoints of the ipsec tunnel


## Procedure

### Test Environment Setup

See IPSEC-1.1 for test environment setup.

### IPSEC-1.3.1: Out-of-order packet processing (Optional)

Optional test: This test SHOULD be run, but if the lab environment does not have a way to run this full test - then a partial/similar test MAY be run if that is feasible, or this test should be deferred to a future point where it is possible.

Change in base setup:

* One tunnel
* Short SA times, O(seconds/minute)
* One of the links between DUT1 \<\> DUT2 has an additional latency added to it; the other DUT1 \<\> DUT2 links have a common (no) latency

Test with latency of +1ms, +5ms, +10ms, +20ms, +50ms, +100ms on one link

Traffic sent over the link with the higher latency should arrive out-of-order, but be processed and decrypted correctly.

Verify:

* All links see packets, meaning the traffic is being hashed across all links
* No traffic loss in steady-state (periods of time without renegotiation)
* Verify SA key (encryption key) renegotiates
* Verify no traffic loss during the SA key renegotiation

### Canonical OC

```json
{
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  ```

## Required DUT platform

FFF
