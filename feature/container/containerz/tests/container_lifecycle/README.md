# CNTR-1: Basic container lifecycle via `gnoi.Containerz`.

## Summary

Verify the correct behaviour of `gNOI.Containerz` when operating containers.

## Procedure

This step only applies if the reference implementation of containerz is being
tested.

Start by pulling the reference implementation:

```shell
$ git clone git@github.com:openconfig/containerz.git
```

Then `cd` into the containerz directory and build containerz:

```shell
$ cd containerz
$ go build .
```

Finally start containerz:

```shell
$ ./containerz start
```

You should see the following output:

```shell
$ ./containerz start
I0620 12:02:57.408496 3615908 janitor.go:33] janitor-starting
I0620 12:02:57.408547 3615908 janitor.go:36] janitor-started
I0620 12:02:57.408583 3615908 server.go:167] server-start
I0620 12:02:57.408595 3615908 server.go:170] Starting up on Containerz server, listening on: [::]:19999
I0620 12:02:57.408608 3615908 server.go:171] server-ready
```

### Build Test Container

The test container is available in the feature profile repository under
`internal/cntrsrv`.

Start by entering in that directory and running the following commands:

```shell
$ cd internal/cntrsrv
$ go mod vendor
$ CGO_ENABLED=0 go build .
$ docker build -f build/Dockerfile.local -t cntrsrv:latest .
```

At this point you will have a container image build for the test container.

```shell
$ docker images
REPOSITORY  TAG            IMAGE ID       CREATED         SIZE
cntrsrv     latest         8d786a6eebc8   3 minutes ago   21.4MB
```

Now export the container to a tarball.

```shell
$ docker save -o /tmp/cntrsrv.tar cntrsrv:latest
$ docker rmi cntrsrv:latest
```

This is the tarball that will be used during tests.

## CNTR-1.1: Deploy and Start a Container

Using the
[`gnoi.Containerz`](https://github.com/openconfig/gnoi/tree/main/containerz) API
(reference implementation to be available
[`openconfig/containerz`](https://github.com/openconfig/containerz), deploy a
container to the DUT. Using `gnoi.Containerz` start the container.

The container should expose a simple health API. The test succeeds if is
possible to connect to the container via the gRPC API to determine its health.

## CNTR-1.2: Retrieve a running container's logs.

Using the container started as part of CNTR-1.1, retrieve the logs from the
container and ensure non-zero contents are returned when using
`gnoi.Containerz.Log`.

## CNTR-1.3: List the running containers on a DUT

Using the container started as part of CNTR-1.1, validate that the container is
included in the listed set of containers when calling `gnoi.Containerz.List`.

## CNTR-1.4: Stop a container running on a DUT.

Using the container started as part of CNTR-1.2, validate that the container can
be stopped, and is subsequently no longer listed in the `gnoi.Containerz.List`
API.

## OpenConfig Path and RPC Coverage

The below yaml defines the RPCs intended to be covered by this test.

```yaml
rpcs:
  gnoi:
    containerz.Containerz.Deploy:
    containerz.Containerz.StartContainer:
    containerz.Containerz.StopContainer:
    containerz.Containerz.Log:
    containerz.Containerz.ListContainer:
```