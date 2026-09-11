# SFLOW-2: sFlow Egress Sampling Configuration and Verification

## Summary

This test verifies OpenConfig configuration and telemetry for interface-level **egress sFlow sampling**, as well as the receipt and structure of egress-sampled flow records on an external sFlow collector.

While standard sFlow sampling captures packets at the ingress pipeline, egress sFlow enables sampling of packets as they egress the device (after header modifications, routing, and encapsulation). This test verifies that:
1. Interface-level `egress-sampling-rate` can be configured via OpenConfig gNMI.
2. The operational state for `egress-sampling-rate` and interface `enabled` reflect correctly in telemetry.
3. Transmitted traffic through the configured egress interface generates valid sFlow datagrams sent to the collector.
4. Captured flow samples correctly identify the sampled egress interface.

## Testbed Type

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

```text
+------------+                 +------------+
|            |  Port 1 (Ing)   |            |
|            |-----------------|            |
|    ATE     |                 |    DUT     |
|            |  Port 2 (Egr)   |            |
|            |-----------------|            |
+------------+                 +------------+
```

* **DUT Port 1**: Connected to ATE Port 1 (Ingress traffic source).
* **DUT Port 2**: Connected to ATE Port 2 (Egress traffic destination & sFlow collector receiver).

## Procedure

### Test Environment Setup

1. Configure IP addressing on DUT Port 1 and Port 2 with IPv4 (`/30`) and IPv6 (`/126`) subnets.
2. Configure ATE Port 1 and Port 2 with matching IP addresses and verify bidirectional reachability (ARP / NDP resolution).
3. Ensure no prior sFlow configuration exists on the DUT.

### SFLOW-2.1: Configure Interface-Level Egress Sampling via OpenConfig

#### 1. Generate DUT Configuration

Configure global sFlow and enable egress sampling on DUT Port 2 with a sampling rate of 1:1,000,000 (or the minimum rate supported by the platform):

```json
{
  "openconfig-sampling:sampling": {
    "sflow": {
      "config": {
        "enabled": true,
        "sample-size": 256
      },
      "collectors": {
        "collector": [
          {
            "address": "192.0.2.2",
            "port": 6343,
            "config": {
              "address": "192.0.2.2",
              "port": 6343,
              "source-address": "192.0.2.1",
              "network-instance": "DEFAULT"
            }
          }
        ]
      },
      "interfaces": {
        "interface": [
          {
            "name": "port2",
            "config": {
              "name": "port2",
              "enabled": true,
              "egress-sampling-rate": 1000000
            }
          }
        ]
      }
    }
  }
}
```

#### 2. Push Configuration
* Push the configuration to the DUT using `gNMI.Set` (Update/Replace).

#### 3. Telemetry Validation
* Query telemetry using `gNMI.Get` or `gNMI.Subscribe` and verify:
  * `/sampling/sflow/state/enabled` is `true`.
  * `/sampling/sflow/collectors/collector[address=192.0.2.2][port=6343]/state/address` matches `192.0.2.2`.
  * `/sampling/sflow/interfaces/interface[name=port2]/state/enabled` is `true`.
  * `/sampling/sflow/interfaces/interface[name=port2]/state/egress-sampling-rate` matches `1000000` (or the configured rate).


### SFLOW-2.2: Verify Egress sFlow Packet Generation on Traffic Flow

#### 1. Traffic Generation
* Configure ATE Port 1 to transmit IPv4 and IPv6 traffic towards destinations reachable via DUT Port 2.
* Set traffic rate high enough to generate statistically meaningful samples based on the configured sampling rate (e.g. 100,000 pps for 1,000,000 sampling rate over 60–120 seconds).
* Start sFlow packet capture on ATE Port 2 listening on UDP port 6343.

#### 2. Verification
* Verify that sFlow datagrams (UDP port 6343) arrive at ATE Port 2 from source IP `192.0.2.1`.
* Parse captured sFlow datagrams and verify:
  * sFlow version is 5.
  * Agent address matches the configured source IP address (`192.0.2.1`).
  * Flow sample records are present.
  * In the flow record, the **output interface** (`output_interface` / `egress interface index`) matches the SNMP ifIndex of DUT Port 2.
  * The number of captured samples conforms to the expected sampling rate within statistical tolerance (e.g. ±20%).

### SFLOW-2.3: Disable Egress Sampling and Verify Teardown

#### 1. Disable Configuration
* Remove `egress-sampling-rate` from DUT Port 2, or set `/sampling/sflow/interfaces/interface[name=port2]/config/enabled` to `false`.

#### 2. Telemetry and Traffic Verification
* Verify telemetry reflects the disabled state.
* Transmit traffic from ATE Port 1 to ATE Port 2 and confirm no further sFlow sample datagrams are generated or exported by the DUT.


## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Parameter Coverage
  /sampling/sflow/config/enabled:
  /sampling/sflow/config/sample-size:
  /sampling/sflow/collectors/collector/config/address:
  /sampling/sflow/collectors/collector/config/port:
  /sampling/sflow/collectors/collector/config/source-address:
  /sampling/sflow/collectors/collector/config/network-instance:
  /sampling/sflow/interfaces/interface/config/name:
  /sampling/sflow/interfaces/interface/config/enabled:
  /sampling/sflow/interfaces/interface/config/egress-sampling-rate:

  ## Telemetry Parameter Coverage
  /sampling/sflow/state/enabled:
  /sampling/sflow/state/sample-size:
  /sampling/sflow/collectors/collector/state/address:
  /sampling/sflow/collectors/collector/state/port:
  /sampling/sflow/collectors/collector/state/source-address:
  /sampling/sflow/collectors/collector/state/network-instance:
  /sampling/sflow/interfaces/interface/state/name:
  /sampling/sflow/interfaces/interface/state/enabled:
  /sampling/sflow/interfaces/interface/state/egress-sampling-rate:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```

