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

// Package gribigo_compliance_test invokes the compliance tests from gribigo.
package gribigo_compliance_test

import (
	"strings"
	"testing"

	"flag"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/compliance"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

var (
	skipFIBACK           = flag.Bool("skip_fiback", false, "skip tests that rely on FIB ACK")
	skipSrvReorder       = flag.Bool("skip_reordering", true, "skip tests that rely on server side transaction reordering")
	skipImplicitReplace  = flag.Bool("skip_implicit_replace", true, "skip tests for ADD operations that perform implicit replacement of existing entries")
	skipIdempotentDelete = flag.Bool("skip_idempotent_delete", true, "Skip tests for idempotent DELETE operations")
	skipNonDefaultNINHG  = flag.Bool("skip_non_default_ni_nhg", true, "skip tests that add entries to non-default network-instance")
	skipMPLS             = flag.Bool("skip_mpls", true, "skip tests that add mpls entries")
	skipIPv6             = flag.Bool("skip_ipv6", true, "skip tests that add ipv6 entries")

	nonDefaultNI = flag.String("non_default_ni", "non-default-vrf", "non-default network-instance name")

	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.0",
		IPv4Len: 31,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.2",
		IPv4Len: 31,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.4",
		IPv4Len: 31,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.1",
		IPv4Len: 31,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.3",
		IPv4Len: 31,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.5",
		IPv4Len: 31,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var moreSkipReasons = map[string]string{
	"Get for installed NH - FIB ACK":                                         "NH FIB_ACK is not needed",
	"Flush non-default network instances preserves the default":              "b/232836921",
	"Election - Ensure that a client with mismatched parameters is rejected": "b/233111738",
}

func shouldSkip(tt *compliance.TestSpec) string {
	switch {
	case *skipFIBACK && tt.In.RequiresFIBACK:
		return "This RequiresFIBACK test is skipped by --skip_fiback"
	case *skipSrvReorder && tt.In.RequiresServerReordering:
		return "This RequiresServerReordering test is skipped by --skip_reordering"
	case *skipImplicitReplace && tt.In.RequiresImplicitReplace:
		return "This RequiresImplicitReplace test is skipped by --skip_implicit_replace"
	case *skipNonDefaultNINHG && tt.In.RequiresNonDefaultNINHG:
		return "This RequiresNonDefaultNINHG test is skipped by --skip_non_default_ni_nhg"
	case *skipIdempotentDelete && tt.In.RequiresIdempotentDelete:
		return "This RequiresIdempotentDelete test is skipped by --skip_idempotent_delete"
	case *skipMPLS && tt.In.RequiresMPLS:
		return "This RequiresMPLS test is skipped by --skip_mpls"
	case *skipIPv6 && tt.In.RequiresIPv6:
		return "This RequiresIPv6 test is skipped by --skip_ipv6"
	}
	return moreSkipReasons[tt.In.ShortName]
}

func syncElectionID(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	clientA := &gribi.Client{
		DUT:         dut,
		Persistence: true,
	}
	t.Log("Establish gRIBI client connection with PERSISTENCE set to True")
	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection for clientA could not be established")
	}
	electionID := clientA.LearnElectionID(t)
	compliance.SetElectionID(electionID.Increment().Low)
	clientA.Close(t)
}

func TestCompliance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
	syncElectionID(t, dut)

	ate := ondatra.ATE(t, "ate")
	configureATE(t, ate)

	gribic := dut.RawAPIs().GRIBI(t)

	for _, tt := range compliance.TestSuite {
		t.Run(tt.In.ShortName, func(t *testing.T) {
			if reason := shouldSkip(tt); reason != "" {
				t.Skip(reason)
			}
			compliance.SetDefaultNetworkInstanceName(deviations.DefaultNetworkInstance(dut))
			compliance.SetNonDefaultVRFName(*nonDefaultNI)

			c := fluent.NewClient()
			c.Connection().WithStub(gribic)

			sc := fluent.NewClient()
			sc.Connection().WithStub(gribic)

			opts := []compliance.TestOpt{
				compliance.SecondClient(sc),
			}

			if tt.FatalMsg != "" {
				if got := testt.ExpectFatal(t, func(t testing.TB) {
					tt.In.Fn(c, t, opts...)
				}); !strings.Contains(got, tt.FatalMsg) {
					t.Fatalf("did not get expected fatal error, got: %s, want: %s", got, tt.FatalMsg)
				}
				return
			}

			if tt.ErrorMsg != "" {
				if got := testt.ExpectError(t, func(t testing.TB) {
					tt.In.Fn(c, t, opts...)
				}); !strings.Contains(strings.Join(got, " "), tt.ErrorMsg) {
					t.Fatalf("did not get expected error, got: %s, want: %s", got, tt.ErrorMsg)
				}
				return
			}

			// Any unexpected error will be caught by being called directly on t from the fluent library.
			tt.In.Fn(c, t, opts...)
		})
	}
}

// configureDUT configures port1-3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(*nonDefaultNI)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*nonDefaultNI).Config(), ni)
	nip := gnmi.OC().NetworkInstance(*nonDefaultNI)
	fptest.LogQuery(t, "nonDefaultNI", nip.Config(), gnmi.Get(t, dut, nip.Config()))
}

// configreATE configures port1-3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)
	atePort3.AddToOTG(top, p3, &dutPort3)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	return top
}
