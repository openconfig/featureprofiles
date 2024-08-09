// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gnoi provides utilities for interacting with the gNOI API.
package gnoi

import (
	"context"
	"fmt"
	"testing"
	"time"

	iSystem "github.com/openconfig/featureprofiles/internal/system"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	gRIBIDaemons = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "Gribi",
		ondatra.CISCO:   "emsd",
		ondatra.JUNIPER: "rpd",
		ondatra.NOKIA:   "sr_grpc_server",
	}
	ocAgentDaemons = map[ondatra.Vendor]string{
		ondatra.ARISTA: "Octa",
	}
	p4rtDaemons = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "P4Runtime",
		ondatra.CISCO:   "emsd",
		ondatra.JUNIPER: "p4-switch",
		ondatra.NOKIA:   "sr_grpc_server",
	}
	routingDaemons = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "Bgp-main",
		ondatra.CISCO:   "emsd",
		ondatra.JUNIPER: "rpd",
		ondatra.NOKIA:   "sr_bgp_mgr",
	}
)

// Daemon is the type of the daemon on the device.
type Daemon string

const (
	// GRIBI is the gRIBI daemon.
	GRIBI Daemon = "GRIBI"
	// OCAGENT is the OpenConfig agent daemon.
	OCAGENT Daemon = "OCAGENT"
	// P4RT is the P4RT daemon.
	P4RT Daemon = "P4RT"
	// ROUTING is the routing daemon.
	ROUTING Daemon = "ROUTING"
)

// TerminateDaemon terminates the daemon on the DUT.
func TerminateDaemon(t *testing.T, dut *ondatra.DUTDevice, daemon Daemon, waitForRestart bool) error {
	t.Helper()

	daemonName, err := GetProcessName(dut, daemon)
	if err != nil {
		t.Fatalf("Daemon %s not defined for vendor %s", daemon, dut.Vendor().String())
	}
	pid, err := iSystem.FindProcessIDByName(t, dut, daemonName)
	if err != nil {
		t.Fatalf("Failed to find PID of process %v with unexpected err: %v", daemonName, err)
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	killProcessRequest := &spb.KillProcessRequest{
		Signal:  spb.KillProcessRequest_SIGNAL_KILL,
		Name:    daemonName,
		Pid:     uint32(pid),
		Restart: true,
	}
	gnoiClient.System().KillProcess(context.Background(), killProcessRequest)

	if waitForRestart {
		gnmi.WatchAll(
			t,
			dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
			gnmi.OC().System().ProcessAny().State(),
			time.Minute,
			func(p *ygnmi.Value[*oc.System_Process]) bool {
				val, ok := p.Val()
				if !ok {
					return false
				}
				return val.GetName() == daemonName && val.GetPid() != pid
			},
		)
	}
	return nil
}

// GetProcessName returns the name of the daemon on the DUT based on the vendor.
func GetProcessName(dut *ondatra.DUTDevice, daemon Daemon) (string, error) {
	var daemonName string
	var ok bool
	switch daemon {
	case GRIBI:
		daemonName, ok = gRIBIDaemons[dut.Vendor()]
	case OCAGENT:
		daemonName, ok = ocAgentDaemons[dut.Vendor()]
	case P4RT:
		daemonName, ok = p4rtDaemons[dut.Vendor()]
	case ROUTING:
		daemonName, ok = routingDaemons[dut.Vendor()]
	default:
		return "", fmt.Errorf("Unsupported daemon type: %v for vendor %s", daemon, dut.Vendor().String())
	}
	if !ok {
		return "", fmt.Errorf("Daemon %s not defined for vendor %s", daemon, dut.Vendor().String())
	}
	return daemonName, nil
}
