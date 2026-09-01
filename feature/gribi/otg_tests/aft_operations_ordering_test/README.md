# TE-3.9: gRIBI AFT Operations Ordering

## Summary

Validates whether the Network Operating System (NOS) gRIBI daemon adheres to the OpenConfig gRIBI specification recommendation (`gribi.proto:62`):
> *"A gRIBI server SHOULD process AFTOperations per the received order."*

The test executes large sequences (up to 10,000 operations per test) of rapid AFT modifications, replacements, and deletions to verify that:
1. Operations within a pipelined gRPC stream across multiple `ModifyRequest` messages without intermediate client-side `await` barriers are processed in exact arrival order (**Inter-Request Stream Ordering**).
2. Operations batched within a single `ModifyRequest` array are processed in exact index order (**Intra-Request Sequential Ordering**).
3. Cross-layer referential integrity is preserved during rapid Make-Before-Break (MBB) migrations ($\text{NH} \to \text{NHG} \to \text{TL}$ bottom-up creation and $\text{TL} \to \text{NHG} \to \text{NH}$ top-down teardown).
4. The gRIBI server acknowledges each operation in the stream with `RIB_ACK` (`fluent.InstalledInRIB` / `AFTResult_RIB_PROGRAMMED`) without transport drops, out-of-order execution, or reference violations.

## Testbed Type

* [TESTBED_DUT_ATE_2LINKS](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)
  * DUT Port 1 (`192.0.2.1/30`) $\leftrightarrow$ ATE Port 1 (`192.0.2.2/30`)
  * DUT Port 2 (`192.0.2.5/30`) $\leftrightarrow$ ATE Port 2 (`192.0.2.6/30`) (Target for NextHop 1)
  * DUT Port 2 Secondary (`192.0.2.10/30`) $\leftrightarrow$ ATE Port 2 Secondary (Target for NextHop 2)

## Procedure

Connect to the gRIBI server running on the DUT, negotiating `RIB_ACK` as the requested `ack_type`, persistence mode `PRESERVE`, and `SINGLE_PRIMARY` redundancy mode. Become leader. Between all subtests, perform `gribi.FlushAll` to ensure a completely clean baseline.

### Group 1: Inter-Request Pipelined Stream (10,000 ModifyRequests, No Client-Side Await)

* **TE-3.9.1: Inter-Request NextHop Churn**
  * Stream 5,000 consecutive pairs of $\langle \text{ADD}(\text{NH}_1), \text{DEL}(\text{NH}_1) \rangle$ ($10{,}000$ operations total), where each operation is sent in an independent `ModifyRequest` on the gRPC stream without waiting for intermediate responses.
  * Verify that all 10,000 operations are acknowledged with `RIB_ACK` (status `OK`).

* **TE-3.9.2: Inter-Request NextHopGroup Mutation**
  * Pre-program static base NextHops $\text{NH}_1$ and $\text{NH}_2$.
  * Stream 3,333 consecutive triplets of $\langle \text{ADD}(\text{NHG}_1 \to [\text{NH}_1]), \text{REPLACE}(\text{NHG}_1 \to [\text{NH}_2]), \text{DEL}(\text{NHG}_1) \rangle$ ($9{,}999$ operations total) across independent `ModifyRequest` messages without awaiting intermediate responses.
  * Verify that all 9,999 operations receive `RIB_ACK`.

* **TE-3.9.3: Inter-Request Top-Level Route Mutation**
  * Pre-program static base NextHops $\text{NH}_1, \text{NH}_2$ and NextHopGroups $\text{NHG}_1, \text{NHG}_2$.
  * Stream 3,333 consecutive triplets of $\langle \text{ADD}(\text{TL}_1 \to \text{NHG}_1), \text{REPLACE}(\text{TL}_1 \to \text{NHG}_2), \text{DEL}(\text{TL}_1) \rangle$ ($9{,}999$ operations total) across independent `ModifyRequest` messages without awaiting intermediate responses.
  * Verify that all 9,999 operations receive `RIB_ACK`.

