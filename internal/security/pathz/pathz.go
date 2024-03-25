// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package authz provides helper APIs to simplify writing authz test cases.
// It also packs authz rotate and get operations with the corresponding verifications to
// prevent code duplications and increase the test code readability.
package pathz

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	pathzpb "github.com/openconfig/gnsi/pathz"
	"github.com/openconfig/lemming/gnsi/acltrie"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Spiffe is an struct to save an Spiffe id and its svid.
type Spiffe struct {
	// ID store Spiffe id.
	ID string
	// TlsConf stores the svid of Spiffe id.
	TLSConf *tls.Config
}

// Delete Authz policy file.
func DeletePolicyData(t *testing.T, dut *ondatra.DUTDevice, file string) {
	cliHandle := dut.RawAPIs().CLI(t)
	resp, err := cliHandle.RunCommand(context.Background(), "run rm /mnt/rdsfs/ems/gnsi/"+file)
	if err != nil {
		t.Error(err)
	}
	t.Logf("delete pathz policy file  %v, %s", resp, file)
}

// findProcessByName uses telemetry to collect and return the process information. It return nill if the process is not found.
func findProcessByName(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string) *oc.System_Process {
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	for _, proc := range pList {
		if proc.GetName() == pName {
			t.Logf("Pid of daemon '%s' is '%d'", pName, proc.GetPid())
			return proc
		}
	}
	return nil
}

func KillEmsdProcess(t *testing.T, dut *ondatra.DUTDevice) {
	// Trigger Section
	pName := "emsd"
	ctx := context.Background()
	proc := findProcessByName(ctx, t, dut, pName)
	pid := uint32(proc.GetPid())
	gnoiClient := dut.RawAPIs().GNOI(t)
	killResponse, err := gnoiClient.System().KillProcess(context.Background(), &spb.KillProcessRequest{Name: pName, Pid: pid, Restart: true, Signal: spb.KillProcessRequest_SIGNAL_TERM})
	t.Logf("Got kill process response: %v\n\n", killResponse)
	if err != nil {
		t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
	}
	time.Sleep(30 * time.Second)
	newProc := findProcessByName(ctx, t, dut, pName)
	if newProc == nil {
		t.Logf("Retry to get the process emsd info after restart")
		time.Sleep(30 * time.Second)
		if newProc = findProcessByName(ctx, t, dut, "emsd"); newProc == nil {
			t.Fatalf("Failed to start process emsd after failure")
		}
	}
	if newProc.GetPid() == proc.GetPid() {
		t.Fatalf("The process id of %s is expected to be changed after the restart", pName)
	}
	if newProc.GetStartTime() <= proc.GetStartTime() {
		t.Fatalf("The start time of process emsd is expected to be larger than %d, got %d ", proc.GetStartTime(), newProc.GetStartTime())
	}
}

// Reload router
func ReloadRouter(t *testing.T, dut *ondatra.DUTDevice) {
	gnoiClient := dut.RawAPIs().GNOI(t)
	_, errs :=
		gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot chassis without delay",
			Force:   true,
		})

	if errs != nil {
		t.Fatalf("Reboot failed %v", errs)
	}
	startReboot := time.Now()
	const maxRebootTime = 30
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())

}

type policyData struct {
	trie      *acltrie.Trie
	rawPolicy *pathzpb.AuthorizationPolicy
	version   string
	createdOn uint64
}

// Server implements the pathz gRPC server.
type Server struct {
	pathzpb.UnimplementedPathzServer
	rotationInProgress atomic.Bool
	sandboxMu          sync.RWMutex
	sandbox            *policyData
	activeMu           sync.RWMutex
	active             *policyData
}

