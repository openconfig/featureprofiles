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

package aft_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen         = 30
	ipv6PrefixLen         = 126
	instance              = "DEFAULT"
	dstPfx                = "198.51.100.1"
	mask                  = "32"
	dstPfxMin             = "198.51.100.1"
	dstPfxCount           = 100
	dstPfx1               = "11.1.1.1"
	dstPfxCount1          = 10
	innersrcPfx           = "200.1.0.1"
	innerdstPfxMin_bgp    = "202.1.0.1"
	innerdstPfxCount_bgp  = 100
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 100
)

const (
	linecardType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx    context.Context
	client *gribi.Client
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    *ondatra.ATETopology
}

const (
	oneMinuteInNanoSecond = 6e10
	oneSecondInNanoSecond = 1e9
	rebootDelay           = 120
	// Maximum reboot time is 900 seconds (15 minutes).
	maxRebootTime = 900
	// Maximum wait time for all components to be in responsive state
	maxCompWaitTime = 600
)

func chassisReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	cases := []struct {
		desc          string
		rebootRequest *spb.RebootRequest
	}{
		{
			desc: "without delay",
			rebootRequest: &spb.RebootRequest{
				Method:  spb.RebootMethod_COLD,
				Delay:   0,
				Message: "Reboot chassis without delay",
				Force:   true,
			}},
	}

	versions := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().SoftwareVersion().State())
	expectedVersion := FetchUniqueItems(t, versions)
	sort.Strings(expectedVersion)
	t.Logf("DUT software version: %v", expectedVersion)

	preRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())
	t.Logf("DUT components status pre reboot: %v", preRebootCompStatus)

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			gnoiClient := dut.RawAPIs().GNOI().New(t)
			bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
			t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
			prevTime, err := time.Parse(time.RFC3339, gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State()))
			if err != nil {
				t.Fatalf("Failed parsing current-datetime: %s", err)
			}
			start := time.Now()

			t.Logf("Send reboot request: %v", tc.rebootRequest)
			rebootResponse, err := gnoiClient.System().Reboot(context.Background(), tc.rebootRequest)
			t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
			if err != nil {
				t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
			}

			if tc.rebootRequest.GetDelay() > 1 {
				t.Logf("Validating DUT remains reachable for at least %d seconds", rebootDelay)
				for {
					time.Sleep(10 * time.Second)
					t.Logf("Time elapsed %.2f seconds since reboot was requested.", time.Since(start).Seconds())
					if uint64(time.Since(start).Seconds()) > rebootDelay {
						t.Logf("Time elapsed %.2f seconds > %d reboot delay", time.Since(start).Seconds(), rebootDelay)
						break
					}
					latestTime, err := time.Parse(time.RFC3339, gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State()))
					if err != nil {
						t.Fatalf("Failed parsing current-datetime: %s", err)
					}
					if latestTime.Before(prevTime) || latestTime.Equal(prevTime) {
						t.Errorf("Get latest system time: got %v, want newer time than %v", latestTime, prevTime)
					}
					prevTime = latestTime
				}
			}

			startReboot := time.Now()
			t.Logf("Wait for DUT to boot up by polling the telemetry output.")
			for {
				var currentTime string
				t.Logf("Time elapsed %.2f seconds since reboot started.", time.Since(startReboot).Seconds())
				time.Sleep(30 * time.Second)
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
				}); errMsg != nil {
					t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
				} else {
					t.Logf("Device rebooted successfully with received time: %v", currentTime)
					break
				}

				if uint64(time.Since(startReboot).Seconds()) > maxRebootTime {
					t.Errorf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
				}
			}
			t.Logf("Device boot time: %.2f seconds", time.Since(startReboot).Seconds())

			bootTimeAfterReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
			t.Logf("DUT boot time after reboot: %v", bootTimeAfterReboot)
			if bootTimeAfterReboot <= bootTimeBeforeReboot {
				t.Errorf("Get boot time: got %v, want > %v", bootTimeAfterReboot, bootTimeBeforeReboot)
			}

			startComp := time.Now()
			t.Logf("Wait for all the components on DUT to come up")

			for {
				postRebootCompStatus := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().OperStatus().State())

				if len(preRebootCompStatus) == len(postRebootCompStatus) {
					t.Logf("All components on the DUT are in responsive state")
					time.Sleep(10 * time.Second)
					break
				}

				if uint64(time.Since(startComp).Seconds()) > maxCompWaitTime {
					t.Logf("DUT components status post reboot: %v", postRebootCompStatus)
					t.Fatalf("All the components are not in responsive state post reboot")
				}
				time.Sleep(10 * time.Second)
			}

			versions = gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().SoftwareVersion().State())
			swVersion := FetchUniqueItems(t, versions)
			sort.Strings(swVersion)
			t.Logf("DUT software version after reboot: %v", swVersion)
			if diff := cmp.Diff(expectedVersion, swVersion); diff != "" {
				t.Errorf("Software version differed (-want +got):\n%v", diff)
			}
		})
	}
}

