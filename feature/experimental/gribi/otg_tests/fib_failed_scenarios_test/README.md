# TE-9.2: FIB FAILURE DUE TO HARDWARE RESOURCE EXHAUST

## Summary

Validate gRIBI FIB_FAILED functionality.

## Topology

ATE port-1 <------> port-1 DUT
DUT port-2 <------> port-2 ATE

## Procedure

*   Connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2.

*   Establish a gRIBI connection (SINGLE_PRIMARY and PRESERVE mode) to the DUT.

*   Establish BGP session between ATE Port1 --- DUT Port1. Inject unique BGP routes to exhaust FIB on DUT.

#### Test-1, Add an IPEntry. The referenced NHG is viable.

1. Continuously injecting the following gRIB structure until FIB FAILED is received. 
Each DstIP and VIP should be unique and of /32. All the NHG and NH should be unique (of unique ID).
DstIP/32 -> NHG -> NH {next-hop:} -> VIP/32 -> NHG -> NH {next-hop: AtePort2Ip}

2. Expect FIB_PROGRAMMED message until the first FIB_FAILED message received (Resource exhaustion).

3. Validate that traffic for the FIB_FAILED route will not get forwarded.

4. Pick any route that received FIB_PROGRAMMED. Validate that traffic hitting the route should be forwarded to port2.

#### Test-2, IPEntry update (Moving from NHG1 to NHG2)

1. Add an IPEntry and point to NHG1.

2. Update IPEntry which point to NHG2.

3. Expect FIB_FAILED message.

#### Test-3, Add NHG.

1. Continuously injecting the gRIB NHG until FIB FAILED is received.

#### Test-4, NH referencing to a down port.

1. Add an IPEntry which point to NHG1 --> NH1 (NH reference to down port)

2. Verify the FIB_PROGRAMMED message received.

#### Test-5, IPEntry add fails due to NHG not being programmed.

1. Add a new IPEntry which reference to a new NHG and NHG is not being programmed.

2. IPEntry is programmed before NHG Program.

3. Expect FIB_FAILED message

#### Test-6, Route referencing NHG1 where NHG1 does not exist

1. Add an IPEntry which referencing to NHG and NHG is not programmed.

2. Expect a FIB_FAILED message

#### Test-7, In-place NHG update.

1. Modify/Update NHG with NH entry or Weight of NHG.

2. Expect FIB_FAILED message

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gribi:
    gRIBI.Get:
    gRIBI.Modify:
    gRIBI.Flush:
```

## Config parameter coverage

## Telemery parameter coverage
