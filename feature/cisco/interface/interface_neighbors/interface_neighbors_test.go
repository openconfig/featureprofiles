package interface_neighbors_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIPv4NeighborsPath(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := true

	configureDUT(t, dut1, IPv4)
	configureDUT(t, dut2, IPv4)
	createIntfAttrib(t, dut1, dut2)
	testInterfaceIPv4Neighbors(t, dut1, dut2)
}

func TestLCReloadIPv4NeighborsPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ReloadLineCards(t, dut1, &wg)
	go ReloadLineCards(t, dut2, &wg)
	wg.Wait()
	time.Sleep(30 * time.Second)
	testInterfaceIPv4Neighbors(t, dut1, dut2)
}
func TestFlapInterfacesIPv4NeighborsPath(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ports := []string{dut1.Port(t, "port1").Name(), dut1.Port(t, "port2").Name(),
		"Bundle-Ether100", "Bundle-Ether101"}

	FlapBulkInterfaces(t, dut1, ports)
	testInterfaceIPv4Neighbors(t, dut1, dut2)
}
func TestDelMemberPortIPv4NeighborsPath(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}

	DelAddMemberPort(t, dut1, dut1Ports)
	DelAddMemberPort(t, dut2, dut2Ports)
	testInterfaceIPv4Neighbors(t, dut1, dut2)
}
func TestAddMemberPortIPv4NeighborsPath(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	bundlePorts := []string{"Bundle-Ether100", "Bundle-Ether101"}

	DelAddMemberPort(t, dut1, dut1Ports, bundlePorts)
	DelAddMemberPort(t, dut2, dut2Ports, bundlePorts)
	testInterfaceIPv4Neighbors(t, dut1, dut2)
}
func TestProcessRestartIPv4NeighborsPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ProcessRestart(t, dut1, "arp", &wg)
	go ProcessRestart(t, dut2, "arp", &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	testInterfaceIPv4Neighbors(t, dut1, dut2)
}
func TestReloadRouterIPv4NeighborsPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ReloadRouter(t, dut1, &wg)
	go ReloadRouter(t, dut2, &wg)
	wg.Wait()
	time.Sleep(5 * time.Second)
	testInterfaceIPv4Neighbors(t, dut1, dut2)
}
func TestRPFOIPv4NeighborsPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go RPFO(t, dut1, &wg)
	go RPFO(t, dut2, &wg)
	wg.Wait()
	time.Sleep(5 * time.Second)
	testInterfaceIPv4Neighbors(t, dut1, dut2)
}

func testInterfaceIPv4Neighbors(t *testing.T, dut1 *ondatra.DUTDevice, dut2 *ondatra.DUTDevice) {

	flag_bit := 0
	IPv4 := true
	reset := false
	backupDut2IPv4 := []string{dut2IntfAttrib[0].attrib.IPv4, dut2IntfAttrib[1].attrib.IPv4,
		dut2IntfAttrib[2].attrib.IPv4, dut2IntfAttrib[3].attrib.IPv4}

	pingNeighbors(t, dut1, dut2, IPv4)
	time.Sleep(5 * time.Second)
	validateIPv4NeighborPath(t, dut1, flag_bit)
	updateIPv4InterfaceDUT(t, dut2, reset)
	updateIPv4InterfaceDUT(t, dut2, reset)
	pingNeighbors(t, dut1, dut2, IPv4)
	time.Sleep(5 * time.Second)
	validateIPv4NeighborPath(t, dut1, flag_bit)
	deleteIPv4NeighborPath(t, dut1)
	flag_bit = flag_bit | delete_bit
	time.Sleep(5 * time.Second)
	validateIPv4NeighborPath(t, dut1, flag_bit)

	reset = true
	dut2IntfAttrib[0].attrib.IPv4 = backupDut2IPv4[0]
	dut2IntfAttrib[1].attrib.IPv4 = backupDut2IPv4[1]
	dut2IntfAttrib[2].attrib.IPv4 = backupDut2IPv4[2]
	dut2IntfAttrib[3].attrib.IPv4 = backupDut2IPv4[3]

	updateIPv4InterfaceDUT(t, dut1, reset)
	updateIPv4InterfaceDUT(t, dut2, reset)
	time.Sleep(5 * time.Second)
	pingNeighbors(t, dut1, dut2, IPv4)
	flag_bit = flag_bit &^ delete_bit
	validateIPv4NeighborPath(t, dut1, flag_bit)
	updateStaticARPDUT(t, dut1)
	time.Sleep(5 * time.Second)
	flag_bit = flag_bit | static_bit
	validateIPv4NeighborPath(t, dut1, flag_bit)
	updateStaticARPDUT(t, dut1)
	updateStaticARPDUT(t, dut1)
	time.Sleep(5 * time.Second)
	validateIPv4NeighborPath(t, dut1, flag_bit)
	deleteIPv4NeighborPath(t, dut1, true)
	flag_bit = flag_bit | delete_bit
	time.Sleep(5 * time.Second)
	validateIPv4NeighborPath(t, dut1, flag_bit)
	updateStaticARPDUT(t, dut1)
	flag_bit = flag_bit &^ delete_bit
	validateIPv4NeighborPath(t, dut1, flag_bit)
}

func TestIPv4ProxyARPPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	configureProxyARP(t, dut1)
	configureProxyARP(t, dut2)
	wg.Add(2)
	go testProxyARP(t, dut1, &wg)
	go testProxyARP(t, dut2, &wg)
	wg.Wait()
}

func TestLCReloadIPv4ProxyARPPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ReloadLineCards(t, dut1, &wg)
	go ReloadLineCards(t, dut2, &wg)
	wg.Wait()
	time.Sleep(20 * time.Second)
	wg.Add(2)
	go testProxyARP(t, dut1, &wg)
	go testProxyARP(t, dut2, &wg)
	wg.Wait()
}
func TestFlapInterfacesIPv4ProxyARPPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ports := []string{dut1.Port(t, "port1").Name(), dut1.Port(t, "port2").Name(),
		"Bundle-Ether100", "Bundle-Ether101"}

	FlapBulkInterfaces(t, dut1, ports)
	wg.Add(2)
	go testProxyARP(t, dut1, &wg)
	go testProxyARP(t, dut2, &wg)
	wg.Wait()
}
func TestDelMemberPortIPv4ProxyARPPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}

	DelAddMemberPort(t, dut1, dut1Ports)
	DelAddMemberPort(t, dut2, dut2Ports)
	wg.Add(2)
	go testProxyARP(t, dut1, &wg)
	go testProxyARP(t, dut2, &wg)
	wg.Wait()
}
func TestAddMemberPortIPv4ProxyARPPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	bundlePorts := []string{"Bundle-Ether100", "Bundle-Ether101"}

	DelAddMemberPort(t, dut1, dut1Ports, bundlePorts)
	DelAddMemberPort(t, dut2, dut2Ports, bundlePorts)
	wg.Add(2)
	go testProxyARP(t, dut1, &wg)
	go testProxyARP(t, dut2, &wg)
	wg.Wait()
}
func TestProcessRestartIPv4ProxyARPPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ProcessRestart(t, dut1, "arp", &wg)
	go ProcessRestart(t, dut2, "arp", &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	wg.Add(2)
	go testProxyARP(t, dut1, &wg)
	go testProxyARP(t, dut2, &wg)
	wg.Wait()
}
func TestReloadRouterIPv4ProxyARPPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ReloadRouter(t, dut1, &wg)
	go ReloadRouter(t, dut2, &wg)
	wg.Wait()
	time.Sleep(5 * time.Second)
	wg.Add(2)
	go testProxyARP(t, dut1, &wg)
	go testProxyARP(t, dut2, &wg)
	wg.Wait()
}
func TestRPFOIPv4ProxyARPPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go RPFO(t, dut1, &wg)
	go RPFO(t, dut2, &wg)
	wg.Wait()
	time.Sleep(5 * time.Second)
	wg.Add(2)
	go testProxyARP(t, dut1, &wg)
	go testProxyARP(t, dut2, &wg)
	wg.Wait()
}

