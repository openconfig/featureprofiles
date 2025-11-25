package basetest

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

var (
	device1 = "dut"

	observer = fptest.NewObserver("Platform").
			AddCsvRecorder("ocreport").
			AddCsvRecorder("Platform")

	// Platform holds dynamically discovered component names.
	// Populated once by InitPlatform(), then read-only by all tests.
	// Enables platform-agnostic testing (works on P200, Q200, different hardware).

	Platform PlatformInfo

	// allComponents caches ALL device components
	allComponents []*oc.Component

	// initOnce ensures InitPlatform() runs exactly once.
	initOnce sync.Once

	// initErr stores any error from initialization.
	// nil = success, non-nil = initialization failed
	// Checked by tests to determine if discovery succeeded.
	initErr error
)

// ============================================================================
// STRUCTS
// ============================================================================

// PlatformInfo holds dynamically discovered platform component information.
//
// WHY THIS EXISTS:
// Different Cisco platforms have different component naming:
// - Cisco 8201 (P200): "Rack 0", "0/0/CPU0"
// - Cisco 8808 (Q200): Different naming conventions
// This struct makes tests platform-agnostic by discovering names at runtime.
//
// HOW IT'S POPULATED:
// Single gNMI query fetches ALL 15,000+ components, then filters locally:
// - Type filtering: CHASSIS, FAN_TRAY, POWER_SUPPLY, CPU, SENSOR, etc.
// - Name filtering: "TEMP" (sensors), "RP0" (CPU), "IOSXR-PKG", "Bios"
// - Custom logic: OpticsModule (strips "0/0/CPU0-" prefix)
//
// EMPTY FIELDS:
// If component not found (e.g., Transceiver type missing on Cisco), field = "".
// Tests should use CheckRequirements() to skip gracefully when components missing.
type PlatformInfo struct {
	// Hardware Components (Type-based Discovery)
	// ------------------------------------------
	// These have unique OpenConfig types, making discovery straightforward.

	Chassis     string // e.g., "Rack 0" - main chassis enclosure
	Linecard    string // e.g., "0/0/CPU0" - route processor line card (used by most tests)
	FanTray     string // e.g., "0/FT0" - cooling fan tray
	PowerSupply string // e.g., "0/PT2-PM0" - power supply module
	FabricCard  string // e.g., "0/FC0" - fabric switching card

	// Transceiver (Often Empty on Cisco - Important!)
	// -----------------------------------------------
	// NOTE: Sim might not have TRANSCEIVER
	// Tests using this MUST call CheckRequirements() to skip if empty.
	Transceiver string

	// Components Requiring Type + Name Filtering
	// ------------------------------------------
	// Type alone matches too many - need additional name pattern filtering.

	TempSensor string // e.g., "0/0/CPU0-TEMP_FET1_DX"
	// Why name filter? 2029 SENSOR components exist - "TEMP" selects temperature sensors

	SWVersionComponent string // e.g., "0/RP0/CPU0-Broadwell-DE (D-1573N)"
	// Why name filter? 4 CPUs exist (RP0, RP1, LC0, LC1) - "RP0" selects Route Processor 0

	// Components with Name-Only Filtering
	// -----------------------------------
	// These have unreliable or missing type fields.

	BiosFirmware string // e.g., "0/0/CPU0-Bios"
	// Why name-only? BIOS components have NO type field in OpenConfig!

	SubComponent string // e.g., "Rack 0-Line Card Slot 0"
	// Parent-child relationship: chassis contains line card slots
	// Discovered after chassis - shows hardware hierarchy

	// Optics Interface (Custom Logic - Prefix Stripping)
	// --------------------------------------------------
	OpticsModule string // e.g., "EightHundredGigE0/0/0/0"
	// Device name: "0/0/CPU0-EightHundredGigE0/0/0/0"
	// Stripped to: "EightHundredGigE0/0/0/0" (clean interface name)
	// Used for: Transceiver tests, optics config, breakout testing
}

