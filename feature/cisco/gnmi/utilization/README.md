# gnmi_resource_utilization: GNMI Set and MDT/EDT support for chassis and linecard resource utilization tree: 8000

This automation tests the implementation of the utilization thresholds for five different types of resources:

* cpu
* memory
* disk0
* harddisk
* power

## Topology

* Spitfire-distributed

## Tests

* TestCopyFile
Copies the `stress` binary to the router

* TestSetSystemThreshold
Test which configures and unconfigures thresholds of each system resource using both gnmi and CLI

* TestReceiveSystemThresholdNotification
Test which configures thresholds, subscribes to the associated resource,
then stresses each of the system resources until each subscription receives the notification that the threshold was exceeded and subsequently cleared

* TestSetInvalid
Test which attempts to configure illegal threshold values via gnmi and CLI

* TestSetComponentThreshold
Test which configures the threshold in the component OC path (i.e. Linecard for all resources except for power, which uses Chassis)


## Paths
```
/openconfig-platform:components/component/chassis/utilization
/openconfig-platform:components/component/chassis/utilization/resources
/openconfig-platform:components/component/chassis/utilization/resources/resource
/openconfig-platform:components/component/chassis/utilization/resources/resource/config
/openconfig-platform:components/component/chassis/utilization/resources/resource/config/name
/openconfig-platform:components/component/chassis/utilization/resources/resource/config/used-threshold-upper
/openconfig-platform:components/component/chassis/utilization/resources/resource/config/used-threshold-upper-clear
/openconfig-platform:components/component/chassis/utilization/resources/resource/name
/openconfig-platform:components/component/chassis/utilization/resources/resource/state
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/committed
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/free
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/high-watermark
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/last-high-watermark
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/max-limit
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/name
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/used
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/used-threshold-upper
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/used-threshold-upper-clear
/openconfig-platform:components/component/chassis/utilization/resources/resource/state/used-threshold-upper-exceeded

/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/config
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/config/name
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/config/used-threshold-upper
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/config/used-threshold-upper-clear
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/name
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/committed
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/free
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/high-watermark
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/last-high-watermark
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/max-limit
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/name
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/used
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/used-threshold-upper
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/used-threshold-upper-clear
/openconfig-platform:components/component/openconfig-platform-linecard:linecard/utilization/resources/resource/state/used-threshold-upper-exceeded

system utilization containers for chassis and linecard related resource

/openconfig-system:system/openconfig-system-utilization:utilization
/openconfig-system:system/openconfig-system-utilization:utilization/resources
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/config
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/config/name
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/config/used-threshold-upper
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/config/used-threshold-upper-clear											
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/name
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/state
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/state/active-component-list
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/state/name
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/state/used-threshold-upper
/openconfig-system:system/openconfig-system-utilization:utilization/resources/resource/state/used-threshold-upper-clear
```