func testProxyARP(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()
	flag_bit := 0

	validateIPv4ProxyARPPath(t, dut, oc.ProxyArp_Mode_ALL, flag_bit)
	updateProxyARPDUT(t, dut, oc.ProxyArp_Mode_REMOTE_ONLY)
	validateIPv4ProxyARPPath(t, dut, oc.ProxyArp_Mode_REMOTE_ONLY, flag_bit)
	updateProxyARPDUT(t, dut, oc.ProxyArp_Mode_ALL)
	validateIPv4ProxyARPPath(t, dut, oc.ProxyArp_Mode_ALL, flag_bit)
	deleteIPv4ProxyARPPath(t, dut)
	flag_bit = flag_bit | delete_bit
	validateIPv4ProxyARPPath(t, dut, oc.ProxyArp_Mode_DISABLE, flag_bit)
	updateProxyARPDUT(t, dut, oc.ProxyArp_Mode_ALL)
	validateIPv4ProxyARPPath(t, dut, oc.ProxyArp_Mode_ALL, flag_bit)
}

func TestIPv6NeighborsPath(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := false

	configureDUT(t, dut1, IPv4)
	configureDUT(t, dut2, IPv4)
	createIntfAttrib(t, dut1, dut2)
	testInterfaceIPv6Neighbors(t, dut1, dut2)
}

func TestLCReloadIPv6NeighborsPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ReloadLineCards(t, dut1, &wg)
	go ReloadLineCards(t, dut2, &wg)
	wg.Wait()
	time.Sleep(20 * time.Second)
	testInterfaceIPv6Neighbors(t, dut1, dut2)
}
func TestFlapInterfaceIPv6NeighborsPath(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ports := []string{dut1.Port(t, "port1").Name(), dut1.Port(t, "port2").Name(),
		"Bundle-Ether100", "Bundle-Ether101"}

	FlapBulkInterfaces(t, dut1, ports)
	testInterfaceIPv6Neighbors(t, dut1, dut2)
}
func TestDelMemberPortIPv6NeighborsPath(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}

	DelAddMemberPort(t, dut1, dut1Ports)
	DelAddMemberPort(t, dut2, dut2Ports)
	testInterfaceIPv6Neighbors(t, dut1, dut2)
}
func TestAddMemberPortIPv6NeighborsPath(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	bundlePorts := []string{"Bundle-Ether100", "Bundle-Ether101"}

	DelAddMemberPort(t, dut1, dut1Ports, bundlePorts)
	DelAddMemberPort(t, dut2, dut2Ports, bundlePorts)
	testInterfaceIPv6Neighbors(t, dut1, dut2)
}
func TestProcessRestartIPv6NeighborsPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	ProcessRestart(t, dut1, "ipv6_nd", &wg)
	ProcessRestart(t, dut2, "ipv6_nd", &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	testInterfaceIPv6Neighbors(t, dut1, dut2)
}
func TestReloadRouterIPv6NeighborsPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ReloadRouter(t, dut1, &wg)
	go ReloadRouter(t, dut2, &wg)
	wg.Wait()
	time.Sleep(5 * time.Second)
	testInterfaceIPv6Neighbors(t, dut1, dut2)
}
func TestRPFOIPv6NeighborsPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go RPFO(t, dut1, &wg)
	go RPFO(t, dut2, &wg)
	wg.Wait()
	time.Sleep(5 * time.Second)
	testInterfaceIPv6Neighbors(t, dut1, dut2)
}

func testInterfaceIPv6Neighbors(t *testing.T, dut1 *ondatra.DUTDevice, dut2 *ondatra.DUTDevice) {

	flag_bit := 0
	IPv4 := false
	reset := false
	backupDut2IPv6 := []string{dut2IntfAttrib[0].attrib.IPv6, dut2IntfAttrib[1].attrib.IPv6,
		dut2IntfAttrib[2].attrib.IPv6, dut2IntfAttrib[3].attrib.IPv6}

	pingNeighbors(t, dut1, dut2, IPv4)
	time.Sleep(10 * time.Second)
	validateIPv6NeighborPath(t, dut1, flag_bit)
	updateIPv6InterfaceDUT(t, dut2, reset)
	time.Sleep(5 * time.Second)
	pingNeighbors(t, dut1, dut2, IPv4)
	validateIPv6NeighborPath(t, dut1, flag_bit)
	deleteIPv6NeighborPath(t, dut1)
	flag_bit = flag_bit | delete_bit
	validateIPv6NeighborPath(t, dut1, flag_bit)

	reset = true
	dut2IntfAttrib[0].attrib.IPv6 = backupDut2IPv6[0]
	dut2IntfAttrib[1].attrib.IPv6 = backupDut2IPv6[1]
	dut2IntfAttrib[2].attrib.IPv6 = backupDut2IPv6[2]
	dut2IntfAttrib[3].attrib.IPv6 = backupDut2IPv6[3]

	updateIPv6InterfaceDUT(t, dut1, reset)
	updateIPv6InterfaceDUT(t, dut2, reset)
	time.Sleep(5 * time.Second)
	pingNeighbors(t, dut1, dut2, IPv4)
	flag_bit = flag_bit &^ delete_bit
	validateIPv6NeighborPath(t, dut1, flag_bit)
	updateNDStaticDUT(t, dut1)
	time.Sleep(5 * time.Second)
	flag_bit = flag_bit | static_bit
	validateIPv6NeighborPath(t, dut1, flag_bit)
	updateNDStaticDUT(t, dut1)
	time.Sleep(5 * time.Second)
	updateNDStaticDUT(t, dut1)
	time.Sleep(5 * time.Second)
	validateIPv6NeighborPath(t, dut1, flag_bit)
	deleteIPv6NeighborPath(t, dut1, true)
	flag_bit = flag_bit | delete_bit
	validateIPv6NeighborPath(t, dut1, flag_bit)
	updateNDStaticDUT(t, dut1)
	time.Sleep(5 * time.Second)
	flag_bit = flag_bit &^ delete_bit
	validateIPv6NeighborPath(t, dut1, flag_bit)
}

func TestIPv6NDRouterAdvPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	configureNDRouterAdvDUT(t, dut1)
	configureNDRouterAdvDUT(t, dut2)
	wg.Add(2)
	go testNDRouterAdv(t, dut1, &wg)
	go testNDRouterAdv(t, dut2, &wg)
	wg.Wait()
}

func TestFlapInterfaceIPv6NDRouterAdvPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ports := []string{dut1.Port(t, "port1").Name(), dut1.Port(t, "port2").Name(),
		"Bundle-Ether100", "Bundle-Ether101"}

	FlapBulkInterfaces(t, dut1, ports)
	wg.Add(2)
	go testNDRouterAdv(t, dut1, &wg)
	go testNDRouterAdv(t, dut2, &wg)
	wg.Wait()
}
func TestDelMemberPortIPv6NDRouterAdvPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}

	DelAddMemberPort(t, dut1, dut1Ports)
	DelAddMemberPort(t, dut2, dut2Ports)
	wg.Add(2)
	go testNDRouterAdv(t, dut1, &wg)
	go testNDRouterAdv(t, dut2, &wg)
	wg.Wait()
}
func TestAddMemberPortIPv6NDRouterAdvPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	bundlePorts := []string{"Bundle-Ether100", "Bundle-Ether101"}

	DelAddMemberPort(t, dut1, dut1Ports, bundlePorts)
	DelAddMemberPort(t, dut2, dut2Ports, bundlePorts)
	wg.Add(2)
	go testNDRouterAdv(t, dut1, &wg)
	go testNDRouterAdv(t, dut2, &wg)
	wg.Wait()
}
func TestProcessRestartIPv6NDRouterAdvPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ProcessRestart(t, dut1, "ipv6_nd", &wg)
	go ProcessRestart(t, dut2, "ipv6_nd", &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	wg.Add(2)
	go testNDRouterAdv(t, dut1, &wg)
	go testNDRouterAdv(t, dut2, &wg)
	wg.Wait()
}

func testNDRouterAdv(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()
	flag_bit := 0
	reset := false

	validateIPv6RouterAdvPath(t, dut, flag_bit)
	updateNDRouterAdvDUT(t, dut, reset)
	flag_bit = flag_bit | update_bit
	validateIPv6RouterAdvPath(t, dut, flag_bit)
	deleteIPv6RouterAdvPath(t, dut)
	flag_bit = flag_bit | delete_bit
	validateIPv6RouterAdvPath(t, dut, flag_bit)
	reset = true
	updateNDRouterAdvDUT(t, dut, reset)
	flag_bit = flag_bit &^ delete_bit
	flag_bit = flag_bit &^ update_bit
	validateIPv6RouterAdvPath(t, dut, flag_bit)
}

