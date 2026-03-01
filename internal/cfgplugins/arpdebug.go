package cfgplugins

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	spb "github.com/openconfig/gnoi/system/system_go_proto"
	tpb "github.com/openconfig/gnoi/types/types_go_proto"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra"
)

const (
	transceiverType        = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
	sensorType             = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR
	sleepDuration          = time.Minute
	minOpticsPower         = -40.0
	maxOpticsPower         = 10.0
	minOpticsHighThreshold = 1.0
	maxOpticsLowThreshold  = -1.0
)

// ATEInterfaceIPInfo holds the device, interface, and IP details.
type ATEInterfaceIPInfo struct {
	DeviceName   string
	EthernetName string
	IPv4s        []string // Just the IPv4 addresses
	IPv6s        []string // Just the IPv6 addresses
}

func DebugARPResolution(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, ipType string) {
	t.Helper()
	otgutils.LogPortMetrics(t, ate.OTG(), top)
	ctx := context.Background()

	ports := dut.Ports() // Get all ports on the DUT
	if len(ports) == 0 {
		t.Logf("No ports found on DUT %s", dut.Name())
		return
	}

	t.Logf("Found %d ports on DUT %s. Iterating through them...", len(ports), dut.Name())
	// Iterate through all ports on DUT
	for _, port := range ports {
		t.Logf("\n--- Processing Port: %s ---", port.Name())
		// Fetch the input and output optic powers for the port.
		inputPowers, outputPowers, err := getPortOpticPowers(t, dut, port.Name())
		if err != nil {
			t.Logf("Error fetching optic powers for port %s: %v", port.Name(), err)
		}
		if len(inputPowers) == 0 {
			t.Logf("No input power readings for port %s", port.Name())
		} else {
			t.Logf("Port %s - Input Powers (dBm): %v", port.Name(), inputPowers)
			for i, power := range inputPowers {
				if power < minOpticsPower || power > maxOpticsPower {
					t.Logf("Port %s Channel %d Input Power %f dBm is out of expected range [%f, %f]", port.Name(), i, power, minOpticsPower, maxOpticsPower)
				}
			}
		}
		if len(outputPowers) == 0 {
			t.Logf("No output power readings for port %s", port.Name())
		} else {
			t.Logf("Port %s - Output Powers (dBm): %v", port.Name(), outputPowers)
			for i, power := range outputPowers {
				if power < minOpticsPower || power > maxOpticsPower {
					t.Logf("Port %s Channel %d Output Power %f dBm is out of expected range [%f, %f]", port.Name(), i, power, minOpticsPower, maxOpticsPower)
				}
			}
		}
		// Fetch the VRF name for the port to be used for ping requests.
		vrfName := getInterfaceVRF(t, dut, port.Name())
		t.Logf("    Port %s is currently in VRF: %s", port.Name(), vrfName)
		ateIPs := getATEInterfaceIPs(t, top)

		var l3Protocol tpb.L3Protocol
		var dutIPs []string // To store IPs from DUT for ping source
		var ateTargetIPs []string
		switch ipType {
		case "IPv4":
			l3Protocol = tpb.L3Protocol_IPV4
			// Construct the gNMI path to query the state of all IPv4 address and counter entries
			// on all subinterfaces of the current port.
			ipAddrsPath := gnmi.OC().Interface(port.Name()).SubinterfaceAny().Ipv4().AddressAny().State()
			addrStates := gnmi.GetAll(t, dut, ipAddrsPath)
			if len(addrStates) == 0 {
				t.Logf("  No IPv4 addresses found on interface %s", port.Name())
				continue // Continue to the next port
			}
			t.Logf("  Found %d IPv4 address entries on %s:", len(addrStates), port.Name())
			for i, addrState := range addrStates {
				ip := addrState.GetIp()
				if ip == "" {
					t.Logf("    Entry %d: Skipping, IP address is missing.", i)
					continue
				}
				t.Logf("    Entry %d: IP Address: %s, Prefix Length: %d", i, ip, addrState.GetPrefixLength())
				dutIPs = append(dutIPs, ip)
			}

			interfaceCountersPath := gnmi.OC().Interface(port.Name()).SubinterfaceAny().Ipv4().Counters().State()
			ipCounterStates := gnmi.GetAll(t, dut, interfaceCountersPath)
			for i, counterState := range ipCounterStates {
				t.Logf("   For portName %s, Entry %d: CounterInPkts: %d, CounterOutPkts: %d, CounterInErrorPkts: %d, CounterOutErrorPkts: %d", port.Name(), i, counterState.GetInPkts(), counterState.GetOutPkts(), counterState.GetInErrorPkts(), counterState.GetOutErrorPkts())
			}

			for _, ateIP := range ateIPs {
				ateTargetIPs = append(ateTargetIPs, ateIP.IPv4s...)
			}

		case "IPv6":
			l3Protocol = tpb.L3Protocol_IPV6
			// Construct the gNMI path to query the state of all IPv6 address and counter entries
			// on all subinterfaces of the current port.
			ipAddrsPath := gnmi.OC().Interface(port.Name()).SubinterfaceAny().Ipv6().AddressAny().State()
			addrStates := gnmi.GetAll(t, dut, ipAddrsPath)
			if len(addrStates) == 0 {
				t.Logf("  No IPv6 addresses found on interface %s", port.Name())
				continue // Continue to the next port
			}
			t.Logf("  Found %d IPv6 address entries on %s:", len(addrStates), port.Name())
			for i, addrState := range addrStates {
				ip := addrState.GetIp()
				if ip == "" {
					t.Logf("    Entry %d: Skipping, IP address is missing.", i)
					continue
				}
				t.Logf("    Entry %d: IP Address: %s, Prefix Length: %d", i, ip, addrState.GetPrefixLength())
				dutIPs = append(dutIPs, ip)
			}

			interfaceCountersPath := gnmi.OC().Interface(port.Name()).SubinterfaceAny().Ipv6().Counters().State()
			ipCounterStates := gnmi.GetAll(t, dut, interfaceCountersPath)
			for i, counterState := range ipCounterStates {
				t.Logf("   For portName %s, Entry %d: CounterInPkts: %d, CounterOutPkts: %d, CounterInErrorPkts: %d, CounterOutErrorPkts: %d", port.Name(), i, counterState.GetInPkts(), counterState.GetOutPkts(), counterState.GetInErrorPkts(), counterState.GetOutErrorPkts())
			}

			for _, ateIP := range ateIPs {
				ateTargetIPs = append(ateTargetIPs, ateIP.IPv6s...)
			}

		default:
			t.Fatalf("Unsupported ipType: %s", ipType)
		}

		// 3. Initiate gNOI Ping from each DUT IP to all collected ATE IPs
		for _, dutIP := range dutIPs {
			pingRequestForATE(ctx, ateTargetIPs, l3Protocol, dut, t, dutIP, port, vrfName)
		}
		otgutils.LogPortMetrics(t, ate.OTG(), top)
	} // End dut.Ports() loop
	otgutils.WaitForARP(t, ate.OTG(), top, ipType)
}

