# IC-1: Integrated Circuit Utilization and Thresholds

## Summary

Test resource utilization and threshold for INTEGRATED_CIRCUIT (IC) components.

## Procedure
* IC-1.1 - Get IC component names and verify resource utilization and threshold leaves exist
* IC-1.2 - Verify IC threshold leaves exist
* IC-1.3 - Configure /system/resource/threshold at 10% for resource IC
* IC-1.4 - Add routes and traffic and verify 'used' leaf increases in value
* IC-1.5 - Add more routes than threshold-upper and verify used-threshold-upper-exceeded upper is true

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

