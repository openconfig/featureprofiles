# MPLS-1.2: MPLS Traffic Class Marking

## Summary

Verify MPLS Traffic Class Marking configuration.

## Testbed type

*  [`featureprofiles/topologies/dutdut.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/dutdut.testbed)

## Topology:

```mermaid
graph LR;
A[DUT] <-- Port1/2(IPv4/6) --> B[DUT];
```

## Procedure

### Initial setup

*   Connect DUT-A port-1 to DUT-B port-2

*   Configure IPv4 addresses on DUT ports

    *   DUT-A port-1 IPv4 address ```dpa1-v4 = 192.168.1.1/30```
    *   DUT-B port-2 IPv4 address ```dpb2-v4 = 192.168.1.2/30```

*   Configure a loopback interface on both the DUT's and assign IPv4 addresses

    *   DUT-A Loopback50 ```dpalo-v4 = 100.100.100.1/32```
    *   DUT-B Loopback50 ```dpblo-v4 = 200.200.200.2/32```

*   Configure ISIS between the DUTs and advertise Loopback50

*   Enable MPLS and LDP on both the DUTs port-1 and port-2

    *   /network-instances/network-instance/mpls/global/interface-attributes/interface/config/mpls-enabled
    *   /network-instances/network-instance/mpls/signaling-protocols/ldp/global/config/lsr-id [DUT-A and B Loopkack50]

### MPLS-1.2.1 - Configure and verify classifier to match MPLS packets and mark Traffic Class

*   Configure a classifier to match MPLS packets

    *   /qos/classifiers/classifier/config/name = 'mpls-class'
    *   /qos/classifiers/classifier=[mpls-class]/config/type = 'MPLS'
    *   /qos/classifiers/classifier=[mpls-class]/terms/term/config/id = 'mpls-class-term'
    *   /qos/classifiers/classifier=[mpls-class]/terms/term=[mpls-class-term]/conditions/mpls/config/start-label-value = 16
    *   /qos/classifiers/classifier=[mpls-class]/terms/term=[mpls-class-term]/conditions/mpls/config/end-label-value = 1048575

*   Configure classifier to mark MPLS TC

    *   /qos/classifiers/classifier=[mpls-class]/terms/term=[mpls-class-term]/actions/remark/config/set-mpls-tc = 5

*   Apply the classifier on DUT-A port-1

    *   /qos/interfaces/interface/input/classifiers[mpls-class]/classifier/config/name

*   Validate the configuration is applied and values are reported correctly

    *   /network-instances/network-instance/mpls/global/interface-attributes/interface/state/mpls-enabled
    *   /qos/classifiers/classifier/state/name = 'mpls-class'
    *   /qos/classifiers/classifier=[mpls-class]/state/type = 'MPLS'
    *   /qos/classifiers/classifier=[mpls-class]/terms/term/state/id = 'mpls-class-term'
    *   /qos/classifiers/classifier=[mpls-class]/terms/term=[mpls-class-term]/conditions/mpls/state/start-label-value = 16
    *   /qos/classifiers/classifier=[mpls-class]/terms/term=[mpls-class-term]/conditions/mpls/state/end-label-value = 1048575
    *   /qos/classifiers/classifier=[mpls-class]/terms/term=[mpls-class-term]/actions/remark/state/set-mpls-tc = 5


## Canonical OC Configuration

### QOS Classifier amd Marking MPLS packet

```json
{
    "qos": {
        "classifers": {
            "classifier": {
                "config": {
                    "name": "mpls-class",
                    "type": "MPLS"
                },
                "terms": {
                    "term": {
                        "config": {
                            "id": "mpls-class-term"
                        },
                        "conditions": {
                            "mpls": {
                                "config": {
                                    "start-label-value": 16,
                                    "end-label-value": 1048575
                                }
                            }
                        },
                        "actions": {
                            "config": {
                                "set-mpls-tc": 5
                            }
                        }
                    }
                }
            }
        }
    }
}
```

## OpenConfig Path and RPC Coverage

```yaml

paths:

## Config paths:
/network-instances/network-instance/mpls/signaling-protocols/ldp/global/config/lsr-id:
/network-instances/network-instance/mpls/global/interface-attributes/interface/config/mpls-enabled:
/qos/classifiers/classifier/config/name:
/qos/classifiers/classifier/config/type:
/qos/classifiers/classifier/terms/term/config/id:
/qos/classifiers/classifier/terms/term/conditions/mpls/config/start-label-value:
/qos/classifiers/classifier/terms/term/conditions/mpls/config/end-label-value:
/qos/classifiers/classifier/terms/term/actions/remark/config/set-mpls-tc:

## State paths:
/network-instances/network-instance/mpls/global/interface-attributes/interface/state/mpls-enabled:
/qos/classifiers/classifier/state/name:
/qos/classifiers/classifier/state/type:
/qos/classifiers/classifier/terms/term/state/id:
/qos/classifiers/classifier/terms/term/conditions/mpls/state/start-label-value:
/qos/classifiers/classifier/terms/term/conditions/mpls/state/end-label-value:
/qos/classifiers/classifier/terms/term/actions/remark/state/set-mpls-tc:

rpcs:
  gnmi:
    gNMI.Set:
      Replace:
```

## Required DUT platform

*   FFF

