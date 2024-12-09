package breakout_configuration

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

var componentNameList []string

func getSpeedValue(speed oc.E_IfEthernet_ETHERNET_SPEED) string {
	// Convert SPEED_100GB to 100
	speedStr := speed.String()
	re := regexp.MustCompile(`SPEED_(\d+)GB`)
	matches := re.FindStringSubmatch(speedStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func isBreakoutSupported(t *testing.T, dut *ondatra.DUTDevice, port string, numBreakouts uint8, speed oc.E_IfEthernet_ETHERNET_SPEED) bool {
	t.Logf("check phy for port %s", port)

	cliHandle := dut.RawAPIs().CLI(t)
	resp, err := cliHandle.RunCommand(context.Background(), fmt.Sprintf("show controllers phy breakout interface %s", port))
	if err != nil {
		t.Errorf("Failed to get breakout info: %v", err)
		return false
	}

	// Create expected format (e.g., "4x100G")
	expectedBreakout := fmt.Sprintf("%dx%sG", numBreakouts, getSpeedValue(speed))

	// Find pattern like "4x100G" in "OPTICS_BO_TYPE_4x100G"
	re := regexp.MustCompile(`TYPE_(\d+x\d+G)`)
	matches := re.FindStringSubmatch(resp.Output())

	var foundBreakout string
	if len(matches) > 1 {
		foundBreakout = matches[1]
	}

	t.Logf("Optic Supports the Following Breakout Mode: %s, "+
		"Target breakout Configuration is: %s", foundBreakout, expectedBreakout)

	return foundBreakout == expectedBreakout
}

// verifyBreakout checks if the breakout configuration matches the expected values.
// It reports errors to the testing object if there is a mismatch.
func verifyBreakout(index uint8, numBreakoutsWant uint8, numBreakoutsGot uint8, breakoutSpeedWant string, breakoutSpeedGot string, t *testing.T) {
	// Ensure that the index is set to the expected value (1 in this case).
	if index != uint8(0) {
		t.Errorf("Index: got %v, want 1", index)
	}
	// Check if the number of breakouts configured matches what was expected.
	if numBreakoutsGot != numBreakoutsWant {
		t.Errorf("Number of breakouts configured: got %v, want %v", numBreakoutsGot, numBreakoutsWant)
	}
	// Verify that the configured breakout speed is as expected.
	if breakoutSpeedGot != breakoutSpeedWant {
		t.Errorf("Breakout speed configured: got %v, want %v", breakoutSpeedGot, breakoutSpeedWant)
	}

}

func verifyDelete(t *testing.T, dut *ondatra.DUTDevice, compname string, schemaValue uint8) {

	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Get(t, dut, gnmi.OC().Component(compname).Port().BreakoutMode().Group(schemaValue).Index().Config()) //catch the error  as it is expected and absorb the panic.
	}); errMsg != nil {
		t.Log("Expected failure as this verifies the breakout config is removed")
	} else {
		t.Errorf("This get on empty config should have failed : %s", *errMsg)
	}
}

// IncrementIPNetworkPart generates a slice of IP addresses incremented based on the initial IP address provided.
// The function takes an initial IP address, the number of increments, whether it is IPv4, and what the last octet should be.
func IncrementIPNetwork(ipStr string, numBreakouts uint8, isIPv4 bool, lastOctet uint8) ([]string, error) {
	// Parse the initial IP address string into an IP object.
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address format")
	}

	// Prepare a slice to hold the generated IP addresses.
	ips := make([]string, numBreakouts)

	// Define a variable to hold the IP address as an integer for IPv4.
	var ip4 uint32

	// If it's an IPv4 address, convert the IP to a 32-bit integer and clear the last octet.
	if isIPv4 {
		ip4 = binary.BigEndian.Uint32(ip.To4())
		ip4 &= 0xFFFFFF00 // Apply a mask to retain only the first three octets.
	}

	// Loop to generate the required number of IP addresses.
	for i := uint8(0); i < numBreakouts; i++ {
		// For IPv4, increment the third octet and set the last octet to the specified value.
		newIP4 := (ip4 | uint32(lastOctet)) + (uint32(i) << 8)
		incrementedIP := make(net.IP, 4)
		binary.BigEndian.PutUint32(incrementedIP, newIP4)
		ips[i] = fmt.Sprintf("%s/%d", incrementedIP.String(), 24) // Format as a string with a /24 subnet.
	}

	return ips, nil
}

