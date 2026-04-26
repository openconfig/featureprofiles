package gnmi_ni_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

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

func TestGNMIAdditionalNetworkInstance(t *testing.T) {
	// configure DUT
	dut := ondatra.DUT(t, "dut")
	// Get current ports from the device
	usedPorts := GetUsedPorts(t, dut)
	// Generate a port number between 10000 and 65535
	gNMIPort1, err := GenerateUniquePort(10000, 65535, usedPorts)
	if err != nil {
		t.Fatalf("Could not generate port: %v", err)
	}
	t.Logf("Generated unique gNMIport1: %d", gNMIPort1)

	usedPorts = append(usedPorts, gNMIPort1)
	gNMIPort2, err := GenerateUniquePort(10000, 65535, usedPorts)
	if err != nil {
		t.Fatalf("Could not generate port: %v", err)
	}
	t.Logf("Generated unique gNMIport2: %d", gNMIPort2)

	batch := &gnmi.SetBatch{}
	ConfigureDUT(batch, t, dut, gNMIPort1)
	ConfigureAdditionalNetworkInstance(batch, t, dut, customVRFName, gNMIPort2)
	t.Log("\nApplying configuration to DUT\n")
	batch.Set(t, dut)
	t.Log("\nConfiguration applied to DUT\n")
	ValidateNetworkInstance(t, dut)
}

// ConfigureDUT configures port1 and port2 on the DUT with default network instance.
func ConfigureDUT(batch *gnmi.SetBatch, t *testing.T, dut *ondatra.DUTDevice, gNMIPort uint16) {

	// Configuring basic interface and subinterfaces.
	cfgplugins.EnableInterfaceAndSubinterfaces(t, dut, batch, dutPort1)

	// Deviations for vendors that require explicit interface to network instance assignment.
	// This is not required for all vendors, as the interface to network instance mapping is implicit to default network instance.
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		cfgplugins.AssignInterfaceToNetworkInstance(t, batch, dut, dut.Port(t, "port1").Name(), &dutPort1NetworkInstanceIParams, dutPort1.Subinterface)
	}

	// TBD Not required #### Configure default network instance.
	//cfgplugins.NewNetworkInstance(t, dut, batch, &dutPort1NetworkInstanceIParams)

	// Configure gNMI server on default network instance.
	transportSecurity := false
	cfgplugins.CreateGNMIServer(t, dut, batch, &dutPort1NetworkInstanceIParams, gNMIPort, transportSecurity)
}

// ConfigureAdditionalNetworkInstance configures a new network instance in DUT and changes the network instance of port2
func ConfigureAdditionalNetworkInstance(batch *gnmi.SetBatch, t *testing.T, dut *ondatra.DUTDevice, ni string, gNMIPort uint16) {
	// Configure interface, non-default network instance
	t.Logf("\nConfiguring network instance and gNMI server: Network instance: %s \n", ni)

	// Configuring basic interface and subinterfaces.
	cfgplugins.EnableInterfaceAndSubinterfaces(t, dut, batch, dutPort2)
	// Assigning interface to non-default network instance.
	cfgplugins.AssignInterfaceToNetworkInstance(t, batch, dut, dut.Port(t, "port2").Name(), &dutPort2NetworkInstanceIParams, dutPort2.Subinterface)

	// Configure non-default network instance.
	cfgplugins.NewNetworkInstance(t, dut, batch, &dutPort2NetworkInstanceIParams)

	// Configure non-default gNMI server.
	transportSecurity := false
	cfgplugins.CreateGNMIServer(t, dut, batch, &dutPort2NetworkInstanceIParams, gNMIPort, transportSecurity)
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

	// As per `CreateGNMIServer`, the custom gNMI server is prefixed with "gnxi-".
	customGnmiServerName := "gnxi-" + customVRFName

	var defaultValidated, customValidated bool
	//if Juniper remove DEFAULT from the list.

	for _, gnmiServer := range gnmiServerList {
		serverName := gnmiServer.GetName()
		// 1. Strict Skip for the internal Juniper system entry
		if dut.Vendor() == ondatra.JUNIPER && serverName == "DEFAULT" {
			t.Logf("Skipping internal Juniper system server placeholder: %s", serverName)
			continue
		}
		// Using gnmiServer.GetName() to get the state is better than hardcoding.
		serverState := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(serverName).State())
		switch serverName {
		case customGnmiServerName:
			validateGnmiServerState(t, serverState)
			customValidated = true
		// Handle default gNMI server, which could have several names.
		case "DEFAULT", deviations.DefaultNetworkInstance(dut), deviations.DefaultNiGnmiServerName(dut):
			// To avoid validating the same server multiple times if names overlap.
			if !defaultValidated {
				validateGnmiServerState(t, serverState)
				defaultValidated = true
			}
		}
	}

	if !defaultValidated {
		t.Error("Default gNMI server was not found or validated.")
	}
	if !customValidated {
		t.Errorf("Custom gNMI server '%s' was not found or validated.", customGnmiServerName)
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
func GetUsedPorts(t *testing.T, dut *ondatra.DUTDevice) []uint16 {
	t.Helper()
	var used []uint16

	// Query all gRPC servers in the system
	servers := gnmi.GetAll(t, dut, gnmi.OC().System().GrpcServerAny().State())

	for _, s := range servers {
		if s.Port != nil {
			used = append(used, s.GetPort())
		}
	}

	return used
}

// GenerateUniquePort picks a random port between min and max that isn't in the used list.
func GenerateUniquePort(min, max int, usedPorts []uint16) (uint16, error) {
	// Seed the random generator
	rand.Seed(time.Now().UnixNano())

	// Create a map for O(1) lookup
	usedMap := make(map[uint16]bool)
	for _, p := range usedPorts {
		usedMap[p] = true
	}

	// Try up to 100 times to find a random free port
	for i := 0; i < 100; i++ {
		p := uint16(rand.Intn(max-min+1) + min)
		if !usedMap[p] {
			return p, nil
		}
	}

	return 0, fmt.Errorf("failed to find a unique port after 100 attempts")
}
