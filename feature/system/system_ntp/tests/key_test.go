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

package system_ntp_test

import (
	"testing"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

// TestNtpKeyConfigurability tests basic configurability of NTP authentication
// keys.
//
// config_path:/system/ntp/ntp-keys/ntp-key/config/key-id
// config_path:/system/ntp/ntp-keys/ntp-key/config/key-type
// config_path:/system/ntp/ntp-keys/ntp-key/config/key-value
// telemetry_path:/system/ntp/ntp-keys/ntp-key/state/key-id
// telemetry_path:/system/ntp/ntp-keys/ntp-key/state/key-type
// telemetry_path:/system/ntp/ntp-keys/ntp-key/state/key-value
func TestNtpKeyConfigurability(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	testCases := []struct {
		description string
		keyId       uint16
		keyType     oc.E_System_NTP_AUTH_TYPE
		keyValue    string
	}{
		{"Zero Index Key", 0, oc.System_NTP_AUTH_TYPE_NTP_AUTH_MD5, "secret"},
		{"Long Secret Key", 1, oc.System_NTP_AUTH_TYPE_NTP_AUTH_MD5, "secret_~+!@#$%^&*(){}|:\"<>?1234567890"},
		{"Max Index Key", 65535, oc.System_NTP_AUTH_TYPE_NTP_AUTH_MD5, "secret"},
	}

	dut := ondatra.DUT(t, "dut1")

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := dut.Config().System().Ntp()
			state := dut.Telemetry().System().Ntp()

			ntpKey := oc.System_Ntp_NtpKey{
				KeyId:    &testCase.keyId,
				KeyType:  testCase.keyType,
				KeyValue: &testCase.keyValue,
			}
			config.NtpKey(testCase.keyId).Replace(t, &ntpKey)

			configGot := config.NtpKey(testCase.keyId).Get(t)
			if keyId := configGot.GetKeyId(); keyId != testCase.keyId {
				t.Errorf("Config NTP Key ID: got %d, want %d", keyId, testCase.keyId)
			}

			if keyType := configGot.GetKeyType(); keyType != testCase.keyType {
				t.Errorf("Config NTP Key Type: got %s, want %s", keyType.String(), testCase.keyType.String())
			}

			if keyValue := configGot.GetKeyValue(); keyValue != testCase.keyValue {
				t.Errorf("Config NTP Key Value: got %s, want %s", keyValue, testCase.keyValue)
			}

			stateGot := state.NtpKey(testCase.keyId).Get(t)
			if keyId := stateGot.GetKeyId(); keyId != testCase.keyId {
				t.Errorf("Telemetry NTP Server: got %d, want %d", keyId, testCase.keyId)
			}

			if keyType := stateGot.GetKeyType(); keyType != testCase.keyType {
				t.Errorf("Telemetry NTP Key Type: got %s, want %s", keyType.String(), testCase.keyType.String())
			}

			if keyValue := stateGot.GetKeyValue(); keyValue != testCase.keyValue {
				t.Errorf("Telemetry NTP Key Value: got %s, want %s", keyValue, testCase.keyValue)
			}

			config.NtpKey(testCase.keyId).Delete(t)
			if qs := config.NtpKey(testCase.keyId).Lookup(t); qs.IsPresent() == true {
				t.Errorf("Delete NTP Server fail: got %v", qs)
			}
		})
	}
}