// Rotate implements the pathz Rotate RPC.
func (s *Server) Rotate(rs pathzpb.Pathz_RotateServer) error {
	if s.rotationInProgress.Load() {
		return status.Error(codes.Unavailable, "another rotation is already in progress")
	}
	s.rotationInProgress.Store(true)
	defer s.rotationInProgress.Store(false)

	receivedUploadReq := false
	for {
		resp, err := rs.Recv()
		if err != nil {
			return err
		}
		switch req := resp.RotateRequest.(type) {
		case *pathzpb.RotateRequest_UploadRequest:
			if receivedUploadReq {
				return status.Error(codes.FailedPrecondition, "only a single upload request can be sent per Rotate RPC")
			}
			receivedUploadReq = true

			t, err := acltrie.FromPolicy(req.UploadRequest.GetPolicy())
			if err != nil {
				return status.Errorf(codes.InvalidArgument, "invalid policy: %v", err)
			}
			s.sandboxMu.Lock()
			s.sandbox = &policyData{
				trie:      t,
				version:   req.UploadRequest.GetVersion(),
				rawPolicy: req.UploadRequest.GetPolicy(),
				createdOn: req.UploadRequest.GetCreatedOn(),
			}
			s.sandboxMu.Unlock()
			if err := rs.Send(&pathzpb.RotateResponse{}); err != nil {
				return err
			}
		case *pathzpb.RotateRequest_FinalizeRotation:
			if !receivedUploadReq {
				return status.Error(codes.FailedPrecondition, "finalize rotation called before upload request")
			}
			s.activeMu.Lock()
			s.sandboxMu.Lock()
			s.active = s.sandbox
			s.sandbox = nil
			s.sandboxMu.Unlock()
			s.activeMu.Unlock()
			if err := rs.Send(&pathzpb.RotateResponse{}); err != nil {
				return err
			}
			return nil
		}
	}
}

func (s *Server) getPolicyWithRLock(i pathzpb.PolicyInstance) (*policyData, *sync.RWMutex, error) {
	switch i {
	case pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX:
		s.sandboxMu.RLock()
		return s.sandbox, &s.sandboxMu, nil
	case pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE:
		s.activeMu.RLock()
		return s.active, &s.activeMu, nil
	default:
		return nil, nil, fmt.Errorf("unknown instance type: %v", i)
	}
}

// CheckPermit implements the gNMI path auth interface, by using Probe.
func (s *Server) CheckPermit(path *gpb.Path, user string, write bool) bool {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()

	if s.active == nil {
		return false
	}
	mode := pathzpb.Mode_MODE_READ
	if write {
		mode = pathzpb.Mode_MODE_WRITE
	}
	act := s.active.trie.Probe(path, user, mode)
	return act == pathzpb.Action_ACTION_PERMIT
}

// IsInitialized implements the gNMI path auth interface, by checking the active policy exists.
func (s *Server) IsInitialized() bool {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return s.active != nil
}

// Probe implements the pathz Probe RPC.
func (s *Server) Probe(_ context.Context, req *pathzpb.ProbeRequest) (*pathzpb.ProbeResponse, error) {
	if req.GetMode() == pathzpb.Mode_MODE_UNSPECIFIED {
		return nil, status.Errorf(codes.InvalidArgument, "mode not specified")
	}
	if req.GetUser() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "user not specified")
	}
	if req.GetPath() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "path not specified")
	}
	policy, mu, err := s.getPolicyWithRLock(req.GetPolicyInstance())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	defer mu.RUnlock()
	if policy == nil {
		return nil, status.Error(codes.Aborted, "requested policy instance is nil")
	}

	act := policy.trie.Probe(req.GetPath(), req.GetUser(), req.GetMode())
	return &pathzpb.ProbeResponse{
		Action:  act,
		Version: policy.version,
	}, nil
}

// Probe implements the pathz Get RPC.
func (s *Server) Get(_ context.Context, req *pathzpb.GetRequest) (*pathzpb.GetResponse, error) {
	policy, mu, err := s.getPolicyWithRLock(req.GetPolicyInstance())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	defer mu.RUnlock()

	if policy == nil {
		return nil, status.Error(codes.Aborted, "requested policy instance is nil")
	}
	return &pathzpb.GetResponse{
		Policy:    policy.rawPolicy,
		CreatedOn: policy.createdOn,
		Version:   policy.version,
	}, nil
}

func ConfigAndVerifyISIS(t testing.TB, d *ondatra.DUTDevice, i string, ni string, si uint32) {
	t.Helper()
	if ni == "" {
		t.Fatalf("Network instance not provided for interface assignment")
	}
	netInst := &oc.NetworkInstance{Name: ygot.String(ni)}
	intf := &oc.Interface{Name: ygot.String(i)}
	netInstIntf, err := netInst.NewInterface(intf.GetName())
	if err != nil {
		t.Errorf("Error fetching NewInterface for %s", intf.GetName())
	}
	netInstIntf.Interface = ygot.String(intf.GetName())
	netInstIntf.Subinterface = ygot.Uint32(si)
	netInstIntf.Id = ygot.String(intf.GetName() + "." + fmt.Sprint(si))
	if intf.GetOrCreateSubinterface(si) != nil {
		gnmi.Update(t, d, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}
