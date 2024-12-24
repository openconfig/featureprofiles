# CNTR-2: Container network connectivity tests

Tests within this directory ensure that a container deployed on a network
device is able to connect to external services via gRPC.

## CNTR-2.1: Connect to container from external client.

Deploy a container to a DUT that is listening on `[::]:60061`. Validate that the
test can connect to tcp/60061 via gRPC and receive a response on a simple
"dummy" service.

## CNTR-2.2: Connect to locally running service.

For a DUT configured with gNMI running on tcp/9339 (IANA standard), and gRIBI
running on tcp/9340 (IANA standard), the test should:

*   Instruct the container to make a gRPC `Dial` call to the running gNMI
    instance, with a specified timeout. The test succeeds if the connection
    succeeds within the timeout, otherwise it fails.
*   Instruct the container to make a gRPC `Dial` call to the running gRIBI
    instance with the same pass/fail logic.

## CNTR-2.3: Connect to a remote node.

Deploy two DUTs running in the following configuration:

```
  [  c1   ]                 [  c2   ]
  ---------                 --------
  [ DUT 1 ] ---- port1 ---- [ DUT 2 ]
```

where c1 is an instance of the "listener" container image, and c2 is an instance
of the "dialer" image.

The test should: * ensure that c1 is listening on `[::]:60071` running a gRPC
service. * use gNMI to configure and/or discover the link local addresses
configured on DUT1 port 1 and DUT2 port1. * instruct c2 to make a dial call and
isue a simple RPC to the address configured by c1. If the dial call succeeds
within a specified timeout, the test passes, otherwise it fails.

## CNTR-2.4: Connect to another container on a local node

Deploy a single DUT with two containers C1 and C2 running on them. C1 should
listen on a gRPC service on `tcp/[::]:60061` and C2 should listen on a gRPC
service on `tcp/[::]60062`.

*   Instruct C1 to make a gRPC dial call to C2's listen port with a specified
    timeout, ensure that an RPC response is received.
*   Instruct C2 to make a gRPC dial call to C2's listen port with a specified
    timeout, ensure that an RPC response is received.

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
