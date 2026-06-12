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

### Test Setup

*   **Topology:** `TESTBED_DUT` (Single DUT with dual control processors).
*   **Variables:** Let `<IMAGE_NAME>` be the test container `cntrsrv_image:latest`.

### Procedure

1.  **Load Image:** Establish a gNOI connection to the primary control processor and load `<IMAGE_NAME>` using `gnoi.Containerz.Deploy`.
2.  **Verify Load:** Call `gnoi.Containerz.ListImage` to verify `<IMAGE_NAME>` is present on the device.
3.  **Identify Standby:** Query the gNMI paths `/components/component/state/redundant-role` and `/components/component/state/switchover-ready` to identify the standby control processor and verify it is ready.
4.  **Trigger Failover:** Trigger a switchover to the standby control processor using `gnoi.System.SwitchControlProcessor`. Wait for the new primary to stabilize.
5.  **Verify Persistence:** Establish a new gNOI connection to the newly active primary control processor. Call `gnoi.Containerz.ListImage` to verify the image persisted.

#### Pass/Fail Criteria

*   **Pass:** The `<IMAGE_NAME>` is successfully returned on the new primary control processor.
*   **Fail:** The RPC returns an error, times out, or the image is absent from the device after the switchover.

## CNTR-3.2, CNTR-3.3, CNTR-3.4: Container and Volume Persistence

### Test Setup

*   **Topology:** `TESTBED_DUT`
*   **Variables:** Let `<IMAGE_NAME>` be `cntrsrv_image:latest`. Let `<VOLUME_NAME>` be a unique volume name.

### Procedure

1.  **Setup Volume:** Using `gnoi.Containerz.CreateVolume`, create a volume named `<VOLUME_NAME>`.
2.  **Deploy Image:** Using `gnoi.Containerz.Deploy`, load `<IMAGE_NAME>`.
3.  **Start Container:** Using `gnoi.Containerz.StartContainer`, start a container that mounts `<VOLUME_NAME>`.
4.  **Verify Setup:** Call `gnoi.Containerz.ListContainer` and `ListVolume` to verify the container is in a `RUNNING` state and the volume exists.
5.  **Identify Standby:** Query `/components/component/state/redundant-role` and `/components/component/state/switchover-ready` to ensure the standby RE is ready.
6.  **Trigger Failover:** Trigger a switchover using `gnoi.System.SwitchControlProcessor`.
7.  **Verify Recovery:** After switchover, call `ListContainer` and `ListVolume` on the new primary.

#### Pass/Fail Criteria

*   **Pass:** The container is still `RUNNING` and the volume still exists.
*   **Fail:** The container is `STOPPED`, missing, or the volume is missing.

## CNTR-3.5: Image Removal Persistence

### Test Setup

*   **Topology:** `TESTBED_DUT`
*   **Variables:** Let `<IMAGE_NAME>` be `cntrsrv_image:latest`.

### Procedure

1.  **Load and Remove Image:** Load `<IMAGE_NAME>`, then remove it using `gnoi.Containerz.RemoveImage`.
2.  **Verify Removal:** Call `gnoi.Containerz.ListImage` and verify the image no longer exists.
3.  **Identify Standby:** Query `/components/component/state/redundant-role` and `/components/component/state/switchover-ready` to ensure the standby RE is ready.
4.  **Trigger Failover:** Trigger a switchover using `gnoi.System.SwitchControlProcessor`.
5.  **Verify Persistence of Removal:** Call `gnoi.Containerz.ListImage` to list images on the new primary.

#### Pass/Fail Criteria

*   **Pass:** The image does not exist on the new primary control processor.
*   **Fail:** The removed image reappears after the switchover.

## CNTR-3.6: Container Removal Persistence

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Start and Remove Container:** Load the image, start a container, then remove it using `gnoi.Containerz.RemoveContainer`.
2.  **Verify Removal:** Call `gnoi.Containerz.ListContainer` and verify the container no longer exists.
3.  **Identify Standby:** Query `/components/component/state/redundant-role` and `/components/component/state/switchover-ready` to ensure the standby RE is ready.
4.  **Trigger Failover:** Trigger a switchover.
5.  **Verify Persistence of Removal:** Call `ListContainer` on the new primary.

#### Pass/Fail Criteria

