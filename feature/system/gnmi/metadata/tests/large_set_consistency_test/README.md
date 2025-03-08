# gNMI-1.14: OpenConfig metadata consistency during large config push

## Summary
This test verify if OpenConfig metadata leaf at root is updated according to
pushed config and not reverted or otherway modified, Even if many rapid,
concurrent non-write requests are served while setRequest is in
process (and SetResponse is not send yet).
In case metadata leaf at root value changes, test fails.

## Topology
dut.testbed

## Procedure
### Subtest-1

* prepare 2 large OpenConfig configurations that only differs in root-level netadata/annotation - 1st_LARGE_CONFIGURATION and 2nd_LARGE_CONFIGURATION. Configuration should exercise following branches of Openconfig:
  *  /interfaces/
  *  /network-instances/
  *  /qos/
  *  /system/
  *  /routing-policy/
  *  /components/
  *  /sampling/
  *  /lldp/
  *  /lacp/
  *  /protobuf-metadata

* Push 1st_LARGE_CONFIGURATION and wait for SetResponse that confirm push was successful.
* Send subscribe `ONCE` for configuration root-level metadata and verify that it is identical corresponding root-level metadata of 1st_LARGE_CONFIGURATION.
* For REQUEST of type (GetRequest, SubscribeRequest, CapabilityRequest)
  * Repeat sending rapidly subsequent REQUEST requests for configuration
    root-level metadata (GetRequest, SubscribeRequest).
### Subtest-2

  * While above REQUESTS are send, push SetRequest replace with 2nd_LARGE_CONFIGURATION.
  * After SetResponse is received confirming successful push of 2nd_LARGE_CONFIGURATION:
    * stop sending REQUEST requests
    * wait 5 seconds
    * SubscribeRequest once for configuration root-level metadata and verify
      that it is identical corresponding root-level metadata of
      2nd_LARGE_CONFIGURATION.
### Subtest-3

* Push a Large metadata of size 100 KiB and wait for SetResponse that confirm push was successful.

* Send subscribe `ONCE` for configuration root-level metadata and verify that it
  is identical to corresponding root-level metadata of the source that was
  earlier configured.
### Subtest-4

* Push a Large metadata of size 1 MiB and wait for SetResponse that confirm push was successful.

* Send subscribe `ONCE` for configuration root-level metadata and verify that it
  is identical to corresponding root-level metadata of the source that was
  earlier configured.


## Configuration path coverage
* /@/openconfig-metadata:protobuf-metadata

> Note: WBB implementations need not support this annotation at paths deeper
> than the root (i.e., a configuration that contains
> openconfig-metadata:protobuf-metadata at any level other than under the root
> can be rejected). The WBB device implementation can map this to an internal
> path to store the configuration.

## Telemetry path coverage
* /@/openconfig-metadata:protobuf-metadata

## Minimum DUT platform
FFF

## OpenConfig Path and RPC Coverage

```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Subscribe:

```
