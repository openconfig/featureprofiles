// Copyright 2025 Google LLC
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
//

package ipsec_scale_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/iputil"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	packetvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

const (
	vlanID = 10

	// ATE OTG topology names.
	ate1LagName = "Lag1"
	ate2LagName = "Lag2"
	ate1DevName = "d1"
	ate2DevName = "d2"

	// VRF names.
	ateVRF    = "ATE_VRF"
	tunnelVRF = "TUNNEL_VRF"

	// Interface names.
	loopbackIfName = "Loopback0"

	// Loopback IPv6 addresses used as IPSec tunnel endpoints (RFC 3849).
	dut1LoopbackIPv6  = "2001:db8:3::1"
	dut2LoopbackIPv6  = "2001:db8:4::1"
	loopbackPrefixLen = 128

	// Static route destination prefixes.
	ate1IPv4Prefix = "192.0.2.0/30"
	ate2IPv4Prefix = "203.0.113.0/30"
	ate1IPv6Prefix = "2001:db8:1::0/126"
	ate2IPv6Prefix = "2001:db8:2::0/126"

	// OTG MACsec peer name.
	macsecPeerName = "Peer A"

	// OTG flow names.
	flowIPv4Fwd = "Flow-IPv4-Fwd"
	flowIPv4Bwd = "Flow-IPv4-Bwd"
	flowIPv6Fwd = "Flow-IPv6-Fwd"
	flowIPv6Bwd = "Flow-IPv6-Bwd"

	// Traffic generation parameters.
	trafficPPS = 100

	// numTunnels is the single-attachment max tunnel count (IPSEC-1.2.1/1.2.2,
	// base VLAN 10).
	numTunnels = 256

	// attachmentTunnels is the default per-attachment tunnel count for the
	// device-max tests (4 * 256 = 1024). Lower-cap platforms override it via
	// attachmentTunnelCount (Arista = 16 -> 4 * 16 = 64).
	attachmentTunnels = 256

	// numAdditionalAttachments is the count of extra attachments (VLAN 20/30/40)
	// added on top of the trimmed base attachment for the device-max tests.
	numAdditionalAttachments = 3

	// Timeout durations.
	lagUpTimeout    = 2 * time.Minute
	trafficWaitTime = 30 * time.Second

	// tunnelUpTimeout allows extra time for the many IPSec tunnels to finish IKE
	// negotiation and come UP at scale.
	tunnelUpTimeout = 10 * time.Minute
)

type SizeWeightPair struct {
	Size   uint32
	Weight float32
}

var (
	// MKA keys.
	cak         = "1234abcd1234abcd1234abcd1234abcd"
	ckn         = "12345678123456781234567812345678"
	fallbackCak = "1234abcd1234abcd1234abcd1234abce"
	fallbackCkn = "12345678123456781234567812345679"

	// DUT port groupings (reference: mpls_gre_ipv4_encap_test.go).
	// custPorts are the DUT customer/ATE-facing ports (MACsec edge).
	custPorts = []string{"port5"}
	// corePorts are the DUT-to-DUT core transport ports, grouped per LAG.
	corePorts = [][]string{
		{"port1", "port2"},
		{"port3", "port4"},
	}
	// ateCustPorts are the ATE OTG customer ports.
	ateCustPorts = []string{"port1", "port2"}

	// ATE LAG configurations (RFC 5737 test networks).
	ate1LagConfig = attrs.Attributes{
		Desc:    "ATE LAG1 configuration",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:1::2",
		IPv6Len: 126,
		MAC:     "00:00:11:01:01:01",
		MTU:     1500,
	}
	ate2LagConfig = attrs.Attributes{
		Desc:    "ATE LAG2 configuration",
		IPv4:    "203.0.113.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:2::2",
		IPv6Len: 126,
		MAC:     "00:00:12:02:02:02",
		MTU:     1500,
	}

	// DUT customer-facing (ATE-facing) interface configurations (RFC 5737 test networks).
	// DUT1: VLAN 10 with MACsec
	dut1CustIntf = attrs.Attributes{
		Desc:         "DUT1 customer interface configuration",
		IPv4:         "192.0.2.1",
		IPv4Len:      30,
		IPv6:         "2001:db8:1::1",
		IPv6Len:      126,
		MAC:          "00:00:11:01:01:03",
		MTU:          9216,
		Subinterface: 10,
	}
	// DUT2: No VLAN
	dut2CustIntf = attrs.Attributes{
		Desc:         "DUT2 customer interface configuration",
		IPv4:         "203.0.113.1",
		IPv4Len:      30,
		IPv6:         "2001:db8:2::1",
		IPv6Len:      126,
		MAC:          "00:00:12:02:02:03",
		MTU:          9216,
		Subinterface: 0,
	}

	// DUT core interface configurations (IPv6-only for DUT-to-DUT links per RFC 5737 test networks)
	dut1CoreIntf1 = attrs.Attributes{
		Desc:    "DUT1 core interface 1 configuration",
		IPv6:    "2001:db8:200:1::1",
		IPv6Len: 126,
		MAC:     "02:00:10:01:01:01",
		MTU:     9216,
	}
	dut1CoreIntf2 = attrs.Attributes{
		Desc:    "DUT1 core interface 2 configuration",
		IPv6:    "2001:db8:200:2::1",
		IPv6Len: 126,
		MAC:     "02:00:10:02:01:01",
		MTU:     9216,
	}
	dut2CoreIntf1 = attrs.Attributes{
		Desc:    "DUT2 core interface 1 configuration",
		IPv6:    "2001:db8:200:1::2",
		IPv6Len: 126,
		MAC:     "02:00:20:01:01:01",
		MTU:     9216,
	}
	dut2CoreIntf2 = attrs.Attributes{
		Desc:    "DUT2 core interface 2 configuration",
		IPv6:    "2001:db8:200:2::2",
		IPv6Len: 126,
		MAC:     "02:00:20:02:01:01",
		MTU:     9216,
	}

	// ATE LAG port MAC addresses.
	ate1LagPortMac = "00:16:01:00:00:01"
	ate2LagPortMac = "00:17:01:00:00:01"

	sizeWeightProfile = []SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 10},
		{Size: 256, Weight: 10},
		{Size: 512, Weight: 10},
		{Size: 1024, Weight: 10},
		{Size: 1500, Weight: 10},
		{Size: 4500, Weight: 10},
		{Size: 9088, Weight: 10},
	}
)

