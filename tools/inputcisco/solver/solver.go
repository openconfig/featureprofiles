package solver

import (
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/tools/inputcisco/testinput"
	"github.com/openconfig/ondatra"
)

// Solver Resolves the port name referances for interfaces
func Solver(t *testing.T, dev *ondatra.DUTDevice, parameter string, variables ...map[string]string) string {
	if !strings.HasPrefix(parameter, "$") {
		return parameter
	}
	if strings.HasPrefix(strings.ToLower(parameter), "$ports.") {
		portID := strings.TrimPrefix(parameter, "$ports.")
		return dev.Port(t, portID).Name()
	}
	if strings.HasPrefix(strings.ToLower(parameter), "$params.") {
		param := strings.TrimPrefix(parameter, "$params.")
		for _, variable := range variables {
			if val, ok := variable[param]; ok {
				return val
			}

		}
		t.Logf("Unable to find parameter variable named %s ", parameter)
		return parameter
	}

	return parameter

}

// Solver Resolves the port name referances for interfaces for ATE
func SolveAte(t *testing.T, dev *ondatra.ATEDevice, parameter string, variables ...map[string]string) string {
	if !strings.HasPrefix(parameter, "$") {
		return parameter
	}
	if strings.HasPrefix(strings.ToLower(parameter), "$ports.") {
		portID := strings.TrimPrefix(parameter, "$ports.")
		return portID
	}
	if strings.HasPrefix(strings.ToLower(parameter), "$params.") {
		param := strings.TrimPrefix(parameter, "$params.")
		for _, variable := range variables {
			if val, ok := variable[param]; ok {
				return val
			}

		}
		t.Logf("Unable to find parameter variable named %s ", parameter)
		return parameter
	}

	return parameter

}

// Solver Resolves the group tag given for interfaces
func Solvetag(t *testing.T, parameter string, input testinput.TestInput, variables ...map[string]string) []string {
	if strings.HasPrefix(strings.ToLower(parameter), "$if-tag.") {
		param := strings.TrimPrefix(parameter, "$if-tag.")
		params := strings.Split(parameter, ".")
		if len(params) != 3 {
			return []string{param}
		}
		device := ondatra.DUT(t, params[0])
		ifgroup := input.Device(device).IFGroup(params[1])
		switch strings.ToLower(params[3]) {
		case "names", "name":
			return ifgroup.Names()

		}
		return []string{param}
	}
	return []string{parameter}
}
