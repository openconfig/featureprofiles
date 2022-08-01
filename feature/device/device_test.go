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

package device

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/bgp"
	"github.com/openconfig/featureprofiles/feature/lldp"
	"github.com/openconfig/featureprofiles/feature/networkinstance"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"google.golang.org/protobuf/testing/protocmp"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// TestNew tests the New function.
func TestNew(t *testing.T) {
	d := New()
	if d == nil {
		t.Errorf("New returned nil")
	}
}

// TestDeepCopy tests the DeepCopy method.
func TestDeepCopy(t *testing.T) {
	d := New()
	dc, err := d.DeepCopy()
	if err != nil {
		t.Errorf("DeepCopy returned error %v", err)
	}
	if dc == nil {
		t.Errorf("DeepCopy returned nil")
	}
	// ygot library implements a thorough test for DeepCopy
	// and hence we don't need to repeat that again.
}

// TestMerge tests the Merge method.
func TestMerge(t *testing.T) {
	// Create destination device with some feature.
	dstDevice := New()
	l := lldp.New().EnableInterface("Ethernet1.1")
	if err := dstDevice.WithFeature(l); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	// Create source device with some feature.
	srcDevice := New()
	ni := networkinstance.New("DEFAULT", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	bgp := bgp.New().WithAS(12345)
	if err := ni.WithFeature(bgp); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if err := srcDevice.WithFeature(ni); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	// Merge source device onto destination device.
	if err := dstDevice.Merge(srcDevice); err != nil {
		t.Fatalf("Merge failed with error %v", err)
	}

	// Create wanted device object for comparison.
	wantDevice := New()
	if err := wantDevice.WithFeature(l); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if err := wantDevice.WithFeature(ni); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if diff := cmp.Diff(wantDevice.oc, dstDevice.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
	// ygot library implements a detailed test for MergeStructInto
	// and hence we don't need to repeat that again.
}

// TestFullReplaceRequest tests the FullReplaceRequest method.
func TestFullReplaceRequest(t *testing.T) {
	tests := []struct {
		name     string
		device   *Device
		wantJSON string
	}{{
		name: "empty struct",
		device: func() *Device {
			return New()
		}(),
		wantJSON: "{}",
	}, {
		name: "device with basic LLDP and BGP",
		device: func() *Device {
			d := New()
			ni := networkinstance.New("DEFAULT", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
			bgp := bgp.New().WithAS(12345)
			if err := ni.WithFeature(bgp); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if err := d.WithFeature(ni); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			return d
		}(),
		wantJSON: `{
  "openconfig-network-instance:network-instances": {
    "network-instance": [
      {
        "config": {
          "enabled": true,
          "name": "DEFAULT",
          "type": "openconfig-network-instance-types:DEFAULT_INSTANCE"
        },
        "name": "DEFAULT",
        "protocols": {
          "protocol": [
            {
              "bgp": {
                "global": {
                  "config": {
                    "as": 12345
                  }
                }
              },
              "config": {
                "identifier": "openconfig-policy-types:BGP",
                "name": "bgp"
              },
              "identifier": "openconfig-policy-types:BGP",
              "name": "bgp"
            }
          ]
        }
      }
    ]
  }
}`,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.device.FullReplaceRequest()
			if err != nil {
				t.Fatalf("%s: FullReplaceRequest(%v): got unexpected error: %v", tt.name, tt.device, err)
			}

			want := &gnmipb.SetRequest{
				Replace: []*gnmipb.Update{{
					Path: &gnmipb.Path{
						Origin: "openconfig",
						Elem:   []*gnmipb.PathElem{},
					},
					Val: &gnmipb.TypedValue{
						Value: &gnmipb.TypedValue_JsonIetfVal{
							JsonIetfVal: []byte(tt.wantJSON),
						},
					},
				}},
			}

			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("%s: FullReplaceRequest(%v): did not get expected Notification, diff(-got,+want):%s\n", tt.name, tt.device, diff)
			}
		})
	}
}

// TestFullReplaceRequestErrors tests the FullReplaceRequest method.
func TestFullReplaceRequestErrors(t *testing.T) {
	tests := []struct {
		name          string
		device        *Device
		wantErrSubStr string
	}{{
		name: "interface reference check",
		device: func() *Device {
			d := New()
			l := lldp.New().EnableInterface("Ethernet1.1")
			if err := d.WithFeature(l); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			ni := networkinstance.New("DEFAULT", fpoc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
			bgp := bgp.New().WithAS(12345)
			if err := ni.WithFeature(bgp); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if err := d.WithFeature(ni); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			return d
		}(),
		wantErrSubStr: "/device/lldp/interfaces/interface/config/name is empty set",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.device.FullReplaceRequest()
			if err == nil {
				t.Fatalf("%s: FullReplaceRequest(%v): error expected but got none", tt.name, tt.device)
			}
			if !strings.Contains(err.Error(), tt.wantErrSubStr) {
				t.Errorf("%s: FullReplaceRequest(%v): error does not match expected: %v", tt.name, tt.device, err)
			}
		})
	}
}

type FakeFeature struct {
	Err           error
	augmentCalled bool
	d             *fpoc.Device
}

func (f *FakeFeature) AugmentDevice(d *fpoc.Device) error {
	f.d = d
	f.augmentCalled = true
	return f.Err
}

// TestWithFeature tests the WithFeature method.
func TestWithFeature(t *testing.T) {
	tests := []struct {
		desc string
		err  error
	}{{
		desc: "error not expected",
	}, {
		desc: "error expected",
		err:  errors.New("some error"),
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			d := New()
			ff := &FakeFeature{Err: test.err}
			gotErr := d.WithFeature(ff)
			if !ff.augmentCalled {
				t.Errorf("AugmentDevice was not called")
			}
			if ff.d != &d.oc {
				t.Errorf("Device ptr is not equal")
			}
			if test.err != nil {
				if gotErr != nil {
					if !strings.Contains(gotErr.Error(), test.err.Error()) {
						t.Errorf("Error strings are not equal")
					}
				}
				if gotErr == nil {
					t.Errorf("Expecting error but got none")
				}
			}
		})
	}
}
