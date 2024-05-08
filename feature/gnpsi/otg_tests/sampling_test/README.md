# gNPSI-1: Sampling and Subscription Check

## Summary

The goal is to validate that packet sampling is working as expected, clients can connect to the gNPSI service and receive samples.  

## Procedure


### Test 1: Verify samples are exported via gNPSI service. 

1. Enable sFlow and gNPSI on the DUT.

*   Set sample source address, 
*   Varying sample sizes (256, 512, 1024)
*   Varying sampling rates (1:32k, 1:1M, 1:4M).
*   DSCP=32

2. Send Traffic via OTG with different traffic profiles. 

*   Ipv4 and Ipv6
*   Varying packet sizes (64, 512, 1500)

3. Start the gRPC client and subscribe to the gRPC service on the DUT.

4. Verify the samples received by the client are as per expectations:

*   Samples are formatted as per the sFLOW datagram specifications.
*   Appropriate number of samples are received based on the sampling raste. e.g. ~1 in 1M samples is received for a sampling rate of 1:1M. 
*   Samples have DSCP=32.
*   Datagram contents are set correctly. 
    *   Sampling rate is correct
    *   Ingress and egress interfaces are correct
    *   Inner packets can be parsed correctly. 

### Test 2: Verify multiple clients can connect to the gNPSI Service and receive samples. 

1. Enable sFlow and gNPSI on the DUT.

*   Set sample source address, sample size 256 bytes, 1:1M sampling rate and DSCP=32

2. Send Traffic via OTG.

3. Start N gRPC clients(N &lt; max number of clients allowed) and subscribe to the gRPC service on the DUT.

4. Verify each gRPC client receives ~1 sample for every 1M packet through the DUT. 


### Test 3: Verify gNPSI service can deal with max number of clients 

1. Enable sFlow and gNPSI on the DUT.

2. Send Traffic via OTG.

3. Configure max number of clients and subscribe to the gRPC service on the DUT.

4. Start one more client and subscribe to gRPC service. Expect this to fail.

5. Cancel one connected client. 

6. Start one more client and subscribe to gRPC service. Expect this to succeed.


### Test 4: Verify client reconnection to the gNPSI service. 

1. Enable sFlow and gNPSI on the DUT.

*   Set sample source address, sample size 256 bytes, 1:1M sampling rate and DSCP=32 

2. Send Traffic via OTG.

3. Start a gRPC client and subscribe to the gRPC service on the DUT, and verify the connection is healthy and samples are received.

4. Disconnect and reconnect the client, and verifying the reconnection is successful, and samples are received.


### Test 5: Verify client connection after gNPSI service restart.

1. Enable sFlow and gNPSI on the DUT.

*   Set sample source address, sample size 256 bytes, 1:1M sampling rate and DSCP=32 

2. Send Traffic via OTG.

3. Start a gRPC client and subscribe to the gRPC service on the DUT, and verify the connection is healthy and samples are received.

4. Restart the gNPSI service (This can be done by a switch reboot).

5. Let the gRPC client try to reconnect to gNPSI service every few seconds. The gRPC client should successfully connect to gNPSI service after gNPSI service is up.

6. Verify that the samples are received.


## Config Parameter coverage

N/A


## Telemetry Parameter coverage

N/A

## Protocol/RPC Parameter coverage

*   gNPSI:
    *   Subscribe
        *   SubscribeRequest


## Minimum DUT platform requirement

N/A
