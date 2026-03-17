# IPSEC-1.2: IPSec Scaling with MACSec over aggregated links.

## Summary

This test verifies the IPSec tunneling scale between a pair of devices. A pair of DUTs establish an IPsec tunnel. Traffic on ingress to the DUT is then encrypted and forwarded over the tunnel to the egress DUT, which then decrypts the packets and forwards to the final destination.

## Testbed Type

The ate-dut testbed configuration would be used, as described below.

*  [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

TODO: when OTG API supports IPSec, refactor the topology to be: `atedut8` where the ATE serves as the endpoints of the ipsec tunnel


## Procedure

### Test Environment Setup

See IPSEC-1.1 for the test environment setup.

### IPSEC-1.2.1: Verify IPv4 Connectivity over a Max \# of Tunnels for Single Attachment

Change in setup:

* Set up a maximum number of parallel tunnels (such as 128 or 256, expected to be based on the device expected limit between ECMP max and tunnel count max and would be the max limit that is expected in production for a single attachment for this hardware) between the pair of DUTs, each pair over a new pair of new loopback interfaces. All the new tunnels would remain in the same VRF as the existing tunnel, with customer traffic (in a single VRF) ECMP’ng in/out of these tunnels.

Generate traffic on ATE-\>DUT1 Ports having a random combination of 1000 source addresses to ATE-2 destination address(es) at line rate IPv4 traffic. Use MTU-RANGE frame size.

Verify:

* All traffic received from ATE (other than any local traffic) gets forwarded as ipsec
* No packet loss
* Traffic equally load balanced across DUT \<\> DUT ports.
* Traffic equally load balanced across the tunnels

### IPSEC-1.2.2: Verify IPv6 Connectivity over a Max \# of Tunnels for Single Attachment

Same setup as the IPv4-Max-Attachment-Tunnel test.

Same traffic generation as the IPv4-Max-Attachment-Tunnel, except with IPv6 traffic.

Verify:

* Same validation steps as the IPv4-Max-Attachment-Tunnel.

### IPSEC-1.2.3: Verify IPv4 Connectivity over Device with Max # of Tunnels

Change in setup:

* Set up a maximum number of parallel tunnels for a single attachment (as described above).
* If device can support additional tunnels - Set up additional attachments (vlans) on the DUT-ATE interfaces, each with a different VLAN & VRF, with the ipsec setup.
    * For each additional attachment, set up the maximum number of parallel tunnels for a single attachment, up to limit of max tunnels for device
    * Repeat until max tunnels for device are configured

Generate traffic on ATE-\>DUT1 Ports **for every attachment**, having a random combination of 1000 source addresses to ATE-2 destination address(es) at line rate IPv4 traffic. Use MTU-RANGE bytes frame size.

Verify:

* All traffic received from ATE (other than any local traffic) gets forwarded as ipsec
* No packet loss
* Traffic equally load balanced across DUT \<\> DUT ports.
* Traffic equally load balanced across the tunnels

### IPSEC-1.2.4: Verify IPv6 Connectivity over Device with Max # of Tunnels

* Set up a maximum number of parallel tunnels for a single attachment (as described above).
* If device can support additional tunnels - Set up additional attachments (vlans) on the DUT-ATE interfaces, each with a different VLAN & VRF, with the ipsec setup.
    * For each additional attachment, set up the maximum number of parallel tunnels for a single attachment, up to limit of max tunnels for device
    * Repeat until max tunnels for device are configured

Generate traffic on ATE-\>DUT1 Ports **for every attachment**, having a random combination of 1000 source addresses to ATE-2 destination address(es) at line rate IPv6 traffic. Use MTU-RANGE bytes frame size.

Verify:

* Same validation steps as the IPv4-Max-Device-Tunnel.

### Canonical OC  

```json
{
}
```

## OpenConfig Path and RPC Coverage

```yaml
paths:
rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  ```

## Required DUT platform

FFF
