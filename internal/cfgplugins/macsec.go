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
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
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

// MACsecRotationKey holds a CKN and CAK for a scheduled MACsec key.
type MACsecRotationKey struct {
	CKN string
	CAK string
}

// MACsecRotationProfile holds one security profile backed by a shared-secret profile.
type MACsecRotationProfile struct {
	ProfileName         string
	SharedSecretProfile string
	AllowUnprotected    bool
}

// MACsecKeyRotationConfig holds the key schedule and profiles for a MACsec rotation chain.
type MACsecKeyRotationConfig struct {
	Keys                []MACsecRotationKey
	Profiles            []MACsecRotationProfile
	Anchor              time.Time
	KeyLifetime         time.Duration
	KeyStagger          time.Duration
	InitialKeyExtension time.Duration
	KeyServerPriority   uint32
	SAKRekeyInterval    uint32
	ReplayProtection    uint16
	IncludeICVIndicator bool
	IncludeSCI          bool
}

func batchMACsec(t *testing.T, batch *gnmi.SetBatch, config string) {
	t.Helper()
	path, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("create MACsec CLI path: %v", err)
	}
	gnmi.BatchUpdate(batch, path, config)
}

func (config MACsecKeyRotationConfig) keyWindow(keyIndex int) (time.Time, time.Time) {
	begin := config.Anchor.Add(time.Duration(keyIndex) * config.KeyStagger)
	end := begin.Add(config.KeyLifetime + config.InitialKeyExtension)
	if keyIndex > 0 {
		begin = begin.Add(config.InitialKeyExtension)
	}
	return begin, end
}

// ConfigureMACsecKeyRotation adds scheduled XPN keys and MACsec profiles to a batch.
func ConfigureMACsecKeyRotation(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, config MACsecKeyRotationConfig) *gnmi.SetBatch {
	t.Helper()
	if len(config.Keys) == 0 || len(config.Profiles) == 0 {
		t.Fatalf("MACsec key rotation requires at least one key and one profile")
	}
	if sb == nil {
		sb = &gnmi.SetBatch{}
	}
	if deviations.MacsecOcUnsupported(dut) {
		if dut.Vendor() != ondatra.ARISTA {
			t.Fatalf("MACsec CLI deviation is not implemented for vendor %v", dut.Vendor())
		}
		var secrets strings.Builder
		for keyIndex, key := range config.Keys {
			begin, end := config.keyWindow(keyIndex)
			fmt.Fprintf(&secrets, "      secret %s 0 %s receive-lifetime %s %s transmit-lifetime %s %s\n",
				key.CKN, key.CAK,
				begin.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"),
				begin.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))
		}
		var cli strings.Builder
		cli.WriteString("\nmanagement security\n   entropy source hardware\n   !\n")
		for profileIndex, profile := range config.Profiles {
			if profile.ProfileName == "" || profile.SharedSecretProfile == "" {
				t.Fatalf("MACsec rotation profile %d requires profile and shared-secret names", profileIndex)
			}
			fmt.Fprintf(&cli, "   session shared-secret profile %s\n%s", profile.SharedSecretProfile, secrets.String())
			if profileIndex < len(config.Profiles)-1 {
				cli.WriteString("   !\n")
			} else {
				cli.WriteString("!\n")
			}
		}
		cli.WriteString("mac security\n")
		for profileIndex, profile := range config.Profiles {
			unprotected := "drop"
			if profile.AllowUnprotected {
				unprotected = "allow"
			}
			fmt.Fprintf(&cli, `   profile %s
      cipher aes256-gcm-xpn
      key source shared-secret profile %s
      mka key-server priority %d
      mka session rekey-period %d
      traffic unprotected %s
      sci
      replay protection window %d
!`, profile.ProfileName, profile.SharedSecretProfile, config.KeyServerPriority,
				config.SAKRekeyInterval, unprotected, config.ReplayProtection)
			if profileIndex < len(config.Profiles)-1 {
				cli.WriteString("   !\n")
			} else {
				cli.WriteString("!\n")
			}
		}
		batchMACsec(t, sb, cli.String())
		return sb
	}

	macsec := &oc.Macsec{}
	for profileIndex, profile := range config.Profiles {
		if profile.ProfileName == "" || profile.SharedSecretProfile == "" {
			t.Fatalf("MACsec rotation profile %d requires profile and shared-secret names", profileIndex)
		}
		keychain := &oc.Keychain{Name: ygot.String(profile.SharedSecretProfile)}
		for keyIndex, key := range config.Keys {
			begin, end := config.keyWindow(keyIndex)
			ocKey := keychain.GetOrCreateKey(oc.UnionString(key.CKN))
			ocKey.SetSecretKey(key.CAK)
			ocKey.SetCryptoAlgorithm(oc.KeychainTypes_CRYPTO_TYPE_AES_256_CMAC)
			ocKey.GetOrCreateSendLifetime().SetStartTime(uint64(begin.Unix()))
			ocKey.GetOrCreateSendLifetime().SetEndTime(uint64(end.Unix()))
			ocKey.GetOrCreateSendLifetime().SetSendAndReceive(true)
			ocKey.GetOrCreateReceiveLifetime().SetStartTime(uint64(begin.Unix()))
			ocKey.GetOrCreateReceiveLifetime().SetEndTime(uint64(end.Unix()))
		}
		gnmi.BatchReplace(sb, gnmi.OC().Keychain(profile.SharedSecretProfile).Config(), keychain)

		policy := macsec.GetOrCreateMka().GetOrCreatePolicy(profile.ProfileName)
		policy.SetName(profile.ProfileName)
		policy.SetKeyServerPriority(uint8(config.KeyServerPriority))
		policy.SetSakRekeyInterval(config.SAKRekeyInterval)
		policy.SetConfidentialityOffset(oc.Macsec_ConfidentialityOffset_0_BYTES)
		policy.SetIncludeIcvIndicator(config.IncludeICVIndicator)
		policy.SetIncludeSci(config.IncludeSCI)
		policy.SetMacsecCipherSuite([]oc.E_Macsec_MacsecCipherSuite{oc.Macsec_MacsecCipherSuite_GCM_AES_XPN_256})
	}
	gnmi.BatchUpdate(sb, gnmi.OC().Macsec().Config(), macsec)
	return sb
}

