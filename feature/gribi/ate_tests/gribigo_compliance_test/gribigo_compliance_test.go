// Package gribigo_compliance_test invokes the compliance tests from gribigo.
package gribigo_compliance_test

import (
	"flag"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/compliance"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
)

var (
	skipFIBACK          = flag.Bool("skip_fiback", false, "skip tests that rely on FIB ACK")
	skipSrvReorder      = flag.Bool("skip_reordering", true, "skip tests that rely on server side transaction reordering")
	skipImplicitReplace = flag.Bool("skip_implicit_replace", true, "skip tests for ADD operations that perform implicit replacement of existing entries")
	skipNonDefaultNINHG = flag.Bool("skip_non_default_ni_nhg", true, "skip tests that add entries to non-default network-instance")

	defaultNI    = flag.String("default_ni", "default", "default network-instance name")
	nonDefaultNI = flag.String("non_default_ni", "non-default-vrf", "non-default network-instance name")
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
	}
	return moreSkipReasons[tt.In.ShortName]
}

func TestCompliance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	ni := d.GetOrCreateNetworkInstance(*nonDefaultNI)
	ni.Type = telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	ni.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "static")
	dut.Config().NetworkInstance(*nonDefaultNI).Replace(t, ni)

	nip := dut.Config().NetworkInstance(*nonDefaultNI)
	fptest.LogYgot(t, "nonDefaultNI", nip, nip.Get(t))

	gribic := dut.RawAPIs().GRIBI().Default(t)

	for _, tt := range compliance.TestSuite {
		t.Run(tt.In.ShortName, func(t *testing.T) {
			if reason := shouldSkip(tt); reason != "" {
				t.Skip(reason)
			}

			compliance.SetDefaultNetworkInstanceName(*defaultNI)
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
