package gnmi_ni_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	customVRFName = "customVRFName"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: 126,
	}
)

func TestCodelab(t *testing.T) {
	// configure DUT
	dut := ondatra.DUT(t, "dut")
	batch := &gnmi.SetBatch{}
	ConfigureDUT(batch, t, dut)
	ConfigureAdditionalNetworkInstance(batch, t, dut, customVRFName)
}

// ConfigureDUT configures port1 and port2 on the DUT with default network instance.
func ConfigureDUT(batch *gnmi.SetBatch, t *testing.T, dut *ondatra.DUTDevice) {

	dp1 := dut.Port(t, "port1")

	// Configure default network instance.
	cfgplugins.ConfigureDefaultNetworkInstance(batch, t, dut)

	// Configure gNMI server on default network instance.
	cfgplugins.CreateGNMIServer(batch, t, dut, deviations.DefaultNetworkInstance(dut))

	// Configuring basic interface and network instance as some devices only populate OC after configuration.
	port1IntfPath := dutPort1.NewOCInterface(dp1.Name(), dut)
	gnmi.BatchUpdate(batch, gnmi.OC().Interface(dp1.Name()).Config(), port1IntfPath)
	// Deviations for vendors that require explicit interface to network instance assignment.
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		cfgplugins.AssignToNetworkInstance(batch, t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// ConfigureAdditionalNetworkInstance configures a new network instance in DUT and changes the network instance of port2
func ConfigureAdditionalNetworkInstance(batch *gnmi.SetBatch, t *testing.T, dut *ondatra.DUTDevice, ni string) {
	// Configure interface, non-default network instance
	t.Logf("\nConfiguring network instance and gNMI server: Network instance: %s \n", ni)
	cfgplugins.ConfigureCustomNetworkInstance(batch, t, dut, ni)

	// Configure non-default gnmi server.
	cfgplugins.CreateGNMIServer(batch, t, dut, customVRFName)

	// Assign port2 to custom network instance for all vendors
	dp2 := dut.Port(t, "port2")
	port2IntfPath := dutPort2.NewOCInterface(dp2.Name(), dut)
	gnmi.BatchUpdate(batch, gnmi.OC().Interface(dp2.Name()).Config(), port2IntfPath)
	cfgplugins.AssignToNetworkInstance(batch, t, dut, dp2.Name(), customVRFName, 0)

	t.Log("\nApplying configuration to DUT\n")

	batch.Set(t, dut)

	for _, netInstance := range gnmi.GetAll(t, dut, gnmi.OC().NetworkInstanceAny().State()) {
		t.Logf("Network instance: %s", netInstance.GetName())
	}

	// Get and validate states for default and custom networkinstances.
	gnmiServerList := gnmi.GetAll(t, dut, gnmi.OC().System().GrpcServerAny().State())
	for _, gnmiServer := range gnmiServerList {
		if gnmiServer.GetName() == deviations.DefaultNetworkInstance(dut) {
			defaultInstanceState := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(deviations.DefaultNetworkInstance(dut)).State())
			validateGnmiServerState(t, defaultInstanceState)
		}
		if gnmiServer.GetName() == customVRFName {
			customInstanceState := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(customVRFName).State())
			validateGnmiServerState(t, customInstanceState)
		}
	}

	// Two Servers should be running on the DUT.
	gnmiServerCount := len(gnmi.GetAll(t, dut, gnmi.OC().System().GrpcServerAny().State()))

	// Two gNMI servers or more are expected.
	if gnmiServerCount < 2 {
		t.Fatalf("Expected 2+ gNMI servers, got %d.", gnmiServerCount)
	}
}

// validateGnmiServerState checks and logs the state of a gNMI server.
func validateGnmiServerState(t *testing.T, state *oc.System_GrpcServer) {
	if state == nil {
		t.Errorf("gNMI server state is nil")
		return
	}
	if !state.GetEnable() {
		t.Errorf("Expected gNMI server '%s' to be enabled, but it is not.", state.GetName())
	}
	t.Logf("gNMI Server: %s, running on network instance: %s, listening port: %v, Enabled: %t",
		state.GetName(), state.GetNetworkInstance(), state.GetPort(), state.GetEnable())
}