* **TE-3.9.4: Inter-Request Cross-Layer Make-Before-Break Ping-Pong**
  * Program base state $\text{NH}_1, \text{NHG}_1 \to [\text{NH}_1], \text{TL}_1 \to \text{NHG}_1$.
  * Stream 1,000 full round-trip cycles ($10{,}000$ operations total) where each round-trip comprises:
    1. Make/Break $1 \to 2$: $\langle \text{ADD}(\text{NH}_2), \text{ADD}(\text{NHG}_2 \to \text{NH}_2), \text{REPLACE}(\text{TL}_1 \to \text{NHG}_2), \text{DEL}(\text{NHG}_1), \text{DEL}(\text{NH}_1) \rangle$
    2. Make/Break $2 \to 1$: $\langle \text{ADD}(\text{NH}_1), \text{ADD}(\text{NHG}_1 \to \text{NH}_1), \text{REPLACE}(\text{TL}_1 \to \text{NHG}_1), \text{DEL}(\text{NHG}_2), \text{DEL}(\text{NH}_2) \rangle$
  * Verify that all 10,000 operations receive `RIB_ACK` with zero reference or in-use errors.

---

### Group 2: Intra-Request Batching (Single ModifyRequest, 10,000 Operations)

* **TE-3.9.5: Intra-Request NextHop Churn**
  * Send a single `ModifyRequest` containing 5,000 consecutive pairs of $\langle \text{ADD}(\text{NH}_1), \text{DEL}(\text{NH}_1) \rangle$ ($10{,}000$ operations in the array).
  * Await stream convergence and verify that all 10,000 operations receive `RIB_ACK`.

* **TE-3.9.6: Intra-Request NextHopGroup Mutation**
  * Pre-program static base NextHops $\text{NH}_1$ and $\text{NH}_2$.
  * Send a single `ModifyRequest` containing 3,333 consecutive triplets of $\langle \text{ADD}(\text{NHG}_1 \to [\text{NH}_1]), \text{REPLACE}(\text{NHG}_1 \to [\text{NH}_2]), \text{DEL}(\text{NHG}_1) \rangle$ ($9{,}999$ operations in the array).
  * Await stream convergence and verify that all 9,999 operations receive `RIB_ACK`.

* **TE-3.9.7: Intra-Request Top-Level Route Mutation**
  * Pre-program static base NextHops $\text{NH}_1, \text{NH}_2$ and NextHopGroups $\text{NHG}_1, \text{NHG}_2$.
  * Send a single `ModifyRequest` containing 3,333 consecutive triplets of $\langle \text{ADD}(\text{TL}_1 \to \text{NHG}_1), \text{REPLACE}(\text{TL}_1 \to \text{NHG}_2), \text{DEL}(\text{TL}_1) \rangle$ ($9{,}999$ operations in the array).
  * Await stream convergence and verify that all 9,999 operations receive `RIB_ACK`.

* **TE-3.9.8: Intra-Request Cross-Layer Make-Before-Break Ping-Pong**
  * Program base state $\text{NH}_1, \text{NHG}_1 \to [\text{NH}_1], \text{TL}_1 \to \text{NHG}_1$.
  * Send a single `ModifyRequest` containing 1,000 full round-trip cycles ($10{,}000$ operations in the array):
    * Cycle $1 \to 2$: $\langle \text{ADD}(\text{NH}_2), \text{ADD}(\text{NHG}_2), \text{REPLACE}(\text{TL}_1 \to 2), \text{DEL}(\text{NHG}_1), \text{DEL}(\text{NH}_1) \rangle$
    * Cycle $2 \to 1$: $\langle \text{ADD}(\text{NH}_1), \text{ADD}(\text{NHG}_1), \text{REPLACE}(\text{TL}_1 \to 1), \text{DEL}(\text{NHG}_2), \text{DEL}(\text{NH}_2) \rangle$
  * Await stream convergence and verify that all 10,000 operations receive `RIB_ACK`.

---

## Config Parameter Coverage

N/A

## Telemetry Parameter Coverage

N/A

## Protocol/RPC Parameter Coverage

* **gRIBI**:
  * `ModifyRequest`:
    * `operation` (repeated `AFTOperation`)
    * `SessionParameters`:
      * `ack_type`: `RIB_ACK`
      * `redundancy`: `SINGLE_PRIMARY`
      * `persistence`: `PRESERVE`

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv4/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Modify:
    gRIBI.Flush:
```