// Packet capture validation definitions, modelled on
// feature/policy_forwarding/otg_tests/mpls_gre_ipv4_encap_test/mpls_gre_ipv4_encap_test.go.
var (
	// MacsecValidations lists the validations performed on the customer-facing
	// capture to confirm traffic is MACsec-encrypted.
	MacsecValidations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateMacsecHeader,
	}
	// MacsecPacketValidation validates MACsec-encrypted packets on port1.
	MacsecPacketValidation = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "macsec-capture",
		MacsecLayer: &packetvalidationhelpers.MacsecLayer{EtherType: 0x88E5},
		Validations: MacsecValidations,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureATE(t *testing.T) gosnappi.Config {
	t.Helper()

	top := gosnappi.NewConfig()

	// Port mapping expected from testbed:
	// - ATE port1 <-> DUT1 port5 (with MACSec on DUT side)
	// - ATE port2 <-> DUT2 port5
	p1 := top.Ports().Add().SetName(ateCustPorts[0])
	p2 := top.Ports().Add().SetName(ateCustPorts[1])

	// add lags
	l1 := top.Lags().Add().SetName(ate1LagName)

	lagPort1 := l1.Ports().Add().SetPortName(p1.Name())
	lagPort1.Lacp().
		SetActorActivity("active").
		SetActorPortNumber(1)
	lagPort1.Ethernet().
		SetName("lag1Eth").
		SetMac(ate1LagPortMac).
		SetMtu(uint32(ate1LagConfig.MTU))
	l1.Protocol().Lacp().
		SetActorSystemId("00:00:00:00:00:01").
		SetActorSystemPriority(0).
		SetActorKey(1)

	// -- Macsec Config
	macsec1 := lagPort1.Macsec()
	secy1 := macsec1.SecureEntity().SetName(macsecPeerName)
	secy1Encapsulation := secy1.DataPlane().Encapsulation()
	secy1Encapsulation.CryptoEngine().EncryptDecrypt().HardwareAcceleration().InlineCrypto()
	// -- MKA Config
	mka := secy1.KeyGenerationProtocol().Mka().SetName("PeerA-Mka")
	mka.Basic().KeySource().Psk()

	mka.Basic().SetKeyDerivationFunction(gosnappi.MkaBasicKeyDerivationFunctionEnum("aes_cmac_128"))
	mka.Basic().SetSendIcvIndicatiorInMkpdu(false)
	mka.Basic().SetMkaVersion(2)
	scs := mka.Basic().SupportedCipherSuites()
	scs.SetGcmAes256(false)
	scs.SetGcmAesXpn256(false)

	onePsk := mka.Basic().KeySource().Psks().Add()
	onePsk.SetCakValue(cak)
	onePsk.SetCakName(ckn)
	secureChannel := mka.Tx().SecureChannels().Add()
	secureChannel.SetName("SecureChannel1").
		SetSystemId(lagPort1.Ethernet().Mac())

	// add devices
	d1 := top.Devices().Add().SetName(ate1DevName)
	// add protocol stacks for device d1
	d1Eth1 := d1.Ethernets().
		Add().
		SetName("d1Eth").
		SetMac(ate1LagConfig.MAC)
	d1Eth1.Connection().SetLagName(l1.Name())

	d1Eth1.Vlans().Add().SetName("d1EthVlan").SetId(vlanID)

	d1ipv4 := d1Eth1.Ipv4Addresses().
		Add().
		SetName("p1d1ipv4").
		SetAddress(ate1LagConfig.IPv4).
		SetGateway(dut1CustIntf.IPv4).
		SetPrefix(uint32(ate1LagConfig.IPv4Len))

	d1ipv6 := d1Eth1.Ipv6Addresses().
		Add().
		SetName("p1d1ipv6").
		SetAddress(ate1LagConfig.IPv6).
		SetGateway(dut1CustIntf.IPv6).
		SetPrefix(uint32(ate1LagConfig.IPv6Len))

	l2 := top.Lags().Add().SetName(ate2LagName)
	lagPort2 := l2.Ports().Add().SetPortName(p2.Name())
	lagPort2.Lacp().
		SetActorActivity("active").
		SetActorPortNumber(1)
	lagPort2.Ethernet().
		SetName("lag2Eth").
		SetMac(ate2LagPortMac).
		SetMtu(uint32(ate2LagConfig.MTU))
	l2.Protocol().Lacp().
		SetActorSystemId("00:00:00:00:00:02").
		SetActorSystemPriority(0).
		SetActorKey(1)

	d2 := top.Devices().Add().SetName(ate2DevName)
	d2Eth1 := d2.Ethernets().
		Add().
		SetName("d2Eth").
		SetMac(ate2LagConfig.MAC)
	d2Eth1.Connection().SetLagName(l2.Name())

	d2ipv4 := d2Eth1.Ipv4Addresses().
		Add().
		SetName("p2d2ipv4").
		SetAddress(ate2LagConfig.IPv4).
		SetGateway(dut2CustIntf.IPv4).
		SetPrefix(uint32(ate2LagConfig.IPv4Len))

	d2ipv6 := d2Eth1.Ipv6Addresses().
		Add().
		SetName("p2d2ipv6").
		SetAddress(ate2LagConfig.IPv6).
		SetGateway(dut2CustIntf.IPv6).
		SetPrefix(uint32(ate2LagConfig.IPv6Len))

	if len(top.Flows().Items()) > 0 {
		top.Flows().Clear()
	}
	flow := top.Flows().Add().SetName(flowIPv4Fwd)
	flow.TxRx().Device().SetTxNames([]string{d1ipv4.Name()}).SetRxNames([]string{d2ipv4.Name()})

	for _, sizeWeight := range sizeWeightProfile {
		flow.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}

	flow.Rate().SetPps(trafficPPS)
	flow.Duration().Continuous()
	flow.Metrics().SetEnable(true)

	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(ate1LagConfig.MAC)

	flow.Packet().Add().Macsec()

	vlan := flow.Packet().Add().Vlan()
	vlan.Id().SetValue(vlanID)

	v4 := flow.Packet().Add().Ipv4()
	// Increment the source address to generate flow entropy so that ECMP hashing
	// spreads the customer traffic across the DUT-to-DUT member links.
	v4.Src().Increment().SetStart(ate1LagConfig.IPv4).SetStep("0.0.0.1").SetCount(1000)
	v4.Dst().SetValue(ate2LagConfig.IPv4)

	flowBwd := top.Flows().Add().SetName(flowIPv4Bwd)
	flowBwd.TxRx().Device().SetTxNames([]string{d2ipv4.Name()}).SetRxNames([]string{d1ipv4.Name()})

	for _, sizeWeight := range sizeWeightProfile {
		flowBwd.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}

	flowBwd.Rate().SetPps(trafficPPS)
	flowBwd.Duration().Continuous()
	flowBwd.Metrics().SetEnable(true)

	e2 := flowBwd.Packet().Add().Ethernet()
	e2.Src().SetValue(ate2LagConfig.MAC)

	v4Bwd := flowBwd.Packet().Add().Ipv4()
	v4Bwd.Src().SetValue(ate2LagConfig.IPv4)
	v4Bwd.Dst().SetValue(ate1LagConfig.IPv4)

	// IPv6 Flow from port1 to port2.
	flowV6 := top.Flows().Add().SetName(flowIPv6Fwd)
	flowV6.TxRx().Device().SetTxNames([]string{d1ipv6.Name()}).SetRxNames([]string{d2ipv6.Name()})

	for _, sizeWeight := range sizeWeightProfile {
		flowV6.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}
	flowV6.Rate().SetPps(trafficPPS)
	flowV6.Duration().Continuous()
	flowV6.Metrics().SetEnable(true)

	e3 := flowV6.Packet().Add().Ethernet()
	e3.Src().SetValue(ate1LagConfig.MAC)

	flowV6.Packet().Add().Macsec()

	vlan3 := flowV6.Packet().Add().Vlan()
	vlan3.Id().SetValue(vlanID)

	v6 := flowV6.Packet().Add().Ipv6()
	// Increment the source address to generate flow entropy so that ECMP hashing
	// spreads the customer traffic across the DUT-to-DUT member links.
	v6.Src().Increment().SetStart(ate1LagConfig.IPv6).SetStep("::1").SetCount(1000)
	v6.Dst().SetValue(ate2LagConfig.IPv6)

	// IPv6 Flow from port2 to port1.
	flowV6Bwd := top.Flows().Add().SetName(flowIPv6Bwd)
	flowV6Bwd.TxRx().Device().SetTxNames([]string{d2ipv6.Name()}).SetRxNames([]string{d1ipv6.Name()})

	for _, sizeWeight := range sizeWeightProfile {
		flowV6Bwd.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}

	flowV6Bwd.Rate().SetPps(trafficPPS)
	flowV6Bwd.Duration().Continuous()
	flowV6Bwd.Metrics().SetEnable(true)

	e4 := flowV6Bwd.Packet().Add().Ethernet()
	e4.Src().SetValue(ate2LagConfig.MAC)

	v6Bwd := flowV6Bwd.Packet().Add().Ipv6()
	v6Bwd.Src().SetValue(ate2LagConfig.IPv6)
	v6Bwd.Dst().SetValue(ate1LagConfig.IPv6)

	return top
}

// tunnelBlock describes one block of parallel IPSec tunnels: size, global
// index/name offset, tunnel VRF, and the loopback/overlay addressing schemes.
// Distinct blocks (one per attachment) use non-overlapping schemes.
type tunnelBlock struct {
	numTunnels int
	startIndex int // global offset added to loopback/tunnel numbering
	tunnelVRF  string
	v4Seed1    string // DUT1 overlay IPv4 seed (incremented by v4Step)
	v4Seed2    string // DUT2 overlay IPv4 seed (incremented by v4Step)
	v4Step     string
	lbV6Fmt1   string // DUT1 loopback IPv6 format (single %x for tunnel index)
	lbV6Fmt2   string // DUT2 loopback IPv6 format
	tunV6Fmt1  string // DUT1 overlay IPv6 format
	tunV6Fmt2  string // DUT2 overlay IPv6 format
}

// baseTunnelBlock returns the base attachment's tunnel block (IPSEC-1.2.1/1.2.2).
func baseTunnelBlock(n int) tunnelBlock {
	return tunnelBlock{
		numTunnels: n,
		startIndex: 0,
		tunnelVRF:  tunnelVRF,
		v4Seed1:    "100.64.0.1",
		v4Seed2:    "100.64.0.2",
		v4Step:     "0.0.0.4",
		lbV6Fmt1:   "2001:db8:31:%x::1",
		lbV6Fmt2:   "2001:db8:32:%x::1",
		tunV6Fmt1:  "2001:db8:100:%x::1",
		tunV6Fmt2:  "2001:db8:100:%x::2",
	}
}

