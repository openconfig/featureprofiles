# gNPSI-1: Sampling and Subscription Check

## Summary

The goal is to validate that packet sampling is working as expected, clients can connect to the gNPSI service and receive samples.  

## Procedure

### Common Test Setup
  * Configure DUT with two ports with IPv4/IPv6 addresses
  * Configure sFlow and gNPSI on DUT with following parameters:
    * Sample size = 256 bytes
    * Sampling rate is 1:1M
  * Configure OTG traffic with different traffic profiles.
    * IPv4 and Ipv6
    * Varying packet sizes (64, 512, 1500)
  * Start OTG traffic

TODO: Add gNPSI client support to OTG. 

### gNPSI 1.1: Validate DUT configuration of gNPSI server, connect OTG client and verify samples. 

* Start the gRPC client and subscribe to the gNPSI service on the DUT.

* Verify the samples received by the client are as per expectations:
  * Samples are formatted as per the sFLOW datagram specifications.
  * Appropriate number of samples are received based on the sampling raste. e.g. ~1 in 1M samples is received for a sampling rate of 1:1M. 
  * Datagram contents are set correctly. 
    * Sampling rate is correct
    * Ingress and egress interfaces are correct
    * Inner packets can be parsed correctly

### gNPSI 1.2: Verify multiple clients can connect to the gNPSI Service and receive samples. 

1. Start 2 gRPC clients and subscribe to the gNPSI service on the DUT.

2. Verify each client receives ~1 sample for every 1M packet through the DUT. 


### gNPSI 1.3: Verify client reconnection to the gNPSI service. 

* Start a gRPC client and subscribe to the gRPC service on the DUT, and verify the connection is healthy and samples are received.

* Disconnect and reconnect the client, and verifying the reconnection is successful, and samples are received.


### gNPSI 1.4: Verify client connection after gNPSI service restart.

* Start a gRPC client and subscribe to the gRPC service on the DUT, and verify the connection is healthy and samples are received.

* Restart the gNPSI service (This can be done by a switch reboot).

* Let the gRPC client try to reconnect to gNPSI service every few seconds. The gRPC client should successfully connect to gNPSI service after gNPSI service is up.

* Verify that the samples are received.


## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnpsi:
    gNPSI.Subscribe:
```

## Minimum DUT platform requirement

N/A
