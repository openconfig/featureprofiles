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

// Package setup is scoped only to be used for scripts in path
// feature/experimental/system/gnmi/benchmarking/otg_tests/
// Do not use elsewhere.
package b4_scale_profile_test

import (
	// "slices"
	// "strconv"
	// "context"
	// "os"
	// "strings"
	"testing"
	"time"

	// "github.com/openconfig/featureprofiles/internal/deviations"
	// "github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"

	// "github.com/openconfig/featureprofiles/internal/gribi"
	// "github.com/openconfig/gribigo/fluent"
	// "github.com/openconfig/ondatra"
	// "github.com/openconfig/featureprofiles/internal/gribi"
	// "github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	// "github.com/openconfig/ondatra/gnmi"
)

const (
	nh1ID                     = 120
	nhg1ID                    = 20
	ipv4OuterDest             = "192.51.100.65"
	innerV4DstIP              = "198.18.1.1"
	innerV4SrcIP              = "198.18.0.255"
	innerV6SrcIP              = "2001:DB8::198:1"
	innerV6DstIP              = "2001:DB8:2:0:192::10"
	transitVrfIP              = "203.0.113.1"
	repairedVrfIP             = "203.0.113.100"
	noMatchSrcIP              = "198.100.200.123"
	decapMixPrefix1           = "192.51.128.0/22"
	decapMixPrefix2           = "192.55.200.3/32"
	IPinIPProtocolFieldOffset = 184
	IPinIPProtocolFieldWidth  = 8
	IPinIPpSrcDstIPOffset     = 236
	IPinIPpSrcDstIPWidth      = 12
	IPinIPpDscpOffset         = 120
	IPinIPpDscpWidth          = 8
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGribiScaleProfile(t *testing.T) {
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")
	// dut := ondatra.DUT(t, "dut")
	// otg := ondatra.ATE(t, "ate")
	// // ctx := context.Background()
	// tcArgs := &testArgs{
	// 	dut:  dut,
	// 	ate:  otg,
	// 	topo: topo,
	// }
	configureBaseProfile(t)
}

func TestGoogleBaseConfPush(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	baseConf := "configpushfiles/google_conf.textproto"
	drainConf := "configpushfiles/google_drain_conf.textproto"
	undrainConf := "configpushfiles/google_undrain_conf.textproto"
	// var dutConf string
	// cwd, err := os.Getwd()
	// if err != nil {
	// 	t.Fatalf("Failed to get current working directory: %v", err)
	// }
	// if strings.Contains(cwd, "/featureprofiles/") {
	// 	rootSrc := strings.Split(cwd, "featureprofiles")[0]
	// 	dutConf = rootSrc + "featureprofiles/topologies/cisco/hw/8818-DUT-PEER/DUT_8818-FOX2714PNY_baseconfig.proto"
	// }
	cases := []struct {
		desc           string
		configFilePath string
		clientTimeout  time.Duration
		wantTime       time.Duration
	}{
		{
			desc:           "Initial Google config push",
			configFilePath: drainConf,
			clientTimeout:  10 * time.Minute,
			wantTime:       5 * time.Minute,
		},
		{
			desc:           "Subsequent same google config push",
			configFilePath: baseConf,
			clientTimeout:  10 * time.Minute,
			wantTime:       2 * time.Minute,
		},
		{
			desc:           "Drain config push",
			configFilePath: drainConf,
			clientTimeout:  10 * time.Minute,
			wantTime:       5 * time.Minute,
		},
		{
			desc:           "Undrain config push",
			configFilePath: undrainConf,
			clientTimeout:  10 * time.Minute,
			wantTime:       3 * time.Minute,
		},
		// {
		// 	desc:           "Initial DUT config",
		// 	configFilePath: dutConf,
		// 	clientTimeout:  10 * time.Minute,
		// 	wantTime:       5 * time.Minute,
		// },
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			// Start the timer.
			start := time.Now()
			t.Log("Config Push start time: ", start)
			util.GnmiProtoSetConfigPush(t, dut, tc.configFilePath, tc.clientTimeout)
			// End the timer and calculate time requied to apply the config on DUT.
			elapsedTime := time.Since(start)
			t.Logf("Time taken for %v configuration replace: %v", tc.desc, elapsedTime)
			if elapsedTime > tc.wantTime {
				t.Errorf("Time taken for %v configuration replace is less than expected. Got: %v, Want: %v", tc.desc, elapsedTime, tc.wantTime)
			}
		})
	}
	t.Run("Config Push after LC reload", func(t *testing.T) {
	})
}
