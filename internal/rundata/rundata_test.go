package rundata

import (
	"testing"

	"github.com/openconfig/ondatra/binding"
)

func TestTopology(t *testing.T) {
	cases := []struct {
		name string
		resv *binding.Reservation
		want string
	}{{
		name: "empty",
		resv: &binding.Reservation{},
		want: "",
	}, {
		name: "atedut_12",
		resv: &binding.Reservation{
			ATEs: map[string]binding.ATE{
				"ate": &binding.AbstractATE{
					Dims: &binding.Dims{
						Ports: map[string]*binding.Port{
							"port1":  nil,
							"port2":  nil,
							"port3":  nil,
							"port4":  nil,
							"port5":  nil,
							"port6":  nil,
							"port7":  nil,
							"port8":  nil,
							"port9":  nil,
							"port10": nil,
							"port11": nil,
							"port12": nil,
						},
					},
				},
			},
			DUTs: map[string]binding.DUT{
				"dut": &binding.AbstractDUT{
					Dims: &binding.Dims{
						Ports: map[string]*binding.Port{
							"port1":  nil,
							"port2":  nil,
							"port3":  nil,
							"port4":  nil,
							"port5":  nil,
							"port6":  nil,
							"port7":  nil,
							"port8":  nil,
							"port9":  nil,
							"port10": nil,
							"port11": nil,
							"port12": nil,
						},
					},
				},
			},
		},
		want: "ate:12,dut:12",
	}, {
		name: "dutdut",
		resv: &binding.Reservation{
			DUTs: map[string]binding.DUT{
				"dut1": &binding.AbstractDUT{
					Dims: &binding.Dims{
						Ports: map[string]*binding.Port{
							"port1": nil,
							"port2": nil,
							"port3": nil,
							"port4": nil,
						},
					},
				},
				"dut2": &binding.AbstractDUT{
					Dims: &binding.Dims{
						Ports: map[string]*binding.Port{
							"port1": nil,
							"port2": nil,
							"port3": nil,
							"port4": nil,
						},
					},
				},
			},
		},
		want: "dut1:4,dut2:4",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := topology(c.resv)
			if got != c.want {
				t.Errorf("topology got %q, want %q", got, c.want)
			}
		})
	}
}

func TestProperties(t *testing.T) {
	TestPlanID = "UnitTest-1.1"
	*knownIssueURL = "https://example.com"

	got := Properties(&binding.Reservation{})
	t.Log(got)

	if got, want := got["test.plan_id"], TestPlanID; got != want {
		t.Errorf("Property test.plan_id got %q, want %q", got, want)
	}
	if got, want := got["known_issue_url"], *knownIssueURL; got != want {
		t.Errorf("Property known_issue_url got %q, want %q", got, want)
	}

	wantKeys := []string{
		"build.go_version",
		"build.path",
		"build.main.path",
		"build.main.version",
		"build.main.sum",
		"test.path",
		"test.plan_id",
		"topology",
		"time.begin",
		"time.end",
		"known_issue_url",
	}

	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("Missing key from Properties: %s", k)
		}
	}
}
