# SFLOW-2: sFlow Sampling with Encapsulation and Decapsulation

## Summary

Verify sFlow sampling functionality when packets undergo encapsulation or
decapsulation on the DUT. This includes GRE and GUE tunnels, and
gRIBI-programmed encapsulation.

## Testbed type

*  [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)

## Procedure

### Test environment setup

*   DUT has an ingress port (Port 1) and egress ports (Port 2, Port 3, Port 4).
*   Configure sFlow on DUT:
    *   Enable sFlow globally.
    *   Configure sFlow collector address (ATE Port 2) and port.
    *   Set sampling rate to 1 sample per 10k packets.
    *   Enable sFlow on ingress and egress interfaces.

### SFLOW-2.1: sFlow sampling with IPoverGRE Encap

*   Configure IPoverGRE encapsulation on DUT.
*   Send IPv4 traffic from ATE Port 1 to DUT Port 1, matching the GRE encap rule.
*   Verify DUT encapsulates traffic and sends GRE packets to ATE Port 3.
*   Verify sFlow collector receives samples.
*   Verify sFlow sample contains:
    *   Ingress interface matches DUT Port 1.
    *   Egress interface matches DUT Port 3.
    *   Sampled packet header matches the expected GRE encapsulated packet.
    *   Sampling rate matches configured rate.

### SFLOW-2.2: sFlow sampling with IPoverGRE Decap

*   Configure IPoverGRE decapsulation on DUT.
*   Send GRE encapsulated IPv4 traffic from ATE Port 1 to DUT Port 1.
*   Verify DUT decapsulates traffic and sends decapsulated IPv4 packets to ATE Port 3.
*   Verify sFlow collector receives samples.
*   Verify sFlow sample contains:
    *   Ingress interface matches DUT Port 1.
    *   Egress interface matches DUT Port 3.
    *   Sampled packet header matches the expected decapsulated packet.

### SFLOW-2.3: sFlow sampling with IPv6overGRE Encap

*   Configure IPv6overGRE encapsulation on DUT.
*   Send IPv6 traffic from ATE Port 1 to DUT Port 1, matching the GRE encap rule.
*   Verify DUT encapsulates traffic and sends GRE packets to ATE Port 3.
*   Verify sFlow collector receives samples.
*   Verify sFlow sample contains:
    *   Ingress interface matches DUT Port 1.
    *   Egress interface matches DUT Port 3.
    *   Sampled packet header matches the expected GRE encapsulated packet.

### SFLOW-2.4: sFlow sampling with IPv6overGRE Decap

*   Configure IPv6overGRE decapsulation on DUT.
*   Send GRE encapsulated IPv6 traffic from ATE Port 1 to DUT Port 1.
*   Verify DUT decapsulates traffic and sends decapsulated IPv6 packets to ATE Port 3.
*   Verify sFlow collector receives samples.
*   Verify sFlow sample contains:
    *   Ingress interface matches DUT Port 1.
    *   Egress interface matches DUT Port 3.
    *   Sampled packet header matches the expected decapsulated packet.

### SFLOW-2.5: sFlow sampling with GUE Encap

*   Configure GUE encapsulation on DUT (static route based).
*   Send traffic to trigger GUE encapsulation.
*   Verify DUT encapsulates traffic and sends GUE packets to ATE Port 3.
*   Verify sFlow collector receives samples.
*   Verify sFlow sample contains:
    *   Ingress interface matches DUT Port 1.
    *   Egress interface matches DUT Port 3.
    *   Sampled packet header matches the expected GUE encapsulated packet.

### SFLOW-2.6: sFlow sampling with gRIBI Encap

*   Configure gRIBI on DUT.
*   Install gRIBI entries to perform encapsulation.
*   Send traffic matching gRIBI entries.
*   Verify DUT encapsulates traffic and sends to ATE Port 3.
*   Verify sFlow collector receives samples.
*   Verify sFlow sample contains:
    *   Ingress interface matches DUT Port 1.
    *   Egress interface matches DUT Port 3.
    *   Sampled packet header matches the expected encapsulated packet.

### Canonical OC
```json
{
  "interfaces": {
    "interface": [
      {
        "name": "eth1",
        "config": {
          "name": "eth1"
        }
      }
    ]
  },
  "sampling": {
    "sflow": {
      "config": {
        "enabled": true,
        "ingress-sampling-rate": 10000
      },
      "collectors": {
        "collector": [
          {
            "address": "192.0.2.2",
            "port": 6343,
            "config": {
              "address": "192.0.2.2",
              "port": 6343
            }
          }
        ]
      },
      "interfaces": {
        "interface": [
          {
            "name": "eth1",
            "config": {
              "name": "eth1",
              "enabled": true
            }
          }
        ]
      }
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  # sFlow config
  /sampling/sflow/config/enabled:
  /sampling/sflow/config/ingress-sampling-rate:
  /sampling/sflow/interfaces/interface/config/enabled:
  /sampling/sflow/interfaces/interface/config/ingress-sampling-rate:
  /sampling/sflow/collectors/collector/config/address:
  /sampling/sflow/collectors/collector/config/port:

  # sFlow state
  /sampling/sflow/state/enabled:
  /sampling/sflow/interfaces/interface/state/enabled:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
  gribi:
    gRIBI.Modify:
```

## Required DUT platform

* FFF
