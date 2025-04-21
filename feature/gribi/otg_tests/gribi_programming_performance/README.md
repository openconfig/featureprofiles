# TE-14.1: gRIBI Scaling

## Summary

Validate and measure gRIBI programming performance for a DUT.

## Topology

```
                      _________
                     |         |
   [ ATE p1 ] --- p1 |   DUT   | p2 --- [ ATE p2 ]     
                     |_________|
```

 * ATE p1 to DUT p1 is assigned `192.0.2.0/30` and `2001:db8::/127`.
 * ATE p2 to DUT p2 is assigned `192.0.2.2/30` and `2001:db8::2/127`.

 * Address pools that are injected to the DUT by the test should correspond to Internet DFZ route distributions.
    * Pool sizes to be tested:
      * 1,000
      * 10,000
      * 100,000
      * 500,000
      * 1,000,000

## Procedure

 * Utilising the topology described above, configure DUT and ATE.
 * Establish a gRIBI connection to the DUT, using `FIB_ACK`, `SINGLE_PRIMARY` and `PERSIST` session parameters.
 * For each of the pool sizes defined above, referred to as N:
  - Select a set of N routes from the DFZ.
  - Order the set, and select the 25th, 50th, 75th, 90th, 95th, 100th percentile entries.
  - Define traffic flows destined to the prefixes selected above.
  - Begin generating traffic.
  - Program all entries into the DUT:
    - Record the programming latency for each prefix for both RIB and FIB ACKs to be received.
  - Measure loss observed for each monitored prefix.
 * For each measurement -- record this output in an artifact that can be collected from the test.

> TODO(robjs): Define acceptable performance numbers for each pool size.

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```
