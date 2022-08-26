package flags

import "flag"

// cisco tests flags.
var (
	GRIBITrafficCheck = flag.Bool("gribi_traffic_check", true,
		"This enable/disable traffic check for gribi tests.")

	GRIBIAFTCheck = flag.Bool("gribi_aft_check", false,
		"This enable/disable AFT check for gribi entries in gribi tests.")

	GRIBIAFTChainCheck = flag.Bool("gribi_aft_chain_check", false,
		"This enable/disable AFT chain check for gribi prefix in gribi tests.")

	GRIBIFIBCheck = flag.Bool("gribi_fib_check", false,
		"This enable/disable FIB ack check for gribi entries in gribi tests.")

	GRIBIRIBCheck = flag.Bool("gribi_rib_check", true,
		"This enable/disable  RIB ack check for gribi entries in gribi tests.")

	GRIBIScale = flag.Uint("gribi_scale", 10,
		"The number of gribi entries to be added in scale test.")

	GRIBIConfidence = flag.Float64("gribi_confidence", 10.0,
		"This defines the how many gribi entries are gonna be validated, float value is represented in percentage")

	PYXRRun = flag.Bool("pyvxr_run", true,
		"This flag is set to true when tests is run using pyvxr. In tests we lower the traffic rate when the run is in pyvxr.")

	DefaultNetworkInstance = flag.String("default_vrf", "default", "The name used for the default network instance for VRF.")

	NonDefaultNetworkInstance = flag.String("nondefault_vrf", "TE", "The name used for the nondefault network instance for VRF.")

	PbrInstance = flag.String("vrf_name", "DEFAULT", "Vrf name under which policy needs to be configured")
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