*   **Pass:** The container does not exist on the new primary.
*   **Fail:** The removed container reappears after the switchover.

## CNTR-3.7: Interrupt Image Transfer During Failover

### Test Setup

*   **Topology:** `TESTBED_DUT`
*   **Variables:** Identify or build a deliberately large dummy image (e.g., >2GB) to ensure the transfer takes sufficient time (e.g., >30 seconds).

### Procedure

1.  **Identify Standby:** Query `/components/component/state/redundant-role` and `/components/component/state/switchover-ready` to ensure the standby RE is ready.
2.  **Interrupt Transfer:** Initiate transferring the large image to the device via `gnoi.Containerz.Deploy`.
3.  **Trigger Failover:** While the transfer is actively in progress, trigger a switchover using `gnoi.System.SwitchControlProcessor`.
4.  **Verify Interruption:** After the switchover on the new primary, call `gnoi.Containerz.ListImage` to check the image presence.

#### Pass/Fail Criteria

*   **Pass:** The dummy image is not present or successfully loaded on the new primary.
*   **Fail:** The system crashes, stop responding, or the partial image remains in an inconsistent state.

## CNTR-3.8: Interrupt Container Start During Failover

### Test Setup

*   **Topology:** `TESTBED_DUT`
*   **Variables:** `<IMAGE_NAME>` = `cntrsrv_image:latest`.

### Procedure

1.  **Setup:** Load the `<IMAGE_NAME>` onto the device.
2.  **Identify Standby:** Query `/components/component/state/redundant-role` and `/components/component/state/switchover-ready` to ensure the standby RE is ready.
3.  **Interrupt Start:** Initiate starting the container using `gnoi.Containerz.StartContainer`.
4.  **Trigger Failover:** Immediately before the container reaches the `RUNNING` state, trigger a switchover.
5.  **Verify Interruption:** Call `gnoi.Containerz.ListContainer` on the new primary.

#### Pass/Fail Criteria

*   **Pass:** The container is not present or is not in a `RUNNING` state on the new primary.
*   **Fail:** The container enters an inconsistent state or causes the system to stop responding.

## CNTR-3.9: Interrupt Volume Creation During Failover

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Identify Standby:** Query `/components/component/state/redundant-role` and `/components/component/state/switchover-ready` to ensure the standby RE is ready.
2.  **Interrupt Volume Creation:** Initiate creating a volume via `gnoi.Containerz.CreateVolume`.
3.  **Trigger Failover:** Immediately trigger a switchover using `gnoi.System.SwitchControlProcessor`.
4.  **Verify Interruption:** Call `gnoi.Containerz.ListVolume` on the new primary.

#### Pass/Fail Criteria

*   **Pass:** The intended volume does not exist or is fully cleaned up.
*   **Fail:** A partial or corrupted volume exists.

## CNTR-3.10: Double Failover Image Persistence

### Test Setup

*   **Topology:** `TESTBED_DUT`
*   **Variables:** `<IMAGE_NAME>` = `cntrsrv_image:latest`.

### Procedure

1.  **Load Image:** Load `<IMAGE_NAME>` onto the device.
2.  **First Failover:** Query `/components/component/state/redundant-role` and `/components/component/state/switchover-ready` to identify the standby RE. Trigger a switchover to the standby.
3.  **Verify Persistence:** After the first switchover, verify `<IMAGE_NAME>` is still available on the new primary.
4.  **Second Failover:** Once the original primary recovers and becomes the new standby (verify via `/components/component/state/redundant-role` and `/components/component/state/switchover-ready`), trigger another switchover back to it.
5.  **Verify Final Persistence:** Call `gnoi.Containerz.ListImage` on the newly active primary (original RE).

#### Pass/Fail Criteria

*   **Pass:** The image remains available on the device after the second failover.
*   **Fail:** The image is lost or corrupted after returning to the original RE.

## CNTR-3.11: Double Failover Container Persistence

### Test Setup

*   **Topology:** `TESTBED_DUT`
*   **Variables:** `<IMAGE_NAME>` = `cntrsrv_image:latest`.

### Procedure

1.  **Setup:** Load `<IMAGE_NAME>` and start a container. Verify it is `RUNNING` via `gnoi.Containerz.ListContainer`.
2.  **First Failover:** Trigger a switchover to the standby control processor.
3.  **Second Failover:** Wait for stabilization and for the original primary to become the `READY` standby. Trigger another switchover back.
4.  **Verify Final State:** Call `gnoi.Containerz.ListContainer`.

