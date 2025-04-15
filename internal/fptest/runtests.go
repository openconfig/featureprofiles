// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fptest

import (
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/metadata"
	"github.com/openconfig/featureprofiles/internal/pathutil"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygnmi/ygnmi"
)

// RunTests initializes the appropriate binding and runs the tests.
// It should be called from every featureprofiles tests like this:
//
//	package test
//
//	import "github.com/openconfig/featureprofiles/internal/fptest"
//
//	func TestMain(m *testing.M) {
//	  fptest.RunTests(m)
//	}
func RunTests(m *testing.M) {
	if err := initMetadata(); err != nil {
		log.Errorf("Unable to initialize test metadata: %v", err)
	}
	ygnmi.WithDatapointValidator(datapointValidator)
	ondatra.RunTests(m, binding.New)
}

func initMetadata() error {
	if err := metadata.Init(); err != nil {
		return err
	}

	// Set the testbed path from the metadata if it is not set.
	flag.Parse()
	if flagVal := flag.Lookup("testbed").Value; flagVal.String() == "" {
		testbedPath, err := testbedPathFromMetadata()
		if err != nil {
			return err
		}
		if err := flagVal.Set(testbedPath); err != nil {
			return err
		}
		log.Infof("Testbed flag set from metadata to %q", testbedPath)
	}
	return nil
}

func testbedPathFromMetadata() (string, error) {
	testbed := metadata.Get().Testbed
	testbedToFile := map[mpb.Metadata_Testbed]string{
		mpb.Metadata_TESTBED_DUT:                   "dut.testbed",
		mpb.Metadata_TESTBED_DUT_DUT_4LINKS:        "dutdut.testbed",
		mpb.Metadata_TESTBED_DUT_ATE_2LINKS:        "atedut_2.testbed",
		mpb.Metadata_TESTBED_DUT_ATE_4LINKS:        "atedut_4.testbed",
		mpb.Metadata_TESTBED_DUT_ATE_9LINKS_LAG:    "atedut_9_lag.testbed",
		mpb.Metadata_TESTBED_DUT_DUT_ATE_2LINKS:    "dutdutate.testbed",
		mpb.Metadata_TESTBED_DUT_ATE_8LINKS:        "atedut_8.testbed",
		mpb.Metadata_TESTBED_DUT_400ZR:             "dut_400zr.testbed",
		mpb.Metadata_TESTBED_DUT_400ZR_PLUS:        "dut_400zr_plus.testbed",
		mpb.Metadata_TESTBED_DUT_400ZR_100G_4LINKS: "dut_400zr_100g_4links.testbed",
		mpb.Metadata_TESTBED_DUT_400FR_100G_4LINKS: "dut_400fr_100g_4links.testbed",
	}
	testbedFile, ok := testbedToFile[testbed]
	if !ok {
		return "", fmt.Errorf("no testbed file for testbed %v", testbed)
	}
	rootPath, err := pathutil.RootPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, "topologies", testbedFile), nil
}

// datapointValidator is a ygnmi.ValidateFn that validates the timestamp of an input datapoint.
// It is called for each gNMI datapoint (<timestamp, path, value> tuple) received by any test that
// uses the ONDATRA gnmi library.
func datapointValidator(dp *ygnmi.DataPoint) error {
	// Validate the timestamp
	if !dp.Timestamp.IsZero() {
		ns := dp.Timestamp.UnixNano()
		if len(strconv.FormatInt(ns, 10)) != 19 {
			return fmt.Errorf("datapoint timestamp %v does not have nanosecond accuracy", dp.Timestamp)
		}
		if dp.RecvTimestamp.Before(dp.Timestamp) {
			return fmt.Errorf("datapoint receive timestamp %v is before notification timestamp %v", dp.RecvTimestamp, dp.Timestamp)
		}
	}

	return nil
}
