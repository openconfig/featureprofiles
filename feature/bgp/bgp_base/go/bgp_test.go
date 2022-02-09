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

package bgp

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// TestNew tests the New function.
func TestNew(t *testing.T) {
	want := &BGP{
		oc: oc.NetworkInstance_Protocol{
			Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
			Name:       ygot.String("bgp"),
		},
	}
	got := New()
	if got == nil {
		t.Fatalf("New returned nil")
	}
	if diff := cmp.Diff(want.oc, got.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithAS tests setting AS for BGP global.
func TestWithAS(t *testing.T) {
	want := oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String("bgp"),
	}
	got := want

	b := &BGP{
		oc: got,
	}

	as := uint32(1234)
	(&want).GetOrCreateBgp().GetOrCreateGlobal().As = ygot.Uint32(as)

	res := b.WithAS(as)
	if res == nil {
		t.Fatalf("WithAS returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestWithRouterID tests setting router-id for BGP global.
func TestWithRouterID(t *testing.T) {
	want := oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String("bgp"),
	}
	got := want

	b := &BGP{
		oc: got,
	}

	routerID := "1.2.3.4"
	(&want).GetOrCreateBgp().GetOrCreateGlobal().RouterId = ygot.String(routerID)

	res := b.WithRouterID(routerID)
	if res == nil {
		t.Fatalf("WithRouterID returned nil")
	}

	if diff := cmp.Diff(want, res.oc); diff != "" {
		t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
	}
}

// TestAugmentNetworkInstance tests the BGP augment to NI OC.
func TestAugmentNetworkInstance(t *testing.T) {
	tests := []struct {
		desc    string
		ni      *oc.NetworkInstance
		wantErr bool
	}{{
		desc: "empty NI",
		ni:   &oc.NetworkInstance{},
	}, {
		desc: "NI contains BGP",
		ni: func() *oc.NetworkInstance {
			p := &oc.NetworkInstance_Protocol{
				Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
				Name:       ygot.String("bgp"),
			}
			ni := &oc.NetworkInstance{}
			if err := ni.AppendProtocol(p); err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			return ni
		}(),
		wantErr: true,
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			l := &BGP{
				oc: oc.NetworkInstance_Protocol{
					Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
					Name:       ygot.String("bgp"),
				},
			}
			dcopy, err := ygot.DeepCopy(test.ni)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			wantNI := dcopy.(*oc.NetworkInstance)

			err = l.AugmentNetworkInstance(test.ni)
			if test.wantErr {
				if err == nil {
					t.Fatalf("error expected")
				}
			} else {
				if err != nil {
					t.Fatalf("error not expected")
				}
				if err := wantNI.AppendProtocol(&l.oc); err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				if diff := cmp.Diff(wantNI, test.ni); diff != "" {
					t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
				}
			}
		})
	}
}

type FakeFeature struct {
	Err           error
	augmentCalled bool
	oc            *oc.NetworkInstance_Protocol_Bgp
}

func (f *FakeFeature) AugmentBGP(oc *oc.NetworkInstance_Protocol_Bgp) error {
	f.oc = oc
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
		b := New().WithRouterID("1.2.3.4")
		ff := &FakeFeature{Err: test.err}
		gotErr := b.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentGlobal was not called")
		}
		if ff.oc != b.oc.GetBgp() {
			t.Errorf("BGP ptr is not equal")
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
	}
}
