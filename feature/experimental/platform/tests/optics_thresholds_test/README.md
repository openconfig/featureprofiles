# gNMI-1.20: Telemetry: Optics Thresholds

## Summary

Validate optics high and low thresholds for input power, output power, temperature and bias-current.

## Procedure

*   Connect at least one optical ethernet interface to ATE.
*   Check all the transceivers with inslalled optcs.
*   Validate that the following optics threshold telemetry paths exist for each optics.
    *   Output power thresholds:
        *   /components/component/Ethernet/properties/property/laser-tx-power-low-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-tx-power-high-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-tx-power-low-warn-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-tx-power-high-warn-threshold/state/value
    *   Input power threshold:
        *   /components/component/Ethernet/properties/property/laser-rx-power-low-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-rx-power-high-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-rx-power-low-warn-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-rx-power-high-warn-threshold/state/value
    *   Optics temperature threshold:
        *   /components/component/Ethernet/properties/property/laser-temperature-low-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-temperature-high-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-temperature-low-warn-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-temperature-high-warn-threshold/state/value
    *   Optics bias-current threshold:
        *   /components/component/Ethernet/properties/property/laser-bias-current-low-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-bias-current-high-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-bias-current-low-warn-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-bias-current-high-warn-threshold/state/value

    
## Config Parameter coverage

*   None

## Telemetry Parameter coverage
    *   Output power thresholds:
        *   /components/component/Ethernet/properties/property/laser-tx-power-low-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-tx-power-high-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-tx-power-low-warn-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-tx-power-high-warn-threshold/state/value
    *   Input power threshold:
        *   /components/component/Ethernet/properties/property/laser-rx-power-low-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-rx-power-high-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-rx-power-low-warn-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-rx-power-high-warn-threshold/state/value
    *   Optics temperature threshold:
        *   /components/component/Ethernet/properties/property/laser-temperature-low-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-temperature-high-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-temperature-low-warn-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-temperature-high-warn-threshold/state/value
    *   Optics bias-current threshold:
        *   /components/component/Ethernet/properties/property/laser-bias-current-low-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-bias-current-high-alarm-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-bias-current-low-warn-threshold/state/value
        *   /components/component/Ethernet/properties/property/laser-bias-current-high-warn-threshold/state/value
        
## Notes:
*   The model for optics threshold paths is not finalized. We may need to update those paths after the model is finalized.    
