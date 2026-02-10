# CNTR-3: Container Supervisor Failover

## Summary

Verify that containers and volumes persist across a control processor switchover (failover).

## Procedure

* Build the test container as described below.
* Pass the tarball of the container to the test as an argument.

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

## CNTR-3.1: Container Supervisor Failover

1.  **Setup**: Using `gnoi.Containerz`, deploy a container image, create a volume, and start a container mounting that volume. Verify the container is running and the volume exists.
2.  **Trigger Failover**: Identify the standby control processor using gNMI. Trigger a switchover using `gnoi.System.SwitchControlProcessor`.
3.  **Verify Recovery**: Wait for the DUT to recover (reboot/reconnect). Verify that the container started in step 1 is in `RUNNING` state and the volume still exists using `gnoi.Containerz`.

## Canonical OC

<!-- This test does not require any specific OpenConfig configuration, so this section is empty to satisfy the validator. -->
```json
{}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the RPCs intended to be covered by this test.

```yaml
rpcs:
  gnoi:
    containerz.Containerz.Deploy:
    containerz.Containerz.StartContainer:
    containerz.Containerz.ListContainer:
    containerz.Containerz.CreateVolume:
    containerz.Containerz.ListVolume:
    system.System.SwitchControlProcessor:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```
