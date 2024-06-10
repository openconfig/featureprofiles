package static_lsp_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	mplsLabel1    = 1000001
	mplsLabel2    = 1000002
	mplsLabel3    = 1000003

	tolerance = 0.01 // 1% Traffic Tolerance
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	ateDst = attrs.Attributes{
		Name:    "ateDst",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)

	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configureDUT configures port1, port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	i1.Enabled = ygot.Bool(true)
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, dut))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, dut))

}

// configureATE configures port1 and port2 on the ATE.
func configureOTG(t *testing.T) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	port1 := top.Ports().Add().SetName("port1")
	port2 := top.Ports().Add().SetName("port2")

	// Port1 Configuration.
	iDut1Dev := top.Devices().Add().SetName(ateSrc.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	iDut1Ipv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	iDut1Ipv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	// Port2 Configuration.
	iDut2Dev := top.Devices().Add().SetName(ateDst.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	iDut2Ipv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6")
	iDut2Ipv6.SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	// enable packet capture on this port
	top.Captures().Add().SetName("mplsPackCapture").SetPortNames([]string{port2.Name()}).SetFormat(gosnappi.CaptureFormat.PCAP)

	return top

}

// configureStaticLSP configures a static MPLS LSP with the provided parameters.
func configureStaticLSP(t *testing.T, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, nextHopIP string) {
	d := &oc.Root{}
	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	staticMplsCfg := mplsCfg.GetOrCreateLsps().GetOrCreateStaticLsp(lspName)
	staticMplsCfg.GetOrCreateEgress().SetIncomingLabel(oc.UnionUint32(incomingLabel))
	staticMplsCfg.GetOrCreateEgress().SetNextHop(nextHopIP)
	staticMplsCfg.GetOrCreateEgress().SetPushLabel(oc.Egress_PushLabel_IMPLICIT_NULL)

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)

}

func createTrafficFlow(t *testing.T,
	ate *ondatra.ATEDevice,
	dut *ondatra.DUTDevice,
	top gosnappi.Config,
	label1 uint32, label2 uint32) gosnappi.Flow {

	// get dut mac interface for traffic mpls flow
	dutDstInterface := dut.Port(t, "port1").Name()
	dstMac := gnmi.Get(t, dut, gnmi.OC().Interface(dutDstInterface).Ethernet().MacAddress().State())
	t.Logf("DUT remote mac address is %s", dstMac)

	// Create a traffic flow with MPLS
	flowName := fmt.Sprintf("MPLS-%d-MPLS-%d::", mplsLabel1, mplsLabel2)
	mplsFlow := top.Flows().Add().SetName(flowName)
	mplsFlow.TxRx().Port().
		SetTxName(ate.Port(t, "port1").ID()).
		SetRxNames([]string{ate.Port(t, "port2").ID()})

	mplsFlow.Metrics().SetEnable(true)
	mplsFlow.Rate().SetPps(500)
	mplsFlow.Size().SetFixed(512)
	mplsFlow.Duration().Continuous()

	// Set up ethernet layer.
	eth := mplsFlow.Packet().Add().Ethernet()
	eth.Src().SetValue(ateSrc.MAC)
	eth.Dst().SetValue(dstMac)

	// Set up MPLS layer with destination label 100.
	mpls := mplsFlow.Packet().Add().Mpls()
	mpls.Label().SetValue(label1)
	mpls.BottomOfStack().SetValue(0)

	// Set up MPLS layer with destination label 100.
	mpls2 := mplsFlow.Packet().Add().Mpls()
	mpls2.Label().SetValue(label2)
	mpls2.BottomOfStack().SetValue(1)

	ip4 := mplsFlow.Packet().Add().Ipv4()
	ip4.Src().SetValue(ateSrc.IPv4)
	ip4.Dst().SetValue(ateDst.IPv4)
	ip4.Version().SetValue(4)

	return mplsFlow

}

