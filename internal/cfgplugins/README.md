# cfgplugins package

## Goal

The [contributing
guide](https://github.com/openconfig/featureprofiles/blob/main/CONTRIBUTING.md)
specifies that `cfgplugins` should be used for DUT configuration generation.  

The goal of the featureprofiles `cfgplugins` is to provide a library of
functions that generate reusable configuration snippets. These in turn are used
to compose configurations used in featureprofiles tests.  

## Implementing cfgplugins

The cfgplugins structure should align with the /feature folder, which in turn
is roughly aligned with the OpenConfig data model tree.  Top level feature
folders should have a cfgplugins file using the same name.  

Each function in `cfgplugins` should define a struct to hold the required
attributes, which is then passed to the configuration generation function.  The
configuration parameters should be defined in a struct and passed by reference.
(See [example](https://github.com/openconfig/featureprofiles/blob/f724d8371724a5045382b350464050df3e027d6c/internal/cfgplugins/sflow.go#L43))

The code in the cfgplugin function should construct and return an ondatra
`gnmi.BatchReplace` object. (See [example](https://github.com/openconfig/featureprofiles/blob/f724d8371724a5045382b350464050df3e027d6c/internal/cfgplugins/sflow.go#L74-L75))

Test code (`_test.go` files) should call as many `cfgplugins` as needed to
compose the desired configuration.  After assembling the configuration needed
for a particular test or subtest, the test should call `batch.Set()` to replace
the portion of the configuration on the DUT described by in the batch object.
(See [example](https://github.com/openconfig/featureprofiles/blob/f724d8371724a5045382b350464050df3e027d6c/feature/sflow/otg_tests/sflow_base_test/sflow_base_test.go#L253-L256).

The idea behind placing the `.Set` call in the test code is this is modifying
the DUT and is what is being tested.  In other words, we are not testing the
configuration generation helper, but rather we are testing if the DUT will
accept the configuration.

## Function parameters

All exported/public functions from a cfgplugin should have a parameter signature like:
`(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, cfg MyConfigToUpdateStruct)`

The purpose of the `*gnmi.SetBatch` is so the caller can pass in a Batch object containing their
configuration and the plugin can add to it.  This allows the caller to call many cfgplugins to
accumulate configuration and then `.Set` that configuration all at once (or whenever the caller
would like to `Set` the config to satisfy their workflow).

An example function and struct should look like:

```go
// StaticARPConfig holds all per-port static ARP entries.
type StaticARPConfig struct {
 Entries []StaticARPEntry
}

// StaticARPWithMagicUniversalIP configures static ARP and static routes per-port.
func StaticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, cfg StaticARPConfig) *gnmi.SetBatch {
   // implementaton goes here
}
```

An example usage of the cfgplugin might look like:

```go
  b := &gnmi.SetBatch{}
  cfg := cfgplugins.SecondaryIPConfig{
   Entries: []cfgplugins.SecondaryIPEntry{
    {PortName: "port2", PortAttr: dutPort2IP, DumIP: otgPort2IP.IPv4, MagicMAC: magicMac},
    {PortName: "port3", PortAttr: dutPort3IP, DumIP: otgPort3IP.IPv4, MagicMAC: magicMac},
    {PortName: "port4", PortAttr: dutPort4IP, DumIP: otgPort4IP.IPv4, MagicMAC: magicMac},
   },
  }

  cfgplugins.StaticARPWithSecondaryIP(t, dut, b, cfg)
  b.Set(t, dut)
```

## Deviations and cfgplugins

Deviations affecting configuration generation SHOULD be placed into the
relevant `cfgplugins` functions.  This way the deviation is used consistently
across all tests, is easier to discover and maintain compared to the deviations
being implemented in individual tests.  

For example, the sflow plugin includes a minimum sampling rate.  But a
deviation exists for some platforms which do not support the required rate. The
logic to implement the deviation is included in the [sflow
cfgplugin](https://github.com/openconfig/featureprofiles/blob/18559420232e5208a5a75c3557cdc4fc0b70f164/internal/cfgplugins/sflow.go#L49).
