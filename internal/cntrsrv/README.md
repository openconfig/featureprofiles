# `cntr` Container Image

This directory contains the source code for a binary which can be run as a
container on a network device to run the `feature/container` tests. It exposes
a gRPC API that can be interacted with through ONDATRA in order to validate
connectivity to specific gRPC services.

The `build` directory contains a `Dockerfile` for building the container.

The `proto/cntr` directory contains a protobuf that defines the gRPC API
exposed by the container for test purposes.
