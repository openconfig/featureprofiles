# RT-1.21: BGP TCP MSS and PMTUD

## Summary
Test for BGP TCP MSS and PMTUD


## Topology
DUT:port-1 <------> port-1:ATE <-----> Fake router behind ATE

## Test Setup
* Between DUT:port1 and ATE:port1 configure IPv4_prefix1/31 and IPv6_prefix1/127
* Ensure that the DUT:port1 and ATE:port1 are configured for MTU of 9202B
* Ensure that the "fake router behind ATE" and the ATE also use IPv4_prefix2/31 and IPv6_prefix2/127
* We should let the MTU between ATE and "Fake router" be default i.e. 1514B

### Test procedure
**Test-1**: Verify EBGP establishment w/ MTU change and confirm negotiated TCP MSS
* Start DUT:port1 and ATE:port1 connection w/ default MTU. Verify,
  * EBGP established on ipv4-unicast and ipv6-unicast afi-safi
  * TCP MSS value is 
