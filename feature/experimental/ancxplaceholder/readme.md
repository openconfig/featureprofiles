# RT-1.35: ancx placeholder test

## Summary
This is a placeholder test to cover for all anCX OCpaths that are not already covered in any FNT.


### Config paths

```yaml
paths:
/components/component/integrated-circuit/utilization/resources/resource/config/name
/components/component/linecard/utilization/resources/resource/config/name
/components/component/transceiver/physical-channels/channel/config/index

/network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/config/name
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/config/extended-next-hop-encoding
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit-received/config/max-prefixes
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit-received/config/warning-threshold-pct
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/config/max-prefixes
/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/config/warning-threshold-pct
/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv6-unicast/prefix-limit/config/max-prefixes
/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv6-unicast/prefix-limit/config/warning-threshold-pct
/network-instances/network-instance/protocols/protocol/config/enabled
/network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/config/metric
/network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/multi-topology/config/afi-name
/network-instances/network-instance/protocols/protocol/isis/global/afi-safi/af/multi-topology/config/safi-name
/network-instances/network-instance/protocols/protocol/isis/global/config/max-ecmp-paths
/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/authentication/config/auth-mode
/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/authentication/config/auth-password
/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/authentication/config/auth-type
/network-instances/network-instance/protocols/protocol/isis/interfaces/interface/config/hello-padding
/network-instances/network-instance/protocols/protocol/isis/levels/level/traffic-engineering/config/enabled
/network-instances/network-instance/protocols/protocol/isis/levels/level/traffic-engineering/config/ipv4-router-id
/network-instances/network-instance/segment-routing/srgbs/srgb/config/dataplane-type
/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit-received/config/prevent-teardown
/network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit-received/config/warning-threshold-pct

/system/aaa/accounting/config/accounting-method
/system/aaa/accounting/events/event/config/event-type
/system/aaa/accounting/events/event/config/record
/system/aaa/authentication/config/authentication-method
/system/aaa/authentication/users/user/config/password-hashed
/system/aaa/authentication/users/user/config/role
/system/aaa/authorization/config/authorization-method
/system/aaa/authorization/events/event/config/event-type
/system/aaa/server-groups/server-group/config/name
/system/aaa/server-groups/server-group/config/type
/system/aaa/server-groups/server-group/servers/server/config/address
/system/aaa/server-groups/server-group/servers/server/config/timeout
/system/aaa/server-groups/server-group/servers/server/tacacs/config/port
/system/aaa/server-groups/server-group/servers/server/tacacs/config/secret-key-hashed
/system/grpc-servers/grpc-server/config/certificate-id
/system/grpc-servers/grpc-server/config/metadata-authentication
/system/grpc-servers/grpc-server/config/network-instance
/system/grpc-servers/grpc-server/config/transport-security
/system/dns/servers/server/config/address
/system/logging/console/selectors/selector/config/facility
/system/logging/console/selectors/selector/config/severity
/system/ssh-server/config/timeout
/system/state/last-configuration-timestamp

/routing-policy/policy-definitions/policy-definition/statements/statement/conditions/match-protocol-instance/config/protocol-identifier

/interfaces/interface/tunnel/ipv4/config/mtu
/interfaces/interface/tunnel/config/dst


/components/component/state/allocated-power
 /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/in-high-priority
 /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/in-low-priority
 /components/component/integrated-circuit/pipeline-counters/drop/fabric-block/state/out-low-priority
 /components/component/integrated-circuit/pipeline-counters/errors/fabric-block/fabric-block-error
 /components/component/properties/property[fpd-status]/state/value
 /components/component/port/optical-port/state/admin-state
 /components/component/cpu/utilization/state/avg
 /components/component/cpu/utilization/state/instant
 /components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/dropped-aggregate
 /components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/dropped-bytes-aggregate
 /components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/queued-aggregate
 /components/component/integrated-circuit/pipeline-counters/control-plane-traffic/state/queued-bytes-aggregate
 /components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor
 /components/component/integrated-circuit/pipeline-counters/drop/state/adverse-aggregate
 /components/component/integrated-circuit/pipeline-counters/drop/state/congestion-aggregate
 /components/component/integrated-circuit/pipeline-counters/drop/state/no-route
 /components/component/integrated-circuit/pipeline-counters/drop/state/urpf-aggregate
 /components/component/integrated-circuit/pipeline-counters/drop/vendor
 /components/component/integrated-circuit/utilization/resources/resource/state/committed
 /components/component/integrated-circuit/utilization/resources/resource/state/high-watermark
 /components/component/integrated-circuit/utilization/resources/resource/state/last-high-watermark
 /components/component/integrated-circuit/utilization/resources/resource/state/max-limit
 /components/component/integrated-circuit/utilization/resources/resource/state/used-threshold-upper-exceeded
 /components/component/linecard/state/power-admin-state
 /components/component/linecard/state/slot-id
 /components/component/linecard/utilization/resources/resource/state/committed
 /components/component/linecard/utilization/resources/resource/state/free
 /components/component/linecard/utilization/resources/resource/state/high-watermark
 /components/component/linecard/utilization/resources/resource/state/last-high-watermark
 /components/component/linecard/utilization/resources/resource/state/max-limit
 /components/component/linecard/utilization/resources/resource/state/name
 /components/component/linecard/utilization/resources/resource/state/used
 /components/component/power-supply/state/capacity
 /components/component/power-supply/state/input-voltage
 /components/component/power-supply/state/output-power
 /components/component/state/base-mac-address
 /components/component/state/memory/available
 /components/component/state/memory/available
 /components/component/state/memory/available
 /components/component/state/memory/utilized
 /components/component/state/memory/utilized
 /components/component/state/memory/utilized
 /components/component/transceiver/physical-channels/channel/state/laser-temperature/instant
 /components/component/integrated-circuit/utilization/resources/resource[TCAM]/state/free
 /components/component/integrated-circuit/utilization/resources/resource[TCAM]/state/used
 /components/component/state/memory/utilized
 /components/component/state/memory/available
 /components/component/integrated-circuit/pipeline-counters/drop/vendor/arista/sand/packet-processor/state/ingress-null-route
 /components/component/integrated-circuit/pipeline-counters/drop/vendor/arista/sand/packet-processor/state/ingress-port-not-vlan-member
 /components/component/integrated-circuit/pipeline-counters/drop/vendor/arista/sand/packet-processor/state/ingress-ipv4-checksum-error
 /components/component/integrated-circuit/pipeline-counters/drop/vendor/arista/sand/packet-processor/state/ingress-ipv6-multicast-source
 /components/component/integrated-circuit/memory/state/uncorrected-parity-errors
 
 /interfaces/interface/subinterfaces/subinterface/state/oper-status
 /interfaces/interface/subinterfaces/subinterface/state/admin-status
 /interfaces/interface/subinterfaces/subinterface/state/counters/in-broadcast-pkts
 /interfaces/interface/subinterfaces/subinterface/state/counters/in-octets
 /interfaces/interface/subinterfaces/subinterface/state/counters/in-unicast-pkts
 /interfaces/interface/subinterfaces/subinterface/state/counters/out-broadcast-pkts
 /interfaces/interface/subinterfaces/subinterface/state/counters/out-multicast-pkts
 /interfaces/interface/subinterfaces/subinterface/state/counters/out-octets
 /interfaces/interface/subinterfaces/subinterface/state/counters/out-unicast-pkts
 /interfaces/interface/ethernet/state/counters/in-oversize-frames
 /interfaces/interface/subinterfaces/subinterface/state/ifindex
 /interfaces/interface/subinterfaces/subinterface/state/name
 /interfaces/interface/state/ifindex
 
 /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/warning-threshold-pct
 /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/ipv6-unicast/prefix-limit/state/warning-threshold-pct
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit-received/state/max-prefixes
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit-received/state/prefix-limit-exceeded
 /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/prefix
 /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/entry-metadata
 /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/next-hop-group-network-instance
 /network-instances/network-instance/afts/ipv4-unicast/ipv4-entry/state/origin-network-instance
 /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/prefix
 /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/origin-network-instance
 /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/next-hop-group-network-instance
 /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/decapsulate-header
 /network-instances/network-instance/afts/ipv6-unicast/ipv6-entry/state/entry-metadata
 /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/packets-forwarded
 /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/next-hop-group
 /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/mpls-label
 /network-instances/network-instance/afts/policy-forwarding/policy-forwarding-entry/state/counters/octets-forwarded
 /network-instances/network-instance/afts/mpls/label-entry
 /network-instances/network-instance/mpls/signaling-protocols/rsvp-te/sessions/session/state/session-name
 /network-instances/network-instance/mpls/signaling-protocols/rsvp-te/sessions/session/state/type
 /network-instances/network-instance/mpls/signaling-protocols/rsvp-te/sessions/session/record-route-objects
 /network-instances/network-instance/route-limits/route-limit/state/installed-routes
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit-received/state/prefix-limit-exceeded
 /network-instances/network-instance/afts/aft-summaries/ipv4-unicast/protocols/protocol/origin-protocol
 /network-instances/network-instance/afts/aft-summaries/ipv4-unicast/protocols/protocol/state/origin-protocol
 /network-instances/network-instance/afts/aft-summaries/ipv6-unicast/protocols/protocol/origin-protocol
 /network-instances/network-instance/afts/aft-summaries/ipv6-unicast/protocols/protocol/state/origin-protocol
 /network-instances/network-instance/mpls/lsps/static-lsps/static-lsp/name
 /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/afi-safi-name
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit-received/state/max-prefixes
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit-received/state/prefix-limit-exceeded
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit-received/state/warning-threshold-pct
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/prefix-limit-exceeded
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv6-unicast/prefix-limit/state/prefix-limit-exceeded
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/last-prefix-limit-exceeded
 /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/messages/sent/last-notification-time
 /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/afi-safi-name
 /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/prefix-limit-exceeded
 /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv6-unicast/prefix-limit/state/max-prefixes
 /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv6-unicast/prefix-limit/state/prefix-limit-exceeded
 /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/peer-group-name
 /network-instances/network-instance/protocols/protocol/identifier
 /network-instances/network-instance/protocols/protocol/name

 /acl/interfaces/interface/id
 /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/set-name
 /acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/type
 
 /routing-policy/defined-sets/bgp-defined-sets/community-sets/community-set/community-set-name
 /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set/state/community-set
 /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-community-set/state/match-set-options
 /routing-policy/policy-definitions/policy-definition/statements/statement/conditions/bgp-conditions/match-ext-community-set/state/ext-community-set
 
 /sampling/sflow/interfaces/interface/name
 
 /system/aaa/accounting/events/event/event-type
 /system/aaa/authentication/users/user/username
 /system/aaa/authorization/events/event/event-type
 /system/aaa/server-groups/server-group/name
 /system/aaa/server-groups/server-group/servers/server/address
 /system/grpc-servers/grpc-server/state/certificate-id
 /system/grpc-servers/grpc-server/state/metadata-authentication
 /system/grpc-servers/grpc-server/state/certificate-version
 /system/grpc-servers/grpc-server/state/certificate-created-on
 /system/grpc-servers/grpc-server/state/ca-trust-bundle-version
 /system/grpc-servers/grpc-server/state/ca-trust-bundle-created-on
 /system/grpc-servers/grpc-server/state/certificate-revocation-list-bundle-version
 /system/grpc-servers/grpc-server/state/certificate-revocation-list-bundle-created-on
 /system/grpc-servers/grpc-server/state/ssl-profile-id
 /system/grpc-servers/grpc-server/state/listen-addresses
 /system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path/state/reads/access-rejects
 /system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path/state/reads/last-access-reject
 /system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path/state/writes/access-rejects
 /system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path/state/writes/last-access-reject
 /system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path/state/reads/access-accepts
 /system/grpc-servers/grpc-server/gnmi-pathz-policy-counters/paths/path/state/writes/access-accepts
 /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/name
 /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-rejects
 /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-reject
 /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/access-accepts
 /system/grpc-servers/grpc-server/authz-policy-counters/rpcs/rpc/state/last-access-accept
 /system/logging/console/selectors/selector/state/facility
 /system/logging/console/selectors/selector/state/severity
 /system/logging/remote-servers/remote-server/host
 /system/logging/remote-servers/remote-server/selectors/selector/facility
 /system/logging/remote-servers/remote-server/selectors/selector/severity
 /system/mount-points/mount-point/state/name
 /system/mount-points/mount-point/state/storage-component
 /system/mount-points/mount-point/state/size
 /system/mount-points/mount-point/state/available
 /system/processes/process/state/start-time


rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
```