// findNewPortNames fetches the parent breakout port interface states and the new port convention.
// Example being 4x100 parent port would be FourHundredGigE0/0/0/10 this will find and return
// the newly broken out ports of OneHundredGigE0/0/0/0/10/0-4
func findNewPortNames(dut *ondatra.DUTDevice, t *testing.T, originalPortName string, numBreakouts uint8) ([]string, error) {
	// Fetch the current state of all interfaces from the device using gNMI.
	intfs := gnmi.Get(t, dut, gnmi.OC().InterfaceMap().State())

	// Split the original port name by '/' to extract the correct index (third-last segment in this case).
	portSegments := strings.Split(originalPortName, "/")
	if len(portSegments) < 4 {
		return nil, fmt.Errorf("invalid port name format: %v", originalPortName)
	}
	portIndex := portSegments[len(portSegments)-2] // Get the third-last segment, which is "30"

	// Define a pattern to match breakout port names that include the original port index.
	breakoutPattern := fmt.Sprintf(`\w+/\d+/\d+/%s/\d+`, portIndex)

	// Compile the pattern into a regular expression.
	re := regexp.MustCompile(breakoutPattern)

	// Loop through all interfaces and collect those that match the breakout pattern
	newPortNames := []string{}
	for intfName := range intfs {
		if re.MatchString(intfName) {
			newPortNames = append(newPortNames, intfName)
		}
	}

	// Check if the number of new ports found is equal to the number of breakouts expected.
	if len(newPortNames) != int(numBreakouts) {
		return nil, fmt.Errorf("expected to find %d new ports, found %d", numBreakouts, len(newPortNames))
	}

	return newPortNames, nil
}

// fetchResponses will fetch the ping response
func fetchResponses(c spb.System_PingClient) ([]*spb.PingResponse, error) {
	pingResp := []*spb.PingResponse{}
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

func sortBreakoutPorts(breakOutPorts []string) {
	sort.Slice(breakOutPorts, func(i, j int) bool {
		// Extract the trailing number after the last "/"
		numberI, errI := strconv.Atoi(strings.Split(breakOutPorts[i], "/")[4])
		numberJ, errJ := strconv.Atoi(strings.Split(breakOutPorts[j], "/")[4])
		// Handle potential errors from Atoi conversion
		if errI != nil || errJ != nil {
			return false
		}
		return numberI < numberJ
	})
}

// Function to get compName and set up port name based on provided interface and port prefix
func getCompName(dut *ondatra.DUTDevice, string, portPrefix string, t *testing.T) (string, string, bool) {

	foundExpectedInterfaceFlag = false
	portsAll := dut.Ports()

	// Look for the port with the specified prefix (e.g., "TenGigE" or "HundredGigE")
	for _, port := range portsAll {
		if strings.HasPrefix(port.Name(), portPrefix) {
			portID := port.ID()
			t.Logf("Found port %s with portIdString %s", port.Name(), portID)
			dutPortName = dut.Port(t, portID).Name()
			foundExpectedInterfaceFlag = true
			break
		}
	}

	// If the expected interface is not found, skip the test case
	if !foundExpectedInterfaceFlag {
		t.Skipf("Could not find required ports for %s breakout", portPrefix)
	}

	// Extract line card slot and port number from the interface name
	var portNumber = string
	var lcSlot = string
	parts := strings.Split(dutPortName, "/")
	if len(parts) >= 4 {
		lcSlot = parts[2]
		portNumber = parts[3]
		t.Logf("Extracted Linecard Slot: %s, Port Number: %s", lcSlot, portNumber)
		compName := fmt.Sprintf("Port0/%s/0/%s", lcSlot, portNumber)
		t.Logf("compName is: %s", compName)
		return compName, dutPortName, true
	} else {
		t.Logf("Invalid location format: %s", dutPortName)
		return "", "", false
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
