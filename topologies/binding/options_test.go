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

package binding

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
)

func TestMerge(t *testing.T) {
	cases := []struct {
		name string
		args []*bindpb.Options
		want *bindpb.Options
	}{{
		name: "One",
		args: []*bindpb.Options{{
			Target:     "target",
			Insecure:   true,
			SkipVerify: true,
			Username:   "username",
			Password:   "password",
		}},
		want: &bindpb.Options{
			Target:     "target",
			Insecure:   true,
			SkipVerify: true,
			Username:   "username",
			Password:   "password",
		},
	}, {
		name: "Disjoint",
		args: []*bindpb.Options{{
			Target:   "target",
			Insecure: true,
			Username: "username",
		}, {
			SkipVerify: true,
			Password:   "password",
		}},
		want: &bindpb.Options{
			Target:     "target",
			Insecure:   true,
			SkipVerify: true,
			Username:   "username",
			Password:   "password",
		},
	}, {
		name: "MultipleOverride",
		args: []*bindpb.Options{{
			Target:     "target1",
			Insecure:   true,
			SkipVerify: true,
			Username:   "username1",
			Password:   "password1",
		}, {
			Username: "username2",
			Password: "password2",
		}, {
			Password: "password3",
		}},
		want: &bindpb.Options{
			Target:     "target1",
			Insecure:   true,
			SkipVerify: true,
			Username:   "username2",
			Password:   "password3",
		},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := merge(c.args...)
			if diff := cmp.Diff(c.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("Merge diff (-want +got):\n%s", diff)
			}
		})
	}
}

var resolverBinding = resolver{&bindpb.Binding{
	Options: &bindpb.Options{
		Username: "global.username",
	},
	Duts: []*bindpb.Device{{
		Id:   "dut",
		Name: "dut.name",
		Ssh: &bindpb.Options{
			Password: "ssh.password",
		},
		Gnmi: &bindpb.Options{
			Password: "gnmi.password",
		},
		Gnoi: &bindpb.Options{
			Password: "gnoi.password",
		},
		Gnsi: &bindpb.Options{
			Password: "gnsi.password",
		},
		Gribi: &bindpb.Options{
			Password: "gribi.password",
		},
		P4Rt: &bindpb.Options{
			Password: "p4rt.password",
		},
	}, {
		Id:   "anotherdut",
		Name: "anotherdut.name",
		Options: &bindpb.Options{
			Password: "anotherdut.password",
		},
	}},
	Ates: []*bindpb.Device{{
		Id:   "ate",
		Name: "ate.name",
		Options: &bindpb.Options{
			Username: "ate.username",
		},
		Ixnetwork: &bindpb.Options{
			Password: "ixnetwork.password",
		},
	}, {
		Id:   "anotherate",
		Name: "anotherate.name",
		Options: &bindpb.Options{
			Password: "anotherate.password",
		},
	}},
}}

func TestResolver_ByID(t *testing.T) {
	r := resolverBinding
	cases := []struct {
		test string
		id   string
		fn   func(name string) *bindpb.Device
		want bool
	}{{
		test: "dutByID(dut)",
		id:   "dut",
		fn:   r.dutByID,
		want: true,
	}, {
		test: "dutByID(anotherdut)",
		id:   "anotherdut",
		fn:   r.dutByID,
		want: true,
	}, {
		test: "ateByID(ate)",
		id:   "ate",
		fn:   r.ateByID,
		want: true,
	}, {
		test: "ateByID(anotherate)",
		id:   "anotherate",
		fn:   r.ateByID,
		want: true,
	}, {
		test: "dutByID(no.such.dut)",
		id:   "no.such.dut",
		fn:   r.dutByID,
		want: false,
	}, {
		test: "ateByID(no.such.ate)",
		id:   "no.such.ate",
		fn:   r.ateByID,
		want: false,
	}, {
		test: "ateByID(dut)",
		id:   "dut",
		fn:   r.ateByID,
		want: false,
	}, {
		test: "dutByID(ate)",
		id:   "ate",
		fn:   r.dutByID,
		want: false,
	}}

	for _, c := range cases {
		t.Run(c.test, func(t *testing.T) {
			d := c.fn(c.id)
			got := d != nil
			if got != c.want {
				t.Errorf("Lookup by ID got %v, want %v", got, c.want)
			}
		})
	}
}