func FetchUniqueItems(t *testing.T, s []string) []string {
	itemExisted := make(map[string]bool)
	var uniqueList []string
	for _, item := range s {
		if _, ok := itemExisted[item]; !ok {
			itemExisted[item] = true
			uniqueList = append(uniqueList, item)
		} else {
			t.Logf("Detected duplicated item: %v", item)
		}
	}
	return uniqueList
}

func aftCheck(ctx context.Context, t *testing.T, args *testArgs) {

	ipv4prefix := "192.0.2.40/32"
	nhlist, nexthopgroup := getaftnh(t, args.dut, ipv4prefix, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)
	nexthop := nhlist[0]

	ipv4prefix_nondefault := "198.51.100.1/32"
	_, nexthopgroup_nondefault := getaftnh(t, args.dut, ipv4prefix_nondefault, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)

	// Telemerty check
	t.Run("Telemetry on AFT TOP Container", func(t *testing.T) {
		gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(instance).Afts().State())
	})
	t.Run("Telemetry on Ipv4Entry", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix)
		ipv4entry := gnmi.Get(t, args.dut, path.State())
		if *ipv4entry.Prefix != ipv4prefix {
			t.Errorf("Incorrect value for AFT Ipv4Entry Prefix got %s, want %s", *ipv4entry.Prefix, ipv4prefix)
		}
		if *ipv4entry.NextHopGroup != nexthopgroup {
			t.Errorf("Incorrect value for NextHopGroup , got:%v,want:%v", *ipv4entry.NextHopGroup, nexthopgroup)
		}
	})
	t.Run("Telemetry on Ipv4Entry NextHopGroup", func(t *testing.T) {
		path := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).State())
		nhgvalue := path.GetNextHopGroup()
		if nhgvalue != nexthopgroup {
			t.Errorf("Incorrect value for NextHopGroup , got:%v,want:%v", nhgvalue, nexthopgroup)
		}
	})
	t.Run("Telemetry on Ipv4Entry Prefix", func(t *testing.T) {
		path := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).State())
		prefixvalue := path.GetPrefix()
		if prefixvalue != ipv4prefix {
			t.Errorf("Incorrect value for AFT Ipv4Entry Prefix got %s, want %s", prefixvalue, ipv4prefix)
		}
	})

	// NOT-SUPPORTED
	// t.Run("Telemetry on Ipv4Entry Prefix", func(t *testing.T) {
	// 	path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).EntryMetadata()
	// 	path.Get(t)
	// })
	t.Run("Telemetry on Ipv4Entry NextHopGroupNetworkInstance", func(t *testing.T) {
		path := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).State())
		path.GetNextHopGroupNetworkInstance()
	})
	// NOT-SUPPORTED
	// t.Run("Telemetry on Ipv4Entry DecapsulateHeader", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).DecapsulateHeader().Get(t)
	// })
	// t.Run("Telemetry on Ipv4Entry OctetsForwarded", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).Counters().OctetsForwarded().Get(t)
	// })
	// t.Run("Telemetry on Ipv4Entry PacketsForwarded", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).Counters().PacketsForwarded().Get(t)
	// })
	// t.Run("Telemetry on Ipv4Entry OriginProtocol", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).OriginProtocol().Get(t)
	// })
	// t.Run("Telemetry on Ipv4Entry OriginNetworkInstance", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).OriginNetworkInstance().Get(t)
	// })

	t.Run("Telemetry on NextHopGroup", func(t *testing.T) {
		aftNHG := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).State())
		if got := len(aftNHG.NextHop); got != 4 {
			t.Fatalf("Prefix %s next-hop entry count: got %d, want 4", dstPfx, got)
		}
	})
	t.Run("Telemetry on NextHopGroup Id", func(t *testing.T) {
		path := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).State())
		value := path.GetId()
		t.Logf("NextHopGroup Id Value %d", value)
		if value == 0 {
			t.Errorf("Incorrect value for NextHopGroup Id  got %d, want non zero value", value)
		}
	})
	t.Run("Telemetry on NextHopGroup NextHopAny", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup)
		gnmi.Get(t, args.dut, path.State())
	})

	t.Run("Telemetry on NextHopGroup NextHop", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup)
		value := gnmi.Get(t, args.dut, path.State())
		t.Logf("NextHopGroup NextHop Value: %d", value.GetNextHop(nexthop).GetIndex())
	})
	t.Run("Telemetry on NextHopGroup NextHop Index", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup)
		value := gnmi.Get(t, args.dut, path.State())
		t.Logf("NextHopGroup NextHop Index Value: %d", value.GetNextHop(nexthop).GetIndex())
	})
	t.Run("Telemetry on NextHopGroup NextHop Weight", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup)
		value := gnmi.Get(t, args.dut, path.State())
		t.Logf("NextHopGroup NextHop Weight Value: %d", value.GetNextHop(nexthop).GetWeight())
	})
	t.Run("Telemetry on NextHopGroup BackupNextHopGroup", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup_nondefault)
		value := gnmi.Get(t, args.dut, path.State()).GetBackupNextHopGroup()
		t.Logf("Value %d", value)
		nhg := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(value).State())
		t.Logf("BackupNextHopGroup ProgrammedId VALUE: %d", nhg.GetProgrammedId())
		if nhg.GetProgrammedId() != 101 {
			t.Errorf("Incorrect value for BackupNextHopGroup ProgrammedId  got %d, want 101", value)
		}
	})
	// NOT-SUPPORTED
	// t.Run("Telemetry on NextHopGroup Color", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).Color().Get(t)
	// })

	t.Run("Telemetry on NextHop", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop)
		value := gnmi.Get(t, args.dut, path.State())
		t.Logf("NextHop Value: %v", value)
	})
	t.Run("Telemetry on NextHop Index", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop)
		value := gnmi.Get(t, args.dut, path.State()).GetIndex()
		if value == 0 {
			t.Errorf("Incorrect value for NextHop Index  got %d, want non zero value", value)
		}
	})

	ipv4prefix = "192.0.2.50/32"
	nhlist, nexthopgroup = getaftnh(t, args.dut, ipv4prefix, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)
	nexthop_interfaceref := nhlist[0]

	ipv4prefix = "192.0.2.51/32"
	nhlist, nexthopgroup = getaftnh(t, args.dut, ipv4prefix, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)
	nexthop_ipinip := nhlist[0]

	p8 := args.dut.Port(t, "port8")
	interfaceref_name := p8.Name()
	t.Run("Telemetry on NextHop InterfaceRef(main interface)", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop_interfaceref)
		value := gnmi.Get(t, args.dut, path.State()).GetInterfaceRef()
		if value.GetInterface() != interfaceref_name {
			t.Errorf("Incorrect value for NextHop InterfaceRef  got %s, want %s", value.GetInterface(), interfaceref_name)
		}
	})

	t.Run("Telemetry on NextHop InterfaceRef Interface", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop_interfaceref)
		value := gnmi.Get(t, args.dut, path.State()).GetInterfaceRef().GetInterface()
		if value != interfaceref_name {
			t.Errorf("Incorrect value for NextHop InterfaceRef  Interface got %s, want %s", value, interfaceref_name)
		}
	})

	ipv4prefix = "192.0.2.52/32"
	nhlist, nexthopgroup = getaftnh(t, args.dut, ipv4prefix, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)
	nexthop_subinterfaceref := nhlist[0]

	t.Run("Telemetry on NextHop InterfaceRef(subinterface)", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop_subinterfaceref)
		value := gnmi.Get(t, args.dut, path.State()).GetInterfaceRef()
		if value.GetSubinterface() != 1 {
			t.Errorf("Incorrect value for InterfaceRef Subinterface  got %d, want %d", value.GetSubinterface(), 1)
		}
	})
	t.Run("Telemetry on NextHop InterfaceRef Subinterface", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop_subinterfaceref)
		value := gnmi.Get(t, args.dut, path.State()).GetInterfaceRef().GetSubinterface()
		if value != 1 {
			t.Errorf("Incorrect value for InterfaceRef Subinterface  got %d, want %d", value, 1)
		}
	})
	// NOT-SUPPORTED
	// t.Run("Telemetry on NextHop EncapsulateHeader", func(t *testing.T) {
	// 	path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).EncapsulateHeader()
	// 	value := path.Get(t)
	// 	t.Logf("NextHop EncapsulateHeader Value: %d", value)
	// })
	// t.Run("Telemetry on NextHop DecapsulateHeader", func(t *testing.T) {
	// 	path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).DecapsulateHeader()
	// 	value := path.Get(t)
	// 	t.Logf("NextHop DecapsulateHeader Value: %d", value)
	// })
	t.Run("Telemetry on NextHop IpAddress", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop)
		value := gnmi.Get(t, args.dut, path.State()).GetIpAddress()
		if !strings.Contains(value, "192") {
			t.Errorf("Incorrect value for NextHop IpAddress  got %s, want an ip address in range 192.x", value)
		}
	})
	// NOT-SUPPORTED
	// t.Run("Telemetry on NextHop MacAddress", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).MacAddress().Get(t)
	// })
	// t.Run("Telemetry on NextHop OriginProtocol", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).OriginProtocol().Get(t)
	// })
	// t.Run("Telemetry on NextHop PushedMplsLabelStack", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).PushedMplsLabelStack().Get(t)
	// })

	t.Run("Telemetry on NextHop ProgrammedIndex", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop)
		value := gnmi.Get(t, args.dut, path.State()).GetProgrammedIndex()
		if value == 0 {
			t.Errorf("Incorrect value for NextHop ProgrammedIndex  got %d, want non-zero", value)
		}
	})
	t.Run("Telemetry on NextHopGroup ProgrammedId", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup)
		value := gnmi.Get(t, args.dut, path.State()).GetProgrammedId()
		if value != 5002 {
			t.Errorf("Incorrect value for NextHopGroup ProgrammedId  got %d, want %d", value, 5002)
		}
	})
	t.Run("Telemetry on NextHop IpInIp", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop_ipinip)
		value := gnmi.Get(t, args.dut, path.State()).GetIpInIp()
		if value.GetDstIp() != "10.10.10.1" {
			t.Errorf("Incorrect value for  NextHop IpInIp DstIp got %s, want %s", value.GetDstIp(), "10.10.10.1")
		}
		if value.GetSrcIp() != "20.20.20.1" {
			t.Errorf("Incorrect value for  NextHop IpInIp SrcIp  got %s, want %s", value.GetSrcIp(), "20.20.20.1")
		}
	})
	t.Run("Telemetry on NextHop IpInIp SrcIp", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop_ipinip)
		value := gnmi.Get(t, args.dut, path.State()).GetIpInIp().GetSrcIp()
		if value != "20.20.20.1" {
			t.Errorf("Incorrect value for  NextHop IpInIp SrcIp  got %s, want %s", value, "20.20.20.1")
		}
	})
	t.Run("Telemetry on NextHop IpInIp DstIp", func(t *testing.T) {
		path := gnmi.OC().NetworkInstance(instance).Afts().NextHop(nexthop_ipinip)
		value := gnmi.Get(t, args.dut, path.State()).GetIpInIp().GetDstIp()
		if value != "10.10.10.1" {
			t.Errorf("Incorrect value for  NextHop IpInIp DstIp got %s, want %s", value, "10.10.10.1")
		}
	})
}