// MACsecProfileBindingConfig describes a profile binding or unbinding on interfaces.
type MACsecProfileBindingConfig struct {
	PortIDs          []string
	ProfileName      string
	KeyChainName     string
	ReplayProtection uint16
	Remove           bool
}

// MACsecSAKRekeyIntervalConfig holds a profile-specific SAK rekey interval.
type MACsecSAKRekeyIntervalConfig struct {
	ProfileName string
	Interval    uint32
}

// ConfigureMACsecSAKRekeyInterval adds an SAK rekey update to a batch.
func ConfigureMACsecSAKRekeyInterval(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, config MACsecSAKRekeyIntervalConfig) *gnmi.SetBatch {
	t.Helper()
	if config.ProfileName == "" {
		t.Fatalf("MACsec profile name is required to configure SAK rekey interval")
	}
	if sb == nil {
		sb = &gnmi.SetBatch{}
	}
	if deviations.MacsecOcUnsupported(dut) {
		if dut.Vendor() != ondatra.ARISTA {
			t.Fatalf("MACsec CLI deviation is not implemented for vendor %v", dut.Vendor())
		}
		cli := fmt.Sprintf("mac security\n   profile %s\n      mka session rekey-period %d\n!", config.ProfileName, config.Interval)
		batchMACsec(t, sb, cli)
		return sb
	}
	policy := &oc.Macsec_Mka_Policy{Name: ygot.String(config.ProfileName)}
	policy.SetSakRekeyInterval(config.Interval)
	gnmi.BatchUpdate(sb, gnmi.OC().Macsec().Mka().Policy(config.ProfileName).Config(), policy)
	return sb
}

