# gNMI-1.10: Telemetry: Basic Check

## Summary

Validate basic telemetry paths required.

## Procedure

In the automated ondatra test, verify the presence of the telemetry paths of the
following features:

*   Ethernet interface

    *   Check the telemetry port-speed exists with correct speed.
        *   /interfaces/interfaces/interface/ethernet/state/mac-address
    *   Check the telemetry mac-address with correct format.
        *   /interfaces/interfaces/interface/ethernet/state/port-speed

*   Interface status

    *   Check admin-status and oper-status exist and correct.
        *   /interfaces/interfaces/interface/state/admin-status
        *   /interfaces/interfaces/interface/state/oper-status

*   Interface physical channel

    *   Check interface physical-channel exists.
        *   /interfaces/interface/state/physical-channel

*   Interface status change

    *   Check admin-status and oper-status are correct after interface flapping.
        *   /interfaces/interfaces/interface/state/admin-status
        *   /interfaces/interfaces/interface/state/oper-status

*   Interface hardware-port

    *   Check hardware-port exists and correct.
        *   /interfaces/interfaces/interface/state/hardware-port

*   Interface counters

    *   Check the presence of the following interface counters.
        *   /interfaces/interface/state/counters/in-octets
        *   /interfaces/interface/state/counters/in-unicast-pkts
        *   /interfaces/interface/state/counters/in-broadcast-pkts
        *   /interfaces/interface/state/counters/in-multicast-pkts
        *   /interfaces/interface/state/counters/in-discards
        *   /interfaces/interface/state/counters/in-errors
        *   /interfaces/interface/state/counters/in-fcs-errors
        *   /interfaces/interface/state/counters/out-unicast-pkts
        *   /interfaces/interface/state/counters/out-broadcast-pkts
        *   /interfaces/interface/state/counters/out-multicast-pkts
        *   /interfaces/interface/state/counters/out-octets
        *   /interfaces/interface/state/counters/out-discards
        *   /interfaces/interface/state/counters/out-errors

*   Send the traffic over the DUT.

    *   Check some counters are updated correctly.

*   QoS counters

    *   Send the traffic with all forwarding class NC1, AF4, AF3, AF2, AF1 and
        BE1 over the DUT
    *   Check the QoS queue counters exist and are updated correctly
        *   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
        *   TODO:
            /qos/interfaces/interface/output/queues/queue/state/transmit-octets
        *   TODO:
            /qos/interfaces/interface/output/queues/queue/state/dropped-pkts

*   Component

    *   Check the following component paths exists
        *   /components/component/integrated-circuit/state/node-id
        *   /components/component/state/parent

*   CPU component state

    *   Check the following component paths exists
        *   (type=CPU) /components/component/state/description
        *   (type=CPU) /components/component/state/mfg-name

*   Controller card last-reboot-time and reason

    *   Check the following component paths exists
        *   (type=CONTROLLER_CARD)
            /components/component[name=<supervisor>]/state/last-reboot-time
        *   (type=CONTROLLER_CARD)
            /components/component[name=<supervisor>]/state/last-reboot-reason

*   Software version

    *   Check the following component paths exists for SwitchChip cards.
        *   /components/component/state/software-version

*   LACP

    *   Check the bundle interface member path and LACP counters and status.
        *   /lacp/interfaces/interface/members/member

*   AFT

    *   Check the following AFT path exists.
        *   TODO: /network-instances/network-instance/afts

*   P4RT

    *   Enable p4-runtime.
    *   configure interface port ID with minimum and maximum uint32 values.
    *   Check the following path exists with correct interface ID.
        *   /interfaces/interfaces/state/id
    *   configure FAP device ID with minimum and maximum uint64 values.
    *   Check the following path exists with correct node ID.
        *   /components/component/integrated-circuit/state/node-id

## Config Parameter coverage

No configuration coverage.

## Telemetry Parameter coverage

*   /interfaces/interface/state/admin-status
*   /lacp/interfaces/interface/members/member
*   /interfaces/interface/ethernet/state/mac-address
*   /interfaces/interface/state/hardware-port /interfaces/interface/state/id
*   /interfaces/interface/state/oper-status
*   /interfaces/interface/ethernet/state/port-speed
*   /interfaces/interface/state/physical-channel
*   /components/component/integrated-circuit/state/node-id
*   /components/component/state/parent
*   /interfaces/interface/state/counters/in-octets
*   /interfaces/interface/state/counters/in-unicast-pkts
*   /interfaces/interface/state/counters/in-broadcast-pkts
*   /interfaces/interface/state/counters/in-multicast-pkts
*   /interfaces/interface/state/counters/in-discards
*   /interfaces/interface/state/counters/in-errors
*   /interfaces/interface/state/counters/in-fcs-errors
*   /interfaces/interface/state/counters/out-unicast-pkts
*   /interfaces/interface/state/counters/out-broadcast-pkts
*   /interfaces/interface/state/counters/out-multicast-pkts
*   /interfaces/interface/state/counters/out-octets
*   /interfaces/interface/state/counters/out-discards
*   /interfaces/interface/state/counters/out-errors
*   /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
*   /qos/interfaces/interface/output/queues/queue/state/transmit-octets
*   /qos/interfaces/interface/output/queues/queue/state/dropped-pkts

## Protocol/RPC Parameter coverage

N/A

## Minimum DUT platform requirement

N/A
