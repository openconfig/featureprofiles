// Copyright 2023 Google LLC
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

package interface_loopback_aggregate_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	plenIPv4       = 30
	plenIPv6       = 126
	lagType        = oc.IfAggregate_AggregationType_STATIC
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
)

var (
	dutPort1Attr = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1Attr = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// verifyAggID verifys aggregate id on port1.
func verifyAggID(t *testing.T, dut *ondatra.DUTDevice, dp, aggID string) {
	dip := gnmi.OC().Interface(dp)
	di := gnmi.Get(t, dut, dip.State())
	if lagID := di.GetEthernet().GetAggregateId(); lagID != aggID {
		t.Errorf("%s LagID got %v, want %v", dp, lagID, aggID)
	}
}

// configureDUTPort1 configures port1 on DUT.
func configureDUTPort1(t *testing.T, dut *ondatra.DUTDevice, dutOcRoot *oc.Root, dutPort1 *ondatra.Port) {
	intf := dutOcRoot.GetOrCreateInterface(dutPort1.Name())
	intf.Type = ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		intf.Enabled = ygot.Bool(true)
	}

	dutOcPath := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", dut), dutOcPath.Config(), dutOcRoot)
	gnmi.Update(t, dut, dutOcPath.Config(), dutOcRoot)
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dutPort1)
	}
}

// configureDUT configures AE interface and adds port1 to AE.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, dutOcRoot *oc.Root, aggID string, dutPort1 *ondatra.Port) {
	agg := dutOcRoot.GetOrCreateInterface(aggID)
	agg.GetOrCreateAggregation().LagType = lagType
	agg.Type = ieee8023adLag

	intf := dutOcRoot.GetOrCreateInterface(dutPort1.Name())
	intf.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
	intf.Type = ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		intf.Enabled = ygot.Bool(true)
	}

	dutOcPath := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", dut), dutOcPath.Config(), dutOcRoot)
	gnmi.Update(t, dut, dutOcPath.Config(), dutOcRoot)

	aggIntf := &oc.Interface{Name: ygot.String(aggID)}
	configAggregateIntf(dut, aggIntf, &dutPort1Attr)
	aggPath := dutOcPath.Interface(aggID)
	fptest.LogQuery(t, aggID, aggPath.Config(), aggIntf)
	gnmi.Replace(t, dut, aggPath.Config(), aggIntf)
}

func configureOTG(t *testing.T, otg *otg.OTG) {
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	iDut1Dev := config.Devices().Add().SetName(atePort1Attr.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1Attr.Name + ".Eth").SetMac(atePort1Attr.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
}

// configSrcDUT configures AE interface.
func configSrcDUT(dut *ondatra.DUTDevice, i *oc.Interface, a *attrs.Attributes) {
	i.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plenIPv4)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plenIPv6)
}

// createCeaseAction creates the BGP cease notification action in gosnappi
// configAggregateIntf configures AE interface on DUT.
func configAggregateIntf(dut *ondatra.DUTDevice, i *oc.Interface, a *attrs.Attributes) {
	configSrcDUT(dut, i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = lagType
}

// TestInterfaceLoopbackMode is to test loopback mode FACILITY.
func TestInterfaceLoopbackMode(t *testing.T) {
	t.Logf("Start DUT config load.")
	dut := ondatra.DUT(t, "dut")
	dutOcRoot := &oc.Root{}
	aggID := netutil.NextAggregateInterface(t, dut)
	dutPort1 := dut.Port(t, "port1")

	t.Run("Configure port1 on DUT ", func(t *testing.T) {
		configureDUTPort1(t, dut, dutOcRoot, dutPort1)
	})

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	t.Run("Configure OTG port1", func(t *testing.T) {
		configureOTG(t, otg)
	})

	t.Run("Validate that DUT port-1 operational status is UP", func(t *testing.T) {
		gnmi.Await(t, dut, gnmi.OC().Interface(dutPort1.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		operStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dutPort1.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; operStatus != want {
			t.Errorf("Get(DUT port1 oper status): got %v, want %v", operStatus, want)
		}
	})

	cs := gosnappi.NewControlState()
	t.Run("Admin down OTG port1", func(t *testing.T) {
		cs.Port().Link().SetState(gosnappi.StatePortLinkState.DOWN)
		otg.SetControlState(t, cs)
	})

	t.Run("Verify DUT port-1 is down on DUT", func(t *testing.T) {
		gnmi.Await(t, dut, gnmi.OC().Interface(dutPort1.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
		operStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dutPort1.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_DOWN; operStatus != want {
			t.Errorf("Get(DUT port1 oper status): got %v, want %v", operStatus, want)
		}
	})

	t.Run("Configure AE on DUT ", func(t *testing.T) {
		configureDUT(t, dut, dutOcRoot, aggID, dutPort1)
	})
	t.Run("Verify aggregate interface", func(t *testing.T) {
		verifyAggID(t, dut, dutPort1.Name(), aggID)
	})

	t.Run("Verify AE interface and port-1 are down on DUT", func(t *testing.T) {

		want := []oc.E_Interface_OperStatus{oc.Interface_OperStatus_LOWER_LAYER_DOWN, oc.Interface_OperStatus_DOWN}
		opStatus, statusCheckResult := gnmi.Watch(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), 2*time.Minute, func(y *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			opStatus, ok := y.Val()
			if !ok {
				return false
			}
			for _, expectedStatus := range want {
				if opStatus == expectedStatus {
					return true
				}
			}
			return false
		}).Await(t)
		if !statusCheckResult {
			val, _ := opStatus.Val()
			t.Errorf("Get(DUT AE interface oper status): got %v, want %v", val.String(), want)
		}

		gnmi.Await(t, dut, gnmi.OC().Interface(dutPort1.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
		operStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dutPort1.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_DOWN; operStatus != want {
			t.Errorf("Get(DUT port1 oper status): got %v, want %v", operStatus, want)
		}
	})

	t.Run("Configure interface loopback mode FACILITY on DUT AE interface", func(t *testing.T) {
		if deviations.InterfaceLoopbackModeRawGnmi(dut) {

			gnmi.Update(t, dut, gnmi.OC().Interface(dutPort1.Name()).LoopbackMode().Config(), oc.Interfaces_LoopbackModeType_TERMINAL)

		} else {
			if deviations.MemberLinkLoopbackUnsupported(dut) {
				gnmi.Update(t, dut, gnmi.OC().Interface(aggID).LoopbackMode().Config(), oc.Interfaces_LoopbackModeType_FACILITY)
			} else {
				gnmi.Update(t, dut, gnmi.OC().Interface(dutPort1.Name()).LoopbackMode().Config(), oc.Interfaces_LoopbackModeType_FACILITY)
			}
		}
	})

	t.Run("Validate that DUT AE and port-1 operational status are UP", func(t *testing.T) {
		gnmi.Await(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		operStatus := gnmi.Get(t, dut, gnmi.OC().Interface(aggID).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; operStatus != want {
			t.Errorf("Get(DUT AE interface oper status): got %v, want %v", operStatus, want)
		}
		gnmi.Await(t, dut, gnmi.OC().Interface(dutPort1.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
		operStatus = gnmi.Get(t, dut, gnmi.OC().Interface(dutPort1.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; operStatus != want {
			t.Errorf("Get(DUT port1 oper status): got %v, want %v", operStatus, want)
		}
	})

	t.Run("Admin up OTG port1", func(t *testing.T) {
		cs.Port().Link().SetState(gosnappi.StatePortLinkState.UP)
		otg.SetControlState(t, cs)
	})
}