// configureScaledTunnels configures the base customer attachment's tunnel block.
func configureScaledTunnels(t *testing.T, dut1, dut2 *ondatra.DUTDevice, numTunnels int) (dut1TunnelV4NHs, dut2TunnelV4NHs, dut1TunnelV6NHs, dut2TunnelV6NHs []string) {
	return configureTunnelBlock(t, dut1, dut2, baseTunnelBlock(numTunnels))
}

// configureTunnelBlock configures blk.numTunnels parallel IPSec tunnels between dut1
// and dut2 in blk.tunnelVRF and returns the per-tunnel next-hops used to ECMP traffic.
func configureTunnelBlock(t *testing.T, dut1, dut2 *ondatra.DUTDevice, blk tunnelBlock) (dut1TunnelV4NHs, dut2TunnelV4NHs, dut1TunnelV6NHs, dut2TunnelV6NHs []string) {
	t.Helper()

	// Tunnel overlay addressing: per-block IPv4 /30 seeds and IPv6 /64s plus
	// loopback /128s derived per tunnel index below.
	dut1TunnelV4s, err := iputil.GenerateIPsWithStep(blk.v4Seed1, blk.numTunnels, blk.v4Step)
	if err != nil {
		t.Fatalf("failed to generate DUT1 tunnel IPv4 addresses: %v", err)
	}
	dut2TunnelV4s, err := iputil.GenerateIPsWithStep(blk.v4Seed2, blk.numTunnels, blk.v4Step)
	if err != nil {
		t.Fatalf("failed to generate DUT2 tunnel IPv4 addresses: %v", err)
	}

	// Accumulate per-tunnel CLI blocks and reachability routes and push them once
	// per DUT after the loop; one Set per tunnel is prohibitively slow at scale.
	var dut1Tunnels, dut2Tunnels []string
	var dut1ReachRoutes, dut2ReachRoutes []*cfgplugins.StaticRouteCfg
	dut1LoopbackBatch := &gnmi.SetBatch{}
	dut2LoopbackBatch := &gnmi.SetBatch{}

	for i := 0; i < blk.numTunnels; i++ {
		global := blk.startIndex + i
		n := global + 1
		lbName := fmt.Sprintf("Loopback%d", global)
		tunName := fmt.Sprintf("Tunnel%d", n)
		dut1LbV6 := fmt.Sprintf(blk.lbV6Fmt1, i)
		dut2LbV6 := fmt.Sprintf(blk.lbV6Fmt2, i)
		dut1TunV6 := fmt.Sprintf(blk.tunV6Fmt1, i)
		dut2TunV6 := fmt.Sprintf(blk.tunV6Fmt2, i)
		ikePolicy := fmt.Sprintf("IKE_POLICY_%d", n)
		saPolicy := fmt.Sprintf("SA_POLICY_%d", n)
		profile := fmt.Sprintf("IPSEC_PROFILE_%d", n)

		// Per-tunnel loopback endpoints.
		cfgplugins.ConfigureLoopback(t, dut1, cfgplugins.LoopbackConfig{
			Name:      lbName,
			IP:        dut1LbV6,
			PrefixLen: loopbackPrefixLen,
			IsIPv6:    true,
			Batch:     dut1LoopbackBatch,
		})
		cfgplugins.ConfigureLoopback(t, dut2, cfgplugins.LoopbackConfig{
			Name:      lbName,
			IP:        dut2LbV6,
			PrefixLen: loopbackPrefixLen,
			IsIPv6:    true,
			Batch:     dut2LoopbackBatch,
		})

		dut1Cfg := cfgplugins.IPSecTunnelCfg{
			TunnelName:  tunName,
			Description: fmt.Sprintf("IPsec Tunnel Pair %d to DUT2", n),
			LocalFQDN:   fmt.Sprintf("dut1-t%d.test.local", n),
			RemoteFQDN:  fmt.Sprintf("dut2-t%d.test.local", n),
			TunnelIPv4:  fmt.Sprintf("%s/30", dut1TunnelV4s[i]),
			TunnelIPv6:  fmt.Sprintf("%s/64", dut1TunV6),
			TunnelSrc:   dut1LbV6,
			TunnelDst:   dut2LbV6,
			TunnelVRF:   blk.tunnelVRF,
			IKEPolicy:   ikePolicy,
			SAPolicy:    saPolicy,
			Profile:     profile,
		}
		dut2Cfg := cfgplugins.IPSecTunnelCfg{
			TunnelName:  tunName,
			Description: fmt.Sprintf("IPsec Tunnel Pair %d to DUT1", n),
			LocalFQDN:   fmt.Sprintf("dut2-t%d.test.local", n),
			RemoteFQDN:  fmt.Sprintf("dut1-t%d.test.local", n),
			TunnelIPv4:  fmt.Sprintf("%s/30", dut2TunnelV4s[i]),
			TunnelIPv6:  fmt.Sprintf("%s/64", dut2TunV6),
			TunnelSrc:   dut2LbV6,
			TunnelDst:   dut1LbV6,
			TunnelVRF:   blk.tunnelVRF,
			IKEPolicy:   ikePolicy,
			SAPolicy:    saPolicy,
			Profile:     profile,
		}

		// On vendors without an OpenConfig IPSec model (e.g. Arista), batch the
		// CLI blocks; otherwise configure each tunnel via OpenConfig immediately.
		if deviations.IpsecOcUnsupported(dut1) {
			dut1Tunnels = append(dut1Tunnels, cfgplugins.BuildIPSecTunnel(dut1Cfg))
		} else {
			batch1 := cfgplugins.ConfigureIPSecTunnel(t, dut1, dut1Cfg)
			batch1.Set(t, dut1)
		}
		if deviations.IpsecOcUnsupported(dut2) {
			dut2Tunnels = append(dut2Tunnels, cfgplugins.BuildIPSecTunnel(dut2Cfg))
		} else {
			batch2 := cfgplugins.ConfigureIPSecTunnel(t, dut2, dut2Cfg)
			batch2.Set(t, dut2)
		}

		// Reachability to the far-end loopback endpoint over both DUT-DUT core LAGs.
		dut1ReachRoutes = append(dut1ReachRoutes,
			&cfgplugins.StaticRouteCfg{Prefix: fmt.Sprintf("%s/128", dut2LbV6), NextHopAddr: dut2CoreIntf2.IPv6},
			&cfgplugins.StaticRouteCfg{Prefix: fmt.Sprintf("%s/128", dut2LbV6), NextHopAddr: dut2CoreIntf1.IPv6},
		)
		dut2ReachRoutes = append(dut2ReachRoutes,
			&cfgplugins.StaticRouteCfg{Prefix: fmt.Sprintf("%s/128", dut1LbV6), NextHopAddr: dut1CoreIntf2.IPv6},
			&cfgplugins.StaticRouteCfg{Prefix: fmt.Sprintf("%s/128", dut1LbV6), NextHopAddr: dut1CoreIntf1.IPv6},
		)

		dut1TunnelV4NHs = append(dut1TunnelV4NHs, dut1TunnelV4s[i])
		dut2TunnelV4NHs = append(dut2TunnelV4NHs, dut2TunnelV4s[i])
		dut1TunnelV6NHs = append(dut1TunnelV6NHs, dut1TunV6)
		dut2TunnelV6NHs = append(dut2TunnelV6NHs, dut2TunV6)
	}

	// Execute the batched loopback configurations.
	dut1LoopbackBatch.Set(t, dut1)
	dut2LoopbackBatch.Set(t, dut2)

	// Push all accumulated tunnel CLI in a single gNMI CLI Set per DUT.
	if len(dut1Tunnels) > 0 {
		helpers.GnmiCLIConfig(t, dut1, strings.Join(dut1Tunnels, "\n"))
	}
	if len(dut2Tunnels) > 0 {
		helpers.GnmiCLIConfig(t, dut2, strings.Join(dut2Tunnels, "\n"))
	}

	// Program all far-end loopback reachability routes in a single batched call
	// per DUT (ConfigureStaticRoutesInVRF coalesces them into one gNMI Set).
	cfgplugins.ConfigureStaticRoutesInVRF(t, dut1, dut1ReachRoutes)
	cfgplugins.ConfigureStaticRoutesInVRF(t, dut2, dut2ReachRoutes)

	return dut1TunnelV4NHs, dut2TunnelV4NHs, dut1TunnelV6NHs, dut2TunnelV6NHs
}

