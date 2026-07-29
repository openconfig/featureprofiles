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

## CNTR-3.1: Image Persistence

1.  **Load Image**: Using `gnoi.Containerz.Deploy`, load a container image onto the device.
2.  **Verify Load**: Verify the image exists on the device using `gnoi.Containerz.List`.
3.  **Trigger Failover**: Identify the standby control processor using gNMI and trigger a switchover using `gnoi.System.SwitchControlProcessor`.
4.  **Verify Persistence**: After the switchover, verify the loaded image is still available on the new primary control processor.

## CNTR-3.2, CNTR-3.3, CNTR-3.4: Container and Volume Persistence

1.  **Setup**:
    *   Using `gnoi.Containerz.CreateVolume`, create a volume.
    *   Using `gnoi.Containerz.Deploy`, load a container image.
    *   Using `gnoi.Containerz.Start`, start a container that mounts the created volume.
2.  **Verify Setup**: Verify the container is in a `RUNNING` state and the volume exists.
3.  **Trigger Failover**: Identify the standby control processor using gNMI. Trigger a switchover using `gnoi.System.SwitchControlProcessor`.
4.  **Verify Recovery**: After the switchover, verify that the container is still `RUNNING` and the volume still exists using `gnoi.Containerz`.

## CNTR-3.5: Image Removal Persistence

1.  **Load and Remove Image**: Load a container image, then remove it using `gnoi.Containerz.Deploy` with the `image_delete` option.
2.  **Verify Removal**: Verify the image no longer exists on the device.
3.  **Trigger Failover**: Trigger a control processor switchover.
4.  **Verify Persistence of Removal**: After the switchover, verify the image does not exist on the new primary control processor.

## CNTR-3.6: Container Removal Persistence

1.  **Start and Remove Container**: Start a container, then remove it using `gnoi.Containerz.Remove`.
2.  **Verify Removal**: Verify the container no longer exists.
3.  **Trigger Failover**: Trigger a control processor switchover.
4.  **Verify Persistence of Removal**: After the switchover, verify the container does not exist on the new primary control processor.

## CNTR-3.7: Double Failover Image Persistence

1.  **Load Image**: Load a container image onto the device.
2.  **First Failover**: Trigger a control processor switchover to the standby.
3.  **Verify Persistence**: After the first switchover, verify the image is still available on the new primary.
4.  **Second Failover**: Trigger another control processor switchover, returning to the original primary.
5.  **Verify Final Persistence**: After the second switchover, verify the image is still available.

## CNTR-3.8: Container Persistence On Cold Reboot

1.  **Setup**:
    *   Using `gnoi.Containerz.CreateVolume`, create a volume.
    *   Using `gnoi.Containerz.Deploy`, load a container image.
    *   Using `gnoi.Containerz.Start`, start a container that mounts the created volume.
2.  **Verify Setup**: Verify the container is in a `RUNNING` state and the volume exists.
2.  **Cold Reboot**: Trigger a cold reboot using `gnoi.System.Reboot`.
3.  **Verify Recovery**: After the cold reboot, verify that the container is still `RUNNING` and the volume still exists using `gnoi.Containerz`.


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
    system.System.Reboot:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```