func TestIPv6NDPrefixPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	configureNDPrefix(t, dut1)
	configureNDPrefix(t, dut2)
	wg.Add(2)
	go testNDPrefix(t, dut1, &wg)
	go testNDPrefix(t, dut2, &wg)
	wg.Wait()
}

func TestFlapInterfaceIPv6NDPrefixPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ports := []string{dut1.Port(t, "port1").Name(), dut1.Port(t, "port2").Name(),
		"Bundle-Ether100", "Bundle-Ether101"}

	FlapBulkInterfaces(t, dut1, ports)
	wg.Add(2)
	go testNDPrefix(t, dut1, &wg)
	go testNDPrefix(t, dut2, &wg)
	wg.Wait()
}
func TestDelMemberPortIPv6NDPrefixPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}

	DelAddMemberPort(t, dut1, dut1Ports)
	DelAddMemberPort(t, dut2, dut2Ports)
	wg.Add(2)
	go testNDPrefix(t, dut1, &wg)
	go testNDPrefix(t, dut2, &wg)
	wg.Wait()
}
func TestAddMemberPortIPv6NDPrefixPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	bundlePorts := []string{"Bundle-Ether100", "Bundle-Ether101"}

	DelAddMemberPort(t, dut1, dut1Ports, bundlePorts)
	DelAddMemberPort(t, dut2, dut2Ports, bundlePorts)
	wg.Add(2)
	go testNDPrefix(t, dut1, &wg)
	go testNDPrefix(t, dut2, &wg)
	wg.Wait()
}
func TestProcessRestartIPv6NDPrefixPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ProcessRestart(t, dut1, "ipv6_nd", &wg)
	go ProcessRestart(t, dut2, "ipv6_nd", &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	wg.Add(2)
	go testNDPrefix(t, dut1, &wg)
	go testNDPrefix(t, dut2, &wg)
	wg.Wait()
}

func testNDPrefix(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()
	flag_bit := 0
	reset := false

	validateNDPrefixPath(t, dut, flag_bit)
	updateNDPrefixDUT(t, dut, reset)
	flag_bit = flag_bit | update_bit
	validateNDPrefixPath(t, dut, flag_bit)
	deleteNDPrefixPath(t, dut)
	flag_bit = flag_bit | delete_bit
	validateNDPrefixPath(t, dut, flag_bit)
	reset = true
	updateNDPrefixDUT(t, dut, reset)
	flag_bit = flag_bit &^ delete_bit
	flag_bit = flag_bit &^ update_bit
	validateNDPrefixPath(t, dut, flag_bit)
}

func TestIPv6NDDadPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	configureNDDad(t, dut1)
	configureNDDad(t, dut2)
	wg.Add(2)
	go testNDDad(t, dut1, &wg)
	go testNDDad(t, dut2, &wg)
	wg.Wait()
}

func TestFlapInterfaceIPv6NDDadPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ports := []string{dut1.Port(t, "port1").Name(), dut1.Port(t, "port2").Name(),
		"Bundle-Ether100", "Bundle-Ether101"}

	FlapBulkInterfaces(t, dut1, ports)
	wg.Add(2)
	go testNDDad(t, dut1, &wg)
	go testNDDad(t, dut2, &wg)
	wg.Wait()
}
func TestDelMemberPortIPv6NDDadPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}

	DelAddMemberPort(t, dut1, dut1Ports)
	DelAddMemberPort(t, dut2, dut2Ports)
	wg.Add(2)
	go testNDDad(t, dut1, &wg)
	go testNDDad(t, dut2, &wg)
	wg.Wait()
}
func TestAddMemberPortIPv6NDDadPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	dut2Ports := []string{dut1.Port(t, "port3").Name(), dut1.Port(t, "port5").Name()}
	bundlePorts := []string{"Bundle-Ether100", "Bundle-Ether101"}

	DelAddMemberPort(t, dut1, dut1Ports, bundlePorts)
	DelAddMemberPort(t, dut2, dut2Ports, bundlePorts)
	wg.Add(2)
	go testNDDad(t, dut1, &wg)
	go testNDDad(t, dut2, &wg)
	wg.Wait()
}
func TestProcessRestartIPv6NDDadPath(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ProcessRestart(t, dut1, "ipv6_nd", &wg)
	go ProcessRestart(t, dut2, "ipv6_nd", &wg)
	wg.Wait()
	time.Sleep(30 * time.Second)
	wg.Add(2)
	go testNDDad(t, dut1, &wg)
	go testNDDad(t, dut2, &wg)
	wg.Wait()
}

func testNDDad(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()
	flag_bit := 0
	reset := false

	validateNDDadPath(t, dut, flag_bit)
	updateNDDadDUT(t, dut, reset)
	flag_bit = flag_bit | update_bit
	validateNDDadPath(t, dut, flag_bit)
	deleteNDDadPath(t, dut)
	flag_bit = flag_bit | delete_bit
	validateNDDadPath(t, dut, flag_bit)
	reset = true
	updateNDDadDUT(t, dut, reset)
	flag_bit = flag_bit &^ delete_bit
	flag_bit = flag_bit &^ update_bit
	validateNDDadPath(t, dut, flag_bit)
}

func TestLCReloadIPv6ND(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ReloadLineCards(t, dut1, &wg)
	go ReloadLineCards(t, dut2, &wg)
	wg.Wait()
	time.Sleep(20 * time.Second)
	wg.Add(2)
	go testNDRouterAdv(t, dut1, &wg)
	go testNDRouterAdv(t, dut2, &wg)
	wg.Wait()
	wg.Add(2)
	go testNDPrefix(t, dut1, &wg)
	go testNDPrefix(t, dut2, &wg)
	wg.Wait()
	wg.Add(2)
	go testNDDad(t, dut1, &wg)
	go testNDDad(t, dut2, &wg)
	wg.Wait()
}

func TestReloadRouterIPv6ND(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go ReloadRouter(t, dut1, &wg)
	go ReloadRouter(t, dut2, &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	wg.Add(2)
	go testNDRouterAdv(t, dut1, &wg)
	go testNDRouterAdv(t, dut2, &wg)
	wg.Wait()
	wg.Add(2)
	go testNDPrefix(t, dut1, &wg)
	go testNDPrefix(t, dut2, &wg)
	wg.Wait()
	wg.Add(2)
	go testNDDad(t, dut1, &wg)
	go testNDDad(t, dut2, &wg)
	wg.Wait()
}
func TestRPFOIPv6ND(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go RPFO(t, dut1, &wg)
	go RPFO(t, dut2, &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	wg.Add(2)
	go testNDRouterAdv(t, dut1, &wg)
	go testNDRouterAdv(t, dut2, &wg)
	wg.Wait()
	wg.Add(2)
	go testNDPrefix(t, dut1, &wg)
	go testNDPrefix(t, dut2, &wg)
	wg.Wait()
	wg.Add(2)
	go testNDDad(t, dut1, &wg)
	go testNDDad(t, dut2, &wg)
	wg.Wait()
}

func TestIPv4Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := true

	mapBaseIPv4Addr[dut1.ID()] = dut1BaseIPv4Addr[:]
	mapBaseIPv4Addr[dut2.ID()] = dut2BaseIPv4Addr[:]

	wg.Add(2)
	go configureScaleDUT(t, dut1, mapBaseIPv4Addr[dut1.ID()], IPv4, &wg)
	go configureScaleDUT(t, dut2, mapBaseIPv4Addr[dut2.ID()], IPv4, &wg)
	wg.Wait()
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv4ScaleNeighbors(t, dut1)
}

func TestLCReloadIPv4Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := true

	wg.Add(2)
	go ReloadLineCards(t, dut1, &wg)
	go ReloadLineCards(t, dut2, &wg)
	wg.Wait()
	time.Sleep(40 * time.Second)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv4ScaleNeighbors(t, dut1)
}

