package helper

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

type p4rtHelper struct{}

// P4RTNodesByPortForAllPorts returns a map of <portID>:<P4RTNodeName> for all the
// ports on the router.
func (p *p4rtHelper) P4RTNodesByPortForAllPorts(t *testing.T, dut *ondatra.DUTDevice) map[string]string {
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

// getAllInterfacesFromDevice retrieves all interfaces from the device filtering out
// non-physical interfaces.
func getAllInterfacesFromDevice(t *testing.T, dut *ondatra.DUTDevice) []string {
	var allIntfs []string

	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())

	for _, intf := range interfaces {
		// Filter based on interface type - only include physical Ethernet interfaces
		if intf.GetType() == oc.IETFInterfaces_InterfaceType_ethernetCsmacd {
			allIntfs = append(allIntfs, intf.GetName())
		}
	}

	return allIntfs
}

// configureDeviceIDs configures p4rt device-id on all P4RT nodes on the DUT using batch configuration.
func (p *p4rtHelper) ConfigureDeviceID(t *testing.T, dut *ondatra.DUTDevice, seedDeviceID uint64) {
	nodes := p.P4RTNodesByPortForAllPorts(t, dut)

	// Create a seeded random number generator
	// Ensure seed is positive for rand.NewSource
	seed := int64(seedDeviceID & ((^uint64(0)) >> 1))
	if seed == 0 {
		seed = 1
	}
	rng := rand.New(rand.NewSource(seed))

	seen := make(map[string]bool)
	usedIDs := make(map[uint64]bool)

	batch := &gnmi.SetBatch{}
	for _, p4rtNode := range nodes {
		if seen[p4rtNode] {
			continue
		}
		seen[p4rtNode] = true

		// Generate random device ID in range [1, 18446744073709551615], ensuring uniqueness
		var deviceID uint64
		for {
			deviceID = rng.Uint64()
			if deviceID == 0 {
				continue
			}
			if !usedIDs[deviceID] {
				usedIDs[deviceID] = true
				break
			}
		}

		t.Logf("Configuring P4RT Node: %s with device ID: %d", p4rtNode, deviceID)
		c := oc.Component{}
		c.Name = ygot.String(p4rtNode)
		c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
		c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
		gnmi.BatchUpdate(batch, gnmi.OC().Component(p4rtNode).Config(), &c)
	}
	batch.Set(t, dut)
}

// ConfigureInterfaceID configures interface IDs using batch configuration.
func (p *p4rtHelper) ConfigureInterfaceID(t *testing.T, dut *ondatra.DUTDevice, seedInterfaceID uint32) {
	rand := rand.New(rand.NewSource(int64(seedInterfaceID)))
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	seenIDs := make(map[uint32]bool)

	batch := &gnmi.SetBatch{}
	for _, intf := range interfaces {
		intfName := intf.GetName()
		if intfName == "" || strings.Contains(intfName, "MgmtEth") ||
			strings.Contains(intfName, "Loopback") ||
			strings.Contains(intfName, "Null") ||
			strings.Contains(intfName, "PTP") ||
			strings.Contains(intfName, "Bundle") {
			continue
		}
		// Generate random interface ID in range [1, 4294967039], ensuring uniqueness
		var id uint32
		for {
			id = uint32(rand.Intn(4294967039) + 1)
			if !seenIDs[id] {
				seenIDs[id] = true
				break
			}
		}

		// Create interface config structure with the ID field
		intfConfig := &oc.Interface{
			Type: intf.GetType(),
			Id:   ygot.Uint32(id),
			Name: ygot.String(intfName),
		}
		gnmi.BatchUpdate(batch, gnmi.OC().Interface(intfName).Config(), intfConfig)
	}
	batch.Set(t, dut)
}