#### Pass/Fail Criteria

*   **Pass:** The container has persisted and remains in the `RUNNING` state.
*   **Fail:** The container is missing, `STOPPED`, or `EXITED`.

## CNTR-3.12: Double Failover Container Stop Persistence

### Test Setup

*   **Topology:** `TESTBED_DUT`
*   **Variables:** `<IMAGE_NAME>` = `cntrsrv_image:latest`.

### Procedure

1.  **Setup:** Load `<IMAGE_NAME>` and start a container.
2.  **First Failover:** Trigger a switchover to the standby control processor.
3.  **Stop Container:** Wait for stabilization, then stop the container via `gnoi.Containerz.StopContainer`. Verify it is `STOPPED`.
4.  **Second Failover:** Trigger another switchover back to the original primary.
5.  **Verify Final State:** Call `gnoi.Containerz.ListContainer`.

#### Pass/Fail Criteria

*   **Pass:** The container remains in the `STOPPED` state and did not automatically restart.
*   **Fail:** The container erroneously restarts or is missing.

## CNTR-3.13: Double Failover Volume Persistence

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Setup:** Create a volume via `gnoi.Containerz.CreateVolume`.
2.  **First Failover:** Trigger a switchover to the standby control processor.
3.  **Second Failover:** Once stabilized, trigger another switchover back to the original primary.
4.  **Verify Final State:** Call `gnoi.Containerz.ListVolume`.

#### Pass/Fail Criteria

*   **Pass:** The volume is still available and intact.
*   **Fail:** The volume is lost, corrupted, or inaccessible.

## CNTR-3.14: Double Failover Container and Volume Persistence

### Test Setup

*   **Topology:** `TESTBED_DUT`
*   **Variables:** Let `<VOLUME_NAME>` be a unique volume name.

### Procedure

1.  **Setup:** Create `<VOLUME_NAME>` and start a container that mounts it. Verify it is `RUNNING` and associated with the volume.
2.  **First Failover:** Trigger a switchover to the standby control processor.
3.  **Second Failover:** Once stabilized, trigger another switchover back to the original primary.
4.  **Verify Final State:** Call `gnoi.Containerz.ListContainer` and `ListVolume`.

#### Pass/Fail Criteria

*   **Pass:** The container is `RUNNING` and successfully maintains its mount association to `<VOLUME_NAME>`.
*   **Fail:** The container fails to start, or loses its volume mount.

## CNTR-3.15: Container Placement: LC_PRIMARY

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Start Primary Container:** Start a container specifying the location tag `LC_PRIMARY` via `gnoi.Containerz.StartContainer`.
2.  **Verify Placement:** Establish separate gNOI connections directly to the individual IP addresses of both the Primary and Backup control processors.
3.  **Assert Constraints:** Call `gnoi.Containerz.ListContainer` on both connections.

#### Pass/Fail Criteria

*   **Pass:** The container appears in the Primary's list and is completely absent from the Backup's list.
*   **Fail:** The container spawns on the backup or fails to spawn on the primary.

## CNTR-3.16: Container Placement: LC_BACKUP

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Start Backup Container:** Start a container specifying the location tag `LC_BACKUP` via `gnoi.Containerz.StartContainer`.
2.  **Verify Placement:** Establish separate gNOI connections to the Primary and Backup control processors.
3.  **Assert Constraints:** Call `gnoi.Containerz.ListContainer` on both.

#### Pass/Fail Criteria

*   **Pass:** The container appears exclusively in the Backup's list and is absent from the Primary's list.
*   **Fail:** The container spawns on the primary or fails to spawn on the backup.

## CNTR-3.17: Container Placement: LC_ALL

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Start All Container:** Start a container specifying the location tag `LC_ALL` via `gnoi.Containerz.StartContainer`.
2.  **Verify Placement:** Establish separate gNOI connections to the Primary and Backup control processors.
3.  **Assert Constraints:** Call `gnoi.Containerz.ListContainer` on both.

#### Pass/Fail Criteria

*   **Pass:** The container is instantiated and `RUNNING` on both control processors simultaneously.
*   **Fail:** The container fails to spawn on one or both processors.

