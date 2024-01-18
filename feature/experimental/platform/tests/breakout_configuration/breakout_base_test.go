package basetest

import (
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
)

const (
	RP           = "0/RP0/CPU0"
	linecardType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
)

var (
	observer = fptest.NewObserver("Platform").AddCsvRecorder("ocreport").
		AddCsvRecorder("Platform")
)

var componentNameList []string

// verifyBreakout checks if the breakout configuration matches the expected values.
// It reports errors to the testing object if there is a mismatch.
func verifyBreakout(index uint8, numBreakoutsWant uint8, numBreakoutsGot uint8, breakoutSpeedWant string, breakoutSpeedGot string, t *testing.T) {
	// Ensure that the index is set to the expected value (1 in this case).
	if index != uint8(1) {
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

	// Extract the numerical index of the original port name to identify related breakout ports.
	lastSlashIndex := strings.LastIndex(originalPortName, "/")
	if lastSlashIndex == -1 || lastSlashIndex+1 >= len(originalPortName) {
		return nil, fmt.Errorf("original port name does not contain '/' or has no index after '/'")
	}
	portIndex := originalPortName[lastSlashIndex+1:]

	// Define a pattern to match breakout port names that include the original port index.
	breakoutPattern := fmt.Sprintf(`\w+/\d+/\d+/%s/\d+`, portIndex)

	// Compile the pattern into a regular expression.
	re := regexp.MustCompile(breakoutPattern)

	// Loop through all interfaces and collect those that match the breakout pattern.
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

// ConvertIntfToCompName takes an interface name like "FourHundredGigE0/0/0/30"
// and returns a string like "<LC Location>-QSFP_DD Optics Port 30".
func ConvertIntfToCompName(interfaceName string, linecardLocation string) string {
	// Define a regular expression to match the pattern of the interface name
	re := regexp.MustCompile(`(\D+)(\d+)/(\d+)/(\d+)/(\d+)`)
	matches := re.FindStringSubmatch(interfaceName)

	// Ensure that the regex matches the string fully
	if len(matches) < 6 {
		return ""
	}

	portNumber := matches[5]
	return fmt.Sprintf("%s-QSFP_DD Optics Port %s", linecardLocation, portNumber)
}

func findMatchedPortAndSlot(compPortsList []string, originalPortName string) (matchedPort string, matchedSlot string, err error) {
	// Compile the regular expression pattern
	pattern := fmt.Sprintf(`(?i)(.*)-(%s)$`, regexp.QuoteMeta(originalPortName))
	regex := regexp.MustCompile(pattern)

	for _, port := range compPortsList {
		// Checking if the string matches the regular expression
		if regex.MatchString(port) {
			// Extracting the matched part
			match := regex.FindStringSubmatch(port)
			if len(match) > 2 {
				matchedSlot = match[1]
				matchedPort = match[2]
				return matchedPort, matchedSlot, nil
			}
		}
	}
	return "", "", fmt.Errorf("no match found")
}

// getOriginalPortName depending on the breakout case it will set the port as port1 is used for 100G testing
// and port2 is used fro 10G breakout testing
func getOriginalPortName(dut *ondatra.DUTDevice, t *testing.T, speed oc.E_IfEthernet_ETHERNET_SPEED) *ondatra.Port {
	if speed == oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB {
		return dut.Port(t, "port2")
	}
	// Default to "port1" for other speeds
	return dut.Port(t, "port1")
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}


