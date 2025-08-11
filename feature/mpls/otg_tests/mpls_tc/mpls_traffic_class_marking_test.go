package mpls_traffic_class_marking_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4           = 30
	plenIPv4Loopback   = 32
	isisInstance       = "DEFAULT"
	mplsClassifierName = "mpls-class"
	mplsTermName       = "mpls-class-term"
	mplsStartLabel     = 16
	mplsEndLabel       = 1048575
	mplsTCValue        = 5
	mplsWaitTime       = 2 * time.Minute
	loopbackIntf       = "Loopback50"
	ldpLabelSpace      = 0 // Use the platform-wide label space.
)

var (
	dutA_p1 = attrs.Attributes{
		Desc:    "DUT-A to DUT-B",
		IPv4:    "192.168.1.1",
		IPv4Len: plenIPv4,
	}

	dutB_p2 = attrs.Attributes{
		Desc:    "DUT-B to DUT-A",
		IPv4:    "192.168.1.2",
		IPv4Len: plenIPv4,
	}

	dutA_lo50 = attrs.Attributes{
		Desc:    loopbackIntf,
		IPv4:    "100.100.100.1",
		IPv4Len: plenIPv4Loopback,
	}

	dutB_lo50 = attrs.Attributes{
		Desc:    loopbackIntf,
		IPv4:    "200.200.200.2",
		IPv4Len: plenIPv4Loopback,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestMplsTcMarking configures and verifies MPLS Traffic Class marking
// based on a QoS classifier.
func TestMplsTcMarking(t *testing.T) {
	dutA := ondatra.DUT(t, "dut1")
	dutB := ondatra.DUT(t, "dut2")

	// Configure initial network setup (interfaces, ISIS, MPLS, LDP).
	configureInitialDUTs(t, dutA, dutB)

	// Verify LDP session is established.
	verifyLDP(t, dutA, dutB_lo50.IPv4)

	t.Run("ConfigureAndVerifyClassifier", func(t *testing.T) {
		// Configure QoS on DUT-A.
		configureQoS(t, dutA)

		// Verify QoS configuration state on DUT-A.
		verifyQoS(t, dutA)
	})
}

// configureDUT is a helper to configure interfaces, ISIS, MPLS, and LDP on a single DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port, dutAttr, loopbackAttr *attrs.Attributes) {
	t.Helper()
	d := gnmi.OC()
	niName := deviations.DefaultNetworkInstance(dut)
	niOC := &oc.Root{}
	ni := niOC.GetOrCreateNetworkInstance(niName)

	// Configure default network instance.
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Configure physical and loopback interfaces.
	dutPortCfg := dutAttr.NewOCInterface(dutPort.Name(), dut)
	gnmi.Replace(t, dut, d.Interface(dutPort.Name()).Config(), dutPortCfg)

	lo50Name := loopbackIntf
	lo50Cfg := loopbackAttr.NewOCInterface(lo50Name, dut)
	gnmi.Replace(t, dut, d.Interface(lo50Name).Config(), lo50Cfg)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dutPort)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dutPort.Name(), niName, 0)
		fptest.AssignToNetworkInstance(t, dut, lo50Name, niName, 0)
	}

	// Configure ISIS protocol.
	isis := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).GetOrCreateIsis()
	isis.GetOrCreateGlobal().GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntf := isis.GetOrCreateInterface(dutPort.Name())
	isisIntf.Enabled = ygot.Bool(true)
	isisIntf.GetOrCreateLevel(2).Enabled = ygot.Bool(true)

	isisLo50 := isis.GetOrCreateInterface(lo50Name)
	isisLo50.Enabled = ygot.Bool(true)
	isisLo50.GetOrCreateLevel(2).Enabled = ygot.Bool(true)
	isisLo50.Passive = ygot.Bool(true)

	// Configure MPLS and LDP.
	mpls := ni.GetOrCreateMpls()
	mpls.GetOrCreateGlobal().GetOrCreateInterface(dutPort.Name()).MplsEnabled = ygot.Bool(true)
	ldp := mpls.GetOrCreateSignalingProtocols().GetOrCreateLdp()
	ldp.GetOrCreateGlobal().LsrId = ygot.String(loopbackAttr.IPv4)
	ldpIntf := ldp.GetOrCreateInterfaceAttributes().GetOrCreateInterface(dutPort.Name())
	ldpIntf.GetOrCreateAddressFamily(oc.MplsLdp_MplsLdpAfi_IPV4).SetEnabled(true)

	gnmi.Update(t, dut, d.NetworkInstance(niName).Config(), ni)
}

// configureInitialDUTs configures both DUTs.
func configureInitialDUTs(t *testing.T, dutA, dutB *ondatra.DUTDevice) {
	t.Helper()
	p1A := dutA.Port(t, "port1")
	p2B := dutB.Port(t, "port2")

	configureDUT(t, dutA, p1A, &dutA_p1, &dutA_lo50)
	configureDUT(t, dutB, p2B, &dutB_p2, &dutB_lo50)
}

