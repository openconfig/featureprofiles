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
		Name:         "port1",
		Desc:         "dutPort1",
		IPv4:         "192.0.2.1",
		IPv4Len:      30,
		IPv6:         "2001:0db8::192:0:2:1",
		IPv6Len:      126,
		Subinterface: 0,
	}

	dutPort2 = attrs.Attributes{
		Name:         "port2",
		Desc:         "dutPort2",
		IPv4:         "192.0.2.5",
		IPv4Len:      30,
		IPv6:         "2001:0db8::192:0:2:5",
		IPv6Len:      126,
		Subinterface: 0,
	}
	dutPort1NetworkInstanceIParams = cfgplugins.NetworkInstanceParams{
		Name:    "DEFAULT",
		Default: true,
	}

	dutPort2NetworkInstanceIParams = cfgplugins.NetworkInstanceParams{
		Name:    customVRFName,
		Default: false,
	}
)

func TestCodelab(t *testing.T) {
	// configure DUT
	dut := ondatra.DUT(t, "dut")
	batch := &gnmi.SetBatch{}
	ConfigureDUT(batch, t, dut)
	ConfigureAdditionalNetworkInstance(batch, t, dut, customVRFName)
	t.Log("\nApplying configuration to DUT\n")
	batch.Set(t, dut)
	t.Log("\nConfiguration applied to DUT\n")
	ValidateNetworkInstance(t, dut)
}

// ConfigureDUT configures port1 and port2 on the DUT with default network instance.
func ConfigureDUT(batch *gnmi.SetBatch, t *testing.T, dut *ondatra.DUTDevice) {

	// Configuring basic interface and subinterfaces.
	cfgplugins.EnableInterfaceAndSubinterfaces(t, batch, dut, dutPort1)

	// Deviations for vendors that require explicit interface to network instance assignment.
	// This is not required for all vendors, as the interface to network instance mapping is implicit to default network instance.
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		cfgplugins.AssignInterfaceToNetworkInstance(t, batch, dut, dut.Port(t, "port1").Name(), &dutPort1NetworkInstanceIParams, dutPort1.Subinterface)
	}

	// Configure default network instance.
	cfgplugins.NewNetworkInstance(t, batch, dut, &dutPort1NetworkInstanceIParams)

	// Configure gNMI server on default network instance.
	cfgplugins.CreateGNMIServer(batch, t, dut, &dutPort1NetworkInstanceIParams)
}

// ConfigureAdditionalNetworkInstance configures a new network instance in DUT and changes the network instance of port2
func ConfigureAdditionalNetworkInstance(batch *gnmi.SetBatch, t *testing.T, dut *ondatra.DUTDevice, ni string) {
	// Configure interface, non-default network instance
	t.Logf("\nConfiguring network instance and gNMI server: Network instance: %s \n", ni)

	// Configuring basic interface and subinterfaces.
	cfgplugins.EnableInterfaceAndSubinterfaces(t, batch, dut, dutPort2)
	// Assigning interface to non-default network instance.
	cfgplugins.AssignInterfaceToNetworkInstance(t, batch, dut, dut.Port(t, "port2").Name(), &dutPort2NetworkInstanceIParams, dutPort2.Subinterface)

	// Configure non-default network instance.
	cfgplugins.NewNetworkInstance(t, batch, dut, &dutPort2NetworkInstanceIParams)

	// Configure non-default gNMI server.
	cfgplugins.CreateGNMIServer(batch, t, dut, &dutPort2NetworkInstanceIParams)
}

func ValidateNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {

	t.Log("\nValidating network instance after configuration\n")
	// Get all network instances.
	netInstanceList := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstanceAny().State())
	t.Logf("Network instance list length: %v", len(netInstanceList))

	// Get and validate states for default and custom networkinstances.
	gnmiServerList := gnmi.GetAll(t, dut, gnmi.OC().System().GrpcServerAny().State())
	t.Logf("gNMI server list length: %v", len(gnmiServerList))

	// Two VRF should be running on the DUT.
	niC := len(netInstanceList)
	if len(netInstanceList) < 2 {
		t.Fatalf("Expected 2+ VRF , got %d.", niC)
	}

	// Two Servers should be running on the DUT.
	gnmiServerCount := len(gnmiServerList)
	if gnmiServerCount < 2 {
		t.Fatalf("Expected 2+ gNMI servers, got %d.", gnmiServerCount)
	}

	for _, gnmiServer := range gnmiServerList {
		// This check is for vendors aligned with OC string expectation for default gRPC server name being "DEFAULT".
		if gnmiServer.GetName() == "DEFAULT" || gnmiServer.GetName() == deviations.DefaultNetworkInstance(dut) {
			defaultInstanceState := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").State())
			validateGnmiServerState(t, defaultInstanceState)
		}

		// This check is for vendors not aligned with OC string expectation for default gRPC server name being "DEFAULT" and custom gRPC server name being "gnxi-customVRFName".
		// This looks for deviation flag "default_ni_gnmi_server_name"
		if gnmiServer.GetName() == deviations.DefaultNiGnmiServerName(dut) {
			customInstanceState := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(deviations.DefaultNiGnmiServerName(dut)).State())
			validateGnmiServerState(t, customInstanceState)
		}

		// Validate custom network instance state.
		if gnmiServer.GetName() == customVRFName {
			customInstanceState := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(customVRFName).State())
			validateGnmiServerState(t, customInstanceState)
		}
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