// getInterfaceVRF determines the VRF (Network Instance) for a given port.
// It returns "DEFAULT" if the port is not explicitly bound to a non-default VRF.
func getInterfaceVRF(t *testing.T, dut *ondatra.DUTDevice, portName string) string {
	t.Helper()

	// This path attempts to look up the 'id' leaf for the given portName under ANY network instance.
	// If the interface is bound to a specific network instance, this lookup will succeed.
	nis := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstanceAny().State())

	for _, ni := range nis {
		niName := ni.GetName()
		t.Logf("niName: %s", niName)
		// Skip well-known names for the default instance.
		// Vendors might use different names for the default NI, e.g., "", "default", "DEFAULT".
		if niName == "" || niName == "DEFAULT" || niName == "default" {
			continue
		}

		// 2. Check if the port exists within this specific network instance's interfaces list.
		// We query the 'id' leaf for the specific portName.
		ifPath := gnmi.OC().NetworkInstance(niName).Interface(portName).Id().State()
		ifVal := gnmi.Lookup(t, dut, ifPath)
		t.Logf("ifVal: %v", ifVal)

		if ifVal.IsPresent() {
			// Found the interface in this non-default NI
			t.Logf("Port %s found in VRF: %s", portName, niName)
			return niName
		}
	}

	// If not found in any named network instance, it's in the default.
	t.Logf("Port %s not found in any non-default named network instance, assuming part of: DEFAULT", portName)
	return "DEFAULT"
}

// fetchResponses collects all streaming responses from a gNOI Ping client.
func fetchResponses(c spb.System_PingClient) ([]*spb.PingResponse, error) {
	var pingResp []*spb.PingResponse
	for {
		resp, err := c.Recv()
		switch {
		case err == io.EOF:
			return pingResp, nil
		case err != nil:
			return nil, err
		default:
			pingResp = append(pingResp, resp)
		}
	}
}

// GetATEInterfaceIPs extracts and returns the IPv4 and IPv6 addresses configured
// on each ATE interface as defined in the gosnappi.Config.
func getATEInterfaceIPs(t *testing.T, top gosnappi.Config) []ATEInterfaceIPInfo {
	t.Helper()
	var interfaceIPs []ATEInterfaceIPInfo

	if top.Devices() == nil || len(top.Devices().Items()) == 0 {
		t.Log("No devices found in the ATE configuration.")
		return interfaceIPs
	}

	for _, d := range top.Devices().Items() {
		deviceName := d.Name()
		if d.Ethernets() == nil || len(d.Ethernets().Items()) == 0 {
			continue
		}

		for _, eth := range d.Ethernets().Items() {
			ethName := eth.Name()

			currentInfo := ATEInterfaceIPInfo{
				DeviceName:   deviceName,
				EthernetName: ethName,
				IPv4s:        []string{},
				IPv6s:        []string{},
			}

			// Extract IPv4 Addresses
			if eth.Ipv4Addresses() != nil && len(eth.Ipv4Addresses().Items()) > 0 {
				for _, ip4 := range eth.Ipv4Addresses().Items() {
					currentInfo.IPv4s = append(currentInfo.IPv4s, ip4.Address())
				}
			}

			// Extract IPv6 Addresses
			if eth.Ipv6Addresses() != nil && len(eth.Ipv6Addresses().Items()) > 0 {
				for _, ip6 := range eth.Ipv6Addresses().Items() {
					currentInfo.IPv6s = append(currentInfo.IPv6s, ip6.Address())
				}
			}
			interfaceIPs = append(interfaceIPs, currentInfo)
		}
	}
	return interfaceIPs
}

