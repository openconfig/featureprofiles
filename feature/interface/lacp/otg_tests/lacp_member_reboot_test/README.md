# RT-5.16: LACP Member Linecard Reboot

## Summary

This test validates that LACP bundle interfaces spanning multiple linecards
function correctly during and after a linecard reboot.
It ensures that traffic continues to flow on remaining members when one
linecard is rebooted, and that interface traffic statistics are correctly
reported (no decrementing or resetting to zero unexpectedly) after the
linecard recovers.

## Testbed Type

* [`featureprofiles/topologies/atedut_4.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_4.testbed)
* The DUT must be a Modular Form Factor (MFF) device.
* At least two DUT ports used for the bundle must be located on different linecards.

## Procedure

### Test environment setup

* Ensure the bundle member ports are on different linecards (e.g., `Linecard 1` and `Linecard 2`).
* Configure a LACP bundle (e.g., `Port-Channel1`) on the DUT with at least two member ports.
* Configure corresponding bundle on the ATE.
* Establish LACP session between DUT and ATE.
* Configure IP addresses on the bundle interfaces in the Default Network Instance (VRF).

### RT-5.16.1: Traffic flow and stats validation during linecard reboot

* **Step 1: Configure Traffic**
  * Set up a traffic profile from ATE to send traffic to the DUT bundle interface.
  * The traffic should be distributed across all member links of the bundle and traffic should be sent continuously during test.

* **Step 2: Verify Initial State**
  * Verify LACP is ESTABLISHED.
  * Verify traffic is flowing and distributed across all member links.
  * Verify interface traffic counters (gNMI paths for counters) are incrementing on all member ports and the bundle interface.

* **Step 3: Reboot Linecard**
  * Issue a `gnoi.system.Reboot` RPC targeting the linecard containing one of the bundle member ports (e.g., `Linecard 1`).
  * During the reboot, verify that:
    * The rebooted member port status goes DOWN.
    * LACP session for that member port goes DOWN.
    * Traffic continues to flow through the remaining member port(s) on the other linecard (e.g., `Linecard 2`).
    * The aggregate bundle traffic counters continue to increment (possibly at a reduced rate if traffic is dropped, but the cumulative counter must not decrement).

* **Step 4: Linecard Recovery**
  * Wait for the rebooted linecard to fully recover and come back online.
  * Verify that the member port status goes UP.
  * Verify LACP session is re-established for that member port.
  * Verify traffic distribution resumes across all member links.

* **Step 5: Verify Statistics Post-Reboot**
  * Verify that the interface traffic counters for the rebooted port resume incrementing.
  * Verify that the cumulative traffic statistics (counters) reported via telemetry do not show unexpected drops, decrements, or reset to zero (for the bundle interface).
  * Note: Individual port counters on the rebooted linecard may reset to zero upon reboot, but they should resume incrementing from zero without causing the aggregate bundle counters to decrement.

## Canonical OC

```json
{
  "openconfig-interfaces:interfaces": {
    "interface": [
      {
        "name": "Port-Channel1",
        "config": {
          "name": "Port-Channel1"
        },
        "openconfig-if-aggregate:aggregation": {
          "config": {
            "lag-type": "LACP"
          }
        }
      },
      {
        "name": "Ethernet1/1",
        "config": {
          "name": "Ethernet1/1",
          "type": "ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Port-Channel1"
          }
        }
      },
      {
        "name": "Ethernet2/1",
        "config": {
          "name": "Ethernet2/1",
          "type": "ethernetCsmacd"
        },
        "ethernet": {
          "config": {
            "aggregate-id": "Port-Channel1"
          }
        }
      }
    ]
  },
  "openconfig-lacp:lacp": {
    "interfaces": {
      "interface": [
        {
          "name": "Port-Channel1",
          "config": {
            "name": "Port-Channel1"
          }
        }
      ]
    }
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
oc_paths:
  ## Config Paths ##
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/ethernet/config/aggregate-id:
  /interfaces/interface/openconfig-if-aggregate:aggregation/config/lag-type:
  /lacp/interfaces/interface/config/name:

  ## State Paths ##
  /interfaces/interface/state/oper-status:
  /interfaces/interface/state/counters/in-octets:
  /interfaces/interface/state/counters/out-octets:
  /interfaces/interface/state/counters/in-pkts:
  /interfaces/interface/state/counters/out-pkts:
  /lacp/interfaces/interface/members/member/state/activity:
  /lacp/interfaces/interface/members/member/state/synchronization:
  /components/component/state/oper-status:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
      replace: true
    gNMI.Subscribe:
      on_change: true
  gnoi:
    system.System.Reboot:
```

## Required DUT platform

* MFF