func TestFlapInterfaceIPv4Scale(t *testing.T) {

	var intfList []string
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := true

	Interfaces := gnmi.GetAll(t, dut1, gnmi.OC().InterfaceAny().State())

	for _, intf := range Interfaces {
		if len(intf.GetName()) >= 17 && intf.GetName()[:17] == "FourHundredGigE0/" {
			intfList = append(intfList, intf.GetName())
		} else if len(intf.GetName()) >= 6 && intf.GetName()[:6] == "Bundle" {
			intfList = append(intfList, intf.GetName())
		}
	}
	FlapBulkInterfaces(t, dut1, intfList)
	time.Sleep(30 * time.Second)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv4ScaleNeighbors(t, dut1)
}

func TestDelMemberPortIPv4Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go DelMemberPortScale(t, dut1, &wg)
	go DelMemberPortScale(t, dut2, &wg)
	wg.Wait()
	time.Sleep(20 * time.Second)
	testIPv4ScaleNeighbors(t, dut1)
}

func TestAddMemberPortIPv4Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go AddMemberPortScale(t, dut1, &wg)
	go AddMemberPortScale(t, dut2, &wg)
	wg.Wait()
	time.Sleep(20 * time.Second)
	testIPv4ScaleNeighbors(t, dut1)
}

func TestProcessRestartIPv4Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := true

	wg.Add(2)
	go ProcessRestart(t, dut1, "ipv6_nd", &wg)
	go ProcessRestart(t, dut2, "ipv6_nd", &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv4ScaleNeighbors(t, dut1)
}

func TestReloadRouterIPv4Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := true

	wg.Add(2)
	go ReloadRouter(t, dut1, &wg)
	go ReloadRouter(t, dut2, &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv4ScaleNeighbors(t, dut1)
}
func TestRPFOIPv4Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := true

	wg.Add(2)
	go RPFO(t, dut1, &wg)
	go RPFO(t, dut2, &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv4ScaleNeighbors(t, dut1)
}

func testIPv4ScaleNeighbors(t *testing.T, dut *ondatra.DUTDevice) {

	path := gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv4()
	got := gnmi.CollectAll(t, dut, path.State(), 30*time.Second).Await(t)
	IntfIPv4Addr = make(map[string]InterfaceIPv4Address)
	wantLen := TOTAL_SCALE_INTF_COUNT

	for _, g := range got {
		val, _ := g.Val()
		ipAddress := val.Address
		neighbor := val.Neighbor
		proxyARP := val.ProxyArp
		for ip := range ipAddress {
			if ip[:2] == "10" || ip[:2] == "11" || ip[:2] == "12" || ip[:2] == "13" ||
				ip[:2] == "20" || ip[:2] == "21" || ip[:2] == "22" || ip[:2] == "23" {
				IntfIPv4Addr[ip] = InterfaceIPv4Address{ipAddress, neighbor, proxyARP}
			}
			// }
		}
	}
	if len(IntfIPv4Addr) < wantLen {
		t.Errorf("Expected Interface count :%v but got %v\n", wantLen, len(IntfIPv4Addr))
	}
	validateIPv4ScaleNeighbors(t)
}

func TestIPv6Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := false

	mapBaseIPv6Addr[dut1.ID()] = dut1BaseIPv6Addr[:]
	mapBaseIPv6Addr[dut2.ID()] = dut2BaseIPv6Addr[:]

	wg.Add(2)
	go configureScaleDUT(t, dut1, mapBaseIPv6Addr[dut1.ID()], IPv4, &wg)
	go configureScaleDUT(t, dut2, mapBaseIPv6Addr[dut2.ID()], IPv4, &wg)
	wg.Wait()
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	time.Sleep(20 * time.Second)
	testIPv6ScaleNeighbors(t, dut1)
}

func TestLCReloadIPv6Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := false

	wg.Add(2)
	go ReloadLineCards(t, dut1, &wg)
	go ReloadLineCards(t, dut2, &wg)
	wg.Wait()
	time.Sleep(40 * time.Second)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv6ScaleNeighbors(t, dut1)
}

func TestFlapInterfaceIPv6Scale(t *testing.T) {

	var intfList []string
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := false
	Interfaces := gnmi.GetAll(t, dut1, gnmi.OC().InterfaceAny().State())

	for _, intf := range Interfaces {
		if len(intf.GetName()) >= 17 && intf.GetName()[:17] == "FourHundredGigE0/" {
			intfList = append(intfList, intf.GetName())
		} else if len(intf.GetName()) >= 6 && intf.GetName()[:6] == "Bundle" {
			intfList = append(intfList, intf.GetName())
		}
	}
	FlapBulkInterfaces(t, dut1, intfList)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	time.Sleep(30 * time.Second)
	testIPv6ScaleNeighbors(t, dut1)
}

func TestDelMemberPortIPv6Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go DelMemberPortScale(t, dut1, &wg)
	go DelMemberPortScale(t, dut2, &wg)
	wg.Wait()
	time.Sleep(30 * time.Second)
	testIPv6ScaleNeighbors(t, dut1)
}

func TestAddMemberPortIPv6Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	wg.Add(2)
	go AddMemberPortScale(t, dut1, &wg)
	go AddMemberPortScale(t, dut2, &wg)
	wg.Wait()
	time.Sleep(20 * time.Second)
	testIPv6ScaleNeighbors(t, dut1)
}
func TestProcessRestartIPv6Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := false

	wg.Add(2)
	go ProcessRestart(t, dut1, "ipv6_nd", &wg)
	go ProcessRestart(t, dut2, "ipv6_nd", &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv6ScaleNeighbors(t, dut1)
}

func TestReloadRouterIPv6Scale(t *testing.T) {

	var wg sync.WaitGroup
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := false

	wg.Add(2)
	go ReloadRouter(t, dut1, &wg)
	go ReloadRouter(t, dut2, &wg)
	wg.Wait()
	time.Sleep(30 * time.Second)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv6ScaleNeighbors(t, dut1)
}

func TestRPFOIPv6Scale(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	IPv4 := false
	var wg sync.WaitGroup

	wg.Add(2)
	go RPFO(t, dut1, &wg)
	go RPFO(t, dut2, &wg)
	wg.Wait()
	time.Sleep(10 * time.Second)
	pingScaleNeighbors(t, dut1, dut2, IPv4)
	testIPv6ScaleNeighbors(t, dut1)
}

func testIPv6ScaleNeighbors(t *testing.T, dut *ondatra.DUTDevice) {

	path := gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv6()
	fmt.Printf("Debug: Calling CollectAll for IPv6\n")
	got := gnmi.CollectAll(t, dut, path.State(), 30*time.Second).Await(t)
	IntfIPv6Addr = make(map[string]InterfaceIPv6Address)
	var dad uint32
	var routerAdv *oc.Interface_Subinterface_Ipv6_RouterAdvertisement
	wantLen := TOTAL_SCALE_INTF_COUNT

	for _, g := range got {
		val, _ := g.Val()
		ipAddress := val.Address
		neighbor := val.Neighbor
		if val.DupAddrDetectTransmits != nil {
			dad = *val.DupAddrDetectTransmits
		}
		if val.RouterAdvertisement != nil {
			routerAdv = val.RouterAdvertisement
		}
		for ip := range ipAddress {
			if ip[:2] == "10" || ip[:2] == "11" || ip[:2] == "12" || ip[:2] == "13" ||
				ip[:2] == "20" || ip[:2] == "21" || ip[:2] == "22" || ip[:2] == "23" {

				IntfIPv6Addr[ip] = InterfaceIPv6Address{ipAddress, neighbor, dad, routerAdv}
			}
		}
	}
	if len(IntfIPv6Addr) < wantLen {
		t.Errorf("Expected Interface count :%v but got %v\n", wantLen, len(IntfIPv6Addr))
	}
	validateIPv6ScaleNeighbors(t)
}