// RequiredComponents defines which components a test needs to run.
//
// WHY THIS EXISTS:
// Handle platform differences gracefully - not all components might be present
//
// USAGE PATTERN:
//
//	CheckRequirements(t, "TestFanTray", RequiredComponents{
//	    FanTray: true,
//	})
//
// BEHAVIOR:
// If FanTray wasn't discovered → test SKIPS (not FAILS)
//
// NOTE: Going with SKIP here but an issue could it might mask legitimately missing components. For that would need a list of must have components which was not there.
type RequiredComponents struct {
	Chassis            bool
	Linecard           bool
	OpticsModule       bool
	FanTray            bool
	PowerSupply        bool
	TempSensor         bool
	BiosFirmware       bool
	Transceiver        bool
	SWVersionComponent bool
	FabricCard         bool
	SubComponent       bool
}

// ============================================================================
// Platform Initialization (Call from TestMain or first test)
// ============================================================================

// InitPlatform discovers and populates platform component information.
// It fetches ALL components in a single gNMI query since the query is cached and reused for all tests.
// Returns error if initialization fails. Safe to call multiple times (idempotent).
func InitPlatform(t *testing.T, dut *ondatra.DUTDevice) error {
	initOnce.Do(func() {
		t.Log("=== Initializing Platform Component Discovery ===")

		// Fetch ALL components with retry logic for robustness.
		const maxRetries = 3
		const retryDelaySeconds = 5

		for attempt := 1; attempt <= maxRetries; attempt++ {
			t.Logf("Fetching all components from device (attempt %d/%d)...", attempt, maxRetries)

			// Wrap in a function to catch panics and convert to errors
			func() {
				defer func() {
					if r := recover(); r != nil {
						initErr = fmt.Errorf("panic during component fetch on attempt %d: %v", attempt, r)
						t.Logf("ERROR: %v", initErr)
					}
				}()

				allComponents = gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())

				// Validate we got reasonable data
				if len(allComponents) == 0 {
					initErr = fmt.Errorf("retrieved 0 components from device on attempt %d - device may not be ready", attempt)
					t.Logf("WARNING: %v", initErr)
					return
				}

				t.Logf("Successfully retrieved %d components from device", len(allComponents))
				initErr = nil // Clear any previous errors
			}()

			// If successful, break out of retry loop
			if initErr == nil && len(allComponents) > 0 {
				break
			}

			// If not last attempt, wait before retrying
			if attempt < maxRetries {
				t.Logf("Retrying in %d seconds...", retryDelaySeconds)
				time.Sleep(time.Duration(retryDelaySeconds) * time.Second)
			}
		}

		// If we still have an error after all retries, log and return
		if initErr != nil {
			t.Errorf("CRITICAL: Failed to fetch components after %d attempts: %v", maxRetries, initErr)
			return
		}

		// Now filter locally (no additional network calls).
		t.Log("Discovering platform components from cached data...")
		Platform = PlatformInfo{
			Chassis:            findComponentByType(oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS),
			Linecard:           findComponentByType(oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD),
			FanTray:            findComponentByType(oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FAN_TRAY),
			PowerSupply:        findComponentByType(oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_POWER_SUPPLY),
			FabricCard:         findComponentByType(oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC),
			Transceiver:        findComponentByType(oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER),
			TempSensor:         findComponentByTypeAndName(oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_SENSOR, "TEMP"),
			SWVersionComponent: findComponentByTypeAndName(oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CPU, "RP0"),
			BiosFirmware:       findComponentByName("Bios"),
			OpticsModule:       findOpticsModule(),
		}

		// SubComponent: find first subcomponent of chassis (if chassis exists).
		if Platform.Chassis != "" {
			Platform.SubComponent = findFirstSubcomponent(Platform.Chassis)
		}

		t.Log("=== Platform Discovery Complete ===")
		logDiscoveredComponents(t)
	})

	return initErr
}

// ============================================================================
// Specialized Component Finders
// ============================================================================

// findOpticsModule finds an optics module and returns just the interface name.
// Strips the component prefix (e.g., "0/0/CPU0-") to get clean interface name.
// Picks a random interface if multiple exist (better than hardcoded preference).
func findOpticsModule() string {
	// Collect all interface-type components (HundredGigE, EightHundredGigE, etc.)
	var interfaces []string
	for _, comp := range allComponents {
		name := comp.GetName()
		// Look for components that are interfaces (contain GigE)
		if strings.Contains(name, "GigE") {
			// Strip the prefix before first "-" to get just the interface name
			// Example: "0/0/CPU0-EightHundredGigE0/0/0/0" → "EightHundredGigE0/0/0/0"
			idx := strings.Index(name, "-")
			if idx != -1 {
				interfaceName := name[idx+1:]
				interfaces = append(interfaces, interfaceName)
			}
		}
	}

	// Return first one found (effectively random since map iteration order is random)
	if len(interfaces) > 0 {
		return interfaces[0]
	}

	return ""
}

