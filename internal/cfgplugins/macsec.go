// Copyright 2024 Google LLC
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

package cfgplugins

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// MACsecCfg holds parameters for configuring a MACsec security profile.
type MACsecCfg struct {
	IntfName    string // interface to apply the MACsec profile to
	ProfileName string // MACsec profile name (defaults to "sampleProfile")
	CKN         string // primary MKA key name (CKN)
	CAK         string // primary MKA key (type-7 encrypted CAK)
	FallbackCKN string
	FallbackCAK string
}

// ConfigureMACsec configures a MACsec security profile and applies it to the given interface
func ConfigureMACsec(t *testing.T, dut *ondatra.DUTDevice, cfg MACsecCfg) {
	t.Helper()
	if deviations.MacsecOcUnsupported(dut) {
		macSecCLI := fmt.Sprintf(`mac security
	profile %[1]s
		key %[2]s 0 %[3]s
		key %[4]s 0 %[5]s fallback
		mka key-server priority 10
		mka session rekey-period 3600
		sci
	!
	interface %[6]s
	mac security profile %[1]s
	!`, cfg.ProfileName, cfg.CKN, cfg.CAK, cfg.FallbackCKN, cfg.FallbackCAK, cfg.IntfName)
		helpers.GnmiCLIConfig(t, dut, macSecCLI)
	} else {
		// OpenConfig path: model the profile as a keychain holding the primary and
		// fallback MKA keys, an MKA policy, and a MACsec interface referencing both.
		d := gnmi.OC()

		// Keychain with the primary (and optional fallback) MKA keys.
		kc := &oc.Keychain{Name: ygot.String(cfg.ProfileName)}
		primary := kc.GetOrCreateKey(oc.UnionString(cfg.CKN))
		primary.SetSecretKey(cfg.CAK)
		primary.SetCryptoAlgorithm(oc.KeychainTypes_CRYPTO_TYPE_AES_256_CMAC)
		if cfg.FallbackCKN != "" {
			fallback := kc.GetOrCreateKey(oc.UnionString(cfg.FallbackCKN))
			fallback.SetSecretKey(cfg.FallbackCAK)
			fallback.SetCryptoAlgorithm(oc.KeychainTypes_CRYPTO_TYPE_AES_256_CMAC)
		}
		gnmi.Update(t, dut, d.Keychain(cfg.ProfileName).Config(), kc)

		// MKA policy and MACsec interface referencing the keychain and policy.
		macsec := &oc.Macsec{}
		policy := macsec.GetOrCreateMka().GetOrCreatePolicy(cfg.ProfileName)
		policy.SetKeyServerPriority(10)
		policy.SetSakRekeyInterval(3600)
		policy.SetIncludeSci(true)
		policy.SetMacsecCipherSuite([]oc.E_Macsec_MacsecCipherSuite{oc.Macsec_MacsecCipherSuite_GCM_AES_256})

		intf := macsec.GetOrCreateInterface(cfg.IntfName)
		intf.SetEnable(true)
		mka := intf.GetOrCreateMka()
		mka.SetKeyChain(cfg.ProfileName)
		mka.SetMkaPolicy(cfg.ProfileName)

		gnmi.Update(t, dut, d.Macsec().Config(), macsec)
	}
}
