# RT-1.110: Single-Hop BFD over eBGP Subinterfaces

## Summary

Validate Bidirectional Forwarding Detection (BFD) integration with customer eBGP peering on the DUT across 802.1Q subinterfaces. Verify sub-second failure detection, timer negotiation, dampening behavior, and immediate BGP session teardown upon BFD link state transitions.

## Testbed type

* `TESTBED_DUT_ATE_2LINKS` (ATE Port 1 simulating Customer CE router with BFD enabled).

## Topology

```mermaid
graph LR;
  ce1[ATE Port 1: Customer CE] -- "802.1Q VLAN 100 (eBGP + BFD)" --> dut1[DUT Port 1.100: Tenant VRF ce1]
```

## Procedure

### Test environment setup

1. Configure tenant VRF `ce1` and subinterface `Port1.100` (VLAN 100, IP `192.0.2.1/30`).
2. Configure eBGP neighbor `192.0.2.2` (AS 65001) in VRF `ce1`.
3. Enable BFD under the BGP neighbor container (`enable-bfd`) and configure BFD interface parameters:
   - `desired-minimum-tx-interval: 300000` (300 ms)
   - `required-minimum-receive: 300000` (300 ms)
   - `detection-multiplier: 3`
4. Configure ATE CE1 with matching BFD configuration.

### RT-1.110.1 - BFD Session Establishment and Timer Negotiation

* **Step 1**: Start BFD and BGP on ATE CE1.
* **Step 2**: Verify BFD session state transitions to `UP` via OpenConfig telemetry `/bfd/interfaces/interface/peers/peer/state/session-state`.
* **Step 3**: Verify negotiated timers match: transmitted interval $\le 300$ ms, received interval $\le 300$ ms (`/bfd/interfaces/interface/peers/peer/state/remote-minimum-receive-interval`).
* **Step 4**: Verify BGP session transitions to `ESTABLISHED`.

### RT-1.110.2 - Fast Failure Detection & BGP Session Teardown

* **Step 1**: Inject steady traffic stream from ATE CE1 through DUT.
* **Step 2**: Abruptly stop BFD control packets from ATE CE1 (simulating unidirectional link failure or remote peer crash) without sending BGP FIN/RST or BGP Notification.
* **Step 3**: Verify DUT detects BFD failure within detection timeout ($3 \times 300\text{ ms} = 900\text{ ms}$) and reports diagnostic code at `/bfd/interfaces/interface/peers/peer/state/local-diagnostic-code`.
* **Step 4**: Verify DUT immediately terminates the BGP session and withdraws customer routes without waiting for standard BGP hold timer expiry (e.g. 90 seconds).
* **Step 5**: Verify telemetry state updates within 1 second.

### RT-1.110.3 - BFD Flap Dampening / Suppression

* **Step 1**: Flap BFD session 3 consecutive times in 10 seconds.
* **Step 2**: Verify BFD dampening or hold-down timer triggers to prevent control-plane starvation.
* **Step 3**: Verify BGP session stays down during the dampening window and stabilizes once flaps subside.

## Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "ce1",
        "protocols": {
          "protocol": [
            {
              "identifier": "openconfig-policy-types:BGP",
              "name": "BGP",
              "bgp": {
                "neighbors": {
                  "neighbor": [
                    {
                      "neighbor-address": "192.0.2.2",
                      "config": {
                        "neighbor-address": "192.0.2.2",
                        "peer-as": 65001
                      },
                      "enable-bfd": {
                        "config": {
                          "enabled": true
                        }
                      }
                    }
                  ]
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # BFD Configuration and Operational State
  /bfd/interfaces/interface/config/desired-minimum-tx-interval:
  /bfd/interfaces/interface/config/required-minimum-receive:
  /bfd/interfaces/interface/config/detection-multiplier:
  /bfd/interfaces/interface/peers/peer/state/session-state:
  /bfd/interfaces/interface/peers/peer/state/local-diagnostic-code:
  /bfd/interfaces/interface/peers/peer/state/remote-minimum-receive-interval:
  # BGP Dependency State
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/enable-bfd/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/session-state:

rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```

## Required DUT platform

* FFF
