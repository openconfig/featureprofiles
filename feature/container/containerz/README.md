# CNTR-1: Basic container lifecycle via `gnoi.Containerz`.

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

