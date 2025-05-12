# gNOI Force transfer, activate and verify

## Summary

Validate the force transfer feature.
when we set the version field as empty string "" or wrong version of os install request, a force image transfer to device will be initated.

## Topology

*   2DUT
    DUT (modular - spitfire_d) <---> DUT (fixed - spitfire_f)

Require to have one device as Modular (spitfire_d) and another as Fixed (spitfire_f)
Note: os upgrade operation will be performed parllely in two device covering modular and fixed devices.

## Procedure

*   **Issue gnoi.os.Install**
    *   Issue gnoi.os.Install.InstallRequest_TransferRequest Provide following parameters:
        *   Version: populate this field with the
            *   Version of the image to be transferred
                *    if correct version provided, normal transfer is triggred
                *    if empty or wrong version provided, force transfer is triggred
        *   StandbySupervisor: populate this field with
            *   bool
            *   CISCO 8000 series does not support install on StandbySupervisor. So always set it to False
            *   set to True if install has to happen on standby Superviosor 
            *   only applicable for device wich support install on StandbySupervisor
    *   Issue gnoi.os.Install.InstallRequest_TransferContent Provide following parameters:
        *   TransferContent: populate this field with the
            *   bytes of the image fragement in sequence
    *   Issue gnoi.os.Install.InstallRequest_TransferEnd Provide following parameters:
        *   None
        *   Indicate the end of transfer to device

-   Normal Transfer:
    if image does not exists on disk it transfer the image
    if image already exists on disk it avoids transfer

-   Force Transfer:
    if image does not exists or already exists on disk it transfers image

*   **Issue gnoi.os.Activate**
    *   Issue gnoi.os.Activate.ActivateRequest Provide following parameters:
        *   Version: populate this field with the
            *   Version of the image to be Activated
            *   fetch the data from the install response
            *   if image of version provided is not existing on device harddisk(/misc/disk1/), responds with error
        *   StandbySupervisor: populate this field with
            *   bool
            *   CISCO 8000 series does not support install on StandbySupervisor. So always set it to False
            *   set to True if install has to happen on standby Superviosor 
            *   only applicable for device wich support install on StandbySupervisor
        *   NoReboot: populate this field with
            *   bool
            *   CISCO 8000 series does not support noboot install. So always set it to False
            *   set to True if reboot should not happen after activate
            *   only applicable for device wich support NoReboot activate.

*   **Issue gnoi.os.Verify**
    *   Issue gnoi.os.Verify.VerifyRequest No parameters required:
        *   Verify the version activated in Activated is loaded properly and all supervisor (in modular system) are up and running properly

## File organisation

```shell
┌─[feature/gnoi/cisco/os_install]
└──> tree
.
├── helper_functions.go             <- helper methods
├── metadata.textproto              <- metadata information file
├── os_install_internal_test.go     <- test case file
├── os_install_methods.go           <- OS.Proto function implementation
├── push_verify_config.go           <- interface, bgp config verifcation methods
├── README.md                       <- this readme file
└── start_test.go                   <- test starting, contains go test Main

0 directories, 7 files
```

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test.  OC paths used for test setup are not listed here.

```yaml
rpcs:
  gnoi:
    os.OS.Install:
    os.OS.Activate:
    os.OS.verify:
```