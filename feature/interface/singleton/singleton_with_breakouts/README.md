# RT-8: Singleton with breakouts

## Summary
This test ensures that all singleton interfaces irrespective of their breakout configuration are streaming all the necessary leaves. More leaves can be added to this test for verification

## Testbed
This test requires a DUT with the following setup
* The DUT should have following PMDs
  * 1x400G-FR4+
  * 4x100G-DR4+
  * 1x100G-LR
  * 1x100G-FR
* ATE connections are not required.

## Procedure
### RT-8.1 - Baseline test:
* Push interface configuration to the DUT including breakout configuration for all the PMDs stated in the Testbed section above.
* Get an inventory of all the singleton interfaces on the DUT used for this test using `GET /interfaces/interface/` subscription.
* For every interface, verify `interfaces/interface/state/hardware-port` is populated with a reference to `/components/component/name`

### RT-8.2 - Reboot test:
* Reboot DUT
* Repeat the test in RT-6.1 above.

## Config Parameter coverage
*   /components/component/port/breakout-mode/groups/group/index
*   /components/component/port/breakout-mode/groups/group/config
*   /components/component/port/breakout-mode/groups/group/config/index
*   /components/component/port/breakout-mode/groups/group/config/num-breakouts
*   /components/component/port/breakout-mode/groups/group/config/breakout-speed
*   /components/component/port/breakout-mode/groups/group/config/num-physical-channels
*   gNOI.Reboot

## Telemetry Parameter Coverage
*   /interfaces/interface/
*   /interfaces/interface/state/hardware-port
