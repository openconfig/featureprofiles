package system_base_test

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	kinit "github.com/openconfig/ondatra/knebind/init"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, kinit.Init)
}

// path:/system/config/hostname
// path:/system/state/hostname
func TestHostname(t *testing.T) {
	var tests = []string{
		"abcdefghijkmnop",
		"123456789012345",
		"x",
		"foo_bar-baz",
		"test.example",
		"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}
	dut := ondatra.DUT(t, "dut1")
	configHn := dut.Config().System().Hostname()
	stateHn := dut.Telemetry().System().Hostname()

	for _, test := range tests {
		configHn.Replace(t, test)

		configGot := configHn.Get(t)
		if configGot != test {
			t.Errorf("Config hostname got %s want %s", configGot, test)
		}

		stateGot := stateHn.Await(t, 5*time.Second, test)
		success := false
		for _, v := range stateGot {
			if v.Present && v.Val(t) == test {
				success = true
			}
		}
		if !success {
			t.Errorf("Telemetry hostname got %v want %s", stateGot, test)
		}
	}

	configHn.Delete(t)
	if qs := configHn.GetFull(t); qs.IsPresent() == true {
		t.Errorf("Delete hostname fail; got %v", qs)
	}
}

// path:/system/config/domain-name
// path:/system/state/domain-name
func TestDomainName(t *testing.T) {
	var tests = []string{
		"abcdefghijkmnop",
		"123456789012345",
		"x",
		"foo_bar-baz",
		"test.example",
		"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}
	dut := ondatra.DUT(t, "dut1")
	configDn := dut.Config().System().DomainName()
	stateDn := dut.Telemetry().System().DomainName()

	for _, test := range tests {
		configDn.Replace(t, test)

		configGot := configDn.Get(t)
		if configGot != test {
			t.Errorf("Config domainname got %s want %s", configGot, test)
		}

		stateGot := stateDn.Await(t, 5*time.Second, test)
		success := false
		for _, v := range stateGot {
			if v.Present && v.Val(t) == test {
				success = true
			}
		}
		if !success {
			t.Errorf("Set domainname got %v want %s", stateGot, test)
		}
	}

	configDn.Delete(t)
	if qs := configDn.GetFull(t); qs.IsPresent() == true {
		t.Errorf("Delete domainname fail")
	}
}

// path:system/state/current-datetime
//func TestCurrentDateTime(t *testing.T) {
//	dut := ondatra.DUT(t, "dut1")
//	dt := dut.Telemetry().System().CurrentDatetime()
//
//}
