/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package intf

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentDevice tests the features of Interface config.
func TestAugmentDevice(t *testing.T) {
	tests := []struct {
		desc       string
		intf       *Interface
		inDevice   *fpoc.Device
		wantDevice *fpoc.Device
	}{{
		desc:     "New Ethernet interface",
		intf:     New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(true),
				},
			},
		},
	}, {
		desc:     "Enabled",
		intf:     New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithEnabled(false),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(false),
				},
			},
		},
	}, {
		desc:     "Forwarding viable",
		intf:     New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithForwardingViable(true),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:             ygot.String("Ethernet1"),
					Description:      ygot.String("Ethernet interface"),
					Type:             fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:          ygot.Bool(true),
					ForwardingViable: ygot.Bool(true),
				},
			},
		},
	}, {
		desc:     "Hold timers",
		intf:     New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithHoldTimers(100*time.Millisecond, 50*time.Millisecond),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(true),
					HoldTime: &fpoc.Interface_HoldTime{
						Up:   ygot.Uint32(100),
						Down: ygot.Uint32(50),
					},
				},
			},
		},
	}, {
		desc:     "MAC address",
		intf:     New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithMACAddress("52:fe:7c:91:6e:c1"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(true),
					Ethernet: &fpoc.Interface_Ethernet{
						MacAddress: ygot.String("52:fe:7c:91:6e:c1"),
					},
				},
			},
		},
	}, {
		desc:     "Port speed",
		intf:     New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithPortSpeed(fpoc.IfEthernet_ETHERNET_SPEED_SPEED_100GB),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(true),
					Ethernet: &fpoc.Interface_Ethernet{
						PortSpeed: fpoc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
					},
				},
			},
		},
	}, {
		desc:     "Duplex mode",
		intf:     New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithDuplexMode(fpoc.IfEthernet_Ethernet_DuplexMode_FULL),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(true),
					Ethernet: &fpoc.Interface_Ethernet{
						DuplexMode: fpoc.IfEthernet_Ethernet_DuplexMode_FULL,
					},
				},
			},
		},
	}, {
		desc:     "Enable flow control",
		intf:     New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithEnableFlowControl(true),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(true),
					Ethernet: &fpoc.Interface_Ethernet{
						EnableFlowControl: ygot.Bool(true),
					},
				},
			},
		},
	}, {
		desc: "Device contains Interface config, no conflicts",
		intf: New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithEnableFlowControl(true),
		inDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(true),
					Ethernet: &fpoc.Interface_Ethernet{
						EnableFlowControl: ygot.Bool(true),
					},
				},
			},
		},
		wantDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(true),
					Ethernet: &fpoc.Interface_Ethernet{
						EnableFlowControl: ygot.Bool(true),
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.intf.AugmentDevice(test.inDevice); err != nil {
				t.Fatalf("error not expected: %v", err)
			}
			if diff := cmp.Diff(test.wantDevice, test.inDevice); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentDeviceErrors tests the error handling of AugmentDevice.
func TestAugmentDeviceErrors(t *testing.T) {
	tests := []struct {
		desc          string
		intf          *Interface
		inDevice      *fpoc.Device
		wantErrSubStr string
	}{{
		desc: "Device contains Interface config with conflicts",
		intf: New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithEnableFlowControl(true),
		inDevice: &fpoc.Device{
			Interface: map[string]*fpoc.Interface{
				"Ethernet1": {
					Name:        ygot.String("Ethernet1"),
					Description: ygot.String("Ethernet interface"),
					Type:        fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					Enabled:     ygot.Bool(true),
					Ethernet: &fpoc.Interface_Ethernet{
						EnableFlowControl: ygot.Bool(false),
					},
				},
			},
		},
		wantErrSubStr: "destination value was set",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.intf.AugmentDevice(test.inDevice)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error sub-string does not match: %v", err)
			}
		})
	}
}

type FakeFeature struct {
	Err           error
	augmentCalled bool
	oc            *fpoc.Interface
}

func (f *FakeFeature) AugmentInterface(oc *fpoc.Interface) error {
	f.oc = oc
	f.augmentCalled = true
	return f.Err
}

// TestWithFeature tests the WithFeature method.
func TestWithFeature(t *testing.T) {
	tests := []struct {
		desc    string
		wantErr error
	}{{
		desc: "error not expected",
	}, {
		desc:    "error expected",
		wantErr: errors.New("some error"),
	}}

	for _, test := range tests {
		i := New("Ethernet1", "Ethernet interface", fpoc.IETFInterfaces_InterfaceType_ethernetCsmacd).WithEnableFlowControl(true)
		ff := &FakeFeature{Err: test.wantErr}
		gotErr := i.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentInterface was not called")
		}
		if ff.oc != &i.oc {
			t.Errorf("Interface ptr is not equal")
		}
		if test.wantErr != nil {
			if gotErr != nil {
				if !strings.Contains(gotErr.Error(), test.wantErr.Error()) {
					t.Errorf("Error strings are not equal: %v", gotErr)
				}
			}
			if gotErr == nil {
				t.Errorf("Expecting error but got none")
			}
		}
	}
}