func validateIPv4NeighborPath(t *testing.T, dut *ondatra.DUTDevice, flag_bit int) {

	t.Run("GetAll interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor", func(t *testing.T) {

		path := gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv4().NeighborAny().State()

		if flag_bit&delete_bit == delete_bit {
			output := gnmi.LookupAll(t, dut, path)
			for _, op := range output {
				value, _ := op.Val()
				if value.GetIp()[:5] == "192.0" && flag_bit&static_bit != static_bit {
					t.Errorf("Neighbor Info Failed To Delete")
				}
			}
		} else {
			output := gnmi.GetAll(t, dut, path)
			if len(output) <= 0 {
				t.Errorf("Neighbor Info Not Available")
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv4

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv4(dut2IntfAttrib[i].attrib.IPv4)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv4(dut1IntfAttrib[i].attrib.IPv4)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).State()
					op := gnmi.Get(t, dut, path)
					if op.GetOrigin().String() != "STATIC" || op.GetIp() != neighbor {
						t.Errorf("Invalid Neighbor State Info for interface %s, want STATIC found %v", portName, op.GetOrigin().String())
						t.Errorf("want %v found %v", neighbor, op.GetIp())
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if op.GetOrigin().String() != "DYNAMIC" || op.GetIp() != neighbor {
						t.Errorf("Invalid Neighbor State Info for interface %s, want DYNAMIC found %v", portName, op.GetOrigin().String())
						t.Errorf("want %v found %v", neighbor, op.GetIp())
					}
				}
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/ip", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv4

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).Ip().State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv4(dut2IntfAttrib[i].attrib.IPv4)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).Ip().State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv4(dut1IntfAttrib[i].attrib.IPv4)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).Ip().State()
					op := gnmi.Get(t, dut, path)
					if op != neighbor {
						t.Errorf("Invalid Static Neighbor IP Address for interface %s, want %v found %v", portName, neighbor, op)
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if op != neighbor {
						t.Errorf("Invalid Neighbor IP Address for interface %s, want %v found %v", portName, neighbor, op)
					}
				}
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/link-layer-address", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv4

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).LinkLayerAddress().State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv4(dut2IntfAttrib[i].attrib.IPv4)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).LinkLayerAddress().State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv4(dut1IntfAttrib[i].attrib.IPv4)
					path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).LinkLayerAddress().State()
					op := gnmi.Get(t, dut, path)
					if op != staticIPv4MAC {
						t.Errorf("Invalid Static Neighbor LinkLayer Address for interface %s", portName)
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if len(op) <= 0 {
						t.Errorf("Invalid Neighbor LinkLayer Address for interface %s", portName)
					}
				}
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor/state/origin", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv4

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).Origin().State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv4(dut2IntfAttrib[i].attrib.IPv4)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).Origin().State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Static Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv4(dut1IntfAttrib[i].attrib.IPv4)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor).Origin().State()
					op := gnmi.Get(t, dut, path)
					if op.String() != "STATIC" {
						t.Errorf("Invalid Neighbor Origin Info for interface %s, want STATIC got %v", portName, op.String())
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if op.String() != "DYNAMIC" {
						t.Errorf("Invalid Neighbor Origin Info for interface %s, want DYNAMIC got %v", portName, op.String())
					}
				}
			}
		}
	})
}

func validateIPv4ProxyARPPath(t *testing.T, dut *ondatra.DUTDevice, mode oc.E_ProxyArp_Mode,
	flag_bit int) {

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv4/proxy-arp", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().ProxyArp().Mode().State()

			if flag_bit&delete_bit == delete_bit {
				op := gnmi.Lookup(t, dut, path)
				value, _ := op.Val()
				if value != mode {
					t.Errorf("ProxyARP mode Failed To Delete for interface %s , want %v got %v", portName, mode, op)
				}
			} else {
				op := gnmi.Get(t, dut, path)
				if op != mode {
					t.Errorf("Invalid ProxyARP mode found for interface %s, want %v got %v", portName, mode, op)
				}
			}
		}
	})
}

func updateIPv4InterfaceDUT(t *testing.T, dut *ondatra.DUTDevice, reset bool) {

	t.Run("Update interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}
		var dutIntfAttrib [4]InterfaceAttributes

		if dut.ID() == "dut1" {
			dutIntfAttrib = dut1IntfAttrib
		} else {
			dutIntfAttrib = dut2IntfAttrib
		}
		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dutIntfAttrib[i].intfName
			attrib := dutIntfAttrib[i].attrib
			path := gnmi.OC().Interface(portName)
			obj := &oc.Interface{}
			obj.Name = ygot.String(portName)

			if reset == true {
				gnmi.BatchReplace(batchConfig, path.Config(), configInterfaceIPv4DUT(obj, attrib))
			} else {
				attrib.IPv4 = getNewIPv4(attrib.IPv4)
				gnmi.BatchReplace(batchConfig, path.Config(), configInterfaceIPv4DUT(obj, attrib))
			}

		}
		batchConfig.Set(t, dut)
	})
}

func updateStaticARPDUT(t *testing.T, dut *ondatra.DUTDevice) {

	t.Run("Update Static interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}
		var dutIntfAttrib [4]InterfaceAttributes

		if dut.ID() == "dut1" {
			dutIntfAttrib = dut1IntfAttrib
		} else {
			dutIntfAttrib = dut2IntfAttrib
		}
		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dutIntfAttrib[i].intfName
			attrib := dutIntfAttrib[i].attrib
			staticIPv4 = getNewStaticIPv4(attrib.IPv4)
			path := gnmi.OC().Interface(portName)

			obj := &oc.Interface{}
			obj.Name = ygot.String(portName)
			obj.GetOrCreateEthernet()
			gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv4DUT(obj, attrib))
		}
		batchConfig.Set(t, dut)
	})
}

func updateProxyARPDUT(t *testing.T, dut *ondatra.DUTDevice, mode oc.E_ProxyArp_Mode) {

	t.Run("Update interfaces/interface/subinterfaces/subinterface/ipv4/proxy-arp", func(t *testing.T) {
		batchConfig := &gnmi.SetBatch{}
		var dutIntfAttrib [4]InterfaceAttributes

		if dut.ID() == "dut1" {
			dutIntfAttrib = dut1IntfAttrib
		} else {
			dutIntfAttrib = dut2IntfAttrib
		}
		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dutIntfAttrib[i].intfName
			attrib := dutIntfAttrib[i].attrib
			idx := attrib.Subinterface

			path := gnmi.OC().Interface(portName)
			obj := &oc.Interface{}
			obj.Name = ygot.String(portName)
			obj.GetOrCreateSubinterface(idx).GetOrCreateIpv4().
				GetOrCreateProxyArp().SetMode(mode)

			gnmi.BatchReplace(batchConfig, path.Config(), configInterfaceIPv4DUT(obj, attrib))
		}
		batchConfig.Set(t, dut)
	})
}

func deleteIPv4NeighborPath(t *testing.T, dut *ondatra.DUTDevice, static ...bool) {

	t.Run("Delete interfaces/interface/subinterfaces/subinterface/ipv4/neighbors/neighbor", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface

			if len(static) > 0 && static[0] == true {
				neighbor := getNewStaticIPv4(dut1IntfAttrib[i].attrib.IPv4)
				path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().Neighbor(neighbor)
				gnmi.BatchDelete(batchConfig, path.Config())
			} else {
				path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv4()
				gnmi.BatchDelete(batchConfig, path.Config())
			}
		}
		batchConfig.Set(t, dut)
	})
}

func deleteIPv4ProxyARPPath(t *testing.T, dut *ondatra.DUTDevice) {

	t.Run("Delete interfaces/interface/subinterfaces/subinterface/ipv4/proxy-arp", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv4().ProxyArp()
			gnmi.BatchDelete(batchConfig, path.Config())

		}
		batchConfig.Set(t, dut)
	})
}

