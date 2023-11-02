# SFLOW-1: sFlow Configuration and Sampling

## Summary

Verify configuration of sflow and sFlow sample data.

## Procedure

*   SFLOW-1.1 Configure sFlow on DUT
    * Configure DUT to send sflow samples to ATE port 2

*   SFLOW-1.2 Send traffic via OTG and verify sFlow packet on OTG
    *   Configure ATE to generate ipv4 and ipv6 traffic and capture sFlow packets
    *   Use sample rate of 1/1M and sample size of 256 bytes
    *   Verify captured packet is formatted like an sFlow packet

## Config Parameter coverage
/sampling/sflow/config/agent-id-ipv4
/sampling/sflow/config/agent-id-ipv6
/sampling/sflow/config/dscp
/sampling/sflow/config/egress-sampling-rate
/sampling/sflow/config/enabled
/sampling/sflow/config/ingress-sampling-rate
/sampling/sflow/config/polling-interval
/sampling/sflow/config/sample-size
/sampling/sflow/config/source-address
/sampling/sflow/interfaces/interface/config/name
/sampling/sflow/interfaces/interface/config/enabled
/sampling/sflow/interfaces/interface/config/egress-sampling-rate
/sampling/sflow/interfaces/interface/config/ingress-sampling-rate
/sampling/sflow/interfaces/interface/config/polling-interval

/sampling/sflow/collectors/collector/address
/sampling/sflow/collectors/collector/config/address
/sampling/sflow/collectors/collector/config/network-instance
/sampling/sflow/collectors/collector/config/port
/sampling/sflow/collectors/collector/config/source-address
/sampling/sflow/collectors/collector/port

## Telemetry Parameter coverage
/sampling/sflow/state/agent-id-ipv4
/sampling/sflow/state/agent-id-ipv6
/sampling/sflow/state/dscp
/sampling/sflow/state/egress-sampling-rate
/sampling/sflow/state/enabled
/sampling/sflow/state/ingress-sampling-rate
/sampling/sflow/state/polling-interval
/sampling/sflow/state/sample-size
/sampling/sflow/state/source-address
/sampling/sflow/interfaces/interface/state/name
/sampling/sflow/interfaces/interface/state/enabled
/sampling/sflow/interfaces/interface/state/egress-sampling-rate
/sampling/sflow/interfaces/interface/state/ingress-sampling-rate
/sampling/sflow/interfaces/interface/state/polling-interval

/sampling/sflow/collectors/collector/address
/sampling/sflow/collectors/collector/state/address
/sampling/sflow/collectors/collector/state/network-instance
/sampling/sflow/collectors/collector/state/port
/sampling/sflow/collectors/collector/state/source-address
/sampling/sflow/collectors/collector/port

## Protocol/RPC Parameter coverage
N/A

## Minimum DUT platform requirement
FFF