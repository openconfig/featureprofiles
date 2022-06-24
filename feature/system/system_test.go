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

package system

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// TestAugmentDevice tests the System augment to device OC.
func TestAugmentDevice(t *testing.T) {
	tests := []struct {
		desc       string
		system     *System
		inDevice   *fpoc.Device
		wantDevice *fpoc.Device
	}{{
		desc:     "System with no params",
		system:   New(),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{},
		},
	}, {
		desc:     "System with hostname",
		system:   New().WithHostname("foobar"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{
				Hostname: ygot.String("foobar"),
			},
		},
	}, {
		desc:     "System with domain-name",
		system:   New().WithDomainName("foobar"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{
				DomainName: ygot.String("foobar"),
			},
		},
	}, {
		desc:     "System with login-banner",
		system:   New().WithLoginBanner("foobar"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{
				LoginBanner: ygot.String("foobar"),
			},
		},
	}, {
		desc:     "System with motd-banner",
		system:   New().WithMOTDBanner("foobar"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{
				MotdBanner: ygot.String("foobar"),
			},
		},
	}, {
		desc:     "System with timezone-name",
		system:   New().WithTimezoneName("foobar"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{
				Clock: &fpoc.System_Clock{
					TimezoneName: ygot.String("foobar"),
				},
			},
		},
	}, {
		desc:     "Add user with ssh key",
		system:   New().AddUserWithSSHKey("user", "user-key"),
		inDevice: &fpoc.Device{},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{
				Aaa: &fpoc.System_Aaa{
					Authentication: &fpoc.System_Aaa_Authentication{
						User: map[string]*fpoc.System_Aaa_Authentication_User{
							"user": {
								Username: ygot.String("user"),
								SshKey:   ygot.String("user-key"),
							},
						},
					},
				},
			},
		},
	}, {
		desc:   "System with non-conflicting users",
		system: New().AddUserWithSSHKey("user", "user-key"),
		inDevice: &fpoc.Device{
			System: &fpoc.System{
				Aaa: &fpoc.System_Aaa{
					Authentication: &fpoc.System_Aaa_Authentication{
						User: map[string]*fpoc.System_Aaa_Authentication_User{
							"user2": {
								Username: ygot.String("user2"),
								SshKey:   ygot.String("user2-key"),
							},
						},
					},
				},
			},
		},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{
				Aaa: &fpoc.System_Aaa{
					Authentication: &fpoc.System_Aaa_Authentication{
						User: map[string]*fpoc.System_Aaa_Authentication_User{
							"user": {
								Username: ygot.String("user"),
								SshKey:   ygot.String("user-key"),
							},
							"user2": {
								Username: ygot.String("user2"),
								SshKey:   ygot.String("user2-key"),
							},
						},
					},
				},
			},
		},
	}, {
		desc:   "System with same user twice",
		system: New().AddUserWithSSHKey("user", "user-key").AddUserWithSSHKey("user", "user-key"),
		inDevice: &fpoc.Device{
			System: &fpoc.System{},
		},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{
				Aaa: &fpoc.System_Aaa{
					Authentication: &fpoc.System_Aaa_Authentication{
						User: map[string]*fpoc.System_Aaa_Authentication_User{
							"user": {
								Username: ygot.String("user"),
								SshKey:   ygot.String("user-key"),
							},
						},
					},
				},
			},
		},
	}, {
		desc:   "Add user when user already exists",
		system: New().AddUserWithSSHKey("user", "user-key"),
		inDevice: &fpoc.Device{
			System: &fpoc.System{
				Aaa: &fpoc.System_Aaa{
					Authentication: &fpoc.System_Aaa_Authentication{
						User: map[string]*fpoc.System_Aaa_Authentication_User{
							"user": {
								Username: ygot.String("user"),
								SshKey:   ygot.String("user-key"),
							},
						},
					},
				},
			},
		},
		wantDevice: &fpoc.Device{
			System: &fpoc.System{
				Aaa: &fpoc.System_Aaa{
					Authentication: &fpoc.System_Aaa_Authentication{
						User: map[string]*fpoc.System_Aaa_Authentication_User{
							"user": {
								Username: ygot.String("user"),
								SshKey:   ygot.String("user-key"),
							},
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := test.system.AugmentDevice(test.inDevice); err != nil {
				t.Fatalf("error not expected")
			}
			if diff := cmp.Diff(test.wantDevice, test.inDevice); diff != "" {
				t.Errorf("did not get expected state, diff(-want,+got):\n%s", diff)
			}
		})
	}
}

// TestAugmentDeviceErrors tests the System augment to Device OC validation.
func TestAugmentDeviceErrors(t *testing.T) {
	tests := []struct {
		desc          string
		system        *System
		inDevice      *fpoc.Device
		wantErrSubStr string
	}{{
		desc:   "Device contains System OC with conflicts",
		system: New().WithHostname("foo"),
		inDevice: &fpoc.Device{
			System: &fpoc.System{
				Hostname: ygot.String("bar"),
			},
		},
		wantErrSubStr: "destination value was set, but was not equal to source value",
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.system.AugmentDevice(test.inDevice)
			if err == nil {
				t.Fatalf("error expected")
			}
			if !strings.Contains(err.Error(), test.wantErrSubStr) {
				t.Errorf("Error strings don't match: %v", err)
			}
		})
	}
}

type FakeFeature struct {
	Err           error
	augmentCalled bool
	oc            *fpoc.System
}

func (f *FakeFeature) AugmentSystem(oc *fpoc.System) error {
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
		s := New().WithHostname("foobar")
		ff := &FakeFeature{Err: test.wantErr}
		gotErr := s.WithFeature(ff)
		if !ff.augmentCalled {
			t.Errorf("AugmentDevice was not called")
		}
		if ff.oc != &s.oc {
			t.Errorf("System ptr is not equal")
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
