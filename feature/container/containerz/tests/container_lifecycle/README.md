# CNTR-1: Basic container lifecycle via `gnoi.Containerz`.

## Summary

Verify the correct behaviour of `gNOI.Containerz` when operating containers.

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
$ docker tag cntrsrv:latest cntrsrv:upgrade
$ docker save -o /tmp/cntrsrv-upgrade.tar cntrsrv:upgrade
$ docker rmi cntrsrv:latest
```

This is the tarball that will be used during tests.

### Build docker volume sshfs plugin tarball

The `TestPlugins` test suite validates the lifecycle of containerz plugins, specifically using the `vieux/docker-volume-sshfs` plugin as an example. To run these tests, you need to build the plugin's `rootfs.tar.gz` tarball.

Follow these steps:

1.  **Clone the `docker-volume-sshfs` repository:**
    If you haven't already, clone the repository from GitHub:
    ```bash
    git clone https://github.com/vieux/docker-volume-sshfs.git
    cd docker-volume-sshfs
    ```

2.  **Update the `Makefile`:**
    Open the `Makefile` in the `docker-volume-sshfs` directory and add the following target. This target is specifically designed to create a `rootfs.tar.gz` that includes the `config.json` manifest at the root, as expected by the `containerz` plugin system.

    ```makefile
    # ... (other Makefile content) ...

    package-plugin-tarball: rootfs
    	@echo "### Creating rootfs.tar.gz for containerz"
    	@tar -czf rootfs.tar.gz -C ./plugin/rootfs . -C ../ config.json
    	@echo "### rootfs.tar.gz created in project root."

    # ... (ensure there's a blank line after this if it's not the end of the file) ...
    ```
    *   The `-C ./plugin/rootfs .` part adds all files from the `plugin/rootfs` directory to the archive.
    *   The `-C ../ config.json` part adds the `config.json` file (which is in the parent directory of `plugin/rootfs`, i.e., the project root) to the archive's root.

3.  **Build the plugin tarball:**
    Run the new make target from the root of the `docker-volume-sshfs` directory:
    ```bash
    make package-plugin-tarball
    ```
    This command will create a `rootfs.tar.gz` file in the `docker-volume-sshfs` project root directory.

4.  **Provide the path to the test:**
    When running your Go tests, you'll need to provide the absolute path to this generated `rootfs.tar.gz` file using the `--plugin_tar` flag. For example:
    ```bash
    go test -v ./feature/container/containerz/tests/container_lifecycle/... --plugin_tar=/path/to/your/docker-volume-sshfs/rootfs.tar.gz
    ```
    Replace `/path/to/your/docker-volume-sshfs/` with the actual path to where you cloned and built the plugin.

5.  **SSHFS Plugin Test: Runtime Configuration:**

    The `TestPlugin` test for the `vieux/docker-volume-sshfs` plugin (CNTR-1.7) requires a runtime configuration JSON file that includes SSH credentials and specific options.

    A default configuration file, `test_sshfs_config.json`, is now provided in the `testdata/` directory (i.e., at `feature/container/containerz/tests/container_lifecycle/testdata/test_sshfs_config.json`). This file is pre-configured for a local SSH server setup with the following default environment variables:
    *   `SSH_HOST`: `localhost`
    *   `SSH_USER`: `testuser`
    *   `SSH_PASSWORD`: `testpass`
    *   `SSHFS_OPTS`: `allow_other,reconnect`

    **Customization:**
    If you want to customize your local SSH testing environment (e.g., with a different hostname, user, password, or SSH options), you can:
    *   Copy the provided `testdata/test_sshfs_config.json` to a new location.
    *   Modify the `env` section in your copied file with your specific credentials and options. The file should still be based on the original `config.json` from the `vieux/docker-volume-sshfs` plugin.
    *   Update the test flag to point to your custom configuration file.

    The JSON structure (like `description`, `entrypoint`, `interface`, `mounts`, etc.) should mirror the original `config.json` from the plugin, with the `env` array modified as needed. The provided `test_sshfs_config.json` already includes these base settings along with the test-specific environment variables:

    By using this provided configuration file (or a customized version), you'll have the necessary runtime settings for the `vieux/docker-volume-sshfs` plugin, allowing the `TestPlugin` (CNTR-1.7) to execute correctly.

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

## CNTR-1.5: Create a volume on the DUT.

Validate the the DUT is capable of creating a volume, reading it back
and removing it. 

## CNTR-1.6: Upgrade a container on the DUT.

Using the same container started as part of CNTR-1.1, validate that the container
can be upgraded to the new version of the image identified by a different tag
than the current running container image. 

## CNTR-1.7: Start a plugin on the DUT

This test validates the complete lifecycle of the `vieux/docker-volume-sshfs` plugin on the DUT.
Using the tarball from 'Build docker volume sshfs plugin tarball', the test installs and activates the plugin via `gnoi.Containerz.StartPlugin`, then verifies its presence and state using `gnoi.Containerz.ListPlugins`.
Subsequently, the plugin is stopped using `gnoi.Containerz.StopPlugin` and removed with `gnoi.Containerz.RemovePlugin`.

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
    containerz.Containerz.CreateVolume:
    containerz.Containerz.RemoveVolume:
    containerz.Containerz.ListVolume:
    containerz.Containerz.UpdateContainer:
    containerz.Containerz.StartPlugin:
    containerz.Containerz.ListPlugins:
    containerz.Containerz.StopPlugin:
    containerz.Containerz.RemovePlugin:
```
