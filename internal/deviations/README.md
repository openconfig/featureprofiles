# Guidelines to add deviations to FNT tests

## When to use deviations

1. Deviations may be created to use alternate OC or use CLI instead of OC to
   achieve the operational intent described in the README.
2. Deviations should not be created which change the operational intent.  See
   below for guidance on changing operational intent.
3. Deviations may be created to change which OC path is used for telemetry or
   even using an implementation's native yang path.  Deviations for telemetry
   should not introduce CLI depedency.
4. As with any pull request (PR), the CODEOWNERs must review and approve (or
   delegate if appropriate).
5. The CODEOWNERs must ensure the README and code reflects the agreed to
   operational support goal.  This may be done via offline discussions or
   directly via approvals in the github PR.

See [Deviation Examples](#deviation-examples) for more information.

## When not to use a deviation

Deviations should not be used to skip configuration or skip validations.  If the
feature is not supported and there is no workaround to achieve
the functionality, then the test should fail for that platform.

If the README is in error, the README can be updated and code can be changed
(without introducing deviation) with approval from the CODEOWNERs.

If the intent of the README needs to be changed (not due to an error, but a
change in the feature request), the CODEOWNER must ensure all parties are
notified.  The CODEOWNER must determine if the change is late or early in the
development cycle.  If late (development is underway and/or nearly complete), it
is recommended to create a new test which represents the change.  If early in
the feature request (development has not started or is very early stage), then
the existing README and code may be updated.

## Adding Deviations

* Add the deviation to the `Deviations` message in the
  [proto/metadata.proto](https://github.com/openconfig/featureprofiles/blob/main/proto/metadata.proto)
  file.

```go
  message Deviations {
    ...
    // Device does not support fragmentation bit for traceroute.
    bool traceroute_fragmentation = 2;
    ...
  }
```

* Run `make proto/metadata_go_proto/metadata.pb.go` from your featureprofiles root directory to generate the Go code for the added proto fields.

```shell
  $ make proto/metadata_go_proto/metadata.pb.go
  mkdir -p proto/metadata_go_proto
  # Set directory to hold symlink
  mkdir -p protobuf-import
  # Remove any existing symlinks & empty directories
  find protobuf-import -type l -delete
  find protobuf-import -type d -empty -delete
  # Download the required dependencies
  go mod download
  # Get ondatra modules we use and create required directory structure
  go list -f 'protobuf-import/{{ .Path }}' -m github.com/openconfig/ondatra | xargs -L1 dirname | sort | uniq | xargs mkdir -p
  go list -f '{{ .Dir }} protobuf-import/{{ .Path }}' -m github.com/openconfig/ondatra | xargs -L1 -- ln -s
  protoc -I='protobuf-import' --proto_path=proto --go_out=./ --go_opt=Mmetadata.proto=proto/metadata_go_proto metadata.proto
  goimports -w proto/metadata_go_proto/metadata.pb.go
```

* Add the accessor function for this deviation to the
  [internal/deviations/deviations.go](https://github.com/openconfig/featureprofiles/blob/main/internal/deviations/deviations.go)
  file. This function will need to accept a parameter `dut` of type
  `*ondatra.DUTDevice` to lookup the deviation value for a specific dut. This
  accessor function must call `lookupDUTDeviations` and return the deviation
  value. Test code will use this function to access deviations.
  * If the default value of the deviation is the same as the default value for
    the proto field, the accessor method can directly call the `Get*()` function
    for the deviation field. For example, the boolean `traceroute_fragmentation`
    deviation, which has a default value of `false`, will have an accessor
    method with the single line `return
    lookupDUTDeviations(dut).GetTracerouteFragmentation()`.

  ```go
   // TraceRouteFragmentation returns if the device does not support fragmentation bit for traceroute.
   // Default value is false.
   func TraceRouteFragmentation(dut *ondatra.DUTDevice) bool {
     return lookupDUTDeviations(dut).GetTracerouteFragmentation()
   }
   ```

  * If the default value of deviation is not the same as the default value of
    the proto field, the accessor method can add a check and return the required
    default value. For example, the accessor method for the float
    `hierarchical_weight_resolution_tolerance` deviation, which has a default
    value of `0`, will call the `GetHierarchicalWeightResolutionTolerance()` to
    check the value set in `metadata.textproto` and return the default value
    `0.2` if applicable.

   ```go
   // HierarchicalWeightResolutionTolerance returns the allowed tolerance for BGP traffic flow while comparing for pass or fail conditions.
   // Default minimum value is 0.2. Anything less than 0.2 will be set to 0.2.
   func HierarchicalWeightResolutionTolerance(dut *ondatra.DUTDevice) float64 {
     hwrt := lookupDUTDeviations(dut).GetHierarchicalWeightResolutionTolerance()
     if minHWRT := 0.2; hwrt < minHWRT {
          return minHWRT
     }
     return hwrt
   }
   ```

* Set the deviation value in the `metadata.textproto` file in the same folder as
  the test. For example, the deviations used in the test
  `feature/gnoi/system/tests/traceroute_test/traceroute_test.go` will be set in
  the file `feature/gnoi/system/tests/traceroute_test/metadata.textproto`. List
  all the vendor and optionally also hardware model regex that this deviation is
  applicable for.

  ```go
  ...
  platform_exceptions: {
    platform: {
      vendor: CISCO
    }
    deviations: {
      traceroute_fragmentation: true
      traceroute_l4_protocol_udp: true
    }
  }
  ...
  ```

* To access the deviation from the test call the accessor function for the
  deviation. Pass the dut to this accessor.

  ```go
  if deviations.TraceRouteFragmentation(dut) {
    ...
  }
  ```

* Example PRs - <https://github.com/openconfig/featureprofiles/pull/1649> and
  <https://github.com/openconfig/featureprofiles/pull/1668>

## Removing Deviations

* Once a deviation is no longer required and removed from all tests, delete the
  deviation by removing them from the following files:

  * metadata.textproto - Remove the deviation field from all metadata.textproto
    in all tests.

  * Remove the accessor method from
    [deviations.go](https://github.com/openconfig/featureprofiles/blob/main/internal/deviations/deviations.go)

  * Remove the field number from
    [metadata.proto](https://github.com/openconfig/featureprofiles/blob/main/proto/metadata.proto)
    by adding the `reserved n` to the `Deviations` message.   Ref:
    <https://protobuf.dev/programming-guides/proto3/#deleting>

* Run `make proto/metadata_go_proto/metadata.pb.go` from your featureprofiles
  root directory to update the Go code for the removed proto fields.

## Deviation examples

```go
conf := configureDUT(dut) // returns *oc.Root

if deviations.AlternateOCEnabled(t, dut) {
    switch dut.Vendor() {
    case ondatra.VENDOR_X:
      conf.SetAlternateOC(val)
    }
} else {
    conf.SetRequiredOC(val)
}
```

```go
conf := configureDUT(dut) // returns *oc.Root

if deviations.RequiredOCNotSupported(t, dut) {
    switch dut.Vendor() {
    case ondatra.VENDOR_X:
        configureDeviceUsingCli(t, dut, vendorXConfig)
    }
}
```

## Notes

* If you run into issues with the `make proto/metadata_go_proto/metadata.pb.go`
  you may need to check if the `protoc` module is installed in your environment.
  Also depending on your Go version you may need to update your PATH and GOPATH.
* After running the `make proto/metadata_go_proto/metadata.pb.go` script, a
  `protobuf-import/` folder will be added in your current directory. Keep an eye
  out for this in case you use `git add .` to add modified files since this
  folder should not be part of your PR.
