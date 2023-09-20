# Summary
BGP TCP MSS and PMTUD
# Priority
P0
# Procedure
## Sub test 1
* Establish BGP sessions between:
* ATE port-1 and DUT-1 (eBGP - ipv4)
* ATE port-1 and DUT-1 (eBGP - ipv6)
* Verify that the TCP MSS value is set to 536 bytes for the IPv4 session and 1240 bytes for the IPv6 session.
## Sub test 2
* Change the Interface MTU to the ATE port as 5040.
* Configure IP TCP MSS value of 5000 bytes on the interface to the ATE port.
* Re-establish the BGP sessions by tcp reset.
* Verify that the TCP MSS value is set to 5000 bytes for IPv4 and IPv6.
## Sub test 3
* Establish iBGP session with MD5 enabled from ATE port-1 to DUT-2 . DUT-2 is directly connected to DUT-1.
* Change the MTU on the DUT-1 to DUT-2 link to 512 bytes and enable PMTUD on the DUT
* Validate that the min MSS value has been adjusted to be below 512 bytes  on the tcp session.

# Config Parameter coverage
```
neighbors/neighbor/transport/config/tcp-mss
/neighbors/neighbor/transport/config/mtu-discovery
```

# Telemetry Parameter coverage
```
/neighbors/neighbor/transport/state/tcp-mss
/neighbors/neighbor/transport/state/mtu-discovery
Protocol/RPC Parameter coverage
```

# Minimum DUT platform requirement
vRX
