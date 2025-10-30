// Package helper provides helper functions for a given component or technology area.
package helper

var (
	fib           *fibHelper
	loadbalancing *loadbalancingHelper
	interfaces    *interfaceHelper
	tgen          *tgenHelper
	sflow         *sflowHelper
	// rib     = &ribHelper{}
)

// SFlowHelper accessor for sflow helper functions.
func SflowHelper() *sflowHelper {
	if sflow == nil {
		sflow = &sflowHelper{}
	}
	return sflow
}

// FIBHelper accessor for fib helper functions.
func FIBHelper() *fibHelper {
	if fib == nil {
		fib = &fibHelper{}
	}
	return fib
}

// LoadbalancingHelper accessor for loadbalancing helper functions.
func LoadbalancingHelper() *loadbalancingHelper {
	if loadbalancing == nil {
		loadbalancing = &loadbalancingHelper{}
	}
	return loadbalancing
}

// InterfaceHelper accessor for interface helper functions.
func InterfaceHelper() *interfaceHelper {
	if interfaces == nil {
		interfaces = &interfaceHelper{}
	}
	return interfaces
}

// TGENHelper accessor for TGEN helper functions.
func TGENHelper() *tgenHelper {
	if tgen == nil {
		tgen = &tgenHelper{}
	}
	return tgen
}