// custAttachment describes one additional device-max attachment (IPSEC-1.2.3/1.2.4):
// a distinct VLAN + VRF pair with its own block of IPSec tunnels.
type custAttachment struct {
	vlanID    uint16 // customer VLAN / subinterface ID
	custVRF   string // customer-facing VRF
	tunnelVRF string // tunnel-facing VRF

	dut1Cust attrs.Attributes // DUT1 customer subinterface (ATE1 side)
	dut2Cust attrs.Attributes // DUT2 customer subinterface (ATE2 side)
	ate1     attrs.Attributes // ATE1-side endpoint
	ate2     attrs.Attributes // ATE2-side endpoint

	ate1MAC string
	ate2MAC string

	// Static route prefixes reachable behind each ATE endpoint.
	ate1V4Prefix string
	ate2V4Prefix string
	ate1V6Prefix string
	ate2V6Prefix string

	// OTG object names.
	ate1DevName string
	ate2DevName string
	v4Name1     string
	v6Name1     string
	v4Name2     string
	v6Name2     string
	flowV4Fwd   string
	flowV4Bwd   string
	flowV6Fwd   string
	flowV6Bwd   string

	tunnels tunnelBlock // tunnel addressing/naming for this attachment
}

// attachmentTunnelCount returns the per-attachment tunnel count, overridden per
// platform where the multi-attachment IPsec forwarding cap is lower than default.
func attachmentTunnelCount(dut *ondatra.DUTDevice) int {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return 16
	default:
		return attachmentTunnels
	}
}

// aggregateSubinterfaceName returns the interface name to use for CLI-based VRF assignment.
func aggregateSubinterfaceName(lagName string, subinterfaceID uint32) string {
	if subinterfaceID == 0 {
		return lagName
	}
	return fmt.Sprintf("%s.%d", lagName, subinterfaceID)
}

// assignAggregateToVRF assigns an aggregate/subinterface to the requested VRF using OC,
// keeping VRF assignment separate from interface modelling.
func assignAggregateToVRF(t *testing.T, dut *ondatra.DUTDevice, lagName string, subinterfaceID uint32, vrfName string) {
	t.Helper()
	if vrfName == "" {
		return
	}

	intfName := aggregateSubinterfaceName(lagName, subinterfaceID)
	d := gnmi.OC()
	ni := d.NetworkInstance(vrfName).Interface(intfName)
	gnmi.Update(t, dut, ni.Config(), &oc.NetworkInstance_Interface{
		Id:           ygot.String(intfName),
		Interface:    ygot.String(lagName),
		Subinterface: ygot.Uint32(subinterfaceID),
	})
}

// createVRFs creates VRFs via OC, or via a single atomic CLI transaction on Arista
// where the routing-enable commands have no OC equivalent.
func createVRFs(t *testing.T, dut *ondatra.DUTDevice, vrfNames []string) {
	t.Helper()
	if len(vrfNames) == 0 {
		return
	}
	if deviations.IpRoutingInVrfOcUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			// Arista needs vrf instance + ip routing + ipv6 unicast-routing in one
			// atomic CLI block; OC cannot express the routing-enable commands.
			var b strings.Builder
			for _, vrfName := range vrfNames {
				if vrfName == "" {
					continue
				}
				fmt.Fprintf(&b, "vrf instance %s\n!\nip routing vrf %s\n!\nipv6 unicast-routing vrf %s\n!\n",
					vrfName, vrfName, vrfName)
			}
			if cli := b.String(); cli != "" {
				t.Logf("Creating VRF(s) via CLI on Arista (routing-enable): %v", vrfNames)
				helpers.GnmiCLIConfig(t, dut, cli)
			}
		}
	} else {
		// All other vendors: create VRF instance using OpenConfig.
		d := gnmi.OC()
		for _, vrfName := range vrfNames {
			if vrfName == "" {
				continue
			}
			ni := &oc.NetworkInstance{Name: ygot.String(vrfName)}
			ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
			gnmi.Update(t, dut, d.NetworkInstance(vrfName).Config(), ni)
		}
		t.Logf("Applied OC VRF creation for %d VRF(s): %v", len(vrfNames), vrfNames)
	}
}

// configureLAGInterface sets up the LAG aggregate interface, LACP, member ports, subinterfaces, and optional VRF assignment.
func configureLAGInterface(t *testing.T, dut *ondatra.DUTDevice, lagName string, ports []*ondatra.Port, a *attrs.Attributes, vrfName string) {
	t.Helper()
	d := gnmi.OC()

	// Configure aggregate interface first, then LACP, then members.
	lacp := &oc.Lacp_Interface{Name: ygot.String(lagName)}
	lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE

	agg := &oc.Interface{Name: ygot.String(lagName)}
	agg.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(dut) {
		agg.Enabled = ygot.Bool(true)
	}
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	subID := a.ID
	if subID == 0 && a.Subinterface != 0 {
		subID = a.Subinterface
	}

	// First transaction: create the aggregate (without subinterfaces/IPs),
	// create the LACP entry and configure member ports.
	if deviations.AggregateAtomicUpdate(dut) {
		batch := &gnmi.SetBatch{}
		gnmi.BatchUpdate(batch, d.Interface(lagName).Config(), agg)
		gnmi.BatchUpdate(batch, d.Lacp().Interface(lagName).Config(), lacp)
		for _, p := range ports {
			i := &oc.Interface{Name: ygot.String(p.Name())}
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			e := i.GetOrCreateEthernet()
			e.AggregateId = ygot.String(lagName)
			gnmi.BatchUpdate(batch, d.Interface(p.Name()).Config(), i)
		}
		batch.Set(t, dut)
	} else {
		gnmi.Update(t, dut, d.Interface(lagName).Config(), agg)
		gnmi.Update(t, dut, d.Lacp().Interface(lagName).Config(), lacp)
		for _, p := range ports {
			i := &oc.Interface{Name: ygot.String(p.Name())}
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			e := i.GetOrCreateEthernet()
			e.AggregateId = ygot.String(lagName)
			gnmi.Update(t, dut, d.Interface(p.Name()).Config(), i)
		}
	}

	// Assign VRF before programming IP addresses.
	assignAggregateToVRF(t, dut, lagName, subID, vrfName)

	full := &oc.Interface{Name: ygot.String(lagName)}
	full.GetOrCreateAggregation().LagType = agg.GetOrCreateAggregation().GetLagType()
	full.Type = agg.Type
	full.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(dut) {
		full.Enabled = ygot.Bool(true)
	}
	// Subinterface 0 with matching IPv4 and IPv6 MTU.
	s0 := full.GetOrCreateSubinterface(0)
	s0.GetOrCreateIpv4().Mtu = ygot.Uint16(a.MTU)
	s0.GetOrCreateIpv6().Mtu = ygot.Uint32(uint32(a.MTU))
	if deviations.InterfaceEnabled(dut) {
		s0.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		s0.GetOrCreateIpv6().Enabled = ygot.Bool(true)
	}
	if subID != 0 {
		s := full.GetOrCreateSubinterface(subID)
		s.GetOrCreateIpv4().Mtu = ygot.Uint16(a.MTU)
		s.GetOrCreateIpv6().Mtu = ygot.Uint32(uint32(a.MTU))
		if deviations.InterfaceEnabled(dut) {
			s.GetOrCreateIpv4().Enabled = ygot.Bool(true)
			s.GetOrCreateIpv6().Enabled = ygot.Bool(true)
		}
		cfgplugins.ConfigureVLAN(s, dut, uint16(subID))
		cfgplugins.ConfigureSubinterfaceIPs(s, dut, a.IPv4, a.IPv4Len, a.IPv6, a.IPv6Len)
	} else {
		cfgplugins.ConfigureSubinterfaceIPs(s0, dut, a.IPv4, a.IPv4Len, a.IPv6, a.IPv6Len)
	}

	if deviations.AggregateAtomicUpdate(dut) {
		post := &gnmi.SetBatch{}
		gnmi.BatchUpdate(post, d.Interface(lagName).Config(), full)
		post.Set(t, dut)
	} else {
		gnmi.Update(t, dut, d.Interface(lagName).Config(), full)
	}
}

// configureDUT configures the LAG aggregates on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, portGroups [][]*ondatra.Port, portAttrs []attrs.Attributes, vrfName string) []string {
	t.Helper()

	if len(portGroups) != len(portAttrs) {
		t.Fatalf("mismatched portGroups and portAttrs lengths")
	}

	lagNames := make([]string, 0, len(portGroups))
	for i := range portGroups {
		lag := netutil.NextAggregateInterface(t, dut)
		configureLAGInterface(t, dut, lag, portGroups[i], &portAttrs[i], vrfName)
		lagNames = append(lagNames, lag)
	}
	return lagNames
}