func testAFT(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	ciscoFlags.GRIBIChecks.AFTChainCheck = false
	ciscoFlags.GRIBIChecks.AFTCheck = false
	ciscoFlags.GRIBIChecks.FIBACK = true
	ciscoFlags.GRIBIChecks.RIBACK = true

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to decap
	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// NH WithInterfaceRef
	p8 := args.dut.Port(t, "port8")
	interfaceref_name := p8.Name()
	args.client.AddNH(t, 5000, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, interfaceref_name, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 5000, 0, map[uint64]uint64{5000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.50/32", 5000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// NH WithIPinIP
	args.client.AddNHWithIPinIP(t, 5001, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", true, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 5001, 0, map[uint64]uint64{5001: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.51/32", 5001, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// NH WithSubinterfaceRef
	args.client.AddNHWithIPinIP(t, 5002, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether1", false, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 5002, 0, map[uint64]uint64{5002: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.52/32", 5002, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Telemerty check
	aftCheck(ctx, t, args)

	// REPLACE
	args.client.ReplaceNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.ReplaceNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Telemerty check after REPLACE
	aftCheck(ctx, t, args)

}

func TestOCAFT(t *testing.T) {
	t.Log("Name: OCAFT")
	t.Log("Description: Verify OC AFT gNMI Subscribe")

	dut := ondatra.DUT(t, "dut")
	resp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
	t.Logf(resp)
	if strings.Contains(resp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}

	// Dial gRIBI
	ctx := context.Background()

	// Configure the DUT
	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	util.GNMIWithText(ctx, t, dut, configToChange)
	configToChange = "hw-module profile pbr vrf-redirect \n"
	util.GNMIWithText(ctx, t, dut, configToChange)
	lcs := components.FindComponentsByType(t, dut, linecardType)
	t.Logf("Found linecard list: %v", lcs)

	if got := len(lcs); got == 0 {
		configToChange = "hw-module profile netflow sflow-enable location 0/RP0/CPU0 \n"
		util.GNMIWithText(ctx, t, dut, configToChange)
	} else {
		for _, lc := range lcs {
			configToChange = fmt.Sprintf("hw-module profile netflow sflow-enable location %s \n", lc)
			util.GNMIWithText(ctx, t, dut, configToChange)
		}
	}
	configureDUT(t, dut)
	configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	defer unconfigbasePBR(t, dut)

	chassisReboot(t)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	addPrototoAte(t, top)

	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{
		{
			name: "AFT Verification",
			desc: "AFT Verification with base use case",
			fn:   testAFT,
		},
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			// Configure the gRIBI client client
			client := gribi.Client{
				DUT:                   dut,
				FibACK:                *ciscoFlags.GRIBIFIBCheck,
				Persistence:           true,
				InitialElectionIDLow:  10,
				InitialElectionIDHigh: 0,
			}
			defer client.Close(t)
			if err := client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}
			args := &testArgs{
				ctx:    ctx,
				client: &client,
				dut:    dut,
				ate:    ate,
				top:    top,
			}
			tt.fn(ctx, t, args)
		})
	}
}
