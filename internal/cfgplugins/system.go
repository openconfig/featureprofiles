// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cfgplugins is a collection of OpenConfig configuration libraries for system and network instance.
//
// Each plugin function has parameters for a gnmi Batch, values to use in the configuration
// and an ondatra.DUTDevice.  Each function returns OpenConfig values.
//
// The configuration function will modify the batch which is passed in by reference. The caller may
// pass nil as the configuration values to use in which case the function will provide a set of
// default values.  The ondatra.DUTDevice is used to determine any configuration deviations which
// may be necessary.
//
// The caller may choose to use the returned OC value to customize the values set by this
// function or for use in a non-batch use case.
package cfgplugins

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// CreateGNMIServer creates a gNMI server on the DUT on a given network-instance.
func CreateGNMIServer(t testing.TB, d *ondatra.DUTDevice, batch *gnmi.SetBatch, nip *NetworkInstanceParams) {
	var niName string
	var gnmiServerName string

	if nip.Default {
		// It is expected that network-instance name and gRPC server name are "DEFAULT" for default network instance.
		niName = nip.Name
		gnmiServerName = nip.Name
		// If not aligned with OC, then use deviation flags to set network-instance name and gRPC server name.
		// This validation to be removed when the deviation flags are removed.
		if deviations.DefaultNetworkInstance(d) != "" {
			niName = deviations.DefaultNetworkInstance(d)
		}
		if deviations.DefaultNiGnmiServerName(d) != "" {
			gnmiServerName = deviations.DefaultNiGnmiServerName(d)
		}
	} else {
		niName, gnmiServerName = nip.Name, "gnxi-"+nip.Name
	}
	t.Logf("Creating gNMI server %s on network instance: %s", gnmiServerName, niName)
	gnmiServerPath := gnmi.OC().System().GrpcServer(gnmiServerName)
	gnmiServer := &oc.System_GrpcServer{
		Name:            ygot.String(gnmiServerName),
		Port:            ygot.Uint16(9339),
		Enable:          ygot.Bool(true),
		NetworkInstance: ygot.String(niName),
		Services:        []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
	}
	gnmi.BatchUpdate(batch, gnmiServerPath.Config(), gnmiServer)
}

// FindLineCardParent traverses up the component hierarchy starting from the given component name
// to find the nearest ancestor component of type LINECARD. It returns the name of the LINECARD
// component if found, or an error otherwise.
func FindLineCardParent(t *testing.T, dut *ondatra.DUTDevice, startComponentName string) string {
	t.Helper()

	currentComponentName := startComponentName
	depth := 0
	maxDepth := 10
	for {
		if depth >= maxDepth {
			t.Fatalf("Exceeded maximum search depth while searching for LINECARD parent for starting component %s.", startComponentName)
		}
		componentTypePath := gnmi.OC().Component(currentComponentName).Type().State()
		currentType, ok := gnmi.Lookup(t, dut, componentTypePath).Val()
		if !ok {
			t.Logf("Component %s not found or missing state data. Continuing to search for parent", currentComponentName)
		}

		t.Logf("Component %s has type: %v", currentComponentName, currentType)
		if currentType == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD {
			t.Logf("Successfully traced to parent line card: %s", currentComponentName)
			return currentComponentName
		}

		parentPath := gnmi.OC().Component(currentComponentName).Parent().State()
		parentName, ok := gnmi.Lookup(t, dut, parentPath).Val()

		if !ok || parentName == "" {
			t.Fatalf("Failed to find a parent component of type LINECARD for starting component %s.", startComponentName)
		}
		currentComponentName = parentName
		depth++
	}
}
