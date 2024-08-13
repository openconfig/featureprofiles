# RT-1.53: prefix-list test

## Summary

-   Prefix list is updated and replaced correctly after restarting the process
    with supports gNOI to validate that internal state of OC agent is in sync
    with the running configuration.

## Testbed type

*   https://github.com/openconfig/featureprofiles/blob/main/topologies/dut.testbed

## Procedure

### Applying configuration

For each section of configuration below, prepare a gnmi.SetBatch with all the
configuration items appended to one SetBatch. Then apply the configuration to
the DUT in one gnmi.Set using the `replace` option

### RT-1.53.1 [TODO:https://github.com/openconfig/featureprofiles/issues/3306]

#### Create a prefix-set with 2 prefixes

*   Create a prefix-set with name "prefix-set-a"
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/name
*   Set the mode to IPv4
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode
*   Define two prefixes 10.240.31.48/28 and 173.36.128.0/20 with mask "exact"
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   Validate that the prefix-list is created correctly with two prefixes i.e.
    10.240.31.48/28 and 173.36.128.0/20
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range

### RT-1.53.2 [TODO:https://github.com/openconfig/featureprofiles/issues/3306]

#### Replace the prefix-set by replacing an existing prefix with new prefix

*   Define two prefixes 10.240.31.48/28 and 173.36.144.0/20 with mask "exact"
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   Replace the previous prefix-list
*   Validate that the prefix-list is created correctly with two prefixes i.e.
    10.240.31.48/28 and 173.36.144.0/20
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range

### RT-1.53.3 [TODO:https://github.com/openconfig/featureprofiles/issues/3306]

### Replace the prefix-set with 2 existing and a new prR

*   Define three prefixes 10.240.31.48/28, 10.240.31.64/28 and 173.36.144.0/20
    with mask "exact"
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range
*   Replace the previous prefix-list
*   Validate that the prefix-list is created correctly with three prefixes
    10.240.31.48/28, 10.240.31.64/28 and 173.36.144.0/20
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/name
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix
    *   /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range

### RT-1.53.4 [TODO:https://github.com/openconfig/featureprofiles/issues/3306]

### Create prefix list and replace with gnmi.

*   Send a gNMI SET request that contains below prefixes under TAG_3_IPV4 prefix-set
    ```
      10.240.31.48/28
      10.244.187.32/28
      173.36.128.0/20
      173.37.128.0/20
      173.38.128.0/20
      173.39.128.0/20
      173.40.128.0/20
      173.41.128.0/20
      173.42.128.0/20
      173.43.128.0/20
     ```
*   Validate that the prefix-list is created correctly with 10 prefixes.
*   Use gNOI to kill the process supporting gNMI.
*   Send a gNMI SET request that contains additional prefixes within the same
    prefix-set, TAG_3_IPV4.
    ```
      173.49.128.0/20
      173.46.128.0/20
      10.240.31.48/28
      173.44.128.0/20
      173.43.128.0/20
      173.47.128.0/20
      173.40.128.0/20
      173.37.128.0/20
      173.39.128.0/20
      173.38.128.0/20
      173.42.128.0/20
      10.244.187.32/28
      173.41.128.0/20
      173.36.128.0/20
      173.50.128.0/20
      173.51.128.0/20
      173.52.128.0/20
      173.53.128.0/20
      173.54.128.0/20
      173.55.128.0/20
      173.48.128.0/20
      173.45.128.0/20
    ```
*   Validate that the prefix-list is created correctly with 22 prefixes.

## OpenConfig Path and RPC Coverage

The below yaml defines the OC paths intended to be covered by this test. OC
paths used for test setup are not listed here.

```yaml
paths:
  ## Config paths
  ### prefix-set
  /routing-policy/defined-sets/prefix-sets/prefix-set/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/config/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/config/masklength-range:

  ## State paths
  ### prefix-list
  /routing-policy/defined-sets/prefix-sets/prefix-set/state/name:
  /routing-policy/defined-sets/prefix-sets/prefix-set/state/mode:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/ip-prefix:
  /routing-policy/defined-sets/prefix-sets/prefix-set/prefixes/prefix/state/masklength-range:

rpcs:
  gnmi:
    gNMI.Set:
    gNMI.Subscribe:
  gnoi:
    system.System.KillProcess:
```

## Required DUT platform

-   vRX
