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

	"github.com/openconfig/featureprofiles/internal/system"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	daemonProcessNames = map[ondatra.Vendor]map[Daemon]string{
		ondatra.ARISTA: {
			GRIBI:   "Gribi",
			OCAGENT: "Octa",
			P4RT:    "P4Runtime",
			ROUTING: "Bgp-main",
		},
		ondatra.CISCO: {
			GRIBI:   "emsd",
			P4RT:    "emsd",
			ROUTING: "emsd",
		},
		ondatra.JUNIPER: {
			GRIBI:   "rpd",
			P4RT:    "p4-switch",
			ROUTING: "rpd",
		},
		ondatra.NOKIA: {
			GRIBI:   "sr_grpc_server",
			OCAGENT: "sr_oc_mgmt_serv",
			P4RT:    "sr_grpc_server",
			ROUTING: "sr_bgp_mgr",
		},
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

// signal type of termination request
const (
	SigTerm        = spb.KillProcessRequest_SIGNAL_TERM
	SigKill        = spb.KillProcessRequest_SIGNAL_KILL
	SigHup         = spb.KillProcessRequest_SIGNAL_HUP
	SigAbort       = spb.KillProcessRequest_SIGNAL_ABRT
	SigUnspecified = spb.KillProcessRequest_SIGNAL_UNSPECIFIED
)

// KillProcess terminates the daemon on the DUT.
func KillProcess(t *testing.T, dut *ondatra.DUTDevice, daemon Daemon, signal spb.KillProcessRequest_Signal, restart bool, waitForRestart bool) {
	t.Helper()

	daemonName, err := FetchProcessName(dut, daemon)
	if err != nil {
		t.Fatalf("Daemon %s not defined for vendor %s", daemon, dut.Vendor().String())
	}
	pid := system.FindProcessIDByName(t, dut, daemonName)
	if pid == 0 {
		t.Fatalf("process %s not found on device", daemonName)
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	killProcessRequest := &spb.KillProcessRequest{
		Signal:  signal,
		Name:    daemonName,
		Pid:     uint32(pid),
		Restart: restart,
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
}

// FetchProcessName returns the name of the daemon on the DUT based on the vendor.
func FetchProcessName(dut *ondatra.DUTDevice, daemon Daemon) (string, error) {
	daemons, ok := daemonProcessNames[dut.Vendor()]
	if !ok {
		return "", fmt.Errorf("unsupported vendor: %s", dut.Vendor().String())
	}
	d, ok := daemons[daemon]
	if !ok {
		return "", fmt.Errorf("daemon %s not defined for vendor %s", daemon, dut.Vendor().String())
	}
	return d, nil
}