// addCustomerAttachmentSubinterface adds a VLAN-tagged customer subinterface to an
// existing aggregate and assigns it to the attachment's VRF.
func addCustomerAttachmentSubinterface(t *testing.T, dut *ondatra.DUTDevice, lagName string, vlan uint16, a attrs.Attributes, vrfName string) {
	t.Helper()

	assignAggregateToVRF(t, dut, lagName, uint32(vlan), vrfName)

	agg := &oc.Interface{Name: ygot.String(lagName)}
	s := agg.GetOrCreateSubinterface(uint32(vlan))
	cfgplugins.ConfigureVLAN(s, dut, vlan)
	s4 := s.GetOrCreateIpv4()
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
		s6.Enabled = ygot.Bool(true)
	}
	s4.Mtu = ygot.Uint16(a.MTU)
	s6.Mtu = ygot.Uint32(uint32(a.MTU))
	cfgplugins.ConfigureSubinterfaceIPs(s, dut, a.IPv4, a.IPv4Len, a.IPv6, a.IPv6Len)
	gnmi.Update(t, dut, gnmi.OC().Interface(lagName).Config(), agg)
}

// buildAttachments computes n additional attachments with non-overlapping VLANs,
// VRFs, IP addressing and tunnel blocks of perAttachmentTunnels tunnels each.
func buildAttachments(n, perAttachmentTunnels int) []custAttachment {
	atts := make([]custAttachment, 0, n)
	for k := 1; k <= n; k++ {
		att := custAttachment{
			vlanID:    uint16(10 * (k + 1)),
			custVRF:   fmt.Sprintf("ATE_VRF_%d", k),
			tunnelVRF: fmt.Sprintf("TUNNEL_VRF_%d", k),
			ate1MAC:   fmt.Sprintf("00:00:11:01:01:%02x", 0x20+k),
			ate2MAC:   fmt.Sprintf("00:00:12:02:02:%02x", 0x20+k),
		}
		att.dut1Cust = attrs.Attributes{
			Desc:         fmt.Sprintf("DUT1 attachment %d customer interface", k),
			IPv4:         fmt.Sprintf("192.0.2.%d", 4*k+1),
			IPv4Len:      30,
			IPv6:         fmt.Sprintf("2001:db8:1:%x::1", k),
			IPv6Len:      126,
			MAC:          fmt.Sprintf("00:00:11:01:01:%02x", 0x40+k),
			MTU:          9216,
			Subinterface: uint32(att.vlanID),
		}
		att.dut2Cust = attrs.Attributes{
			Desc:         fmt.Sprintf("DUT2 attachment %d customer interface", k),
			IPv4:         fmt.Sprintf("203.0.113.%d", 4*k+1),
			IPv4Len:      30,
			IPv6:         fmt.Sprintf("2001:db8:2:%x::1", k),
			IPv6Len:      126,
			MAC:          fmt.Sprintf("00:00:12:02:02:%02x", 0x40+k),
			MTU:          9216,
			Subinterface: uint32(att.vlanID),
		}
		att.ate1 = attrs.Attributes{
			Desc:    fmt.Sprintf("ATE1 attachment %d", k),
			IPv4:    fmt.Sprintf("192.0.2.%d", 4*k+2),
			IPv4Len: 30,
			IPv6:    fmt.Sprintf("2001:db8:1:%x::2", k),
			IPv6Len: 126,
			MAC:     att.ate1MAC,
			MTU:     1500,
		}
		att.ate2 = attrs.Attributes{
			Desc:    fmt.Sprintf("ATE2 attachment %d", k),
			IPv4:    fmt.Sprintf("203.0.113.%d", 4*k+2),
			IPv4Len: 30,
			IPv6:    fmt.Sprintf("2001:db8:2:%x::2", k),
			IPv6Len: 126,
			MAC:     att.ate2MAC,
			MTU:     1500,
		}
		att.ate1V4Prefix = fmt.Sprintf("192.0.2.%d/30", 4*k)
		att.ate2V4Prefix = fmt.Sprintf("203.0.113.%d/30", 4*k)
		att.ate1V6Prefix = fmt.Sprintf("2001:db8:1:%x::/126", k)
		att.ate2V6Prefix = fmt.Sprintf("2001:db8:2:%x::/126", k)

		att.ate1DevName = fmt.Sprintf("d1att%d", k)
		att.ate2DevName = fmt.Sprintf("d2att%d", k)
		att.v4Name1 = fmt.Sprintf("p1d1ipv4att%d", k)
		att.v6Name1 = fmt.Sprintf("p1d1ipv6att%d", k)
		att.v4Name2 = fmt.Sprintf("p2d2ipv4att%d", k)
		att.v6Name2 = fmt.Sprintf("p2d2ipv6att%d", k)
		att.flowV4Fwd = fmt.Sprintf("Flow-IPv4-Fwd-att%d", k)
		att.flowV4Bwd = fmt.Sprintf("Flow-IPv4-Bwd-att%d", k)
		att.flowV6Fwd = fmt.Sprintf("Flow-IPv6-Fwd-att%d", k)
		att.flowV6Bwd = fmt.Sprintf("Flow-IPv6-Bwd-att%d", k)

		att.tunnels = tunnelBlock{
			numTunnels: perAttachmentTunnels,
			startIndex: k * perAttachmentTunnels,
			tunnelVRF:  att.tunnelVRF,
			v4Seed1:    fmt.Sprintf("100.%d.0.1", 64+k),
			v4Seed2:    fmt.Sprintf("100.%d.0.2", 64+k),
			v4Step:     "0.0.0.4",
			lbV6Fmt1:   fmt.Sprintf("2001:db8:a1%02x:%%x::1", k),
			lbV6Fmt2:   fmt.Sprintf("2001:db8:a2%02x:%%x::1", k),
			tunV6Fmt1:  fmt.Sprintf("2001:db8:b0%02x:%%x::1", k),
			tunV6Fmt2:  fmt.Sprintf("2001:db8:b0%02x:%%x::2", k),
		}
		atts = append(atts, att)
	}
	return atts
}

// configureAttachments configures the DUT side of each attachment: VRFs, VLAN
// subinterfaces, tunnel block and the customer ECMP static routes.
func configureAttachments(t *testing.T, dut1, dut2 *ondatra.DUTDevice, dut1CustLag, dut2CustLag string, atts []custAttachment) {
	t.Helper()

	for _, att := range atts {
		createVRFs(t, dut1, []string{att.custVRF, att.tunnelVRF})
		createVRFs(t, dut2, []string{att.custVRF, att.tunnelVRF})

		addCustomerAttachmentSubinterface(t, dut1, dut1CustLag, att.vlanID, att.dut1Cust, att.custVRF)
		addCustomerAttachmentSubinterface(t, dut2, dut2CustLag, att.vlanID, att.dut2Cust, att.custVRF)

		d1V4NHs, d2V4NHs, d1V6NHs, d2V6NHs := configureTunnelBlock(t, dut1, dut2, att.tunnels)

		// DUT1 routing: customer return path towards ATE1, plus ECMP of customer
		// traffic destined to ATE2 across every tunnel's DUT2-side next-hop.
		dut1Routes := []*cfgplugins.StaticRouteCfg{
			{Prefix: att.ate1V4Prefix, NextHopAddr: att.ate1.IPv4, NetworkInstance: att.tunnelVRF, NextNetworkInstance: att.custVRF},
			{Prefix: att.ate1V6Prefix, NextHopAddr: att.ate1.IPv6, NetworkInstance: att.tunnelVRF, NextNetworkInstance: att.custVRF},
		}
		for _, nh := range d2V4NHs {
			dut1Routes = append(dut1Routes, &cfgplugins.StaticRouteCfg{Prefix: att.ate2V4Prefix, NextHopAddr: nh, NetworkInstance: att.custVRF, NextNetworkInstance: att.tunnelVRF})
		}
		for _, nh := range d2V6NHs {
			dut1Routes = append(dut1Routes, &cfgplugins.StaticRouteCfg{Prefix: att.ate2V6Prefix, NextHopAddr: nh, NetworkInstance: att.custVRF, NextNetworkInstance: att.tunnelVRF})
		}
		cfgplugins.ConfigureStaticRoutesInVRF(t, dut1, dut1Routes)

		// DUT2 routing: customer return path towards ATE2, plus ECMP of customer
		// traffic destined to ATE1 across every tunnel's DUT1-side next-hop.
		dut2Routes := []*cfgplugins.StaticRouteCfg{
			{Prefix: att.ate2V4Prefix, NextHopAddr: att.ate2.IPv4, NetworkInstance: att.tunnelVRF, NextNetworkInstance: att.custVRF},
			{Prefix: att.ate2V6Prefix, NextHopAddr: att.ate2.IPv6, NetworkInstance: att.tunnelVRF, NextNetworkInstance: att.custVRF},
		}
		for _, nh := range d1V4NHs {
			dut2Routes = append(dut2Routes, &cfgplugins.StaticRouteCfg{Prefix: att.ate1V4Prefix, NextHopAddr: nh, NetworkInstance: att.custVRF, NextNetworkInstance: att.tunnelVRF})
		}
		for _, nh := range d1V6NHs {
			dut2Routes = append(dut2Routes, &cfgplugins.StaticRouteCfg{Prefix: att.ate1V6Prefix, NextHopAddr: nh, NetworkInstance: att.custVRF, NextNetworkInstance: att.tunnelVRF})
		}
		cfgplugins.ConfigureStaticRoutesInVRF(t, dut2, dut2Routes)
	}
}