// verifyLDP waits for the LDP session between dutA and its peer to be established.
func verifyLDP(t *testing.T, dut *ondatra.DUTDevice, peerIP string) {
	t.Helper()
	niName := deviations.DefaultNetworkInstance(dut)
	// FIX 1: The LDP neighbor path requires two keys: lsr-id and label-space-id.
	// The label-space-id is 0 for the default platform-wide label space.
	ldpPath := gnmi.OC().NetworkInstance(niName).Mpls().SignalingProtocols().Ldp()

	// Wait for neighbor session to become OPERATIONAL.
	_, ok := gnmi.Watch(t, dut, ldpPath.Neighbor(peerIP, ldpLabelSpace).SessionState().State(), mplsWaitTime, func(val *ygnmi.Value[oc.E_MplsLdp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == oc.MplsLdp_Neighbor_SessionState_OPERATIONAL
	}).Await(t)

	if !ok {
		t.Fatalf("LDP session to peer %s on DUT %s did not become OPERATIONAL", peerIP, dut.Name())
	}
	t.Logf("LDP session to peer %s on DUT %s is OPERATIONAL", peerIP, dut.Name())
}

// configureQoS configures a QoS classifier on DUT-A to match MPLS packets and mark their TC.
func configureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dutPort1 := dut.Port(t, "port1").Name()
	d := &oc.Root{}
	qos := d.GetOrCreateQos()

	// Define the MPLS classifier.
	classifier := qos.GetOrCreateClassifier(mplsClassifierName)
	classifier.SetType(oc.Qos_Classifier_Type_MPLS)

	// Define the term to match MPLS label range.
	term := classifier.GetOrCreateTerm(mplsTermName)
	term.SetId(mplsTermName)

	// Define MPLS matching conditions.
	mplsCond := term.GetOrCreateConditions().GetOrCreateMpls()
	mplsCond.SetStartLabelValue(oc.UnionUint32(mplsStartLabel))
	mplsCond.SetEndLabelValue(oc.UnionUint32(mplsEndLabel))

	// Define remark action to set MPLS TC.
	actions := term.GetOrCreateActions()
	remark := actions.GetOrCreateRemark()
	remark.SetSetMplsTc(mplsTCValue)

	// Apply the classifier to the input of the interface.
	iface := qos.GetOrCreateInterface(dutPort1)
	ifaceIn := iface.GetOrCreateInput()
	// FIX: Use the correct enum type for the input classifier.
	ifaceIn.GetOrCreateClassifier(oc.Input_Classifier_Type_MPLS).SetName(mplsClassifierName)

	// Push QoS configuration to the DUT.
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qos)
}

// verifyQoS verifies the QoS classifier configuration and state on the DUT.
func verifyQoS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dutPort1Name := dut.Port(t, "port1").Name()

	// State paths for QoS verification.
	qosPath := gnmi.OC().Qos()
	classifierPath := qosPath.Classifier(mplsClassifierName)
	termPath := classifierPath.Term(mplsTermName)
	mplsCondPath := termPath.Conditions().Mpls()
	mplsActionPath := termPath.Actions().Remark()

	// Verify classifier state.
	t.Logf("Verifying QoS classifier state for %s", mplsClassifierName)
	if got := gnmi.Get(t, dut, classifierPath.Name().State()); got != mplsClassifierName {
		t.Errorf("Classifier name mismatch: got %q, want %q", got, mplsClassifierName)
	}
	if got := gnmi.Get(t, dut, classifierPath.Type().State()); got != oc.Qos_Classifier_Type_MPLS {
		t.Errorf("Classifier type mismatch: got %v, want %v", got, oc.Qos_Classifier_Type_MPLS)
	}

	// Verify term state.
	if got := gnmi.Get(t, dut, termPath.Id().State()); got != mplsTermName {
		t.Errorf("Term ID mismatch: got %q, want %q", got, mplsTermName)
	}

	// Verify MPLS condition state.
	startLabelUnion := gnmi.Get(t, dut, mplsCondPath.StartLabelValue().State())
	startLabelWrapper, ok := startLabelUnion.(oc.UnionUint32)
	if !ok {
		t.Fatalf("StartLabelValue is not of type oc.UnionUint32, it is %T", startLabelUnion)
	}
	if uint32(startLabelWrapper) != mplsStartLabel {
		t.Errorf("MPLS Start Label Value mismatch: got %d, want %d", startLabelWrapper, mplsStartLabel)
	}

	endLabelUnion := gnmi.Get(t, dut, mplsCondPath.EndLabelValue().State())
	endLabelWrapper, ok := endLabelUnion.(oc.UnionUint32)
	if !ok {
		t.Fatalf("EndLabelValue is not of type oc.UnionUint32, it is %T", endLabelUnion)
	}
	if uint32(endLabelWrapper) != mplsEndLabel {
		t.Errorf("MPLS End Label Value mismatch: got %d, want %d", endLabelWrapper, mplsEndLabel)
	}

	// Verify MPLS TC marking action state.
	if got := gnmi.Get(t, dut, mplsActionPath.SetMplsTc().State()); got != mplsTCValue {
		t.Errorf("Set MPLS TC value mismatch: got %d, want %d", got, mplsTCValue)
	}

	// Verify that the classifier is correctly applied to the interface.
	ifaceClassifierPath := qosPath.Interface(dutPort1Name).Input().Classifier(oc.Input_Classifier_Type_MPLS)
	if got := gnmi.Get(t, dut, ifaceClassifierPath.Name().State()); got != mplsClassifierName {
		t.Errorf("Classifier on interface %s has name %q, want %q", dutPort1Name, got, mplsClassifierName)
	}

	t.Logf("Successfully verified all QoS classifier states.")
}
