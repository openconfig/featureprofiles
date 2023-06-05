## Guidelines to add deviations to FNT tests

* Add the deviation to the `Deviations` message in the [proto/metadata.proto](https://github.com/openconfig/featureprofiles/blob/main/proto/metadata.proto) file.

  ```
  message Deviations {
    ...
    // Device does not support fragmentation bit for traceroute.
    bool traceroute_fragmentation = 2;
    ...
  }
  ```

* Run `make proto/metadata_go_proto/metadata.pb.go` from your featureprofiles root directory to generate the Go code for the added proto fields.

  ```
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

* Add the accessor function for this deviation to the [internal/deviations/deviations.go](https://github.com/openconfig/featureprofiles/blob/main/internal/deviations/deviations.go) file. This function will need to accept a parameter `dut` of type `*ondatra.DUTDevice` to lookup the deviation value for a specific dut. This accessor function must call `lookupDUTDeviations` and return the deviation value. Test code will use this function to access deviations.
	* If the default value of the deviation is the same as the default value for the proto field, the accessor method can directly call the `Get*()` function for the deviation field. For example, the boolean `traceroute_fragmentation` deviation, which has a default value of `false`, will have an accessor method with the single line `return lookupDUTDeviations(dut).GetTracerouteFragmentation()`.

	  ```
	  // TraceRouteFragmentation returns if the device does not support fragmentation bit for traceroute.
	  // Default value is false.
	  func TraceRouteFragmentation(dut *ondatra.DUTDevice) bool {
	    return lookupDUTDeviations(dut).GetTracerouteFragmentation()
	  }
	  ```

	* If the default value of deviation is not the same as the default value of the proto field, the accessor method can add a check and return the required default value. For example, the accessor method for the float `hierarchical_weight_resolution_tolerance` deviation, which has a default value of `0`, will call the `GetHierarchicalWeightResolutionTolerance()` to check the value set in `metadata.textproto` and return the default value `0.2` if applicable.

	  ```
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

* Set the deviation value in the `metadata.textproto` file in the same folder as the test. For example, the deviations used in the test `feature/gnoi/system/tests/traceroute_test/traceroute_test.go` will be set in the file `feature/gnoi/system/tests/traceroute_test/metadata.textproto`. List all the vendor and hardware models that this deviation is applicable for.

  ```
  ...
  platform_exceptions: {
    platform: {
      vendor: CISCO
      hardware_model: 'CISCO-8808'
      hardware_model: 'CISCO-8202-32FH-M'
    }
    deviations: {
      traceroute_fragmentation: true
      traceroute_l4_protocol_udp: true
    }
  }
  ...
  ```

* To access the deviation from the test call the accessor function for the deviation. Pass the dut to this accessor.

  ```
  if deviations.TraceRouteFragmentation(dut) {
    ...
  }
  ```

* Example PRs - https://github.com/openconfig/featureprofiles/pull/1649 and
  https://github.com/openconfig/featureprofiles/pull/1668

## Notes
* If you run into issues with the `make proto/metadata_go_proto/metadata.pb.go` you may need to check if the `protoc` module is installed in your environment. Also depending on your Go version you may need to update your PATH and GOPATH.
* After running the `make proto/metadata_go_proto/metadata.pb.go` script, a `protobuf-import/` folder will be added in your current directory. Keep an eye out for this in case you use `git add .` to add modified files since this folder should not be part of your PR.
