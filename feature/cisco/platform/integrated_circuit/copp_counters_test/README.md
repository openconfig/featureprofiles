# copp_counters: Control Plane Policing Counters GNMI Test

## Summary

Tests path resolution and accuracy of control plane counters over GNMI

## Procedure

*   Establish GNMI client

*   Compile pathlist based on topology

*   Test CoPP counters Per NPU over GNMI

*   Check Aggregate counter path and compare against manual aggregation of counter results over GNMI

*   Introduce control plane traffic

*   Compare CLI counter with GNMI counter

## Trap Counter Paths

openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/arp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/arp-bcast-bvi
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/slow-proto-lacp-synce-eoam
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/cisco-protocols-cdp-vtp-dtp-pagp-udld
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l3-isis-drain
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/isis-l3
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ptp-over-ethernet
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l2-dhcp-server-snoop
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l2-dhcp-client-snoop
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/dhcpv4-server
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/dhcpv4-client
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l3-ip-multicast-rpf
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l3-ip-mc-egress-punt
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l3-ip-mc-punt-rpf-fail
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l3-ip-mc-s-g-punt-member
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l3-ip-mc-g-punt-member
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/macsec
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/cfm-l2-ac
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/cfm-mcast
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/cfm-meg-id-no-match
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l2mc-igmp-punt
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l2mc-mld-punt
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/l2mc-mirror-trap
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/lldp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/lldp-snoop
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/pfc
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/mstp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/macsec-fips-post
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/gdp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/online-diag
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/mldp-ingress-punt

## LPTS Counter Paths

openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/fragment
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ospf-mc-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ospf-mc-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ospf-uc-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ospf-uc-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/bfd-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/bfd-mp-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/bgp-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/bgp-cfg-peer
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/bgp-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/pim-mcast-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/pim-mcast-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/pim-ucast
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/igmp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/icmp-local
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/icmp-control
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/icmp-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ldp-tcp-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ldp-tcp-cfg-peer
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ldp-tcp-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ldp-udp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/all-routers
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/rsvp-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/rsvp-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/snmp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ssh-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ssh-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/http-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/shttp-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/telnet-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/telnet-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/udp-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/udp-listen
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/udp-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/tcp-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/tcp-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/raw-default
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ip-sla
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/gre
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/vrrp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/hsrp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/mpls-oam
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/dns
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/ntp-known
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/dhcpv4
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/dhcpv6
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/tpa
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/pm-twamp
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/tpa-appmgr-high
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/tpa-appmgr-med
openconfig:components/component/integrated-circuit/pipeline-counters/control-plane-traffic/vendor/cisco/c8000/state/tpa-appmgr-low
