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

func TestResolver(t *testing.T) {
	r := resolverBinding
	cases := []struct {
		name string
		fn   func(id string) (dialer, error)
		id   string
		want dialer
	}{{
		name: "ssh",
		fn:   r.ssh,
		id:   "dut",
		want: dialer{&bindpb.Options{
			Target:   "dut.name",
			Username: "global.username",
			Password: "ssh.password",
		}},
	}, {
		name: "gnmi",
		fn:   r.gnmi,
		id:   "dut",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*gnmiPort),
			Username: "global.username",
			Password: "gnmi.password",
		}},
	}, {
		name: "gnoi",
		fn:   r.gnoi,
		id:   "dut",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*gnoiPort),
			Username: "global.username",
			Password: "gnoi.password",
		}},
	}, {
		name: "gnsi",
		fn:   r.gnsi,
		id:   "dut",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*gnsiPort),
			Username: "global.username",
			Password: "gnsi.password",
		}},
	}, {
		name: "gribi",
		fn:   r.gribi,
		id:   "dut",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*gribiPort),
			Username: "global.username",
			Password: "gribi.password",
		}},
	}, {
		name: "p4rt",
		fn:   r.p4rt,
		id:   "dut",
		want: dialer{&bindpb.Options{
			Target:   "dut.name:" + strconv.Itoa(*p4rtPort),
			Username: "global.username",
			Password: "p4rt.password",
		}},
	}, {
		name: "ixnetwork",
		fn:   r.ixnetwork,
		id:   "ate",
		want: dialer{&bindpb.Options{
			Target:   "ate.name",
			Username: "ate.username",
			Password: "ixnetwork.password",
		}},
	}, {
		name: "anotherdut",
		fn:   r.ssh,
		id:   "anotherdut",
		want: dialer{&bindpb.Options{
			Target:   "anotherdut.name",
			Username: "global.username",
			Password: "anotherdut.password",
		}},
	}, {
		name: "anotherate",
		fn:   r.ixnetwork,
		id:   "anotherate",
		want: dialer{&bindpb.Options{
			Target:   "anotherate.name",
			Username: "global.username",
			Password: "anotherate.password",
		}},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.fn(c.id)
			if err != nil {
				t.Fatalf("Could not get options: %v", err)
			}
			if diff := cmp.Diff(c.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("Resolve diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolver_Error(t *testing.T) {
	r := resolverBinding
	cases := []struct {
		name   string
		fn     func(id string) (dialer, error)
		id     string
		reason string
	}{{
		name:   "no.such.dut",
		fn:     r.ssh,
		id:     "no.such.dut",
		reason: "id not found",
	}, {
		name:   "no.such.ate",
		fn:     r.ixnetwork,
		id:     "no.such.ate",
		reason: "id not found",
	}, {
		name:   "ate.ssh",
		fn:     r.ssh,
		id:     "ate",
		reason: "ssh never looks up ate",
	}, {
		name:   "ate.gnmi",
		fn:     r.gnmi,
		id:     "ate",
		reason: "gnmi never looks up ate",
	}, {
		name:   "ate.gnoi",
		fn:     r.gnoi,
		id:     "ate",
		reason: "gnoi never looks up ate",
	}, {
		name:   "ate.gnsi",
		fn:     r.gnsi,
		id:     "ate",
		reason: "gnsi never looks up ate",
	}, {
		name:   "ate.gribi",
		fn:     r.gribi,
		id:     "ate",
		reason: "gribi never looks up ate",
	}, {
		name:   "ate.p4rt",
		fn:     r.p4rt,
		id:     "ate",
		reason: "p4rt never looks up ate",
	}, {
		name:   "dut.ixnetwork",
		fn:     r.ixnetwork,
		id:     "dut",
		reason: "ixnetwork never looks up dut",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.fn(c.id)
			t.Logf("Resolve got error: %v", err)
			if err == nil {
				t.Errorf("Resolve error got nil, want error because %s", c.reason)
			}
		})
	}
}