// trimBaseAttachment shrinks the base attachment from numTunnels to
// perAttachmentTunnels, removing the surplus tunnels/loopbacks and their routes.
func trimBaseAttachment(t *testing.T, dut1, dut2 *ondatra.DUTDevice, perAttachmentTunnels int, d1V4NHs, d2V4NHs, d1V6NHs, d2V6NHs []string) {
	t.Helper()

	// Remove tunnels/loopbacks with index [perAttachmentTunnels, numTunnels-1]
	// (tunnel numbers [perAttachmentTunnels+1, numTunnels]).
	cfgplugins.RemoveTunnelRange(t, dut1, dut2, cfgplugins.TunnelRangeParams{
		StartTunnel: perAttachmentTunnels + 1,
		EndTunnel:   numTunnels,
	})

	blk := baseTunnelBlock(numTunnels)
	var dut1Rm, dut2Rm []*cfgplugins.StaticRouteCfg
	for i := perAttachmentTunnels; i < numTunnels; i++ {
		// Base customer-ECMP routes that pointed at the removed tunnels.
		dut1Rm = append(dut1Rm,
			&cfgplugins.StaticRouteCfg{Prefix: ate2IPv4Prefix, NextHopAddr: d2V4NHs[i], NetworkInstance: ateVRF, NextNetworkInstance: tunnelVRF},
			&cfgplugins.StaticRouteCfg{Prefix: ate2IPv6Prefix, NextHopAddr: d2V6NHs[i], NetworkInstance: ateVRF, NextNetworkInstance: tunnelVRF},
		)
		dut2Rm = append(dut2Rm,
			&cfgplugins.StaticRouteCfg{Prefix: ate1IPv4Prefix, NextHopAddr: d1V4NHs[i], NetworkInstance: ateVRF, NextNetworkInstance: tunnelVRF},
			&cfgplugins.StaticRouteCfg{Prefix: ate1IPv6Prefix, NextHopAddr: d1V6NHs[i], NetworkInstance: ateVRF, NextNetworkInstance: tunnelVRF},
		)
		// Loopback reachability routes for the removed loopbacks (default VRF).
		d1LbV6 := fmt.Sprintf(blk.lbV6Fmt1, i)
		d2LbV6 := fmt.Sprintf(blk.lbV6Fmt2, i)
		dut1Rm = append(dut1Rm,
			&cfgplugins.StaticRouteCfg{Prefix: fmt.Sprintf("%s/128", d2LbV6), NextHopAddr: dut2CoreIntf2.IPv6},
			&cfgplugins.StaticRouteCfg{Prefix: fmt.Sprintf("%s/128", d2LbV6), NextHopAddr: dut2CoreIntf1.IPv6},
		)
		dut2Rm = append(dut2Rm,
			&cfgplugins.StaticRouteCfg{Prefix: fmt.Sprintf("%s/128", d1LbV6), NextHopAddr: dut1CoreIntf2.IPv6},
			&cfgplugins.StaticRouteCfg{Prefix: fmt.Sprintf("%s/128", d1LbV6), NextHopAddr: dut1CoreIntf1.IPv6},
		)
	}
	cfgplugins.RemoveStaticRoutesInVRF(t, dut1, dut1Rm)
	cfgplugins.RemoveStaticRoutesInVRF(t, dut2, dut2Rm)
}

// addAttachmentOTG adds the ATE-side devices and flows for one additional
// attachment on the existing customer LAGs (ate1LagName / ate2LagName).
func addAttachmentOTG(top gosnappi.Config, att custAttachment) {
	d1 := top.Devices().Add().SetName(att.ate1DevName)
	eth1 := d1.Ethernets().Add().SetName(att.ate1DevName + "Eth").SetMac(att.ate1MAC)
	eth1.Connection().SetLagName(ate1LagName)
	eth1.Vlans().Add().SetName(att.ate1DevName + "Vlan").SetId(uint32(att.vlanID))
	eth1.Ipv4Addresses().Add().SetName(att.v4Name1).SetAddress(att.ate1.IPv4).SetGateway(att.dut1Cust.IPv4).SetPrefix(uint32(att.ate1.IPv4Len))
	eth1.Ipv6Addresses().Add().SetName(att.v6Name1).SetAddress(att.ate1.IPv6).SetGateway(att.dut1Cust.IPv6).SetPrefix(uint32(att.ate1.IPv6Len))

	d2 := top.Devices().Add().SetName(att.ate2DevName)
	eth2 := d2.Ethernets().Add().SetName(att.ate2DevName + "Eth").SetMac(att.ate2MAC)
	eth2.Connection().SetLagName(ate2LagName)
	eth2.Vlans().Add().SetName(att.ate2DevName + "Vlan").SetId(uint32(att.vlanID))
	eth2.Ipv4Addresses().Add().SetName(att.v4Name2).SetAddress(att.ate2.IPv4).SetGateway(att.dut2Cust.IPv4).SetPrefix(uint32(att.ate2.IPv4Len))
	eth2.Ipv6Addresses().Add().SetName(att.v6Name2).SetAddress(att.ate2.IPv6).SetGateway(att.dut2Cust.IPv6).SetPrefix(uint32(att.ate2.IPv6Len))

	addAttachmentFlows(top, att)
}

