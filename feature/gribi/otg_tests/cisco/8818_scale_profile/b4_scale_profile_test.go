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
	"context"
	"fmt"
	"sync"
	"strings"

	// "os"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"

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

// func TestGribiScaleProfile(t *testing.T) {
// 	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")
// 	// dut := ondatra.DUT(t, "dut")
// 	// otg := ondatra.ATE(t, "ate")
// 	// // ctx := context.Background()
// 	// tcArgs := &testArgs{
// 	// 	dut:  dut,
// 	// 	ate:  otg,
// 	// 	topo: topo,
// 	// }
// 	configureBaseProfile(t)
// }

func TestGoogleBaseConfPush(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// baseConf := "configpushfiles/google_conf.textproto"
	// test := "configpushfiles/set1.textproto"
	// // drainConf := "configpushfiles/google_drain_conf.textproto"
	// // undrainConf := "configpushfiles/google_undrain_conf.textproto"
	// // var dutConf string
	// // cwd, err := os.Getwd()
	// // if err != nil {
	// // 	t.Fatalf("Failed to get current working directory: %v", err)
	// // }
	// // if strings.Contains(cwd, "/featureprofiles/") {
	// // 	rootSrc := strings.Split(cwd, "featureprofiles")[0]
	// // 	dutConf = rootSrc + "featureprofiles/topologies/cisco/hw/8818-DUT-PEER/DUT_8818-FOX2714PNY_baseconfig.proto"
	// // }
	// cases := []struct {
	// 	desc           string
	// 	configFilePath string
	// 	clientTimeout  time.Duration
	// 	wantTime       time.Duration
	// }{
	// 	{
	// 		desc:           "Initial Google config push",
	// 		configFilePath: test,
	// 		clientTimeout:  10 * time.Minute,
	// 		wantTime:       5 * time.Minute,
	// 	},
		// {
		// 	desc:           "Subsequent same google config push",
		// 	configFilePath: baseConf,
		// 	clientTimeout:  10 * time.Minute,
		// 	wantTime:       2 * time.Minute,
		// },
		// {
		// 	desc:           "Drain config push",
		// 	configFilePath: drainConf,
		// 	clientTimeout:  10 * time.Minute,
		// 	wantTime:       5 * time.Minute,
		// },
		// {
		// 	desc:           "Undrain config push",
		// 	configFilePath: undrainConf,
		// 	clientTimeout:  10 * time.Minute,
		// 	wantTime:       3 * time.Minute,
		// },
		// {
		// 	desc:           "Initial DUT config",
		// 	configFilePath: dutConf,
		// 	clientTimeout:  10 * time.Minute,
		// 	wantTime:       5 * time.Minute,
		// },
	// }
	// for _, tc := range cases {
	// 	t.Run(tc.desc, func(t *testing.T) {
	// 		// Start the timer.
	// 		start := time.Now()
	// 		t.Log("Config Push start time: ", start)
	// 		util.GnmiProtoSetConfigPush(t, dut, tc.configFilePath, tc.clientTimeout)
	// 		// End the timer and calculate time requied to apply the config on DUT.
	// 		elapsedTime := time.Since(start)
	// 		t.Logf("Time taken for %v configuration replace: %v", tc.desc, elapsedTime)
	// 		if elapsedTime > tc.wantTime {
	// 			t.Errorf("Time taken for %v configuration replace is less than expected. Got: %v, Want: %v", tc.desc, elapsedTime, tc.wantTime)
	// 		}
	// 	})
	// }
	// t.Run("Config Push after LC reload", func(t *testing.T) {
// 	// })
	t.Run("Config Push after chassis reboot followed by Switchover", func(t *testing.T) {
		t.Logf("Doing chassis reboot")
		gnoiClient := dut.RawAPIs().GNOI(t)
		_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot chassis without delay",
			Force:   false,
		})
		if err != nil {
			t.Fatalf("Reboot failed %v", err)
		}
		time.Sleep(350 * time.Second)
		t.Logf("Check for cfgmgr LC restore config sessions started")
		cliHandle := dut.RawAPIs().CLI(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Minute)
		defer cancel()
		for {
			showConfigSession, _ := cliHandle.RunCommand(ctx, "show configuration sessions detail")
			fmt.Println(showConfigSession.Output())
			// if err != nil {
			// 	t.Error(err)
			// }
			if strings.Contains(showConfigSession.Output(), "Client: cfgmgr-req-mgr") {
				t.Logf("Cfgmgr restore session has started")
				break
			}
		}
		time.Sleep(5 * time.Second)
		//Active RP0 Reload
		useNameOnly := deviations.GNOISubcomponentPath(dut)
		rebootSubComponentRequest := &spb.RebootRequest{
			Method: spb.RebootMethod_COLD,
			Subcomponents: []*tpb.Path{
				components.GetSubcomponentPath("0/RP0/CPU0", useNameOnly),
			},
		}
		t.Logf("Initiate Active RP0 reboot: %v", rebootSubComponentRequest)
		rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
		if err != nil {
			t.Fatalf("Failed to perform component reboot with unexpected err: %v", err)
		}
		t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)
		time.Sleep(3 * time.Minute)
		var wg sync.WaitGroup
		wg.Add(1)
		// go func() {
		// 	defer wg.Done()
		// 	t.Log("Start Google config push")
		// 	util.GnmiProtoSetConfigPush(t, dut, baseConf, 10*time.Minute)
		// }()

		go func() {
			defer wg.Done()
			t.Log("Check for cfgmgr LC restore config sessions & lock state")
			cliHandle := dut.RawAPIs().CLI(t)
			startTime := time.Now()
			duration := 20 * time.Minute
			var iter int
			for time.Since(startTime) < duration  {
				iter = iter + 1
				showConfigLock, err1 := cliHandle.RunCommand((context.Background()), "show configuration lock")
				showConfigSession, err2 := cliHandle.RunCommand((context.Background()), "show configuration sessions detail")

				fmt.Println(showConfigLock.Output())
				fmt.Println(showConfigSession.Output())
				time.Sleep(5 * time.Second)
				if err1 != nil && err2 != nil {
					t.Log(err)
				}
				if iter > 180 {
					t.Errorf("Failed to get out of lock state")
					break
				}
				if !strings.Contains(showConfigLock.Output(),"lock_subtree") && !strings.Contains(showConfigSession.Output(), "Client: cfgmgr-req-mgr") {
					t.Logf("No config session is in lock state")
					break
				}
			}

		}()
		wg.Wait() // Wait for all four goroutines to finish before exiting.
	})
}
