# ACL-1.2: ACL Update (Make-before-break)

## Summary

Configure an IP ACL, then test changing the ACL configuration to ensure a make-before-break behavior is performed.  Make before break for ACL is defined as

## ACL-1 Layer 3 terms

* IP src
* IP dst
* TCP src port
* TCP src port range
* TCP dst port
* TCP dst port range
* UDP src port
* UDP src port range
* ICMP proto
* ICMP type

* IPv4 initial fragment
* IPv4 non-initial fragment
* IPv6 fragmentation (1st next-header)
* MatchAll

## Procedure

### Sub Test 1

* Configure DUT with input and output interfaces and static routing.
* Configure IPv4 and IPv6 ACLs with terms specified in the table.
  * All terms should have Deny action.
  * “Match all” term should have Accept and Count actions.
* Apply these ACLs in ingress direction on the DUT input interface.
* Start IP traffic flows matching these terms.
* Verify received packets and ACL term counters on DUT.

### Sub Test 2

* Inverse filtering logic: permit traffic on all terms, deny traffic on MatchAll terms.
* Perform ACL update by adding a single matching condition to all terms (additional address or port).
* Verify that the device is running an updated ACL version.
  * No config error
  * No difference between committed ACL and intended config ACL
* Verify traffic drops for sent flows on ATE ingress interface (no more than 50ms of traffic should be dropped).

### Sub test 3

* Repeat the same test by moving ACLs to the DUT egress interface.

## Config Parameter coverage

```
acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/destination-address
acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/protocol
acl/acl-sets/acl-set/acl-entries/acl-entry/ipv4/config/source-address

acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/destination-address
acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/protocol
acl/acl-sets/acl-set/acl-entries/acl-entry/ipv6/config/source-address

acl/interfaces/interface/ingress-acl-sets/ingress-acl-set
acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/acl-entries
acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/acl-entries/acl-entry

acl/interfaces/interface/egress-acl-sets/egress-acl-set
acl/interfaces/interface/egress-acl-sets/egress-acl-set/acl-entries
acl/interfaces/interface/egress-acl-sets/egress-acl-set/acl-entries/acl-entry
```

## Telemetry Parameter coverage

```
acl/interfaces/interface/ingress-acl-sets/ingress-acl-set/acl-entries/acl-entry/state/matched-packets
acl/interfaces/interface/egress-acl-sets/egress-acl-set/acl-entries/acl-entry/state/matched-packets
```

## Protocol/RPC Parameter coverage

None

## Minimum DUT platform requirement

MFF