// addAttachmentFlows adds the IPv4/IPv6 forward/backward flows for one attachment.
// Forward flows ride the MACsec edge; both directions carry the VLAN tag.
func addAttachmentFlows(top gosnappi.Config, att custAttachment) {
	// IPv4 forward: ATE1 -> ATE2, MACsec + VLAN on the customer edge.
	f4 := top.Flows().Add().SetName(att.flowV4Fwd)
	f4.TxRx().Device().SetTxNames([]string{att.v4Name1}).SetRxNames([]string{att.v4Name2})
	for _, sw := range sizeWeightProfile {
		f4.Size().WeightPairs().Custom().Add().SetSize(sw.Size).SetWeight(sw.Weight)
	}
	f4.Rate().SetPps(trafficPPS)
	f4.Duration().Continuous()
	f4.Metrics().SetEnable(true)
	f4.Packet().Add().Ethernet().Src().SetValue(att.ate1MAC)
	f4.Packet().Add().Macsec()
	f4.Packet().Add().Vlan().Id().SetValue(uint32(att.vlanID))
	v4 := f4.Packet().Add().Ipv4()
	// Increment the source address to spread traffic across tunnels/core links.
	v4.Src().Increment().SetStart(att.ate1.IPv4).SetStep("0.0.0.1").SetCount(1000)
	v4.Dst().SetValue(att.ate2.IPv4)

	// IPv4 backward: ATE2 -> ATE1, VLAN-tagged (DUT2 subinterface is tagged).
	f4b := top.Flows().Add().SetName(att.flowV4Bwd)
	f4b.TxRx().Device().SetTxNames([]string{att.v4Name2}).SetRxNames([]string{att.v4Name1})
	for _, sw := range sizeWeightProfile {
		f4b.Size().WeightPairs().Custom().Add().SetSize(sw.Size).SetWeight(sw.Weight)
	}
	f4b.Rate().SetPps(trafficPPS)
	f4b.Duration().Continuous()
	f4b.Metrics().SetEnable(true)
	f4b.Packet().Add().Ethernet().Src().SetValue(att.ate2MAC)
	f4b.Packet().Add().Vlan().Id().SetValue(uint32(att.vlanID))
	v4b := f4b.Packet().Add().Ipv4()
	v4b.Src().SetValue(att.ate2.IPv4)
	v4b.Dst().SetValue(att.ate1.IPv4)

	// IPv6 forward: ATE1 -> ATE2.
	f6 := top.Flows().Add().SetName(att.flowV6Fwd)
	f6.TxRx().Device().SetTxNames([]string{att.v6Name1}).SetRxNames([]string{att.v6Name2})
	for _, sw := range sizeWeightProfile {
		f6.Size().WeightPairs().Custom().Add().SetSize(sw.Size).SetWeight(sw.Weight)
	}
	f6.Rate().SetPps(trafficPPS)
	f6.Duration().Continuous()
	f6.Metrics().SetEnable(true)
	f6.Packet().Add().Ethernet().Src().SetValue(att.ate1MAC)
	f6.Packet().Add().Macsec()
	f6.Packet().Add().Vlan().Id().SetValue(uint32(att.vlanID))
	v6 := f6.Packet().Add().Ipv6()
	v6.Src().Increment().SetStart(att.ate1.IPv6).SetStep("::1").SetCount(1000)
	v6.Dst().SetValue(att.ate2.IPv6)

	// IPv6 backward: ATE2 -> ATE1.
	f6b := top.Flows().Add().SetName(att.flowV6Bwd)
	f6b.TxRx().Device().SetTxNames([]string{att.v6Name2}).SetRxNames([]string{att.v6Name1})
	for _, sw := range sizeWeightProfile {
		f6b.Size().WeightPairs().Custom().Add().SetSize(sw.Size).SetWeight(sw.Weight)
	}
	f6b.Rate().SetPps(trafficPPS)
	f6b.Duration().Continuous()
	f6b.Metrics().SetEnable(true)
	f6b.Packet().Add().Ethernet().Src().SetValue(att.ate2MAC)
	f6b.Packet().Add().Vlan().Id().SetValue(uint32(att.vlanID))
	v6b := f6b.Packet().Add().Ipv6()
	v6b.Src().SetValue(att.ate2.IPv6)
	v6b.Dst().SetValue(att.ate1.IPv6)
}

