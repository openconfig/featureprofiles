# GNMI-2: gnmi_subscriptionlist_test

## Summary
This is to test for gNMI `Subscription` to multiple paths with different `SubscriptionMode` in a single `SubscriptionRequest` message using the `Subscriptionlist` field. Goal here is to,
  * Ensure that the NOS supports "Subscriptionlist" field with multiple `Subscription` messages and also supports the desired `Subscriptionmode` per path in each `Subscription` message.
  * The tests also check if the DUT is responding back everytime with a `SubscriptionResponse` message that has the `sync_response` field set to `true`

## Procedure
### GNMI-2.1: Verify single subscription request with a Subscriptionlist and different SubscriptionModes:
  * Send a single `SubscribeRequest` message to the DUT with a **SubcriptionList** and **SubscriptionMode** matching the "Telemetry Parameter Coverage" section below. Use `Stream` mode for the `SubcribeRequest`.
  * Ensure that the implementation successfully allows subscription to all the paths mentioned below and a `SubscribeResponse` message is received by the client with the `sync_reponse` field set to `true`. The RPC via which the `SubscribeRequest` was recieved should eventually be closed by the client.
### GNMI-2.2: Change SubscriptionModes in the subscription list and verify receipt of sync_response:
  * In the "Telemetry Parameter coverage" section below, change the `Subscribe` message for each of the paths with `SubscriptionMode` as `ON_CHANGE` to `TARGET_DEFINED` and the ones that are `TARGET_DEFINED` to `SAMPLE` w/ a sampe_interval of 10secs and send all the subscribe messages in a single `SubscribeRequest` message to the DUT. Confirm that a `SubscribeResponse` message is received by the client with the `sync_reponse` field set to `true`. The client should then close the RPC session
  * Again, switch the `SubscriptionMode` in each `Subscription` message to its original state i.e. from `TARGET_DEFINED` to `ON_CHANGE` and from `SAMPLE` to `TARGET_DEFINED` and resend the `SubscriptionRequest` with `Mode` as `STREAM`. Confirm that the DUT is responding back to the client with a `SubscriptionResponse` and the `Sync_Response` field set to `true`

## Telemetry Parameter Coverage

  * SubscriptionMode: ON_CHANGE
    * /interfaces/interface/state/admin-status
    * /lacp/interfaces/interface/members/member/interface
    * /interfaces/interface/ethernet/state/macaddress
    * /interfaces/interface/state/hardware-port
    * /interfaces/interface/state/id
    * /interfaces/interface/state/oper-status
    * /interfaces/interface/ethernet/state/port-speed
    * /components/component/integrated-circuit/state/node-id
    * /components/component/state/parent
    * /components/component/state/oper-status
    * /interfaces/interface/state/forwarding-viable
    * /components/component/integrated-circuit/backplane-facing-capacity/state/total-operational-ca
pacity
  * SubscriptionMode: TARGET_DEFINED
    * /interfaces/interface/state/counters/in-unicast-pkts
    * /interfaces/interface/state/counters/in-broadcast-pkts
    * /interfaces/interface/state/counters/in-multicast-pkts
    * /interfaces/interface/state/counters/out-unicast-pkts
    * /interfaces/interface/state/counters/out-broadcast-pkts
    * /interfaces/interface/state/counters/out-multicast-pkts
    * /interfaces/interface/state/counters/in-octets
    * /interfaces/interface/state/counters/out-octets
    * /interfaces/interface/state/counters/in-discards
    * /interfaces/interface/state/counters/out-discards
    * /interfaces/interface/state/counters/in-errors
    * /interfaces/interface/state/counters/out-errors
    * /interfaces/interface/state/counters/in-fcs-errors
    * /qos/interfaces/interface/output/queues/queue/state/transmit-pkts
    * /qos/interfaces/interface/output/queues/queue/state/transmit-octets
    * /qos/interfaces/interface/output/queues/queue/state/dropped-pkts
    * /components/component/integrated-circuit/backplane-facing-capacity/state/available-pct
    * /components/component/integrated-circuit/backplane-facing-capacity/state/consumed-capacity
    * /components/component/integrated-circuit/backplane-facing-capacity/state/total”
