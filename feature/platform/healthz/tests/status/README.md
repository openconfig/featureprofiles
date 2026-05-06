# Health-1.2: Healthz component status paths

## Summary

Validate healthz status paths exist for select OC component types.  There
are two operational use cases for this test.

1. As a network operator, I want to know if a device is healthy.  If the
   device is unhealthy, I may choose to execute a mitigation or repair action.
   The choice of action is influenced by which component(s) are not healthy.
2. As a SDN controller, I want to know if a device is ready to be programmed
   for traffic forwarding.

## Testbed type

* [Single DUT](https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed)

## Testbed prerequisites

There are no DUT configuration pre-requisites for this test.  The DUT must
contain the following component types:
        "CONTROLLER_CARD",
        "LINECARD",
        "FABRIC",
        "POWER_SUPPLY",
        "INTEGRATED_CIRCUIT"

The DUT should have a HEALTHY state for all the above components.

## Procedure

* Healthz-1.2.1: Validate healthz status
  * gNMI Subscribe to the /components tree using ON_CHANGE option.
  * Validate `/components/component/healthz/state/status` returns `HEALTHY`
    for each of the following component types:
        "CONTROLLER_CARD",
        "LINECARD",
        "FABRIC",
        "POWER_SUPPLY",
        "INTEGRATED_CIRCUIT"
  * Validate the following paths return a valid value:
    * /components/component/healthz/state/last-unhealthy
    * /components/component/healthz/state/unhealthy-count

* Healthz-1.2.3: Reboot DUT and validate status converges to healthy
  * Use gnoi.System.Reboot to reboot the DUT
  * Repeatedly attempt to open a gNMI subscribe request to the DUT
  * Upon success, subscribe to the /components tree using ON_CHANGE option.
  * Validate `/components/component/healthz/state/status` exists for each of
    the following component types:
        "CONTROLLER_CARD",
        "LINECARD",
        "FABRIC",
        "POWER_SUPPLY",
        "INTEGRATED_CIRCUIT"
  * Validate status transitions to `HEALTHY` within a timeout of 15 minutes
    from the reboot start time.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths and RPC intended to be covered by this test.

```yaml
paths:
  ## State Paths ##
    /components/component/healthz/state/status:
        platform_type: [
            "CONTROLLER_CARD",
            "LINECARD",
            "FABRIC",
            "POWER_SUPPLY",
            "INTEGRATED_CIRCUIT"
        ]
    /components/component/healthz/state/last-unhealthy:
        platform_type: [
            "CONTROLLER_CARD",
            "LINECARD",
            "FABRIC",
            "POWER_SUPPLY",
            "INTEGRATED_CIRCUIT"
        ]
    /components/component/healthz/state/unhealthy-count:
        platform_type: [
            "CONTROLLER_CARD",
            "LINECARD",
            "FABRIC",
            "POWER_SUPPLY",
            "INTEGRATED_CIRCUIT"
        ]

rpcs:
    gnmi:
        gNMI.Subscribe:
            ON_CHANGE: true

    gnoi:
        system.System.Reboot:

```

## Minimum DUT platform requirement

MFF - Modular form factor is specified to ensure coverage for `CONTROLLER_CARD` and `FABRIC` platform_types.