## CNTR-3.18: Container Persistence: LC_PRIMARY Failover

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Start Container:** Start a container with location tag `LC_PRIMARY`.
2.  **Trigger Failover:** Trigger a control processor switchover using `gnoi.System.SwitchControlProcessor`.
3.  **Verify Location:** Once the old backup transitions to the new primary role, establish a gNOI connection to it. Call `gnoi.Containerz.ListContainer`.

#### Pass/Fail Criteria

*   **Pass:** The container dynamically migrated or instantiated to run on the new primary RE.
*   **Fail:** The container is absent from the new primary.

## CNTR-3.19: Container Persistence: LC_BACKUP Failover

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Start Container:** Start a container with location tag `LC_BACKUP`. Ensure it runs only on the backup processor.
2.  **Trigger Failover:** Trigger a control processor switchover.
3.  **Verify Location:** Establish gNOI connections to both the new primary and the new backup (once available). Call `gnoi.Containerz.ListContainer` on both.

#### Pass/Fail Criteria

*   **Pass:** The container stops running on the new primary and is exclusively running on the new backup.
*   **Fail:** The container runs on the new primary or fails to migrate to the new backup.

## CNTR-3.20: Container Persistence: LC_ALL Failover

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Start Container:** Start a container with location tag `LC_ALL`. Ensure instances run on both processors.
2.  **Trigger Failover:** Trigger a control processor switchover.
3.  **Verify Location:** Immediately after switchover, verify the container continues running on the new primary. Once the new backup processor comes online, verify a second instance spawns on it.

#### Pass/Fail Criteria

*   **Pass:** The container runs on the new primary and later on the new backup.
*   **Fail:** The container fails to persist on the new primary or fails to spawn on the new backup.

## CNTR-3.21: Container Persistence On Cold Reboot

### Test Setup

*   **Topology:** `TESTBED_DUT`
*   **Variables:** Let `<VOLUME_NAME>` be a unique volume name and `<IMAGE_NAME>` be `cntrsrv_image:latest`.

### Procedure

1.  **Setup:** Create `<VOLUME_NAME>`, deploy `<IMAGE_NAME>`, and start a container that mounts `<VOLUME_NAME>`. Verify it is `RUNNING`.
2.  **Cold Reboot:** Trigger a cold reboot using `gnoi.System.Reboot`.
3.  **Verify Recovery:** After the cold reboot, establish a gNOI connection to the primary. Call `gnoi.Containerz.ListContainer` and `ListVolume`.

#### Pass/Fail Criteria

*   **Pass:** The container is `RUNNING` and the volume exists.
*   **Fail:** The container or volume is absent.

## CNTR-3.22: Volume Data Integrity Across Failover

### Test Setup

*   **Topology:** `TESTBED_DUT`

### Procedure

1.  **Setup:** Create a volume, deploy the image, and start a container that mounts this volume. Ensure the application is running.
2.  **Write Data:** Send an RPC or HTTP request to the application's exposed endpoint to write a unique verifiable string (e.g., a UUID or timestamp) to a file within the mounted volume path.
3.  **Trigger Failover:** Trigger a switchover to the standby control processor using `gnoi.System.SwitchControlProcessor`.
4.  **Verify Integrity:** Once the container recovers on the new primary, send another request to read the file from the volume.

#### Pass/Fail Criteria

*   **Pass:** The returned contents perfectly match the unique string written prior to the failover.
*   **Fail:** The file is missing, empty, or contains corrupted/mismatched data.

## Canonical OC

<!-- This test does not require any specific OpenConfig configuration, so this section is empty to satisfy the validator. -->
```json
{}
```

## OpenConfig Path and RPC Coverage

The below yaml defines the RPCs intended to be covered by this test.

```yaml
paths:
  /components/component/state/redundant-role:
  /components/component/state/switchover-ready:
rpcs:
  gnoi:
    containerz.Containerz.Deploy:
    containerz.Containerz.StartContainer:
    containerz.Containerz.StopContainer:
    containerz.Containerz.RemoveImage:
    containerz.Containerz.RemoveContainer:
    containerz.Containerz.ListImage:
    containerz.Containerz.ListContainer:
    containerz.Containerz.CreateVolume:
    containerz.Containerz.ListVolume:
    system.System.SwitchControlProcessor:
    system.System.Reboot:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:
```