func TestResolver_ByName_Protocols(t *testing.T) {
	r := resolverBinding
	cases := []struct {
		test string
		fn   func(name string) (dialer, error)
		name string
		want dialer
	}{{
		test: "ssh",
		fn:   r.ssh,
		name: "dut.name",
		want: dialer{&bindpb.Options{
			Target:   "dut.name",
			Username: "global.username",
			Password: "ssh.password",
		}},
	}, {
		test: "gnmi",
		fn:   r.gnmi,
		name: "dut.name",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*gnmiPort),
			Username: "global.username",
			Password: "gnmi.password",
		}},
	}, {
		test: "gnoi",
		fn:   r.gnoi,
		name: "dut.name",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*gnoiPort),
			Username: "global.username",
			Password: "gnoi.password",
		}},
	}, {
		test: "gnsi",
		fn:   r.gnsi,
		name: "dut.name",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*gnsiPort),
			Username: "global.username",
			Password: "gnsi.password",
		}},
	}, {
		test: "gribi",
		fn:   r.gribi,
		name: "dut.name",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*gribiPort),
			Username: "global.username",
			Password: "gribi.password",
		}},
	}, {
		test: "p4rt",
		fn:   r.p4rt,
		name: "dut.name",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*p4rtPort),
			Username: "global.username",
			Password: "p4rt.password",
		}},
	}, {
		test: "ixnetwork",
		fn:   r.ixnetwork,
		name: "ate.name",
		want: dialer{&bindpb.Options{
			Target:   "ate.name",
			Username: "ate.username",
			Password: "ixnetwork.password",
		}},
	}, {
		test: "anotherdut",
		fn:   r.ssh,
		name: "anotherdut.name",
		want: dialer{&bindpb.Options{
			Target:   "anotherdut.name",
			Username: "global.username",
			Password: "anotherdut.password",
		}},
	}, {
		test: "anotherate",
		fn:   r.ixnetwork,
		name: "anotherate.name",
		want: dialer{&bindpb.Options{
			Target:   "anotherate.name",
			Username: "global.username",
			Password: "anotherate.password",
		}},
	}}

	for _, c := range cases {
		t.Run(c.test, func(t *testing.T) {
			got, err := c.fn(c.name)
			if err != nil {
				t.Fatalf("Could not get options: %v", err)
			}
			if diff := cmp.Diff(c.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("Resolve diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolver_ByName_Protocols_Error(t *testing.T) {
	r := resolverBinding
	cases := []struct {
		test   string
		fn     func(name string) (dialer, error)
		name   string
		reason string
	}{{
		test:   "no.such.dut",
		fn:     r.ssh,
		name:   "no.such.dut.name",
		reason: "name not found",
	}, {
		test:   "no.such.ate",
		fn:     r.ixnetwork,
		name:   "no.such.ate.name",
		reason: "name not found",
	}, {
		test:   "ate.ssh",
		fn:     r.ssh,
		name:   "ate.name",
		reason: "ssh never looks up ate",
	}, {
		test:   "ate.gnmi",
		fn:     r.gnmi,
		name:   "ate.name",
		reason: "gnmi never looks up ate",
	}, {
		test:   "ate.gnoi",
		fn:     r.gnoi,
		name:   "ate.name",
		reason: "gnoi never looks up ate",
	}, {
		test:   "ate.gnsi",
		fn:     r.gnsi,
		name:   "ate.name",
		reason: "gnsi never looks up ate",
	}, {
		test:   "ate.gribi",
		fn:     r.gribi,
		name:   "ate.name",
		reason: "gribi never looks up ate",
	}, {
		test:   "ate.p4rt",
		fn:     r.p4rt,
		name:   "ate.name",
		reason: "p4rt never looks up ate",
	}, {
		test:   "dut.ixnetwork",
		fn:     r.ixnetwork,
		name:   "dut.name",
		reason: "ixnetwork never looks up dut",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.fn(c.name)
			t.Logf("Resolve got error: %v", err)
			if err == nil {
				t.Errorf("Resolve error got nil, want error because %s", c.reason)
			}
		})
	}
}