// TestIPSecScaleWithMACSecOverAggregatedLinks implements IPSEC-1.2: it brings up the
// max number of parallel IPSec tunnels over MACsec and validates IPv4/IPv6 connectivity.
func TestIPSecScaleWithMACSecOverAggregatedLinks(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	// Step: Configure DUT customer-facing interfaces, VLANs, VRFs, MACSec, and DUT-DUT transport aggregates.
	// Create two core LAGs (each with 2 member ports) and apply to both DUTs.
	// Use per-DUT aggregate IDs from netutil to ensure device-valid agg names.
	// ATE still uses logical names ate1LagName/ate2LagName.

	// Build DUT port objects from the custPorts/corePorts groupings.
	dut1CorePortGroups := make([][]*ondatra.Port, len(corePorts))
	dut2CorePortGroups := make([][]*ondatra.Port, len(corePorts))
	var dut1CorePorts []*ondatra.Port
	for i, group := range corePorts {
		for _, name := range group {
			dut1CorePortGroups[i] = append(dut1CorePortGroups[i], dut1.Port(t, name))
			dut2CorePortGroups[i] = append(dut2CorePortGroups[i], dut2.Port(t, name))
			dut1CorePorts = append(dut1CorePorts, dut1.Port(t, name))
		}
	}
	// port5 on each DUT connects to ATE; on DUT1 it carries MACsec.
	dut1CustPort := dut1.Port(t, custPorts[0])
	dut2CustPort := dut2.Port(t, custPorts[0])

	// DUT-specific attributes: core LAGs on each DUT use the core interface attributes.
	dut1PortAttrs := []attrs.Attributes{dut1CoreIntf1, dut1CoreIntf2}
	dut2PortAttrs := []attrs.Attributes{dut2CoreIntf1, dut2CoreIntf2}

	// Create all VRFs upfront before configuring interfaces.
	createVRFs(t, dut1, []string{ateVRF, tunnelVRF})
	createVRFs(t, dut2, []string{ateVRF, tunnelVRF})

	// Configure DUTs: generate one aggregate per port group inside configureDUT.
	// The core port groups are the LAGs in TUNNEL_VRF for DUT-to-DUT communication.
	configureDUT(t, dut1, dut1CorePortGroups, dut1PortAttrs, "")
	dut1CustLag := configureDUT(t, dut1, [][]*ondatra.Port{{dut1CustPort}}, []attrs.Attributes{dut1CustIntf}, ateVRF)[0]

	configureDUT(t, dut2, dut2CorePortGroups, dut2PortAttrs, "")
	dut2CustLag := configureDUT(t, dut2, [][]*ondatra.Port{{dut2CustPort}}, []attrs.Attributes{dut2CustIntf}, ateVRF)[0]

	// Configure loopback interfaces used as IPSec tunnel endpoints.
	cfgplugins.ConfigureLoopback(t, dut1, cfgplugins.LoopbackConfig{
		Name:      loopbackIfName,
		IP:        dut1LoopbackIPv6,
		PrefixLen: loopbackPrefixLen,
		IsIPv6:    true,
	})
	cfgplugins.ConfigureLoopback(t, dut2, cfgplugins.LoopbackConfig{
		Name:      loopbackIfName,
		IP:        dut2LoopbackIPv6,
		PrefixLen: loopbackPrefixLen,
		IsIPv6:    true,
	})

	// Configure loopback interfaces used as IPSec tunnel endpoints.
	// All per-tunnel loopback endpoints are configured by configureScaledTunnels below.

	batchMACsec := cfgplugins.ConfigureMACsec(t, dut1, cfgplugins.MACsecCfg{
		IntfName:    dut1CustPort.Name(),
		ProfileName: "macSecProfile",
		CKN:         ckn,
		CAK:         cak,
		FallbackCKN: fallbackCkn,
		FallbackCAK: fallbackCak,
	})
	batchMACsec.Set(t, dut1)

	// Configure the maximum number of parallel IPSec tunnels between the two
	// DUTs. Each tunnel uses a dedicated loopback pair as its endpoints and an
	// independent IKE/SA/profile, all sharing TUNNEL_VRF. The returned per-tunnel
	// next-hop addresses are used to ECMP customer traffic across every tunnel.
	dut1TunnelV4NHs, dut2TunnelV4NHs, dut1TunnelV6NHs, dut2TunnelV6NHs := configureScaledTunnels(t, dut1, dut2, numTunnels)

	// DUT1 routing: customer return path towards ATE1, plus ECMP of customer
	// traffic destined to ATE2 across every tunnel's DUT2-side next-hop.
	dut1Routes := []*cfgplugins.StaticRouteCfg{
		{Prefix: ate1IPv4Prefix, NextHopAddr: ate1LagConfig.IPv4, NetworkInstance: tunnelVRF, NextNetworkInstance: ateVRF},
		{Prefix: ate1IPv6Prefix, NextHopAddr: ate1LagConfig.IPv6, NetworkInstance: tunnelVRF, NextNetworkInstance: ateVRF},
	}
	for _, nh := range dut2TunnelV4NHs {
		dut1Routes = append(dut1Routes, &cfgplugins.StaticRouteCfg{Prefix: ate2IPv4Prefix, NextHopAddr: nh, NetworkInstance: ateVRF, NextNetworkInstance: tunnelVRF})
	}
	for _, nh := range dut2TunnelV6NHs {
		dut1Routes = append(dut1Routes, &cfgplugins.StaticRouteCfg{Prefix: ate2IPv6Prefix, NextHopAddr: nh, NetworkInstance: ateVRF, NextNetworkInstance: tunnelVRF})
	}
	cfgplugins.ConfigureStaticRoutesInVRF(t, dut1, dut1Routes)

	// DUT2 routing: customer return path towards ATE2, plus ECMP of customer
	// traffic destined to ATE1 across every tunnel's DUT1-side next-hop.
	dut2Routes := []*cfgplugins.StaticRouteCfg{
		{Prefix: ate2IPv4Prefix, NextHopAddr: ate2LagConfig.IPv4, NetworkInstance: tunnelVRF, NextNetworkInstance: ateVRF},
		{Prefix: ate2IPv6Prefix, NextHopAddr: ate2LagConfig.IPv6, NetworkInstance: tunnelVRF, NextNetworkInstance: ateVRF},
	}
	for _, nh := range dut1TunnelV4NHs {
		dut2Routes = append(dut2Routes, &cfgplugins.StaticRouteCfg{Prefix: ate1IPv4Prefix, NextHopAddr: nh, NetworkInstance: ateVRF, NextNetworkInstance: tunnelVRF})
	}
	for _, nh := range dut1TunnelV6NHs {
		dut2Routes = append(dut2Routes, &cfgplugins.StaticRouteCfg{Prefix: ate1IPv6Prefix, NextHopAddr: nh, NetworkInstance: ateVRF, NextNetworkInstance: tunnelVRF})
	}
	cfgplugins.ConfigureStaticRoutesInVRF(t, dut2, dut2Routes)

	// Step: Configure ATE topology and flows.
	top := configureATE(t)
	// Enable capture should be part of setconfig
	packetvalidationhelpers.ConfigurePacketCapture(t, top, MacsecPacketValidation)
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	otgvalidationhelpers.WaitForOTGMACSecUp(t, ate, otgvalidationhelpers.WaitForMACSecParams{
		InterfaceName: macsecPeerName,
		Timeout:       lagUpTimeout,
	})
	otgvalidationhelpers.WaitForOTGLAGUP(t, ate, otgvalidationhelpers.LagParams{
		LagName:       ate1LagName,
		WantMembersUp: 1,
		Timeout:       lagUpTimeout,
	})
	otgvalidationhelpers.WaitForOTGLAGUP(t, ate, otgvalidationhelpers.LagParams{
		LagName:       ate2LagName,
		WantMembersUp: 1,
		Timeout:       lagUpTimeout,
	})

	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	// Wait for the base attachment's IPSec tunnels to come UP on both DUTs.
	helpers.WaitForAllIPSECTunnelsUP(t, dut1, dut2, helpers.WaitForIPSECTunnelsParams{
		StartTunnel: 1,
		Count:       numTunnels,
		Timeout:     tunnelUpTimeout,
	})

	// Start the customer-port capture before traffic; it is consumed once by
	// IPSEC-1.2.1 to confirm MACsec encryption.
	cs := packetvalidationhelpers.StartCapture(t, ate)
	captureValidated := false

	// runTrafficAndVerify runs a traffic window and verifies the named flows are
	// forwarded with no loss; used by each IPSEC-1.2.x subtest.
	runTrafficAndVerify := func(t *testing.T, flowNames ...string) {
		otg.StartTraffic(t)
		// Wait for traffic to flow and stabilize.
		time.Sleep(trafficWaitTime)
		otg.StopTraffic(t)

		// Consume the MACsec capture on the first traffic window and confirm the
		// customer traffic egressing towards the ATE is MACsec-encrypted.
		if !captureValidated {
			packetvalidationhelpers.StopCapture(t, ate, cs)
			if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, MacsecPacketValidation); err != nil {
				t.Errorf("CaptureAndValidatePackets() MACsec: %v", err)
			}
			captureValidated = true
		}

		for _, flowName := range flowNames {
			if err := otgvalidationhelpers.VerifyTraffic(t, ate, otgvalidationhelpers.VerifyTrafficParams{
				Config:      top,
				FlowName:    flowName,
				TestResults: true,
			}); err != nil {
				t.Errorf("traffic verification failed: %v", err)
			}
		}
	}

	// Phase 1 (IPSEC-1.2.1/1.2.2): single customer attachment, max tunnels.
	t.Run("IPSEC-1.2.1: Verify IPv4 Connectivity over a Max # of Tunnels for Single Attachment", func(t *testing.T) {
		pre := helpers.ReadMemberOutPkts(t, dut1, dut1CorePorts)
		runTrafficAndVerify(t, flowIPv4Fwd, flowIPv4Bwd)
		if err := helpers.VerifyDUTDUTLoadBalance(t, dut1, helpers.DUTDUTLoadBalanceParams{
			MemberPorts: dut1CorePorts,
			Baseline:    pre,
			Tolerance:   0.25,
		}); err != nil {
			t.Errorf("load balance verification failed: %v", err)
		}
	})
	t.Run("IPSEC-1.2.2: Verify IPv6 Connectivity over a Max # of Tunnels for Single Attachment", func(t *testing.T) {
		pre := helpers.ReadMemberOutPkts(t, dut1, dut1CorePorts)
		runTrafficAndVerify(t, flowIPv6Fwd, flowIPv6Bwd)
		if err := helpers.VerifyDUTDUTLoadBalance(t, dut1, helpers.DUTDUTLoadBalanceParams{
			MemberPorts: dut1CorePorts,
			Baseline:    pre,
			Tolerance:   0.25,
		}); err != nil {
			t.Errorf("load balance verification failed: %v", err)
		}
	})

	// Phase 2 (IPSEC-1.2.3/1.2.4): trim the base attachment and add extra ones up
	// to device-max, then verify traffic on every attachment. Flow lists include
	// the base attachment plus all extras.
	v4FwdFlows := []string{flowIPv4Fwd}
	v4BwdFlows := []string{flowIPv4Bwd}
	v6FwdFlows := []string{flowIPv6Fwd}
	v6BwdFlows := []string{flowIPv6Bwd}

	attTunnels := attachmentTunnelCount(dut1)
	attachments := buildAttachments(numAdditionalAttachments, attTunnels)
	if len(attachments) > 0 {
		// Trim the base attachment to attTunnels, then add the extras.
		trimBaseAttachment(t, dut1, dut2, attTunnels, dut1TunnelV4NHs, dut2TunnelV4NHs, dut1TunnelV6NHs, dut2TunnelV6NHs)

		configureAttachments(t, dut1, dut2, dut1CustLag, dut2CustLag, attachments)
		for _, att := range attachments {
			addAttachmentOTG(top, att)
			v4FwdFlows = append(v4FwdFlows, att.flowV4Fwd)
			v4BwdFlows = append(v4BwdFlows, att.flowV4Bwd)
			v6FwdFlows = append(v6FwdFlows, att.flowV6Fwd)
			v6BwdFlows = append(v6BwdFlows, att.flowV6Bwd)
		}

		// Re-push OTG with the extra attachment devices/flows and bring them up.
		otg.PushConfig(t, top)
		otg.StartProtocols(t)
		otgvalidationhelpers.WaitForOTGMACSecUp(t, ate, otgvalidationhelpers.WaitForMACSecParams{
			InterfaceName: macsecPeerName,
			Timeout:       lagUpTimeout,
		})
		otgvalidationhelpers.WaitForOTGLAGUP(t, ate, otgvalidationhelpers.LagParams{
			LagName:       ate1LagName,
			WantMembersUp: 1,
			Timeout:       lagUpTimeout,
		})
		otgvalidationhelpers.WaitForOTGLAGUP(t, ate, otgvalidationhelpers.LagParams{
			LagName:       ate2LagName,
			WantMembersUp: 1,
			Timeout:       lagUpTimeout,
		})
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
		for _, att := range attachments {
			helpers.WaitForAllIPSECTunnelsUP(t, dut1, dut2, helpers.WaitForIPSECTunnelsParams{
				StartTunnel: att.tunnels.startIndex + 1,
				Count:       att.tunnels.numTunnels,
				Timeout:     tunnelUpTimeout,
			})
		}
	}

	t.Run("IPSEC-1.2.3: Verify IPv4 Connectivity over Device with Max # of Tunnels", func(t *testing.T) {
		pre := helpers.ReadMemberOutPkts(t, dut1, dut1CorePorts)
		runTrafficAndVerify(t, append(append([]string{}, v4FwdFlows...), v4BwdFlows...)...)
		if err := helpers.VerifyDUTDUTLoadBalance(t, dut1, helpers.DUTDUTLoadBalanceParams{
			MemberPorts: dut1CorePorts,
			Baseline:    pre,
			Tolerance:   0.25,
		}); err != nil {
			t.Errorf("load balance verification failed: %v", err)
		}
	})
	t.Run("IPSEC-1.2.4: Verify IPv6 Connectivity over Device with Max # of Tunnels", func(t *testing.T) {
		pre := helpers.ReadMemberOutPkts(t, dut1, dut1CorePorts)
		runTrafficAndVerify(t, append(append([]string{}, v6FwdFlows...), v6BwdFlows...)...)
		if err := helpers.VerifyDUTDUTLoadBalance(t, dut1, helpers.DUTDUTLoadBalanceParams{
			MemberPorts: dut1CorePorts,
			Baseline:    pre,
			Tolerance:   0.25,
		}); err != nil {
			t.Errorf("load balance verification failed: %v", err)
		}
	})
}
