# CFM-1.1: CFM over ETHoCWoMPLSoGRE

## Summary

This test verifies CFM "DOWN MEP" can be established over "EthoCWoMPLSoGRE" dataplane.
## Testbed type

*  [`featureprofiles/topologies/atedut_8.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_8.testbed)

## Procedure

### Test environment setup

```text
DUT has an ingress and 2 egress aggregate interfaces.
                         
    [ ATE Port 1 ]----| Port1  :DUT1:  Port2 | ---- |Port2 :DUT2: Port1 | ---- |ATE Port 2 |  
```

Test uses aggregate 802.3ad bundled interfaces (Aggregate Interfaces).

* Ingress Port: nTraffic is generated from Aggregate1 (ATE Ports 1).

* Egress Ports: Aggregate2 (ATE Port 2) is used as the destination ports for encapsulated traffic.

* Transit Ports: Aggregate3 (DUT1 port 2 and DUT2 Port 2)

### PF-1.11.1: Generate DUT Configuration

Aggregate 1 and Aggregate2 i.e "customer interface's" are the  customer facing ports that could either have port mode configuration or attachment mode configuration as described below. 

EACH test should be run twice - once with port mode configuration and once with attachment mode configuration.

#### Aggregate 1 "customer interface" Port mode configuration

* Configure DUT1 port 1 to be a member of aggregate interface named "customer interface"
* "customer interface" is a static Layer 2 bundled interface part of pseudowire that accepts packets from all VLANs.
* MTU default 9216

#### Aggregate 1 "customer interface" Attachment mode configuration

* Configure DUT1 port 1 to be a member of aggregate interface named "customer interface"
* Create a sub interface of the aggregated interface and assign a VLAN ID to it. 
* This sub interface will be a static Layer 2 bundled interface part of pseudowire that accepts packets from vlan ID associated with it. 
* MTU default 9216

#### Aggregate 2 "customer interface" Port mode configuration

* Configure DUT2 port 1 to be a member of aggregate interface named "customer interface"
* "customer interface" is a static Layer 2 bundled interface part of pseudowire that accepts packets from all VLANs.
* MTU default 9216

#### Aggregate 2 "customer interface" Attachment mode configuration

* Configure DUT2 port 1 to be a member of aggregate interface named "customer interface"
* Create a sub interface of the aggregated interface and assign a VLAN ID to it. 
* This sub interface will be a static Layer 2 bundled interface part of pseudowire that accepts packets from vlan ID associated with it. 
* MTU default 9216

#### Policy Forwarding Configuration 

* Policy-forwarding enabling EthoMPLSoGRE encapsulation of all incoming traffic:

  * The forwarding policy must allow forwarding of incoming traffic across 16 tunnels. 16 Tunnels has 16 source address and a single tunnel destination.

  * Source address must be configurable as:
    * Loopback address OR
    * 16 source addresses corresponding to a single tunnel destinations to achieve maximum entropy.

  * DSCP of the innermost IP packet header must be preserved during encapsulation

  * DSCP of the GRE/outermost IP header must be configurable (Default TOS 96) during encapsulation

  * TTL of the outer GRE must be configurable (Default TTL 64)

  * QoS Hardware queues for all traffic must be configurable (default QoS hardaware class selected is 3)

### Pseudowire (PW) Configuration 

* "Customer interface" is Aggregate 1 and Aggregate 2  pointing towards  Aggregare3
* Two unique static MPLS label for local label and remote label. 
* Enable control word

### Aggregate 3 configuration

* IPV4 and IPV6 addresses

* MTU (default 9216)

* LACP Member link configuration

* Lag id

* LACP (default: period short)

* Carrier-delay (default up:3000 down:150)

* Statistics load interval (default:30 seconds)

### Routing

* Create static route for tunnel destination pointing towards Aggregate 2. 
* Static mapping of MPLS label for encapsulation must be configurable

### MPLS Label

* Entire Label block must be reallocated for static MPLS
* Labels from start/end/mid ranges must be usable and configured corresponding to EthoMPLSoGRE encapsulation


### CFM 

CFM is cnfigured as UP MEP. The control plane is between the customer attachments on either PF.
