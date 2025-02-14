package checktb_test

import (
	"flag"
	"fmt"
	"sort"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

var (
	isOTG    = flag.Bool("otg", false, "Check OTG instead of ATE")
	dutPorts = []*attrs.Attributes{}
	atePorts = []*attrs.Attributes{}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func initPorts(n int) {
	for i := 1; i <= n; i++ {
		dutPorts = append(dutPorts, &attrs.Attributes{
			Desc:    fmt.Sprintf("DUT Port %d", i),
			IPv4:    fmt.Sprintf("192.0.%d.1", i+1),
			IPv4Len: 30,
		})

		atePorts = append(atePorts, &attrs.Attributes{
			Name:    fmt.Sprintf("port%d", i),
			MAC:     fmt.Sprintf("02:00:%02d:01:01:01", i),
			Desc:    fmt.Sprintf("ATE Port %d", i),
			IPv4:    fmt.Sprintf("192.0.%d.2", i+1),
			IPv4Len: 30,
		})
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	ports := dut.Ports()
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})

	for i, p := range ports {
		gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Config(), dutPorts[i].NewOCInterface(p.Name(), dut))
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) {
	ports := ate.Ports()
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})

	top := ate.Topology().New()
	for i, p := range ports {
		atePorts[i].AddToATE(top, p, dutPorts[i])
	}
	top.Push(t)
	top.StartProtocols(t)
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice) {
	ports := ate.Ports()
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})

	top := gosnappi.NewConfig()
	for i, p := range ports {
		atePorts[i].AddToOTG(top, p, dutPorts[i])
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
}

func TestTBChecks(t *testing.T) {
	for _, dut := range ondatra.DUTs(t) {
		t.Logf("Checking gNMI connection on dut %v", dut.ID())
		gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	}

	if ate, ok := ondatra.ATEs(t)["ate"]; ok {
		if dut, ok := ondatra.DUTs(t)["dut"]; ok {
			if len(dut.Ports()) == len(ate.Ports()) {
				initPorts(len(dut.Ports()))
				configureDUT(t, dut)
				if *isOTG {
					configureOTG(t, ate)
				} else {
					configureATE(t, ate)
				}
			}
		}
	}
}
