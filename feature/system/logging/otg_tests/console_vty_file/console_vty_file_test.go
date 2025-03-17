// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package console_vty_file_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSystemLogging(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	testCases := []struct {
		name      string
		configure func(*testing.T, *ondatra.DUTDevice)
		validate  func(*testing.T, *ondatra.DUTDevice)
	}{
		{
			name:      "console",
			configure: configureConsoleLogging,
			validate:  validateConsoleLogging,
		},
		{
			name:      "vty",
			configure: configureVTYLogging,
			validate:  validateVTYLogging,
		},
		{
			name:      "file",
			configure: configureFileLogging,
			validate:  validateFileLogging,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.configure(t, dut)
			tc.validate(t, dut)
		})
	}
}

func configureConsoleLogging(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	root := &oc.Root{}
	logging := root.GetOrCreateSystem().GetOrCreateLogging()
	consoleLogger := logging.GetOrCreateConsole()
	consoleLogger.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL7,
		oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
	)
	consoleLogger.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL6,
		oc.SystemLogging_SyslogSeverity_ALERT,
	)
	consoleLogger.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL5,
		oc.SystemLogging_SyslogSeverity_CRITICAL,
	)

	gnmi.Replace(t, dut, gnmi.OC().System().Logging().Config(), logging)
}

func validateConsoleLogging(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	consoleLogger := gnmi.Get[*oc.System_Logging_Console](t, dut, gnmi.OC().System().Logging().Console().State())
	t.Logf("consoleLogger: %v", consoleLogger)
	s1 := consoleLogger.GetSelector(oc.SystemLogging_SYSLOG_FACILITY_LOCAL7, oc.SystemLogging_SyslogSeverity_INFORMATIONAL)
	if s1 == nil {
		t.Errorf("consoleLogger.GetSelector(%v, %v) = nil, want non-nil", oc.SystemLogging_SYSLOG_FACILITY_LOCAL7, oc.SystemLogging_SyslogSeverity_INFORMATIONAL)
	}
	s2 := consoleLogger.GetSelector(oc.SystemLogging_SYSLOG_FACILITY_LOCAL6, oc.SystemLogging_SyslogSeverity_ALERT)
	if s2 == nil {
		t.Errorf("consoleLogger.GetSelector(%v, %v) = nil, want non-nil", oc.SystemLogging_SYSLOG_FACILITY_LOCAL6, oc.SystemLogging_SyslogSeverity_ALERT)
	}
	s3 := consoleLogger.GetSelector(oc.SystemLogging_SYSLOG_FACILITY_LOCAL5, oc.SystemLogging_SyslogSeverity_CRITICAL)
	if s3 == nil {
		t.Errorf("consoleLogger.GetSelector(%v, %v) = nil, want non-nil", oc.SystemLogging_SYSLOG_FACILITY_LOCAL5, oc.SystemLogging_SyslogSeverity_CRITICAL)
	}
}

func configureVTYLogging(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	root := &oc.Root{}
	logging := root.GetOrCreateSystem().GetOrCreateLogging()
	vtyLogger := logging.GetOrCreateVty()
	vtyLogger.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL7,
		oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
	)
	vtyLogger.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL5,
		oc.SystemLogging_SyslogSeverity_ALERT,
	)

	gnmi.Replace(t, dut, gnmi.OC().System().Logging().Config(), logging)
}

func validateVTYLogging(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	vtyLogger := gnmi.Get[*oc.System_Logging_Vty](t, dut, gnmi.OC().System().Logging().Vty().State())
	t.Logf("vtyLogger: %v", vtyLogger)
	s1 := vtyLogger.GetSelector(oc.SystemLogging_SYSLOG_FACILITY_LOCAL7, oc.SystemLogging_SyslogSeverity_INFORMATIONAL)
	if s1 == nil {
		t.Errorf("vtyLogger.GetSelector(%v, %v) = nil, want non-nil", oc.SystemLogging_SYSLOG_FACILITY_LOCAL7, oc.SystemLogging_SyslogSeverity_INFORMATIONAL)
	}
	s2 := vtyLogger.GetSelector(oc.SystemLogging_SYSLOG_FACILITY_LOCAL5, oc.SystemLogging_SyslogSeverity_ALERT)
	if s2 == nil {
		t.Errorf("vtyLogger.GetSelector(%v, %v) = nil, want non-nil", oc.SystemLogging_SYSLOG_FACILITY_LOCAL5, oc.SystemLogging_SyslogSeverity_ALERT)
	}
}