func validateIPv6NeighborPath(t *testing.T, dut *ondatra.DUTDevice, flag_bit int) {

	t.Run("GetAll interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor", func(t *testing.T) {

		path := gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv6().NeighborAny().State()

		if flag_bit&delete_bit == delete_bit {
			output := gnmi.LookupAll(t, dut, path)
			if len(output) > 0 && flag_bit&static_bit != static_bit {
				t.Errorf("Neighbor Info Failed To Delete")
			}
		} else {
			op := gnmi.GetAll(t, dut, path)
			if len(op) <= 0 {
				t.Errorf("Neighbor Info Not Available")
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv6

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).State()
					op := gnmi.Get(t, dut, path)
					if op.GetOrigin().String() != "STATIC" || op.GetIsRouter() != true ||
						op.GetNeighborState().String() != "REACHABLE" || op.GetIp() != neighbor {
						t.Errorf("Invalid Neighbor Info for interface %s", portName)
						t.Errorf("want Origin DYNAMIC found %s", op.GetOrigin().String())
						t.Errorf("want IsRouter true found %v", op.GetIsRouter())
						t.Errorf("want State REACHABLE found %s", op.GetNeighborState().String())
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if op.GetOrigin().String() != "DYNAMIC" || op.GetIsRouter() != true ||
						op.GetNeighborState().String() != "REACHABLE" || op.GetIp() != neighbor {
						t.Errorf("Invalid Neighbor Info for interface %s", portName)
						t.Errorf("want Origin DYNAMIC found %s", op.GetOrigin().String())
						t.Errorf("want IsRouter true found %v", op.GetIsRouter())
						t.Errorf("want State REACHABLE found %s", op.GetNeighborState().String())
					}
				}
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/ip", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv6

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).Ip().State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).Ip().State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).Ip().State()
					op := gnmi.Get(t, dut, path)
					if op != neighbor {
						t.Errorf("Invalid Static Neighbor IP Addresse for interface %s, want %v found %v", portName, neighbor, op)
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if op != neighbor {
						t.Errorf("Invalid Neighbor IP Address for interface %s, want %v found %v", portName, neighbor, op)
					}
				}
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/link-layer-address", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv6

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).LinkLayerAddress().State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).LinkLayerAddress().State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).LinkLayerAddress().State()
					op := gnmi.Get(t, dut, path)
					if len(op) <= 0 && op != staticIPv6MAC {
						t.Errorf("Invalid Static Neighbor LinkLayer Address for interface %s", portName)
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if len(op) <= 0 {
						t.Errorf("Invalid Neighbor LinkLayer Address for interface %s", portName)
					}
				}
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/origin", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv6

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).Origin().State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).Origin().State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete  for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).Origin().State()
					op := gnmi.Get(t, dut, path)
					if op.String() != "STATIC" {
						t.Errorf("Invalid Static Neighbor Origin Info for interface %s, want STATIC found %s", portName, op.String())
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if op.String() != "DYNAMIC" {
						t.Errorf("Invalid Neighbor Origin Info for interface %s, want DYNAMIC found %s", portName, op.String())
					}
				}
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/is-router", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv6

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).IsRouter().State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).IsRouter().State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Static Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).IsRouter().State()
					op := gnmi.Get(t, dut, path)
					if op != true {
						t.Errorf("Invalid Static Neighbor IsRouter Info for interface %s, want true found %v", portName, op)
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if op != true {
						t.Errorf("Invalid Neighbor IsRouter Info for interface %s, want true found %v", portName, op)
					}
				}
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor/state/neighbor-state", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			neighbor := dut2IntfAttrib[i].attrib.IPv6

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).NeighborState().State()

			if flag_bit&delete_bit == delete_bit {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).NeighborState().State()
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Static Neighbor Info Failed To Delete for interface %s", portName)
					}
				} else {
					op := gnmi.Lookup(t, dut, path)
					if op.IsPresent() {
						t.Errorf("Neighbor Info Failed To Delete for interface %s", portName)
					}
				}
			} else {
				if flag_bit&static_bit == static_bit {
					neighbor = getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
					path = gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor).NeighborState().State()
					op := gnmi.Get(t, dut, path)
					if op.String() != "REACHABLE" {
						t.Errorf("Invalid Static Neighbor State Info for interface %s, want REACHABLE found %s", portName, op.String())
					}
				} else {
					op := gnmi.Get(t, dut, path)
					if op.String() != "REACHABLE" {
						t.Errorf("Invalid Neighbor State Info for interface %s, want REACHABLE found %s", portName, op.String())
					}
				}
			}
		}
	})
}

func validateIPv6RouterAdvPath(t *testing.T, dut *ondatra.DUTDevice, flag_bit int) {

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().State()

			if flag_bit&delete_bit == delete_bit {
				op := gnmi.Lookup(t, dut, path)
				value, _ := op.Val()
				if value.GetInterval() == RAIntervalDefault && value.GetLifetime() == RALifetimeDefault &&
					value.GetOtherConfig() == false && value.GetSuppress() == false {
					t.Log("Router Adv deleted successfully")
				} else {
					t.Errorf("Router Adv Failed To Delete for interface %s", portName)
					t.Errorf("want RAInterval %v found %v", RAIntervalDefault, value.GetInterval())
					t.Errorf("want RALifetime %v found %v", RALifetimeDefault, value.GetLifetime())
					t.Errorf("want RAOtherConfig false found %v", value.GetOtherConfig())
					t.Errorf("want RASuppress false found %v", value.GetSuppress())
				}
			} else if flag_bit&update_bit == update_bit {
				op := gnmi.Get(t, dut, path)
				RAIntervalTemp := op.GetInterval()
				RAIntervalMin := uint32(RAInterval_Update / 3)
				RAIntervalAvg := uint32((RAInterval_Update + RAIntervalMin) / 2)
				if RAIntervalTemp != RAIntervalAvg || op.GetLifetime() != RALifetime_Update ||
					op.GetOtherConfig() != RAOtherConfig_Update || op.GetSuppress() != RASuppress_Update {
					t.Errorf("Invalid Router Advertisement value for interface %s", portName)
					t.Errorf("want RAInterval %v found %v", RAIntervalAvg, op.GetInterval())
					t.Errorf("want RALifetime %v found %v", RALifetime_Update, op.GetLifetime())
					t.Errorf("want RAOtherConfig %v found %v", RAOtherConfig_Update, op.GetOtherConfig())
					t.Errorf("want RASuppress %v found %v", RASuppress_Update, op.GetSuppress())
				}
			} else {
				op := gnmi.Get(t, dut, path)
				if op.GetInterval() != RAInterval || op.GetLifetime() != RALifetime ||
					op.GetOtherConfig() != RAOtherConfig || op.GetSuppress() != RASuppress {
					t.Errorf("Invalid Router Advertisement value for interface %s", portName)
					t.Errorf("want RAInterval %v found %v", RAInterval, op.GetInterval())
					t.Errorf("want RALifetime %v found %v", RALifetime, op.GetLifetime())
					t.Errorf("want RAOtherConfig %v found %v", RAOtherConfig, op.GetOtherConfig())
					t.Errorf("want RASuppress %v found %v", RASuppress, op.GetSuppress())
				}
			}
		}
	})
}

func validateNDPrefixPath(t *testing.T, dut *ondatra.DUTDevice, flag_bit int) {

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/prefixes", func(t *testing.T) {

		path := gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv6().RouterAdvertisement().PrefixAny().State()

		if flag_bit&delete_bit == delete_bit {
			output := gnmi.LookupAll(t, dut, path)
			if len(output) > 8 {
				t.Errorf("Prefix Info Failed To Delete")
			} else {
				t.Log("Prefix Info deleted successfully")
			}
		} else if flag_bit&update_bit == update_bit {
			op := gnmi.GetAll(t, dut, path)
			if len(op) < 12 {
				t.Errorf("Prefix Value Not Found")
			} else {
				op := gnmi.GetAll(t, dut, path)
				if len(op) < 8 {
					t.Errorf("Prefix Value Not Found")
				}
			}
		}
	})

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/prefixes/prefix", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface

			if flag_bit&delete_bit == delete_bit {
				path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().Prefix("400:0:2::/124").State()
				op := gnmi.Lookup(t, dut, path)
				if op.IsPresent() {
					t.Errorf("Prefix Info Failed To Delete for interface %s", portName)
				} else {
					t.Log("Prefix Info deleted successfully")
				}
			} else if flag_bit&update_bit == update_bit {
				path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().Prefix("400:0:2::/124").State()
				op := gnmi.Get(t, dut, path)
				if op.GetPreferredLifetime() != NDPrefixPreferredLifetime_Update ||
					op.GetDisableAutoconfiguration() != NDPrefixDisableAutoconfiguration_Update ||
					op.GetEnableOnlink() != NDPrefixEnableOnlink_Update ||
					op.GetPrefix() != "400:0:2::/124" {
					t.Errorf("Invalid Prefix Value Found for interface %s", portName)
					t.Errorf("want 400:0:2::/124 found %v", op.GetPrefix())
					t.Errorf("want PreferredLifetime %v found %v", op.GetPreferredLifetime(), NDPrefixPreferredLifetime_Update)
					t.Errorf("want DisableAutoconfiguration %v found %v", op.GetDisableAutoconfiguration(), NDPrefixDisableAutoconfiguration_Update)
					t.Errorf("want EnableOnlink %v found %v", op.GetEnableOnlink(), NDPrefixEnableOnlink_Update)
				}
			} else {
				path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().Prefix("300:0:2::/124").State()
				op := gnmi.Get(t, dut, path)
				if op.GetPreferredLifetime() != NDPrefixPreferredLifetime ||
					op.GetDisableAutoconfiguration() != NDPrefixDisableAutoconfiguration ||
					op.GetEnableOnlink() != NDPrefixEnableOnlink ||
					op.GetPrefix() != "300:0:2::/124" {
					t.Errorf("Invalid Prefix Value Found for interface %s", portName)
					t.Errorf("want 300:0:2::/124 found %v", op.GetPrefix())
					t.Errorf("want PreferredLifetime %v found %v", op.GetPreferredLifetime(), NDPrefixPreferredLifetime)
					t.Errorf("want DisableAutoconfiguration %v found %v", op.GetDisableAutoconfiguration(), NDPrefixDisableAutoconfiguration)
					t.Errorf("want EnableOnlink %v found %v", op.GetEnableOnlink(), NDPrefixEnableOnlink)
				}
			}
		}
	})
}

