// Package verifiers offers APIs to verify operational data for components.
package verifiers

var (
	loadbalancing *loadbalancingVerifier
	interfaces    *interfaceVerifier
	tgen          *tgenVerifier
	fib           *fibVerifier
	// rib     = &ribverifier{}
)

// Loadbalancingverifier accessor for loadbalancing verifier functions.
func Loadbalancingverifier() *loadbalancingVerifier {
	if loadbalancing == nil {
		loadbalancing = &loadbalancingVerifier{}
	}
	return loadbalancing
}

// Interfaceverifier accessor for interface verifier functions.
func Interfaceverifier() *interfaceVerifier {
	if interfaces == nil {
		interfaces = &interfaceVerifier{}
	}
	return interfaces
}

// TGENverifier accessor for TGEN verifier functions.
func TGENverifier() *tgenVerifier {
	if tgen == nil {
		tgen = &tgenVerifier{}
	}
	return tgen
}

// FIBverifier accessor for CEF verifier functions.
func FIBverifier() *fibVerifier {
	if fib == nil {
		fib = &fibVerifier{}
	}
	return fib
}
