package isis_base_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/utils"
	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/input_cisco"
	"github.com/openconfig/ondatra"
)

const (
	input_file = "isis.yaml"
)

var (
	ixiaTopology = make(map[string]*ondatra.ATETopology)
	testInput    = ipb.LoadInput(input_file)
	device1      = "dut"
	device2      = "peer"
	ate          = "ate"
	observer     = fptest.
			NewObserver("ISIS").AddAdditionalCsvRecorder("ocreport").
			AddCsvRecorder()
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func getIXIATopology(t *testing.T, ateName string) *ondatra.ATETopology {
	topo, ok := ixiaTopology[ateName]
	if !ok {
		ate := ondatra.ATE(t, ateName)
		topo = ate.Topology().New()
		generateBaseScenario(t, ate, topo)
		topo.Push(t)
		ixiaTopology[ateName] = topo
	}
	return topo
}

func generateBaseScenario(t *testing.T, ate *ondatra.ATEDevice, topoobj *ondatra.ATETopology) {
	for _, p := range ate.Device.Ports() {
		intf := topoobj.AddInterface(p.Name())
		intf.WithPort(ate.Port(t, p.ID()))
		for i := 0; i < 9; i++ {
			if fmt.Sprintf("1/%d", i+1) == p.Name() {
				intf.IPv4().WithAddress(fmt.Sprintf("100.%d.1.2/24", 120+i)).WithDefaultGateway(fmt.Sprintf("100.%d.1.1", 120+i))
				intf.IPv6().WithAddress(fmt.Sprintf("2000::100:%d:1:2/126", 120+i)).WithDefaultGateway(fmt.Sprintf("2000::100:%d:1:1", 120+i))
			}
		}
	}
	addNetworkAndProtocolsToAte(t, ate, topoobj)
}

func addNetworkAndProtocolsToAte(t *testing.T, ate *ondatra.ATEDevice, topo *ondatra.ATETopology) {
	//Add prefixes/networks on ports
	scale := uint32(10)
	utils.AddIpv4Network(t, topo, "1/1", "network101", "101.1.1.1/32", scale)
	utils.AddIpv4Network(t, topo, "1/2", "network102", "102.1.1.1/32", scale)
	//Configure ISIS, BGP on TGN
	utils.AddAteISISL2(t, topo, "1/1", "490001", "isis_network1", 20, "120.1.1.1/32", scale)
	utils.AddAteISISL2(t, topo, "1/2", "490002", "isis_network2", 20, "121.1.1.1/32", scale)
	utils.AddAteEBGPPeer(t, topo, "1/1", "100.120.1.1", 64001, "bgp_network", "100.120.0.2", "130.1.1.1/32", scale, false)
	utils.AddAteEBGPPeer(t, topo, "1/2", "100.121.1.1", 64001, "bgp_network", "100.121.0.2", "131.1.1.1/32", scale, false)
	//Configure loopbacks for BGP to use as source addresses
	utils.AddLoopback(t, topo, "1/1", "11.11.11.1/32")
	utils.AddLoopback(t, topo, "1/2", "12.12.12.1/32")
}
