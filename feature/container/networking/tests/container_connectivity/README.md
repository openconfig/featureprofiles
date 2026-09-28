# CNTR-2: Container network connectivity tests

Tests within this directory ensure that a container deployed on a network
device is able to connect to external services via gRPC.

## Procedure

* Build the test and upgrade container as described below
* Pass the tarballs of the container the test as arguments.

### Build Test Container

The test container is available in the feature profile repository under
`internal/cntrsrv`.

Start by entering in that directory and running the following commands:

```shell
$ cd internal/cntrsrv
$ go mod vendor
$ CGO_ENABLED=0 go build .
$ docker build -f build/Dockerfile.local -t cntrsrv_image:latest .
```

At this point you will have a container image build for the test container.

```shell
$ docker images
REPOSITORY        TAG            IMAGE ID       CREATED         SIZE
cntrsrv_image     latest         8d786a6eebc8   3 minutes ago   21.4MB
```

Now export the container to a tarball.

```shell
$ docker save -o /tmp/cntrsrv.tar cntrsrv_image:latest
```

This is the tarball that will be used during tests.

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

## Canonical OC

<!-- This test does not require any specific OpenConfig configuration, so this section is empty to satisfy the validator. -->
```json
{}
```

## OpenConfig Path and RPC Coverage
```yaml
paths:
  /interfaces/interface/config/enabled:
  /interfaces/interface/config/name:
  /interfaces/interface/config/type:
  /interfaces/interface/subinterfaces/subinterface/ipv4/config/enabled:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/config/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/ip:
  /interfaces/interface/subinterfaces/subinterface/ipv6/addresses/address/state/prefix-length:
  /interfaces/interface/subinterfaces/subinterface/ipv6/config/enabled:
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    containerz.Containerz.StartContainer:
  gribi:
    gRIBI.Get:
```
