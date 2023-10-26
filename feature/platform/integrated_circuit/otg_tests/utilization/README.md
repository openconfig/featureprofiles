# IC-1: Integrated Circuit Utilization and Thresholds

## Summary

Test resource utilization and threshold for `INTEGRATED_CIRCUIT` (IC) components.

## Procedure

* IC-1.1
  * Get IC component names
  * Verify resource utilization and threshold leaves exist for component
  type `INTEGRATED_CIRCUIT`
* IC-1.2
  * Configure `used-threshold-upper` at 10% for resource IC
  * Configure `used-threshold-upper-clear` at 5% for resource IC
  * Verify state for the above leaves equals the configured values
* IC-1.4
  * Configure ISIS and BGP on DUT and ATE
  * subscribe using SAMPLE mode to  `used` leaf.
  * Advertise 100 IPv4 and IPv6 routes (200 total) from ATE to DUT
  * Verify `used` leaf increases in value
* IC-1.5
  * subscribe ON_CHANGE to `max-limit` from DUT
  * subscribe ON_CHANGE to `used-threshold-upper-exceeded`
  * Add more IPv4 and IPv6 routes than `max-limit` * `used-threshold-upper`
  * Verify `used-threshold-upper-exceeded` upper is true
* IC-1.6
  * Reduce ATE route advertisement to 100 IPv4 and IPv6 routes (200 total)
  * Verify `used-threshold-upper-exceeded` upper is true

## Config Parameter Coverage

/components/component/chassis/utilization/resources/resource/config/name
/system/utilization/resources/resource/config/name/used-threshold-upper
/system/utilization/resources/resource/config/name/used-threshold-upper-clear
/system/utilization/resources/resource/

## Telemetry Parameter Coverage

/system/utilization/resources/resource/state/active-component-list
/components/component/chassis/utilization/resources/resource/state/committed
/components/component/chassis/utilization/resources/resource/state/free
/components/component/chassis/utilization/resources/resource/state/high-watermark
/components/component/chassis/utilization/resources/resource/state/last-high-watermark
/components/component/chassis/utilization/resources/resource/state/max-limit
/components/component/chassis/utilization/resources/resource/state/name
/components/component/chassis/utilization/resources/resource/state/used
/components/component/chassis/utilization/resources/resource/state/used-threshold-upper
/components/component/chassis/utilization/resources/resource/state/used-threshold-upper-clear

## Protocol/RPC Parameter Coverage

None

## Required DUT platform

This test should run on both

* MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
* FFF - fixed form factor