func validateNDDadPath(t *testing.T, dut *ondatra.DUTDevice, flag_bit int) {

	t.Run("Get interfaces/interface/subinterfaces/subinterface/ipv6/dad", func(t *testing.T) {

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface

			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().DupAddrDetectTransmits().State()

			if flag_bit&delete_bit == delete_bit {
				op := gnmi.Lookup(t, dut, path)
				value, _ := op.Val()
				if value != 1 {
					t.Errorf("DAD Value Failed To Delete for interface %s, want 1 found %v", portName, value)
				} else {
					t.Log("DAD Value deleted successfully")
				}
			} else if flag_bit&update_bit == update_bit {
				op := gnmi.Get(t, dut, path)
				if op != NDDad_Update {
					t.Errorf("Invalid DAD Value Found for interface %s, want %v found %v", portName, NDDad_Update, op)
				}
			} else {
				op := gnmi.Get(t, dut, path)
				if op != NDDad {
					t.Errorf("Invalid DAD Value Found for interface %s, want %v found %v", portName, NDDad, op)
				}
			}
		}
	})
}

func updateIPv6InterfaceDUT(t *testing.T, dut *ondatra.DUTDevice, reset bool) {

	t.Run("Update interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}
		var dutIntfAttrib [4]InterfaceAttributes

		if dut.ID() == "dut1" {
			dutIntfAttrib = dut1IntfAttrib
		} else {
			dutIntfAttrib = dut2IntfAttrib
		}
		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dutIntfAttrib[i].intfName
			attrib := dutIntfAttrib[i].attrib
			path := gnmi.OC().Interface(portName)
			obj := &oc.Interface{}
			obj.Name = ygot.String(portName)

			if reset == true {
				gnmi.BatchReplace(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
			} else {
				attrib.IPv6 = getNewIPv6(attrib.IPv6)
				gnmi.BatchReplace(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
			}
		}
		batchConfig.Set(t, dut)
	})
}

func updateNDStaticDUT(t *testing.T, dut *ondatra.DUTDevice) {

	t.Run("Update Static interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}
		var dutIntfAttrib [4]InterfaceAttributes

		if dut.ID() == "dut1" {
			dutIntfAttrib = dut1IntfAttrib
		} else {
			dutIntfAttrib = dut2IntfAttrib
		}
		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dutIntfAttrib[i].intfName
			attrib := dutIntfAttrib[i].attrib
			staticIPv6 = getNewStaticIPv6(attrib.IPv6)
			path := gnmi.OC().Interface(portName)

			obj := &oc.Interface{}
			obj.Name = ygot.String(portName)
			obj.GetOrCreateEthernet()
			gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
		}
		batchConfig.Set(t, dut)
	})
}

func updateNDRouterAdvDUT(t *testing.T, dut *ondatra.DUTDevice, reset bool) {

	t.Run("Update interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}
		var dutIntfAttrib [4]InterfaceAttributes

		if dut.ID() == "dut1" {
			dutIntfAttrib = dut1IntfAttrib
		} else {
			dutIntfAttrib = dut2IntfAttrib
		}
		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dutIntfAttrib[i].intfName
			attrib := dutIntfAttrib[i].attrib
			idx := attrib.Subinterface

			path := gnmi.OC().Interface(portName)
			obj := &oc.Interface{}
			obj.Name = ygot.String(portName)
			ra := obj.GetOrCreateSubinterface(idx).GetOrCreateIpv6().
				GetOrCreateRouterAdvertisement()

			if reset == true {
				ra.SetInterval(RAInterval)
				ra.SetLifetime(RALifetime)
				ra.SetOtherConfig(RAOtherConfig)
				ra.SetSuppress(RASuppress)

				gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
			} else {
				ra.SetInterval(RAInterval_Update)
				ra.SetLifetime(RALifetime_Update)
				ra.SetOtherConfig(RAOtherConfig_Update)
				ra.SetSuppress(RASuppress_Update)

				gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
			}
		}
		batchConfig.Set(t, dut)
	})
}

func updateNDPrefixDUT(t *testing.T, dut *ondatra.DUTDevice, reset bool) {

	t.Run("Update interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/prefixes", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}
		var dutIntfAttrib [4]InterfaceAttributes

		if dut.ID() == "dut1" {
			dutIntfAttrib = dut1IntfAttrib
		} else {
			dutIntfAttrib = dut2IntfAttrib
		}
		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dutIntfAttrib[i].intfName
			attrib := dutIntfAttrib[i].attrib
			idx := attrib.Subinterface

			path := gnmi.OC().Interface(portName)
			obj := &oc.Interface{}
			obj.Name = ygot.String(portName)

			if reset == true {
				ndp := obj.GetOrCreateSubinterface(idx).GetOrCreateIpv6().
					GetOrCreateRouterAdvertisement().GetOrCreatePrefix(NDPrefix)
				ndp.SetPreferredLifetime(NDPrefixPreferredLifetime)
				ndp.SetValidLifetime(NDPrefixValidLifetime)
				ndp.SetDisableAutoconfiguration(NDPrefixDisableAutoconfiguration)
				ndp.SetEnableOnlink(NDPrefixEnableOnlink)

				gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
			} else {
				ndp := obj.GetOrCreateSubinterface(idx).GetOrCreateIpv6().
					GetOrCreateRouterAdvertisement().GetOrCreatePrefix(NDPrefix_Update)

				ndp.SetPreferredLifetime(NDPrefixPreferredLifetime_Update)
				ndp.SetValidLifetime(NDPrefixValidLifetime_Update)
				ndp.SetDisableAutoconfiguration(NDPrefixDisableAutoconfiguration_Update)
				ndp.SetEnableOnlink(NDPrefixEnableOnlink_Update)

				gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
			}
		}
		batchConfig.Set(t, dut)
	})
}

func updateNDDadDUT(t *testing.T, dut *ondatra.DUTDevice, reset bool) {

	t.Run("Update interfaces/interface/subinterfaces/subinterface/ipv6/dad", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}
		var dutIntfAttrib [4]InterfaceAttributes

		if dut.ID() == "dut1" {
			dutIntfAttrib = dut1IntfAttrib
		} else {
			dutIntfAttrib = dut2IntfAttrib
		}
		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dutIntfAttrib[i].intfName
			attrib := dutIntfAttrib[i].attrib
			idx := attrib.Subinterface

			path := gnmi.OC().Interface(portName)
			obj := &oc.Interface{}
			obj.Name = ygot.String(portName)

			if reset == true {
				obj.GetOrCreateSubinterface(idx).GetOrCreateIpv6().
					SetDupAddrDetectTransmits(NDDad)

				gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
			} else {
				obj.GetOrCreateSubinterface(idx).GetOrCreateIpv6().
					SetDupAddrDetectTransmits(NDDad_Update)

				gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
			}
		}
		batchConfig.Set(t, dut)
	})
}

func deleteIPv6NeighborPath(t *testing.T, dut *ondatra.DUTDevice, static ...bool) {

	t.Run("Delete interfaces/interface/subinterfaces/subinterface/ipv6/neighbors/neighbor", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface

			if len(static) > 0 && static[0] == true {
				neighbor := getNewStaticIPv6(dut1IntfAttrib[i].attrib.IPv6)
				path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().Neighbor(neighbor)

				gnmi.BatchDelete(batchConfig, path.Config())
			} else {
				path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6()

				gnmi.BatchDelete(batchConfig, path.Config())
			}
		}
		batchConfig.Set(t, dut)
	})
}