func configureFileLogging(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	root := &oc.Root{}
	logging := root.GetOrCreateSystem().GetOrCreateLogging()
	fileLogger1 := logging.GetOrCreateFile("/var/log/syslog", "logfile_1")
	fileLogger1.SetMaxSize(1000000)
	fileLogger1.SetMaxOpenTime(1440)
	fileLogger1.SetRotate(3)
	fileLogger1.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL7,
		oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
	)
	fileLogger1.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL6,
		oc.SystemLogging_SyslogSeverity_ALERT,
	)

	fileLogger2 := logging.GetOrCreateFile("/var/log/syslog", "logfile_2")
	fileLogger2.SetMaxSize(10000000)
	fileLogger2.SetMaxOpenTime(1)
	fileLogger2.SetRotate(10)
	fileLogger2.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL5,
		oc.SystemLogging_SyslogSeverity_INFORMATIONAL,
	)
	fileLogger2.GetOrCreateSelector(
		oc.SystemLogging_SYSLOG_FACILITY_LOCAL6,
		oc.SystemLogging_SyslogSeverity_WARNING,
	)

	gnmi.Replace(t, dut, gnmi.OC().System().Logging().Config(), logging)
}

func validateFileLogging(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	fileLogger1 := gnmi.Get[*oc.System_Logging_File](t, dut, gnmi.OC().System().Logging().File("/var/log/syslog", "logfile_1").State())
	t.Logf("fileLogger1: %v", fileLogger1)
	s1 := fileLogger1.GetSelector(oc.SystemLogging_SYSLOG_FACILITY_LOCAL7, oc.SystemLogging_SyslogSeverity_INFORMATIONAL)
	if s1 == nil {
		t.Errorf("fileLogger1.GetSelector(%v, %v) = nil, want non-nil", oc.SystemLogging_SYSLOG_FACILITY_LOCAL7, oc.SystemLogging_SyslogSeverity_INFORMATIONAL)
	}
	s2 := fileLogger1.GetSelector(oc.SystemLogging_SYSLOG_FACILITY_LOCAL6, oc.SystemLogging_SyslogSeverity_ALERT)
	if s2 == nil {
		t.Errorf("fileLogger1.GetSelector(%v, %v) = nil, want non-nil", oc.SystemLogging_SYSLOG_FACILITY_LOCAL6, oc.SystemLogging_SyslogSeverity_ALERT)
	}

	fileLogger2 := gnmi.Get[*oc.System_Logging_File](t, dut, gnmi.OC().System().Logging().File("/var/log/syslog", "logfile_1").State())
	t.Logf("fileLogger2: %v", fileLogger2)
	s3 := fileLogger2.GetSelector(oc.SystemLogging_SYSLOG_FACILITY_LOCAL5, oc.SystemLogging_SyslogSeverity_INFORMATIONAL)
	if s3 == nil {
		t.Errorf("fileLogger2.GetSelector(%v, %v) = nil, want non-nil", oc.SystemLogging_SYSLOG_FACILITY_LOCAL5, oc.SystemLogging_SyslogSeverity_INFORMATIONAL)
	}
	s4 := fileLogger2.GetSelector(oc.SystemLogging_SYSLOG_FACILITY_LOCAL6, oc.SystemLogging_SyslogSeverity_WARNING)
	if s4 == nil {
		t.Errorf("fileLogger2.GetSelector(%v, %v) = nil, want non-nil", oc.SystemLogging_SYSLOG_FACILITY_LOCAL6, oc.SystemLogging_SyslogSeverity_WARNING)
	}
}
