# RT-8: Singleton with breakouts

## Summary
This test ensures that all singleton interfaces irrespective of their breakout configuration are streaming all the necessary leaves. More leaves can be added to this test for verification

## Testbed
This test requires a DUT (or set of DUT testbeds) configured with a combination of the following singleton and breakout PMDs:
* Singleton (non-breakout) PMDs:
  * 1x800G-ZR (`ETH_800GBASE_ZR` / `800GBASE_ZR` / `OSFP-800G-ZR`)
  * 1x400G-FR4+ (`ETH_400GBASE_FR4` / `400GBASE_FR4`)
  * 1x100G-LR (`ETH_100GBASE_LR4` / `100GBASE_LR4`)
  * 1x100G-FR (`ETH_100GBASE_FR` / `100GBASE_FR`)
* Breakout PMDs:
  * 4x100G-DR4+ (`ETH_400GBASE_DR4` / `400GBASE_DR4` / `OSFP-400G-DR4`)
  * 2x400G-FR4 (`ETH_800GBASE_2XFR4` / `800GBASE_2XFR4` / `OSFP-800G-2FR4`)
  * 2x400G-LR4 (`ETH_800GBASE_2XLR4` / `800GBASE-2xLR4` / `OSFP-800G-2LR4`)
  * 8x100G-LR (`ETH_800GBASE_2XPLR4` / `800GBASE-2xPLR4` / `OSFP-800G-2PLR4`)
  * 8x100G-FR / 8x100G-DR8+ (`ETH_800GBASE_2XDR4` / `800GBASE-2xDR4` / `OSFP-800G-DR8+`)
* ATE connections are not required.
* Note: `openconfig-transport-types v1.5.0` (merged in `openconfig/public` PR #1505) defines the OpenConfig PMD identities `ETH_800GBASE_2XDR4` (`800GBASE-2xDR4`), `ETH_800GBASE_2XLR4` (`800GBASE-2xLR4`), and `ETH_800GBASE_2XPLR4` (`800GBASE-2xPLR4`) alongside `ETH_800GBASE_2XFR4` (`800GBASE_2XFR4`). Until a future Ondatra release imports `openconfig-transport-types v1.5.0` and exposes corresponding `ondatra.PMD` enum constants, the test automation matches both `ondatra.PMD` values and component/transceiver PMD names and descriptions to dynamically apply the appropriate singleton or breakout configuration to the ports present on each DUT.


## Procedure
### RT-8.1 - Baseline test:
* Push interface configuration to the DUT including breakout configuration for all the PMDs stated in the Testbed section above.
* Get an inventory of all the singleton interfaces on the DUT used for this test using `GET /interfaces/interface/` subscription.
* For configured interface, verify `interfaces/interface/state/hardware-port` is populated with a reference to `/components/component/name`

### RT-8.2 - Reboot test:
* Reboot DUT
* Repeat the test in RT-8.1 above.

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

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /components/component/port/breakout-mode/groups/group/config/breakout-speed:
    platform_type: [PORT]
  /components/component/port/breakout-mode/groups/group/config/num-breakouts:
    platform_type: [PORT]
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/port-speed:
  /interfaces/interface/state/hardware-port:
  /interfaces/interface/state/name:
  /system/state/boot-time:
  /system/state/current-datetime:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```

## Canonical OC
```json
{
  "components": {
    "component": [
      {
        "config": {
          "name": "port-1"
        },
        "name": "port-1",
        "port": {
          "breakout-mode": {
            "groups": {
              "group": [
                {
                  "config": {
                    "breakout-speed": "SPEED_100GB",
                    "index": 1,
                    "num-breakouts": 4
                  },
                  "index": 1
                }
              ]
            }
          }
        }
      }
    ]
  },
  "interfaces": {
    "interface": [
      {
        "config": {
          "enabled": true,
          "name": "et-1/1/1",
          "type": "ethernetCsmacd"
        },
        "name": "et-1/1/1"
      }
    ]
  }
}
```


