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

package system_ntp_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv4Len: 32,
	}
	loopbackIntf = map[ondatra.Vendor]int{
		ondatra.JUNIPER: 0,
		ondatra.ARISTA:  1,
		ondatra.CISCO:   1,
		ondatra.NOKIA:   1,
	}
)

// TestNtpServerConfigurability validates that NTP servers can be configured on the DUT.
func TestNtpServerConfigurability(t *testing.T) {
	testCases := []struct {
		description string
		addresses   []string
		vrf         string
	}{
		{
			description: "4x IPv4 NTP in default VRF",
			addresses:   []string{"192.0.2.1", "192.0.2.2", "192.0.2.3", "192.0.2.4"},
		},
		{
			description: "4x IPv6 NTP (RFC5952) in default VRF",
			addresses:   []string{"2001:db8::1", "2001:db8::2", "2001:db8::3", "2001:db8::4"},
		},
		{
			description: "4x IPv4 & 4x IPv6 (RFC5952) in default VRF",
			addresses:   []string{"192.0.2.5", "192.0.2.6", "192.0.2.7", "192.0.2.8", "2001:db8::5", "2001:db8::6", "2001:db8::7", "2001:db8::8"},
		},
		{
			description: "4x IPv4 NTP in non-default VRF",
			addresses:   []string{"192.0.2.9", "192.0.2.10", "192.0.2.11", "192.0.2.12"},
			vrf:         "VRF-1",
		},
		{
			description: "4x IPv6 NTP (RFC5952) in non-default VRF",
			addresses:   []string{"2001:db8::9", "2001:db8::a", "2001:db8::b", "2001:db8::c"},
			vrf:         "VRF-1",
		},
		{
			description: "4x IPv4 & 4x IPv6 (RFC5952) in non-default VRF",
			addresses:   []string{"192.0.2.13", "192.0.2.14", "192.0.2.15", "192.0.2.16", "2001:db8::d", "2001:db8::e", "2001:db8::f", "2001:db8::10"},
			vrf:         "VRF-1",
		},
	}

	dut := ondatra.DUT(t, "dut")
	loopbackIntfName := netutil.LoopbackInterface(t, dut, loopbackIntf[dut.Vendor()])
	for _, testCase := range testCases {
		if testCase.vrf != "" && !deviations.NtpNonDefaultVrfUnsupported(dut) {
			createVRF(t, dut, testCase.vrf)
			addLoopbackToVRF(t, dut, testCase.vrf, loopbackIntfName)
		}
	}

	for _, testCase := range testCases {
		// NTP source address is not supported, CLI is used to configure NTP source interface.
		if deviations.NtpSourceAddressUnsupported(dut) {
			t.Run(testCase.description, func(t *testing.T) {
				if testCase.vrf != "" && deviations.NtpNonDefaultVrfUnsupported(dut) {
					t.Skip("NTP non default vrf unsupported")
				}
				for _, address := range testCase.addresses {
					if testCase.vrf != "" {
						ntpServer := fmt.Sprintf("ntp server vrf %s %s version 4 source %s ", testCase.vrf, address, loopbackIntfName)
						helpers.GnmiCLIConfig(t, dut, ntpServer)
					} else {
						ntpServer := fmt.Sprintf("ntp server %s version 4 source %s ", address, loopbackIntfName)
						helpers.GnmiCLIConfig(t, dut, ntpServer)
					}
				}
				ntpPath := gnmi.OC().System().Ntp()
				ntpState := gnmi.Get(t, dut, ntpPath.State())
				for _, address := range testCase.addresses {
					ntpServer := ntpState.GetServer(address)
					if ntpServer == nil {
						t.Errorf("Missing NTP server from NTP state: %s", address)
					}
					if got, want := ntpServer.GetNetworkInstance(), testCase.vrf; want != "" && got != want {
						t.Errorf("Incorrect NTP Server network instance for address %s: got %s, want %s", address, got, want)
					}
				}
			})
		} else {
			t.Run(testCase.description, func(t *testing.T) {
				if testCase.vrf != "" && deviations.NtpNonDefaultVrfUnsupported(dut) {
					t.Skip("NTP non default vrf unsupported")
				}
				ntpPath := gnmi.OC().System().Ntp()

				d := &oc.Root{}

				ntp := d.GetOrCreateSystem().GetOrCreateNtp()
				ntp.SetEnabled(true)
				for _, address := range testCase.addresses {
					server := ntp.GetOrCreateServer(address)
					server.SetSourceAddress(dutlo0Attrs.IPv4)
					if testCase.vrf != "" {
						server.SetNetworkInstance(testCase.vrf)
					}
				}

				gnmi.Replace(t, dut, ntpPath.Config(), ntp)

				ntpState := gnmi.Get(t, dut, ntpPath.State())
				for _, address := range testCase.addresses {
					ntpServer := ntpState.GetServer(address)
					if ntpServer == nil {
						t.Errorf("Missing NTP server from NTP state: %s", address)
					}
					if got, want := ntpServer.GetNetworkInstance(), testCase.vrf; want != "" && got != want {
						t.Errorf("Incorrect NTP Server network instance for address %s: got %s, want %s", address, got, want)
					}
				}
			})
		}
	}
}

// createVRF creates an empty VRF with vrfName on dut.
func createVRF(t *testing.T, dut *ondatra.DUTDevice, vrfName string) {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(vrfName)
	ni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF)

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), ni)
}

func addLoopbackToVRF(t *testing.T, dut *ondatra.DUTDevice, vrfname string, loopbackIntfName string) {
	root := &oc.Root{}
	i := root.GetOrCreateInterface(loopbackIntfName)
	i.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	i.Description = ygot.String(fmt.Sprintf("Port %s", loopbackIntfName))
	si := i.GetOrCreateSubinterface(0)
	si.Enabled = ygot.Bool(true)
	gnmi.Update(t, dut, gnmi.OC().Interface(loopbackIntfName).Config(), i)
}
