// Package verifier provides verifier functions for a given component or technology area.
package verifiers

var (
	loadbalancing *loadbalancingVerifier
	interfaces    *interfaceVerifier
	tgen          *tgenVerifier
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
