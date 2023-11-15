// Package flags defince flags the are required by tests.
package flags

import (
	"flag"

	"github.com/ogier/pflag"
)

// cisco tests flags.
var (
	GRIBITrafficCheck = flag.Bool("gribi_traffic_check", true,
		"This enable/disable traffic check for gribi tests.")

	GRIBIAFTCheck = flag.Bool("gribi_aft_check", true,
		"This enable/disable AFT check for gribi entries in gribi tests.")

	GRIBINHTimer = flag.Int("gribi_nh_timer", 0,
		"Wait time before executing aft call for NH")

	GRIBINHGTimer = flag.Int("gribi_nhg_timer", 0,
		"Wait time before executing aft call for NHG")

	GRIBIIPv4Timer = flag.Int("gribi_ipv4_timer", 0,
		"Wait time before executing aft call for IPv4")

	GRIBIRemoveTimer = flag.Int("gribi_remove_timer", 2,
		"Wait time before executing aft call for IPv4")

	GRIBIAFTChainCheck = flag.Bool("gribi_aft_chain_check", true,
		"This enable/disable AFT chain check for gribi prefix in gribi tests.")

	GRIBIAFTChainCheckWait = flag.Int("gribi_aft_chain_check_wait_timer", 5,
		"Wait time before executing aft call for IPv4")

	GRIBIFIBCheck = flag.Bool("gribi_fib_check", true,
		"This enable/disable FIB ack check for gribi entries in gribi tests.")

	GRIBIRIBCheck = flag.Bool("gribi_rib_check", true,
		"This enable/disable  RIB ack check for gribi entries in gribi tests.")

	GRIBIScale = flag.Uint("gribi_scale", 2, //30208 scale value
		"The number of gribi entries to be added in scale test.")

	GRIBIConfidence = flag.Float64("gribi_confidence", 100.0,
		"This defines the how many gribi entries are gonna be validated, float value is represented in percentage")

	PYXRRun = flag.Bool("pyvxr_run", true,
		"This flag is set to true when tests is run using pyvxr. In tests we lower the traffic rate when the run is in pyvxr.")

	DefaultNetworkInstance = flag.String("default_vrf", "DEFAULT", "The name used for the default network instance for VRF.")

	NonDefaultNetworkInstance = flag.String("nondefault_vrf", "TE", "The name used for the nondefault network instance for VRF.")

	PbrInstance = flag.String("pbr_instance", "DEFAULT", "pbr network instance")

	P4RTOcNPU = flag.String("p4rt_oc_npu", "0/RP0/CPU0-NPU0", "P4RT device npu")

	FlowFps   = flag.Uint64("flow_fps", 100, "The traffic flow frame rate in Frames Per Second")
	FrameSize = pflag.Uint32("frame_size", 1024, "The traffic flow frame size")

	// flags to selectively run a set of P4RT PacketIO tests

	GDPTests         = flag.Bool("run_gdp_tests", false, "Run only GDP tests")
	LLDPTests        = flag.Bool("run_lldp_tests", false, "Run only LLDP tests")
	TTLTests         = flag.Bool("run_ttl_tests", false, "Run only TTL tests")
	TTL1v4           = flag.Bool("run_ttl_1_v4_tests", true, "Run only IPv4 TTL 1 tests") // default run TTL1 V4
	TTL1v6           = flag.Bool("run_ttl_1_v6_tests", false, "Run only IPv6 TTL 1 tests")
	TTL1n2v4         = flag.Bool("run_ttl_1n2_v4_tests", false, "Run only IPv4 TTL 1, 2 tests")
	TTL1n2v6         = flag.Bool("run_ttl_1n2_v6_tests", false, "Run only IPv6 TTL 1, 2 tests")
	TTL1v4n6         = flag.Bool("run_ttl_1_v4n6_tests", false, "Run IPv4 and IPv6 TTL 1 tests")
	TTL1n2v4n6       = flag.Bool("run_ttl_1n2_v4n6_tests", false, "Run IPv4 and IPv6 TTL 1, 2 tests")
	ScaleTests       = flag.Bool("skip_scale_tests", true, "Run only scale tests") // skip scale tests
	HATests          = flag.Bool("run_ha_tests", false, "Run only HA tests")
	ComplianceTests  = flag.Bool("run_compliance_tests", false, "Run only Compliance tests")
	PacketIOTests    = flag.Bool("run_packetio_tests", false, "Run only PacketIO tests")
	BaseConfigBundle = flag.Bool("run_with_bundle_baseconfig", false, "Run tests with bundle-ether base configuration instead of physical")

	PbrPrecommitTests = flag.Bool("skip_hw_module", true, "Run only hw-module") // Pbr

)

// GRIBICheck struct
type GRIBICheck struct {
	RIBACK        bool
	FIBACK        bool
	AFTCheck      bool
	AFTChainCheck bool
}

// GRIBIChecks variable
var GRIBIChecks *GRIBICheck

func init() {
	GRIBIChecks = &GRIBICheck{
		RIBACK:        *GRIBIFIBCheck,
		FIBACK:        *GRIBIFIBCheck,
		AFTCheck:      *GRIBIAFTCheck,
		AFTChainCheck: *GRIBIAFTChainCheck,
	}
}
