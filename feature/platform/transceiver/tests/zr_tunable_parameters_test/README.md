# TRANSCEIVER-5: Configuration: 400ZR channel frequency, output TX launch power and operational mode setting.

## Summary

Validate setting 400ZR tunable parameters channel frequency, output TX launch
power and operational mode and verify corresponding telemetry values.

### Goals

*   Verify full C band frequency tunability for 100GHz line system grid.
*   Verify full C band frequency tunability for 75GHz line system grid.
*   Verify adjustable range of transmit output power across -13 to -9 dBm in
    steps of 1 dB.
*   Verify that the ZR module Host Interface ID and Media Interface ID
    combination to ZR module AppSel mapping can be configured through the OC
    `operational-mode`. `operational-mode` is a construct in OpenConfig that
    masks features related to line port transmission. OC operational modes
    provides a platform-defined summary of information such as symbol rate,
    modulation, pulse shaping, etc.

**Note** For standard ZR, OIF 400ZR with C-FEC is the default mode however as we
move to 400ZR++ and 800ZR, optic AppSel code would need to be configured
explicitly through OC operational mode.

## TRANSCEIVER-5.1

*   Connect two ZR interfaces using a duplex LC fiber jumper such that TX output
    power of one is the RX input power of the other module. Connection between
    the modules should pass through an optical switch that can be controlled
    through automation to simulate a fiber cut.
*   To establish a point to point ZR link ensure the following:

    *   Both transceivers states are enabled.
    *   Validate setting 400ZR optics module tunable laser center frequency
        across frequency range 196.100 - 191.400 THz for 100GHz grid.
    *   Validate setting 400ZR optics module tunable laser center frequency
        across frequency range 196.100 - 191.375 THz for 75GHz grid.
    *   Specific frequency details can be found in 400ZR implementation
        agreement under sections 15.1 ad 15.2 Operating frequency channel
        definitions. Link to IA below,
        *   https://www.oiforum.com/wp-content/uploads/OIF-400ZR-01.0_reduced2.pdf
    *   Validate adjustable range of transmit output power across -13 to -9 dBm
        range in steps of 1dB. So the module’s output power will be set to -13,
        -12, -11, -10, -9 dBm in each step. As an example this can be validated
        for the module's default frequency of 193.1 THz.

*   With the ZR link established as explained above, for each configured
    frequency and TX output power value verify that the following ZR transceiver
    telemetry paths exist and are streamed for both the ZR optics.

    *   Frequency
        *   /components/component/optical-channel/state/frequency
        *   /components/component/optical-channel/state/carrier-frequency-offset/instant
        *   /components/component/optical-channel/state/carrier-frequency-offset/avg
        *   /components/component/optical-channel/state/carrier-frequency-offset/min
        *   /components/component/optical-channel/state/carrier-frequency-offset/max
    *   TX Output Power
        *   /components/component/optical-channel/state/output-power/instant
        *   /components/component/optical-channel/state/output-power/avg
        *   /components/component/optical-channel/state/output-power/min
        *   /components/component/optical-channel/state/output-power/max
    *   Operational Mode
        *   /components/component/optical-channel/state/operational-mode

*   With above streamed data verify

    *   For each center frequency, laser frequency offset should not be more
        than +/- 1.8 GHz max.
    *   For each center frequency, streamed value should be in Mhz units. Test
        should fail if the streamed value is in Hz or THz units. As an example
        193.1 THz would be 193100000 in MHz.
    *   When set to a specific target output power, transmit power control
        absolute accuracy should be within +/- 1 dBm of the target value.
    *   For reported data check for validity: min <= avg/instant <= max

## TRANSCEIVER-5.2

*   When the modules or the devices are still in a boot stage, they must not
    stream any invalid string values like "nil" or "-inf".

*   Frequency must be specified as uint64 in MHz. Streamed values for frequency
    offset must be of type decimal64.

*   TX Output power must be of type decimal64.

## TRANSCEIVER-5.3

*   Verify that the optics Tunable Frequency and TX output power tunes back to
    the correct value as per configuration after the interface flaps.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics frequency and TX output power telemetry values are
        set in the normal range.
    *   Disable or shut down the interface on the DUT.
    *   Verify with interfaces in down state both optics are still streaming
        configured value for frequency.
    *   Verify for the TX output power with interface in down state a decimal64
        value of -40 dB is streamed.
    *   Re-enable the interfaces on the DUT.
    *   Verify the ZR optics tune back to the correct frequency and TX output
        power as per the configuration and related telemetry values are updated
        to the value in the normal range again.

*   With above test also verify

    *   Laser frequency offset should not be more than +/- 1.8 GHz max from the
        configured centre frequency.
    *   When set to a specific target output power, transmit power control
        absolute accuracy should be within +/- 1 dBm of the target configured
        output power.
    *   For reported data check for validity: min <= avg/instant <= max

## TRANSCEIVER-5.4

*   Verify that the optics Tunable Frequency and TX output power tunes back to
    the correct value as per configuration after a fiber cut.

    *   Enable a pair of ZR interfaces on the DUT as explained above.
    *   Verify the ZR optics Frequency and TX output power telemetry values are
        in the normal range.
    *   Simulate a fiber cut using the optical switch that sits in-between the
        DUT ports.
    *   Verify with link in down state due to fiber cut both optics are
        streaming uint64 for frequency and decimal64 for TX output power.
    *   Re-enable the optical switch connection to clear the fiber cut fault.
    *   Verify the ZR optics is able to stay tuned to the correct frequency and
        TX output power as per the configuration.

*   With above test also verify

    *   Laser frequency offset should not be more than +/- 1.8 GHz max from the
        configured centre frequency.
    *   When set to a specific target output power, transmit power control
        absolute accuracy should be within +/- 1 dBm of the target configured
        output power.
    *   For reported data check for validity: min <= avg/instant <= max

**Note:** For min, max, and avg values, 10 second sampling is preferred. If 10
seconds is not supported, the sampling interval used must be communicated.

## OpenConfig Path and RPC Coverage

```yaml
paths:
  /components/component/transceiver/config/enabled:
    platform_type: ["TRANSCEIVER"]
  /components/component/optical-channel/config/frequency:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/config/target-output-power:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/config/operational-mode:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/frequency:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/carrier-frequency-offset/instant:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/carrier-frequency-offset/avg:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/carrier-frequency-offset/min:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/carrier-frequency-offset/max:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/output-power/instant:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/output-power/avg:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/output-power/min:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/output-power/max:
    platform_type: ["OPTICAL_CHANNEL"]
  /components/component/optical-channel/state/operational-mode:
    platform_type: ["OPTICAL_CHANNEL"]

rpcs:
  gnmi:
    gNMI.Set:
      replace: true
    gNMI.Subscribe:
      on_change: true
```

## Required DUT platform

FFF