// ============================================================================
// Low-Level Component Search Functions (Work on Cached Data)
// ============================================================================

// findComponentByType finds the first component matching the given type.
// Searches the cached allComponents slice (no network calls).
func findComponentByType(compType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) string {
	for _, comp := range allComponents {
		if comp.Type != nil && comp.GetType() == compType {
			return comp.GetName()
		}
	}
	return ""
}

// findComponentByTypeAndName finds first component matching type and name pattern.
// Searches the cached allComponents slice (no network calls).
func findComponentByTypeAndName(compType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT, namePattern string) string {
	for _, comp := range allComponents {
		if comp.Type != nil && comp.GetType() == compType {
			if strings.Contains(strings.ToUpper(comp.GetName()), strings.ToUpper(namePattern)) {
				return comp.GetName()
			}
		}
	}
	return ""
}

// findComponentByName finds first component with name containing the pattern.
// Searches the cached allComponents slice (no network calls).
// Case-insensitive to match behavior of other finder functions.
func findComponentByName(namePattern string) string {
	for _, comp := range allComponents {
		if strings.Contains(
			strings.ToLower(comp.GetName()),
			strings.ToLower(namePattern)) {
			return comp.GetName()
		}
	}
	return ""
}

// findFirstSubcomponent finds the first subcomponent of a given parent.
// Searches the cached allComponents slice (no network calls).
//
// SubComponent Logic Explanation:
// OpenConfig models hardware hierarchy as parent-child relationships.
// Example: Chassis "Rack 0" has children like "Rack 0-Line Card Slot 0", "Rack 0-Power Slot 1", etc.
// This function finds the first component where Parent field == parentName.
func findFirstSubcomponent(parentName string) string {
	// Prioritize "Slot" subcomponents (e.g., "Rack 0-Line Card Slot 0")
	for _, comp := range allComponents {
		if comp.Parent != nil && comp.GetParent() == parentName {
			if strings.Contains(comp.GetName(), "Slot") {
				return comp.GetName()
			}
		}
	}
	return ""
}

// logDiscoveredComponents logs all discovered platform components.
func logDiscoveredComponents(t *testing.T) {
	t.Logf("Chassis:            %q", Platform.Chassis)
	t.Logf("Linecard:           %q", Platform.Linecard)
	t.Logf("FanTray:            %q", Platform.FanTray)
	t.Logf("PowerSupply:        %q", Platform.PowerSupply)
	t.Logf("TempSensor:         %q", Platform.TempSensor)
	t.Logf("Transceiver:        %q", Platform.Transceiver)
	t.Logf("OpticsModule:       %q", Platform.OpticsModule)
	t.Logf("FabricCard:         %q", Platform.FabricCard)
	t.Logf("SWVersionComponent: %q", Platform.SWVersionComponent)
	t.Logf("BiosFirmware:       %q", Platform.BiosFirmware)
	t.Logf("SubComponent:       %q", Platform.SubComponent)
}

// ============================================================================
// Test-Specific Requirement Checkers
// ============================================================================

