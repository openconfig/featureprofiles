# dp-1.4: QoS Interface Output Queue Counters

## Summary

Validate QoS interface output queue counters.

## Procedure

*   Configure ATE port-1 connected to DUT port-1, and ATE port-2 connected to DUT port-2, with the relevant IPv4 addresses.
*   Send the traffic with forwarding class NC1, AF4, AF3, AF2, AF1 and BE1 over the DUT.
*   Verify that the following telemetry paths exist on the QoS output interface of the DUT.
    *   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
    *   /qos/interfaces/interface/output/queues/queue/state/transmit-octets
    *   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
    *   /qos/interfaces/interface/output/queues/queue/state/dropped-octets
    
## Config Parameter coverage

*   /interfaces/interface/config/enabled
*   /interfaces/interface/config/name
*   /interfaces/interface/config/description

## Telemetry Parameter coverage

*   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
*   /qos/interfaces/interface/output/queues/queue/state/transmit-octets
*   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
*   /qos/interfaces/interface/output/queues/queue/state/dropped-octets