func deleteIPv6RouterAdvPath(t *testing.T, dut *ondatra.DUTDevice) {

	t.Run("Delete interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			path1 := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().Interval()
			path2 := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().Lifetime()
			path3 := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().OtherConfig()
			path4 := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().Suppress()

			gnmi.BatchDelete(batchConfig, path1.Config())
			gnmi.BatchDelete(batchConfig, path2.Config())
			gnmi.BatchDelete(batchConfig, path3.Config())
			gnmi.BatchDelete(batchConfig, path4.Config())
		}
		batchConfig.Set(t, dut)
	})
}

func deleteNDPrefixPath(t *testing.T, dut *ondatra.DUTDevice) {

	t.Run("Delete interfaces/interface/subinterfaces/subinterface/ipv6/router-advertisement/prefixes", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().Prefix("400:0:2::1/124")

			gnmi.BatchDelete(batchConfig, path.Config())
		}
		batchConfig.Set(t, dut)
	})
}

func deleteNDDadPath(t *testing.T, dut *ondatra.DUTDevice) {

	t.Run("Delete interfaces/interface/subinterfaces/subinterface/ipv6/dad", func(t *testing.T) {

		batchConfig := &gnmi.SetBatch{}

		for i := 0; i < TOTAL_INTF_COUNT; i++ {
			portName := dut1IntfAttrib[i].intfName
			idx := dut1IntfAttrib[i].attrib.Subinterface
			path := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().DupAddrDetectTransmits()

			gnmi.BatchDelete(batchConfig, path.Config())
		}
		batchConfig.Set(t, dut)
	})
}

func validateIPv4ScaleNeighbors(t *testing.T) {

	for ip := range IntfIPv4Addr {

		DynamicNeighbor := getNeighbor(ip, true)
		StaticNeighbor := getStaticNeighbor(ip, true)

		if IntfIPv4Addr[ip].neighbor[DynamicNeighbor].GetLinkLayerAddress() == "" {
			t.Errorf("Invalid MAC Address for Neighbor %s", DynamicNeighbor)
		}
		if IntfIPv4Addr[ip].neighbor[DynamicNeighbor].GetOrigin().String() != "DYNAMIC" {
			t.Errorf("Invalid Origin state %s for Neighbor %s",
				IntfIPv4Addr[ip].neighbor[DynamicNeighbor].GetOrigin().String(), DynamicNeighbor)
		}
		if IntfIPv4Addr[ip].neighbor[StaticNeighbor].GetLinkLayerAddress() == "" {
			t.Errorf("Invalid MAC Address for Static Neighbor %s", StaticNeighbor)
		}
		if IntfIPv4Addr[ip].neighbor[StaticNeighbor].GetOrigin().String() != "STATIC" {
			t.Errorf("Invalid Origin state %s for Static Neighbor %s",
				IntfIPv4Addr[ip].neighbor[StaticNeighbor].GetOrigin().String(), StaticNeighbor)
		}
		if IntfIPv4Addr[ip].proxyArp.GetMode().String() != "ALL" {
			t.Errorf("Invalid Proxy Arp Mode %s for IP %s",
				IntfIPv4Addr[ip].proxyArp.GetMode().String(), ip)
		}
	}
}

func validateIPv6ScaleNeighbors(t *testing.T) {

	for ip := range IntfIPv6Addr {

		DynamicNeighbor := getNeighbor(ip, false)
		StaticNeighbor := getStaticNeighbor(ip, false)

		if IntfIPv6Addr[ip].neighbor[DynamicNeighbor].GetLinkLayerAddress() == "" {
			t.Errorf("Invalid MAC Address for Neighbor %s", DynamicNeighbor)
		}
		if IntfIPv6Addr[ip].neighbor[DynamicNeighbor].GetOrigin().String() != "DYNAMIC" {
			t.Errorf("Invalid Origin state %s for Neighbor %s",
				IntfIPv6Addr[ip].neighbor[DynamicNeighbor].GetOrigin().String(), DynamicNeighbor)
		}
		if IntfIPv6Addr[ip].neighbor[DynamicNeighbor].GetIsRouter() != true {
			t.Errorf("Invalid IsRouter %v info for Neighbor %s",
				IntfIPv6Addr[ip].neighbor[DynamicNeighbor].GetIsRouter(), DynamicNeighbor)
		}
		if IntfIPv6Addr[ip].neighbor[DynamicNeighbor].GetNeighborState().String() != "REACHABLE" &&
			IntfIPv6Addr[ip].neighbor[DynamicNeighbor].GetNeighborState().String() != "DELAY" {
			t.Errorf("Invalid NeighborState %s for Neighbor %s",
				IntfIPv6Addr[ip].neighbor[DynamicNeighbor].GetNeighborState().String(), DynamicNeighbor)
		}
		if IntfIPv6Addr[ip].neighbor[StaticNeighbor].GetLinkLayerAddress() == "" {
			t.Errorf("Invalid MAC Address for Static Neighbor %s", StaticNeighbor)
		}
		if IntfIPv6Addr[ip].neighbor[StaticNeighbor].GetOrigin().String() != "STATIC" {
			t.Errorf("Invalid Origin state %s for Static Neighbor %s",
				IntfIPv6Addr[ip].neighbor[StaticNeighbor].GetOrigin().String(), StaticNeighbor)
		}
		if IntfIPv6Addr[ip].neighbor[StaticNeighbor].GetIsRouter() != true {
			t.Errorf("Invalid IsRouter %v info for Static Neighbor %s",
				IntfIPv6Addr[ip].neighbor[StaticNeighbor].GetIsRouter(), StaticNeighbor)
		}
		if IntfIPv6Addr[ip].neighbor[StaticNeighbor].GetNeighborState().String() != "REACHABLE" {
			t.Errorf("Invalid NeighborState %s for Static Neighbor %s",
				IntfIPv6Addr[ip].neighbor[StaticNeighbor].GetNeighborState().String(), StaticNeighbor)
		}
		if IntfIPv6Addr[ip].routerAdv.GetInterval() != RAInterval {
			t.Errorf("Invalid RA Interval %d for IP %s",
				IntfIPv6Addr[ip].routerAdv.GetInterval(), ip)
		}
		if IntfIPv6Addr[ip].routerAdv.GetLifetime() != RALifetime {
			t.Errorf("Invalid RA Lifetime %v for IP %s",
				IntfIPv6Addr[ip].routerAdv.GetLifetime(), ip)
		}
		if IntfIPv6Addr[ip].routerAdv.GetOtherConfig() != RAOtherConfig {
			t.Errorf("Invalid RA OtherConfig %v for IP %s",
				IntfIPv6Addr[ip].routerAdv.GetOtherConfig(), ip)
		}
		if IntfIPv6Addr[ip].routerAdv.GetSuppress() != RASuppress {
			t.Errorf("Invalid RA Suppress %v for IP %s",
				IntfIPv6Addr[ip].routerAdv.GetSuppress(), ip)
		}
		if IntfIPv6Addr[ip].dad != NDDad {
			t.Errorf("Invalid DAD %v for IP %s", IntfIPv6Addr[ip].dad, ip)
		}
		prefixData := IntfIPv6Addr[ip].routerAdv.Prefix["300:0:2::/124"]

		if prefixData == nil {
			t.Errorf("Prefix info not available for IP %s", ip)
			continue
		}
		if prefixData.GetPreferredLifetime() != NDPrefixPreferredLifetime {
			t.Errorf("Invalid ND PreferredLifetime %v for IP %s",
				prefixData.GetPreferredLifetime(), ip)
		}
		if prefixData.GetDisableAutoconfiguration() != NDPrefixDisableAutoconfiguration {
			t.Errorf("Invalid ND DisableAutoconfiguration %v for IP %s",
				prefixData.GetDisableAutoconfiguration(), ip)
		}
		if prefixData.GetEnableOnlink() != NDPrefixEnableOnlink {
			t.Errorf("Invalid ND EnableOnlink %v for IP %s",
				prefixData.GetEnableOnlink(), ip)
		}
		if prefixData.GetValidLifetime() != NDPrefixValidLifetime {
			t.Errorf("Invalid ND ValidLifetime %v for IP %s",
				prefixData.GetValidLifetime(), ip)
		}
	}
}
