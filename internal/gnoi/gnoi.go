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
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/system"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	daemonProcessNames = map[ondatra.Vendor]map[Daemon]string{
		ondatra.ARISTA: {
			GRIBI:   "Gribi",
			OCAGENT: "Octa",
			P4RT:    "P4Runtime",
			ROUTING: "Bgp-main",
			ISIS:    "Isis",
			GNPSI:   "Gnpsi",
		},
		ondatra.CISCO: {
			GRIBI:   "emsd",
			OCAGENT: "emsd",
			P4RT:    "emsd",
			ROUTING: "emsd",
		},
		ondatra.JUNIPER: {
			GRIBI:   "rpd",
			OCAGENT: "mgd-api",
			P4RT:    "p4-switch",
			ROUTING: "rpd",
		},
		ondatra.NOKIA: {
			GRIBI:   "sr_grpc_server",
			OCAGENT: "sr_oc_mgmt_server",
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
	// ISIS is the Isis daemon
	ISIS Daemon = "ISIS"
	// GNPSI is the gNPSI daemon
	GNPSI Daemon = "GNPSI"
)

// signal type of termination request
const (
	SigTerm        = spb.KillProcessRequest_SIGNAL_TERM
	SigKill        = spb.KillProcessRequest_SIGNAL_KILL
	SigHup         = spb.KillProcessRequest_SIGNAL_HUP
	SigAbort       = spb.KillProcessRequest_SIGNAL_ABRT
	SigUnspecified = spb.KillProcessRequest_SIGNAL_UNSPECIFIED
)

// KillProcess terminates the daemon on the DUT. If waitForRestart is true, it
// waits up to a minute for gNMI to report the daemon with a new, nonzero PID,
// retrying temporary transport failures while the management server recovers.
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
	var processClient *ygnmi.Client
	if waitForRestart {
		processClient, err = ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
		if err != nil {
			t.Fatalf("Could not create process telemetry client: %v", err)
		}
	}
	killProcessRequest := &spb.KillProcessRequest{
		Signal:  signal,
		Name:    daemonName,
		Pid:     uint32(pid),
		Restart: restart,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	start := time.Now()
	newPID, err := killProcess(ctx, gnoiClient.System(), killProcessRequest, waitForRestart, processClient)
	if err != nil {
		t.Fatalf("Failed to kill or restart process %s: %v", daemonName, err)
	}
	if waitForRestart {
		t.Logf("Process %s restarted from PID %d to %d in %v", daemonName, pid, newPID, time.Since(start))
	}
}

// killProcess only retries a rejected TERM with KILL. A transport error can mean
// that the server terminated itself before replying; verify recovery instead of
// sending another kill that could terminate the newly restarted process.
func killProcess(ctx context.Context, client spb.SystemClient, request *spb.KillProcessRequest, waitForRestart bool, processClient *ygnmi.Client) (uint64, error) {
	_, killErr := client.KillProcess(ctx, request)
	if request.GetSignal() == SigTerm && (status.Code(killErr) == codes.Unimplemented || status.Code(killErr) == codes.InvalidArgument) {
		fallback := proto.Clone(request).(*spb.KillProcessRequest)
		fallback.Signal = SigKill
		_, killErr = client.KillProcess(ctx, fallback)
	}
	if killErr != nil && !(request.GetRestart() && waitForRestart && transientProcessError(killErr)) {
		return 0, fmt.Errorf("KillProcess: %w", killErr)
	}
	if !waitForRestart {
		return 0, nil
	}
	pid, err := waitForProcessRestart(ctx, processClient, request.GetName(), uint64(request.GetPid()))
	if err != nil && killErr != nil {
		return 0, fmt.Errorf("KillProcess returned %v; %w", killErr, err)
	}
	return pid, err
}

// waitForProcessRestart uses fresh ONCE subscriptions so a connection reset does
// not abort the caller's test. Each request and the retry interval are bounded by
// ctx; an unresponsive server cannot consume the entire recovery window in one RPC.
func waitForProcessRestart(ctx context.Context, client *ygnmi.Client, name string, oldPID uint64) (uint64, error) {
	last := "no process telemetry received"
	for {
		if err := ctx.Err(); err != nil {
			return 0, fmt.Errorf("waiting for process %s to restart from PID %d: %w (last observation: %s)", name, oldPID, err, last)
		}
		queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		processes, err := ygnmi.LookupAll(queryCtx, client, gnmi.OC().System().ProcessAny().State())
		cancel()
		if err != nil {
			if !transientProcessError(err) {
				return 0, fmt.Errorf("reading process %s telemetry: %w", name, err)
			}
			last = err.Error()
		} else {
			last = fmt.Sprintf("process %s not present", name)
			for _, p := range processes {
				val, present := p.Val()
				if !present || val.GetName() != name {
					continue
				}
				if pid := val.GetPid(); pid != 0 && pid != oldPID {
					return pid, nil
				}
				last = fmt.Sprintf("process %s still has PID %d", name, val.GetPid())
			}
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func transientProcessError(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.Unavailable || status.Code(err) == codes.DeadlineExceeded
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

// RestartRoutingProcess restarts the routing daemon on the DUT via gNOI and waits for it to restart.
// If the device does not support restarting the routing process via gNOI, the test is skipped.
func RestartRoutingProcess(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if deviations.RoutingRestartViaGnoiUnsupported(dut) {
		t.Skip("Skipping routing restart via gNOI due to deviation")
	}
	KillProcess(t, dut, ROUTING, SigTerm, true, true)
}