func ValidatePackets(t *testing.T,
	filename string,
	expectedLabel uint32,
	tolerancePercentage float64) {

	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	var expectedMPLSPackets int
	var unexpectedMPLSPackets int

	// Convert string to net.IP for comparison
	targetSrcIP := net.ParseIP(ateSrc.IPv4)
	targetDstIP := net.ParseIP(ateDst.IPv4)

	t.Logf("Checking Packets to verify MPLS label was popped and searching for "+
		"src %s and dst %s", ateSrc.IPv4, ateDst.IPv4)

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			t.Log("IP layer not found in packet")
			continue
		}
		ipv4, _ := ipLayer.(*layers.IPv4)

		// Compare the source IP address in the packet to the target source IP
		if ipv4.SrcIP.Equal(targetSrcIP) && ipv4.DstIP.Equal(targetDstIP) {
			// Now check for an MPLS layer
			var mpls *layers.MPLS
			mplsLayer := packet.Layer(layers.LayerTypeMPLS)
			if mplsLayer != nil {
				mplsPkt, _ := mplsLayer.(*layers.MPLS)
				// check if the expected label is found
				if mplsPkt.Label == expectedLabel {
					expectedMPLSPackets++
				} else {
					t.Errorf("Unexpected Label Found MPLS packet with label: %v", mplsPkt.Label)
					unexpectedMPLSPackets++
				}
			} else {
				// increment the unexpected packet counter
				unexpectedMPLSPackets++
				t.Errorf("Found MPLS packet with label: %v", mpls.Label)

			}
		}
	}

	// Calculate the tolerance based on the number of expected MPLS packets
	pktCount := int(float64(expectedMPLSPackets) * (tolerancePercentage / 100))

	// Check if the unexpected packets are within the tolerance
	if unexpectedMPLSPackets > pktCount {
		t.Errorf("Test failed: found %d unexpected MPLS packets, "+
			"which is above the tolerance of 1%% of expected MPLS packets (%d)", unexpectedMPLSPackets, pktCount)
	} else {
		t.Logf("Test Passed: processed (%d) expected packets with top label "+
			"popped and label RX on OTG is (%d) and found (%d) unexpected packets "+
			"with a tolerance of %d", expectedMPLSPackets, expectedLabel,
			unexpectedMPLSPackets, pktCount)
	}
	t.Log("Finished checking packets for source IP.")
}

// Send traffic and validate traffic.
func verifyTrafficStreams(t *testing.T,
	ate *ondatra.ATEDevice,
	top gosnappi.Config,
	otg *otg.OTG,
	mplsFlow gosnappi.Flow) {
	t.Helper()

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)

	t.Log("starting traffic for 30 seconds")
	ate.OTG().StartTraffic(t)
	time.Sleep(30 * time.Second)
	t.Log("Stopping traffic and waiting 10 seconds for traffic stats to complete")
	ate.OTG().StopTraffic(t)
	time.Sleep(10 * time.Second)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)

	txPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(mplsFlow.Name()).Counters().OutPkts().State()))
	rxPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(mplsFlow.Name()).Counters().InPkts().State()))

	// Calculate the acceptable lower and upper bounds for rxPkts
	lowerBound := txPkts * (1 - tolerance)
	upperBound := txPkts * (1 + tolerance)

	if rxPkts < lowerBound || rxPkts > upperBound {
		t.Fatalf("Received packets are outside of the acceptable range: %v (1%% tolerance from %v)", rxPkts, txPkts)
	} else {
		t.Logf("Received packets are within the acceptable range: %v (1%% tolerance from %v)", rxPkts, txPkts)
	}

	// create packet capture pcap file
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(top.Ports().Items()[1].Name()))
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, fileOutput := f.Write(bytes); fileOutput != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", fileOutput)
	}

	// Log the file name
	t.Logf("Created temporary pcap file at: %s\n", f.Name())

	if _, fileOutput := f.Write(bytes); fileOutput != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", fileOutput)
	}

	fileClose := f.Close()
	if fileClose != nil {
		return
	}
	ValidatePackets(t, f.Name(), mplsLabel2, tolerance)

}

// TestMplsStaticLabel
func TestMplsStaticLabel(t *testing.T) {
	var top gosnappi.Config
	var mplsFlow gosnappi.Flow
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otgObj := ate.OTG()

	t.Run("configureDUT Interfaces", func(t *testing.T) {
		// Configure the DUT
		configureDUT(t, dut)

	})

	t.Run("ConfigureOTG", func(t *testing.T) {
		t.Logf("Configure ATE")
		top = configureOTG(t)

	})

	t.Run("Configure static LSP on DUT", func(t *testing.T) {
		// configure static lsp from ateSrc to ateDst
		configureStaticLSP(t, dut, "lsp1", mplsLabel1, ateDst.IPv4)
		// configure static lsp from ateDst to ateSrc
		configureStaticLSP(t, dut, "lsp2", mplsLabel3, ateSrc.IPv4)

	})

	t.Run("Build OTG Traffic Flow", func(t *testing.T) {
		// Build MPLS Traffic Flow
		mplsFlow = createTrafficFlow(t, ate, dut, top, mplsLabel1, mplsLabel2)

		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		t.Logf("OTG Config is %s\n", top.String())

	})

	t.Run("Verify Static Label Traffic Flow", func(t *testing.T) {
		verifyTrafficStreams(t, ate, top, otgObj, mplsFlow)

	})

}
