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

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// CreateGNMIServer creates a gNMI server on the DUT on a given network-instance.
func CreateGNMIServer(batch *gnmi.SetBatch, t testing.TB, d *ondatra.DUTDevice, ni string) {
	gnmiServerPath := gnmi.OC().System().GrpcServer(ni)
	gnmi.BatchUpdate(batch, gnmiServerPath.Config(), &oc.System_GrpcServer{
		Name:            ygot.String(ni),
		Port:            ygot.Uint16(9339),
		Enable:          ygot.Bool(true),
		NetworkInstance: ygot.String(ni),
		// Services:        []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI},
	})
}