// GetPortOpticPowers fetches the instant input and output power for all channels
// associated with the transceiver for the given portName.
// It returns a slice of input powers, a slice of output powers (in dBm), and an error if an issue occurs.
func getPortOpticPowers(t *testing.T, dut *ondatra.DUTDevice, portName string) ([]float64, []float64, error) {
	t.Helper()

	// 1. Find the transceiver component name for the given port.
	// This path corresponds to /interfaces/interface[name=<portName>]/state/transceiver
	transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Transceiver().State())
	if transceiverName == "" {
		return nil, nil, fmt.Errorf("no transceiver name found for port %s", portName)
	}
	t.Logf("Port %s is associated with transceiver component %s", portName, transceiverName)

	// 2. Define the base path for the transceiver channels.
	channelsPath := gnmi.OC().Component(transceiverName).Transceiver().ChannelAny()

	// 3. Query for instant input powers across all channels.
	// Path: /components/component[name=<transceiverName>]/transceiver/physical-channels/channel[index=*]/state/input-power/instant
	var inputPowers []float64
	inPowerVals := gnmi.LookupAll(t, dut, channelsPath.InputPower().Instant().State())
	for _, val := range inPowerVals {
		if power, ok := val.Val(); ok {
			inputPowers = append(inputPowers, power)
		} else {
			t.Logf("Failed to get value for an input power path on %s", transceiverName)
		}
	}

	// 4. Query for instant output powers across all channels.
	// Path: /components/component[name=<transceiverName>]/transceiver/physical-channels/channel[index=*]/state/output-power/instant
	var outputPowers []float64
	outPowerVals := gnmi.LookupAll(t, dut, channelsPath.OutputPower().Instant().State())
	for _, val := range outPowerVals {
		if power, ok := val.Val(); ok {
			outputPowers = append(outputPowers, power)
		} else {
			t.Logf("Failed to get value for an output power path on %s", transceiverName)
		}
	}

	if len(inputPowers) == 0 && len(outputPowers) == 0 {
		// Log a warning if no data was found, but don't fail the helper.
		// The caller can decide if this is an error condition.
		t.Logf("No input or output power data found for transceiver %s on port %s", transceiverName, portName)
	}

	return inputPowers, outputPowers, nil
}

func pingRequestForATE(ctx context.Context, ateIPs []string, l3Protocol tpb.L3Protocol, dut *ondatra.DUTDevice, t *testing.T, ip string, port *ondatra.Port, vrfname string) {
	if vrfname == "DEFAULT" || vrfname == "default" {
		vrfname = ""
	}
	for _, ateIP := range ateIPs {
		gnoiClient := dut.RawAPIs().GNOI(t)
		pingRequest := &spb.PingRequest{
			Destination:     ateIP,
			Count:           3,
			Source:          ip,
			L3Protocol:      l3Protocol,
			Interval:        500 * 1000 * 1000,      // 500ms in nanoseconds
			Wait:            2 * 1000 * 1000 * 1000, // 2s in nanoseconds
			NetworkInstance: vrfname,
		}
		t.Logf("      Issuing gNOI Ping: %v", pingRequest)
		pingClient, err := gnoiClient.System().Ping(ctx, pingRequest)
		if err != nil {
			t.Logf("      Failed to start gNOI Ping to %s from %s: %v", ateIP, port.Name(), err)
			continue
		}

		responses, err := fetchResponses(pingClient)
		if err != nil {
			t.Logf("      Failed to fetch gNOI Ping responses for ping to %s: %v", ateIP, err)
			continue
		}

		if len(responses) == 0 {
			t.Logf("      No responses stream received for broadcast ping (this can be normal).")
		} else {
			// The last message is typically the summary.
			summary := responses[len(responses)-1]
			t.Logf("      gNOI Ping Summary: Sent: %d, Received: %d, Time: %d ns", summary.GetSent(), summary.GetReceived(), summary.GetTime())

			// Basic check on Sent count
			if summary.GetSent() != pingRequest.Count {
				t.Logf("        Expected to send %d pings, but summary shows %d", pingRequest.Count, summary.GetSent())
			}
			// Note: summary.GetReceived() is expected to be 0 for broadcast pings in most environments.
			t.Logf("        Received %d responses (expected 0 for broadcast)", summary.GetReceived())
		}
	}
}