// CheckRequirements skips the test if any required components are missing.
// This is the Go best-practice way to handle test prerequisites.
//
// Example:
//
//	CheckRequirements(t, "TestTempSensor", RequiredComponents{
//	    TempSensor: true,
//	})
func CheckRequirements(t *testing.T, testName string, reqs RequiredComponents) {
	t.Helper()

	missing := []string{}

	if reqs.Chassis && Platform.Chassis == "" {
		missing = append(missing, "Chassis")
	}
	if reqs.Linecard && Platform.Linecard == "" {
		missing = append(missing, "Linecard")
	}
	if reqs.OpticsModule && Platform.OpticsModule == "" {
		missing = append(missing, "OpticsModule")
	}
	if reqs.FanTray && Platform.FanTray == "" {
		missing = append(missing, "FanTray")
	}
	if reqs.PowerSupply && Platform.PowerSupply == "" {
		missing = append(missing, "PowerSupply")
	}
	if reqs.TempSensor && Platform.TempSensor == "" {
		missing = append(missing, "TempSensor")
	}
	if reqs.BiosFirmware && Platform.BiosFirmware == "" {
		missing = append(missing, "BiosFirmware")
	}
	if reqs.Transceiver && Platform.Transceiver == "" {
		missing = append(missing, "Transceiver")
	}
	if reqs.SWVersionComponent && Platform.SWVersionComponent == "" {
		missing = append(missing, "SWVersionComponent")
	}
	if reqs.FabricCard && Platform.FabricCard == "" {
		missing = append(missing, "FabricCard")
	}
	if reqs.SubComponent && Platform.SubComponent == "" {
		missing = append(missing, "SubComponent")
	}

	if len(missing) > 0 {
		t.Skipf("%s: Required components not found: %v - platform may not support this feature",
			testName, missing)
	}
}

// ============================================================================
// Breakout Configuration Helpers
// ============================================================================

// convertInterfaceToPortComponent converts an interface name to its port component name.
// Example: "FourHundredGigE0/0/0/1" -> "Port0/0/0/1"
//
//	"EightHundredGigE0/0/0/1/2" -> "Port0/0/0/1" (removes channel/sub-port)
//
// Logic: Extract the numeric path (x/y/z/w) and prefix with "Port"
func convertInterfaceToPortComponent(interfaceName string) string {
	// Use regex to extract the port path (e.g., "0/0/0/1" or "0/0/0/1/2")
	// Pattern: any letters followed by digits and slashes
	// Example: "FourHundredGigE0/0/0/1" -> "0/0/0/1"
	re := regexp.MustCompile(`[A-Za-z]+(\d+/\d+/\d+/\d+)(?:/\d+)?`)
	matches := re.FindStringSubmatch(interfaceName)

	if len(matches) < 2 {
		// Defensive: This should never happen with valid interface names from dut.Port()
		// but we return the original as a safe fallback rather than panicking.
		return interfaceName
	}

	// matches[1] contains the captured group: "0/0/0/1"
	// This automatically handles both "0/0/0/1" and "0/0/0/1/2" -> "0/0/0/1"
	return "Port" + matches[1]
}

// getPortComponentForBreakout returns the correct port component name for breakout config.
// It verifies that the /port/breakout-mode path exists for the component.
func getPortComponentForBreakout(t *testing.T, dut *ondatra.DUTDevice, interfaceName string) string {
	t.Helper()

	portComponent := convertInterfaceToPortComponent(interfaceName)

	// Verify the component supports breakout by checking if the path exists.
	_, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(portComponent).Port().BreakoutMode().State()).Val()
	if !ok {
		t.Logf("Warning: Port component %q does not support breakout-mode", portComponent)
		return ""
	}

	t.Logf("Port component for breakout: %q -> %q", interfaceName, portComponent)
	return portComponent
}

func verifyBreakout(index uint8, numBreakoutsWant uint8, numBreakoutsGot uint8, breakoutSpeedWant string, breakoutSpeedGot string, t *testing.T) {

	if index != uint8(0) {
		t.Errorf("Index: got %v, want 1", index)
	}
	if numBreakoutsGot != numBreakoutsWant {
		t.Errorf("Number of breakouts configured : got %v, want %v", numBreakoutsGot, numBreakoutsWant)
	}
	if breakoutSpeedGot != breakoutSpeedWant {
		t.Errorf("Breakout speed configured : got %v, want %v", breakoutSpeedGot, breakoutSpeedWant)
	}
}

func verifyDelete(t *testing.T, dut *ondatra.DUTDevice, compname string) {
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.LookupConfig(t, dut, gnmi.OC().Component(compname).Port().BreakoutMode().Group(1).Index().Config()) //catch the error as it is expected and absorb the panic.
	}); errMsg != nil {
		t.Log("Expected failure - config deleted successfully")
	} else {
		t.Errorf("This Get on empty config should have failed - delete operation may not have worked")
	}
}