// ConfigureMACsecProfileBinding adds a MACsec profile binding to a batch.
func ConfigureMACsecProfileBinding(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, config MACsecProfileBindingConfig) *gnmi.SetBatch {
	t.Helper()
	if len(config.PortIDs) == 0 {
		t.Fatalf("MACsec profile binding requires at least one port ID")
	}
	if !config.Remove && config.ProfileName == "" {
		t.Fatalf("MACsec profile name is required when binding")
	}
	if sb == nil {
		sb = &gnmi.SetBatch{}
	}
	if deviations.MacsecOcUnsupported(dut) {
		if dut.Vendor() != ondatra.ARISTA {
			t.Fatalf("MACsec CLI deviation is not implemented for vendor %v", dut.Vendor())
		}
		for _, portID := range config.PortIDs {
			interfaceName := dut.Port(t, portID).Name()
			command := fmt.Sprintf("\ninterface %s\n   ", interfaceName)
			if config.Remove {
				command += "no mac security profile\n!"
			} else {
				command += fmt.Sprintf("mac security profile %s\n!", config.ProfileName)
			}
			batchMACsec(t, sb, command)
		}
		return sb
	}
	for _, portID := range config.PortIDs {
		interfaceName := dut.Port(t, portID).Name()
		if config.Remove {
			gnmi.BatchDelete(sb, gnmi.OC().Macsec().Interface(interfaceName).Mka().Config())
			continue
		}
		intf := &oc.Macsec_Interface{Name: ygot.String(interfaceName)}
		intf.SetName(interfaceName)
		intf.SetEnable(true)
		intf.SetReplayProtection(config.ReplayProtection)
		mka := intf.GetOrCreateMka()
		mka.SetKeyChain(config.KeyChainName)
		mka.SetMkaPolicy(config.ProfileName)
		gnmi.BatchUpdate(sb, gnmi.OC().Macsec().Interface(interfaceName).Config(), intf)
	}
	return sb
}

// ConfigureMACsec returns a gnmi SetBatch for configuring a MACsec security profile and applies it to the given interface.
func ConfigureMACsec(t *testing.T, dut *ondatra.DUTDevice, cfg MACsecCfg) *gnmi.SetBatch {
	t.Helper()

	batch := &gnmi.SetBatch{}

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
		gnmi.BatchUpdate(batch, d.Keychain(cfg.ProfileName).Config(), kc)

		// MKA policy and MACsec interface referencing the keychain and policy.
		macsec := &oc.Macsec{}
		policy := macsec.GetOrCreateMka().GetOrCreatePolicy(cfg.ProfileName)
		policy.SetKeyServerPriority(10)
		policy.SetSakRekeyInterval(3600)
		policy.SetIncludeSci(true)
		policy.SetMacsecCipherSuite([]oc.E_Macsec_MacsecCipherSuite{oc.Macsec_MacsecCipherSuite_GCM_AES_128})

		intf := macsec.GetOrCreateInterface(cfg.IntfName)
		intf.SetEnable(true)
		mka := intf.GetOrCreateMka()
		mka.SetKeyChain(cfg.ProfileName)
		mka.SetMkaPolicy(cfg.ProfileName)

		gnmi.BatchUpdate(batch, d.Macsec().Config(), macsec)
	}

	return batch
}

const (
	macsecConfigArista = `
management security
   entropy source hardware
   !
   session shared-secret profile BAYBRIDGE
      secret f123456789abcdef0123456789abcdeff123456789abcdef0123456789abcdef f123456789abcdef0123456789abcdeff123456789abcdef0123456789abcdef receive-lifetime 2026-03-17 18:30:00 infinite transmit-lifetime 2026-03-17 18:30:00 infinite
!
mac security
      profile macsec-test
      cipher aes256-gcm-xpn
      key source shared-secret profile BAYBRIDGE
      sci
      mka key-server priority 15
      mka session rekey-period 28800
      traffic unprotected drop
      replay protection window 64
!
`
)

// MacsecConfig configures macsec.
func MacsecConfig(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.MacsecOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, macsecConfigArista)
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'macsec'", dut.Vendor())
		}
	}
}
