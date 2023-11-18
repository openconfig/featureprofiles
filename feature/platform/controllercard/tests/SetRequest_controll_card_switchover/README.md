# gNMI-1.19 ConfigPush after Control Card switchover

## Summary
This test verifys if large config can be bushed via gNMI SetRequest within 2 minutes after Control Card switchover. 

## Procedure

* Prepare large OpenConfig config file to be pushed within single setRequest.
  * 150 LAG interfaces w/ ip, ipv6 configuration
  * 800 Ethernet interfaces as memeber s of LAG
  * 28 IPv4 and 28 IPv6 BGP neighbours
  * ISIS on all trunk/LAG ports
* Initiate Control Card switchover using gNOI SwitchControlProcessorRequest 
* Immedietly after reciving SwitchControlProcessorResponce for  gNOI switchover, but no later then 120 second after calling gNOI SwitchControlProcessorRequest, send gNMI setRequest with prepared large config.
* wait 120 second
* Retrive configuration form DUT using gNMI GetRequest.
* Verify:
  * The gNMI setResponce has been received within 120s after setRequest, and 
  * The gNOI SwitchControlProcessorResponce has been recived and switchover was executed by DUT
  * The configuration retrived form DUT is the same as one prepared^1

^1 some small deviations are expected. This is OK to verify that retrived configuration is not smaller in sixe then prepared one.

## Testbed topology
dut.testbed

## Config Parameter coverage
N/A

## Telemetry Parameter coverage
N/A

##
MFF