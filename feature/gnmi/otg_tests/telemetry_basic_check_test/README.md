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

    *   Check hardware-port exists
        *   /interfaces/interfaces/interface/state/hardware-port
    *   Check that [hardware-port leaf]  (https://github.com/openconfig/public/blob/0c9fb6b0ab96fdd96bb9e88365abe11e51a11e62/release/models/platform/openconfig-platform-port.yang#L306) exists as a component in the Device's component tree and has a type as [PORT](https://github.com/openconfig/public/blob/76f77b566449af43f941f6dd3b0e42fddaadacc6/release/models/platform/openconfig-platform-types.yang#L315-L320)
        * For example,  /components/component[name=<hardware-port-leaf-val>]/state/type == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS_PORT
    *   Use the parent leaf of the hardware-port component to traverse the component tree to verify an ancestor of type CHASSIS exists.   Components in between the PORT and the CHASSIS  may vary in quantity and type.

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

*   Active Controller Card Software version

    *   Check the following component path and value exists.
        *   /system/state/software-version

*   Controller Card Software versions

    *   Check the following component path and value exists for component type
        `OPERATING_SYSTEM` that is present/installed, and whose parent component type is `CONTROLLER_CARD`.
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
*   /qos/interfaces/interface/output/queues/queue/state/dropped-octets

## Protocol/RPC Parameter coverage

N/A

## Minimum DUT platform requirement

N/A
