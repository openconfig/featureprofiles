# SYS-6.1: SSO Extended Forwarding and Stability Validation

## Summary

Validate long-duration device stability, agent/process health, and forwarding integrity (no traffic drops or VOQ drops) for 10 minutes following each of the two Supervisor Switchovers (SSO).

## Testbed type

* [`4_router_links.testbed`](https://github.com/openconfig/featureprofiles/tree/main/topologies)

## Procedure

### Test environment setup

* Configure DUT AS Number: `65000`
* Create two L3 VRFs representing the DCGate architecture: `TRANSIT_VRF` and `DECAP_TE_VRF` by setting `/network-instances/network-instance/config/type` to `L3VRF`.
* Configure VRF 1: `TRANSIT_VRF`
  * Assign Port 1 and Port 2 to `TRANSIT_VRF` via `/network-instances/network-instance/interfaces/interface/config/id`.
  * **Port 1 Setup:**
    * DUT IP: `192.0.2.1/30` | ATE IP: `192.0.2.2/30`
    * BGP Neighbor: `192.0.2.2` | Remote AS: `65001`
    * ATE Advertised Routes: `198.51.100.0/24`
  * **Port 2 Setup:**
    * DUT IP: `192.0.2.5/30` | ATE IP: `192.0.2.6/30`
    * BGP Neighbor: `192.0.2.6` | Remote AS: `65001`
    * ATE Advertised Routes: `198.51.101.0/24`
* Configure VRF 2: `DECAP_TE_VRF`
  * Assign Port 3 and Port 4 to `DECAP_TE_VRF` via `/network-instances/network-instance/interfaces/interface/config/id`.
  * **Port 3 Setup:**
    * DUT IP: `192.0.2.9/30` | ATE IP: `192.0.2.10/30`
    * BGP Neighbor: `192.0.2.10` | Remote AS: `65002`
    * ATE Advertised Routes: `198.51.102.0/24`
  * **Port 4 Setup:**
    * DUT IP: `192.0.2.13/30` | ATE IP: `192.0.2.14/30`
    * BGP Neighbor: `192.0.2.14` | Remote AS: `65002`
    * ATE Advertised Routes: `198.51.103.0/24`
* Configure basic egress queue management profiles representing `AF4` and `BE0` traffic classes and attach them to all output ports by mapping them to `/qos/interfaces/interface/output/queues/queue/config/name`.
* Enable BGP Graceful Restart by setting `/network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/enabled` to `true`, `/network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/restart-time` to `120` and configuring `stale-routes-time` to ensure routes survive the SSO.

### SYS-6.1.1 - Extended Post-SSO Traffic and Process Health Soak Test

* Step 1 - Start Background Traffic and Record Process State

    * Initiate continuous background traffic from ATE matching the `AF4` and `BE0` QoS queues throughout the duration of the test.
    * Ensure BGP has converged and traffic flows with `0` dropped packets.
    * Query `/system/processes/process` using gNMI Get to find the PIDs for critical hardware and routing agents (e.g. `AsicResourceMgr`, `SandL3Ni` equivalents) via `/system/processes/process/state/name`.
    * Record their `/system/processes/process/state/start-time` and `/system/processes/process/state/pid`.
    * Record the baseline `/system/processes/process/state/memory-usage` for memory leak detection.

* Step 2 - Trigger Supervisor Switchover

    * Trigger a supervisor switchover using `gnoi.System.SwitchControlProcessor`.
    * For 10 minutes post switchover, every 2 minutes validate that:
        * **Crash Detection:** Query the recorded processes and verify their `/system/processes/process/state/start-time` and `/system/processes/process/state/pid` have **not changed**. A changed `start-time` or a missing PID indicates the process crashed and restarted.
        * **Memory Leak Detection:** Poll `/system/processes/process/state/memory-usage` for the critical agents.
            * Memory usage should not keep on increasing over time when compared to the baseline.
    * Validate the supervisors are switchover ready using `/components/component/state/switchover-ready`
    * Trigger another supervisor switchover using `gnoi.System.SwitchControlProcessor`.

* Step 3 - Soak Phase

    * Continue generating traffic from the ATE for **10 minutes** post-switchover.
    * For 10 minutes post switchover, every 2 minutes validate that:
        * **Crash Detection:** Query the recorded processes and verify their `/system/processes/process/state/start-time` and `/system/processes/process/state/pid` have **not changed**. A changed `start-time` or a missing PID indicates the process crashed and restarted.
        * **Memory Leak Detection:** Poll `/system/processes/process/state/memory-usage` for the critical agents.
            * Memory usage should not keep on increasing over time when compared to the baseline.
    * Validate the supervisors are switchover ready using `/components/component/state/switchover-ready`

* Step 4 - Validation with pass/fail criteria

    * Validate that traffic loss is 0% during the entire duration of the test.
    * Verify that QoS queue drop telemetry (`/qos/interfaces/interface/output/queues/queue/state/dropped-pkts`) remains `0`.

#### Canonical OC

```json
{
  "network-instances": {
    "network-instance": [
      {
        "name": "TRANSIT_VRF",
        "config": {
          "name": "TRANSIT_VRF",
          "type": "openconfig-network-instance-types:L3VRF"
        },
        "protocols": {
          "protocol": [
            {
              "identifier": "BGP",
              "name": "BGP",
              "config": {
                "identifier": "BGP",
                "name": "BGP"
              },
              "bgp": {
                "global": {
                  "graceful-restart": {
                    "config": {
                      "restart-time": 120,
                      "stale-routes-time": 300
                    }
                  }
                }
              }
            }
          ]
        }
      }
    ]
  }
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
  ## Config Paths ##
  /network-instances/network-instance/config/type:
  /network-instances/network-instance/interfaces/interface/config/id:
  /network-instances/network-instance/protocols/protocol/bgp/global/config/as:
  /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/enabled:
  /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/restart-time:
  /network-instances/network-instance/protocols/protocol/bgp/global/graceful-restart/config/stale-routes-time:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address:
  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as:
  /qos/interfaces/interface/output/queues/queue/config/name:
  /qos/interfaces/interface/output/queues/queue/config/queue-management-profile:
  
  ## State Paths ##
  /qos/interfaces/interface/output/queues/queue/state/dropped-pkts:
  /components/component/state/switchover-ready:
    platform_type: [CONTROLLER_CARD]
  /system/processes/process/state/memory-usage:
  /system/processes/process/state/name:
  /system/processes/process/state/pid:
  /system/processes/process/state/start-time:

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
    gNMI.Get:
  gnoi:
    system.System.SwitchControlProcessor:
```

## Required DUT platform

* MFF
