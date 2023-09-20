# gNMI-1.14 OpenConfig metadata consistency during large config push

## Summary
This test verify if OpenConfig metadata leaf at root is updated according to pushed config and not reverted or otherway modified if rapid, subsequent Subscribe request are served while setRequest is in process and SetResponce is not recived yet.

## Topology
dut.testbed

## Procedure

* prepare 2 large OpenConfig configurations that only differs in root-level netadata/annotation - 1st-large-configuration and 2nd_large_configuration. Configuration should exercise following branches of Openconfig:
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
* Push 1st-large-configuration and wait for SetResponce that confirm push was sucesfull.
* Subscribe once (note: GNMI getRequest is deprecated) for configuration root-level metadata and verify that it is identical corresponding root-level metadata of 1st-large-configuration.
* Repeat sending subsequent Subscribe once requests for configuration root-level metadata rapidly.
* While above Subscribe once requests are send, push SetRequest replace with 2nd_large_configuration.
* After SetResponse is recived confirming sucesfull push of 2nd_large_configuration:
  * stop sending Subscribe once requests
  * wait 5 seconds
  * Subscribe once for configuration root-level metadata and verify that it is identical corresponding root-level metadata of 2nd-large-configuration.

## Configuration path coverage
* /config/protobuf-metadata

## Telemetry path coverage
* /state/protobuf-metadata

## Minimum DUT platform
FFF