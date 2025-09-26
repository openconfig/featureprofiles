package b4_scale_profile_test

import (
	"strings"
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// P4RTNodesByPort returns a map of <portID>:<P4RTNodeName> for the reserved ondatra
// ports using the component and the interface OC tree.
func P4RTNodesByPort(t *testing.T, dut *ondatra.DUTDevice) map[string]string {
	t.Helper()
	ports := make(map[string][]string) // <hardware-port>:[<portID>]
	for _, p := range getAllInterfacesFromDevice(t, dut) {
		hp := gnmi.Lookup(t, dut, gnmi.OC().Interface(p).HardwarePort().State())
		if v, ok := hp.Val(); ok {
			if _, ok = ports[v]; !ok {
				ports[v] = []string{p}
			} else {
				ports[v] = append(ports[v], p)
			}
		}
	}
	nodes := make(map[string]string) // <hardware-port>:<p4rtComponentName>
	for hp := range ports {
		p4Node := gnmi.Lookup(t, dut, gnmi.OC().Component(hp).Parent().State())
		if v, ok := p4Node.Val(); ok {
			nodes[hp] = v
		}
	}
	res := make(map[string]string) // <portID>:<P4RTNodeName>
	for k, v := range nodes {
		cType := gnmi.Lookup(t, dut, gnmi.OC().Component(v).Type().State())
		ct, ok := cType.Val()
		if !ok {
			continue
		}
		if ct != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT {
			continue
		}
		for _, p := range ports[k] {
			res[p] = v
		}
	}
	return res
}

func getAllInterfacesFromDevice(t *testing.T, dut *ondatra.DUTDevice) []string {
	var allIntfs []string

	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())

	for _, intf := range interfaces {
		intfType := intf.GetType()
		intfName := intf.GetName()

		// Filter based on interface type
		switch intfType {
		case oc.IETFInterfaces_InterfaceType_ethernetCsmacd:
			// Physical Ethernet interfaces
			allIntfs = append(allIntfs, intfName)
		case oc.IETFInterfaces_InterfaceType_ieee8023adLag:
			// Bundle/LAG interfaces
			continue
		case oc.IETFInterfaces_InterfaceType_softwareLoopback:
			// Loopback interfaces
			continue
		default:
			continue
		}
	}

	return allIntfs
}

// configureDeviceIDs configures p4rt device-id on all P4RT nodes on the DUT using batch configuration.
func configureDeviceID(t *testing.T, dut *ondatra.DUTDevice) {
	nodes := P4RTNodesByPort(t, dut)
	deviceID := uint64(1)
	seen := make(map[string]bool)

	batch := &gnmi.SetBatch{}
	for _, p4rtNode := range nodes {
		if seen[p4rtNode] {
			continue
		}
		seen[p4rtNode] = true

		t.Logf("Configuring P4RT Node: %s with device ID: %d", p4rtNode, deviceID)
		c := oc.Component{}
		c.Name = ygot.String(p4rtNode)
		c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
		c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
		gnmi.BatchReplace(batch, gnmi.OC().Component(p4rtNode).Config(), &c)
		deviceID++
	}
	batch.Set(t, dut)
}

// configureInterfaceID configures interface IDs using batch configuration.
func configureInterfaceID(t *testing.T, dut *ondatra.DUTDevice) {
	id := uint32(100)
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())

	batch := &gnmi.SetBatch{}
	for _, intf := range interfaces {
		intfName := intf.GetName()
		if intfName == "" || strings.Contains(intfName, "MgmtEth") ||
			strings.Contains(intfName, "Loopback") ||
			strings.Contains(intfName, "Null") ||
			strings.Contains(intfName, "PTP") {
			continue
		}
		// Create interface config structure with the ID field
		intfConfig := &oc.Interface{
			Type: intf.GetType(),
			Id:   ygot.Uint32(id),
			Name: ygot.String(intfName),
		}
		gnmi.BatchUpdate(batch, gnmi.OC().Interface(intfName).Config(), intfConfig)
		id++
	}
	batch.Set(t, dut)
}
