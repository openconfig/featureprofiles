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

// Package authz_test performs functional tests for authz service
// TODO: Streaming Validation will be done in the subsequent PR
package authz_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/authz"
	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	"github.com/openconfig/featureprofiles/internal/security/svid"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	gnps "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type UsersMap map[string]authz.Spiffe

var (
	testInfraID = flag.String("test_infra_id", "cafyauto", "SPIFFE-ID used by test Infra ID user for authz operation")
	// reuse the pem files form authz test
	caCertPem = flag.String("ca_cert_pem", "../tests/authz/testdata/ca.cert.pem", "a pem file for ca cert that will be used to generate svid")
	caKeyPem  = flag.String("ca_key_pem", "../tests/authz/testdata/ca.key.pem", "a pem file for ca key that will be used to generate svid")
	policyMap map[string]authz.AuthorizationPolicy

	usersMap = UsersMap{
		"cert_user_admin": {
			ID: "spiffe://test-abc.foo.bar/xyz/admin",
		},
		"cert_deny_all": {
			ID: "spiffe://test-abc.foo.bar/xyz/deny-all", // a user with valid svid but no permission (has a deny rule for target *)
		},
		"cert_gribi_modify": {
			ID: "spiffe://test-abc.foo.bar/xyz/gribi-modify",
		},
		"cert_gnmi_set": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnmi-set",
		},
		"cert_gnoi_time": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnoi-time",
		},
		"cert_gnoi_ping": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnoi-ping",
		},
		"cert_gnsi_probe": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnsi-probe",
		},
		"cert_read_only": {
			ID: "spiffe://test-abc.foo.bar/xyz/read-only",
		},
	}
)

type access struct {
	allowed []*gnxi.RPC
	denied  []*gnxi.RPC
}

type authorizationTable map[string]access

var authTable = authorizationTable{
	//table: map[string]access{
	"cert_user_admin": struct {
		allowed []*gnxi.RPC
		denied  []*gnxi.RPC
	}{
		allowed: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnoiSystemTime, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe, gnxi.RPCs.GnmiSet},
	},
	"cert_deny_all": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnoiSystemTime, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe},
	},
	"cert_gribi_modify": {
		denied: []*gnxi.RPC{gnxi.RPCs.GnmiSet, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnoiSystemTime, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe},
		allowed: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify},
	},
	"cert_gnmi_set": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify,
			gnxi.RPCs.GnoiSystemTime, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnmiGet, gnxi.RPCs.GnmiSet},
	},
	"cert_gnoi_time": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet,
			gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnoiSystemTime},
	},
	"cert_gnoi_ping": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet,
			gnxi.RPCs.GnsiAuthzRotate, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe, gnxi.RPCs.GnoiSystemTime},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnoiSystemPing},
	},
	"cert_gnsi_probe": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet,
			gnxi.RPCs.GnsiAuthzRotate, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnoiSystemTime},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnsiAuthzProbe},
	},
	"cert_read_only": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet,
			gnxi.RPCs.GnsiAuthzRotate, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnoiSystemTime,
			gnxi.RPCs.GnsiAuthzProbe},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GribiGet, gnxi.RPCs.GnmiGet},
	},
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setUpBaseline(t *testing.T, dut *ondatra.DUTDevice) {
	// reuse the policy from authz test
	policyMap = authz.LoadPolicyFromJSONFile(t, "../tests/authz/testdata/policy.json")

	caKey, trustBundle, err := svid.LoadKeyPair(*caKeyPem, *caCertPem)
	if err != nil {
		t.Fatalf("Could not load ca key/cert: %v", err)
	}
	caCertBytes, err := os.ReadFile(*caCertPem)
	if err != nil {
		t.Fatalf("Could not load the ca cert: %v", err)
	}
	trusBundle := x509.NewCertPool()
	if !trusBundle.AppendCertsFromPEM(caCertBytes) {
		t.Fatalf("Could not create the trust bundle: %v", err)
	}
	for user, v := range usersMap {
		userSvid, err := svid.GenSVID("", v.ID, 300, trustBundle, caKey, x509.RSA)
		if err != nil {
			t.Fatalf("Could not generate svid for user %s: %v", user, err)
		}
		tlsConf := tls.Config{
			Certificates: []tls.Certificate{*userSvid},
			RootCAs:      trusBundle,
		}
		usersMap[user] = authz.Spiffe{
			ID:      v.ID,
			TLSConf: &tlsConf,
		}
	}
}

// verifyAuthTable takes an authorization Table and verify the expected rpc/access
func verifyAuthTable(t *testing.T, dut *ondatra.DUTDevice, authTable authorizationTable) {
	for certName, access := range authTable {
		t.Run(fmt.Sprintf("Validating access for user %s", certName), func(t *testing.T) {
			for _, allowedRPC := range access.allowed {
				authz.Verify(t, dut, getSpiffe(t, dut, certName), allowedRPC, &authz.HardVerify{})
			}
			for _, deniedRPC := range access.denied {
				authz.Verify(t, dut, getSpiffe(t, dut, certName), deniedRPC, &authz.ExceptDeny{}, &authz.HardVerify{})
			}
		})
	}
}

func getSpiffe(t *testing.T, dut *ondatra.DUTDevice, certName string) *authz.Spiffe {
	spiffe, ok := usersMap[certName]
	if !ok {
		t.Fatalf("Could not find Spiffe ID for user %s", certName)
	}
	return &spiffe
}

func TestHARedundancySwithOver(t *testing.T) {
	// RP SwitchOver Reload Test Case with Pre and Post Trigger Policy Verification

	// Pre-Test Section
	dut := ondatra.DUT(t, "dut")
	setUpBaseline(t, dut)
	_, policyBefore := authz.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Reboot Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
	defer policyBefore.Rotate(t, dut, uint64(time.Now().Unix()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

	// Trigger Section
	// Perform RP Switchover
	utils.Dorpfo(context.Background(), t, true)

	// Verification Section
	_, policyAfter := authz.Get(t, dut)
	t.Logf("Authz Policy of the device %s after the Trigger is %s", dut.Name(), policyAfter.PrettyPrint(t))
	if !cmp.Equal(policyBefore, policyAfter) {
		t.Fatalf("Not Expecting Policy Mismatch before and after the Trigger):\n%s", cmp.Diff(policyBefore, policyAfter))
	}
	// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
	newpolicy, ok := policyMap["policy-normal-1"]
	if !ok {
		t.Fatal("Policy policy-normal-1 is not loaded from policy json file")
	}
	newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
	// Rotate the policy.
	newpolicy.Rotate(t, dut, uint64(100), "policy-normal-1_v1", false)

	// Verify all results match per the above table for policy policy-normal-1
	verifyAuthTable(t, dut, authTable)
}

func TestHAEMSDProcessKill(t *testing.T) {
	// Process Restart EMSD Test Case with Pre and Post Trigger Policy Verification

	// Pre-Trigger Section
	dut := ondatra.DUT(t, "dut")
	setUpBaseline(t, dut)
	_, policyBefore := authz.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Reboot Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
	defer policyBefore.Rotate(t, dut, uint64(time.Now().Unix()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

	// Trigger Section
	pName := "emsd"
	ctx := context.Background()
	proc := findProcessByName(ctx, t, dut, pName)
	pid := uint32(proc.GetPid())
	killResponse, err := dut.RawAPIs().GNOI(t).System().KillProcess(context.Background(), &gnps.KillProcessRequest{Name: pName, Pid: pid, Restart: true, Signal: gnps.KillProcessRequest_SIGNAL_TERM})
	t.Logf("Got kill process response: %v\n\n", killResponse)
	if err != nil {
		t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
	}

	newProc := checkProcess(ctx, t, dut, pName)

	// time.Sleep(30 * time.Second)
	// newProc := findProcessByName(ctx, t, dut, pName)
	// if newProc == nil {
	// 	t.Logf("Retry to get the process emsd info after restart")
	// 	time.Sleep(30 * time.Second)
	// 	if newProc = findProcessByName(ctx, t, dut, pName); newProc == nil {
	// 		t.Fatalf("Failed to start process emsd after failure")
	// 	}
	// }
	// newProc := findProcessByName(ctx, t, dut, pName)
	if newProc.GetPid() == proc.GetPid() {
		t.Fatalf("The process id of %s is expected to be changed after the restart", pName)
	}
	if newProc.GetStartTime() <= proc.GetStartTime() {
		t.Fatalf("The start time of process emsd is expected to be larger than %d, got %d ", proc.GetStartTime(), newProc.GetStartTime())
	}

	// Verification Section
	_, policyAfter := authz.Get(t, dut)
	t.Logf("Authz Policy of the device %s after the Trigger is %s", dut.Name(), policyAfter.PrettyPrint(t))
	if !cmp.Equal(policyBefore, policyAfter) {
		t.Fatalf("Not Expecting Policy Mismatch before and after the Trigger):\n%s", cmp.Diff(policyBefore, policyAfter))
	}
	// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
	newpolicy, ok := policyMap["policy-normal-1"]
	if !ok {
		t.Fatal("Policy policy-normal-1 is not loaded from policy json file")
	}
	newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})

	createdOn := uint64(100)
	version := "policy-normal-1_v1"
	policyExpected := authz.GrpcAuthzPolicy{
		GrpcAuthzPolicyCreatedOn: createdOn * 1e9,
		GrpcAuthzPolicyVersion:   version,
	}
	watchersampleEdtSystemAaaAuthz := edtSystemAaaAuthz(t, dut, gpb.SubscriptionMode_SAMPLE, policyExpected)
	watcherOnChangeEdtSystemAaaAuthz := edtSystemAaaAuthz(t, dut, gpb.SubscriptionMode_ON_CHANGE, policyExpected)
	watcherTaargetDefinedEdtSystemAaaAuthz := edtSystemAaaAuthz(t, dut, gpb.SubscriptionMode_TARGET_DEFINED, policyExpected)
	// Rotate the policy.
	newpolicy.Rotate(t, dut, createdOn, version, false)

	// Verification EDT Section
	t.Run("watchersampleEdtSystemAaaAuthz", func(t *testing.T) {
		t.Parallel()
		if _, present := watchersampleEdtSystemAaaAuthz.Await(t); !present {
			t.Errorf("EDT data was not sent sample SystemAaaAuthz  %v", policyExpected)
		}
	})
	t.Run("watcherOnChangeEdtSystemAaaAuthz", func(t *testing.T) {
		t.Parallel()
		if _, present := watcherOnChangeEdtSystemAaaAuthz.Await(t); !present {
			t.Errorf("EDT data was not sent on-change SystemAaaAuthz  %v", policyExpected)
		}
	})
	t.Run("watcherTaargetDefinedEdtSystemAaaAuthz", func(t *testing.T) {
		t.Parallel()
		if _, present := watcherTaargetDefinedEdtSystemAaaAuthz.Await(t); !present {
			t.Errorf("EDT data was not sent target-defined SystemAaaAuthz  %v", policyExpected)
		}
	})
	// Verify all results match per the above table for policy policy-normal-1
	verifyAuthTable(t, dut, authTable)
}

func rpcExecute(dut *ondatra.DUTDevice, spiffe *authz.Spiffe, rpc *gnxi.RPC) {
	// time.Sleep(time.Second * 10)
	ctx := context.Background()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(spiffe.TLSConf))}
	rpc.Exec(ctx, dut, opts)
}

func TestAuthzPolicyCounterAllNodes(t *testing.T) {
	// Pre-Trigger Section
	dut := ondatra.DUT(t, "dut")
	setUpBaseline(t, dut)
	_, policyBefore := authz.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Reboot Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))

	setGrpcServername(t, dut, "TEST")
	grpcServerName := getGrpcServername(t, dut)
	rpc := gnxi.RPCs.GnmiSet
	spiffe := getSpiffe(t, dut, "cert_user_admin")

	counterBefore := authz.GrpcCounter{}
	if !strings.Contains(spiffe.ID, "deny-all") {
		err := counterBefore.FetchUpdate(t, dut, grpcServerName, rpc.Path)
		t.Logf("counterBefore: %v", counterBefore)

		if err != nil {
			t.Fatalf("Failed to fetch update: %v", err)
		}
	}

	watcherSampleAuthzPolicyCountersRpc := edtWatchSampleAuthzPolicyCountersRpc(t, dut, grpcServerName, rpc.Path, counterBefore)
	watcherSampleAuthzPolicyCounters := edtWatchSampleAuthzPolicyCounters(t, dut, grpcServerName, rpc.Path, counterBefore)
	watcherSampleGrpcServer := edtWatchSampleGrpcServer(t, dut, grpcServerName, rpc.Path, counterBefore)
	WatcherSampleSystem := edtWatchSampleSystem(t, dut, grpcServerName, rpc.Path, counterBefore)

	// Trigger Section
	// Execute the RPC
	go rpcExecute(dut, spiffe, rpc)

	// Verification Section
	t.Run("watcherSampleAuthzPolicyCountersRpc", func(t *testing.T) {
		t.Parallel()
		if _, present := watcherSampleAuthzPolicyCountersRpc.Await(t); !present {
			t.Errorf("EDT data was not sent for RPC %s", rpc.Path)
		}
	})
	t.Run("watcherSampleAuthzPolicyCounters", func(t *testing.T) {
		t.Parallel()
		if _, present := watcherSampleAuthzPolicyCounters.Await(t); !present {
			t.Errorf("EDT data was not sent for RPC %s", rpc.Path)
		}
	})
	t.Run("watcherSampleGrpcServer", func(t *testing.T) {
		t.Parallel()
		if _, present := watcherSampleGrpcServer.Await(t); !present {
			t.Errorf("EDT data was not sent for RPC %s", rpc.Path)
		}
	})

	t.Run("watcherSampleSystem", func(t *testing.T) {
		t.Parallel()
		if _, present := WatcherSampleSystem.Await(t); !present {
			t.Errorf("EDT data was not sent for RPC %s", rpc.Path)
		}
	})

}

func checkProcess(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string) *oc.System_Process {
	const (
		pollInterval  = 30 * time.Second
		totalDuration = 120 * time.Second
	)

	endTime := time.Now().Add(totalDuration)
	for time.Now().Before(endTime) {
		newProc := findProcessByName(ctx, t, dut, pName)
		if newProc != nil {
			t.Logf("Process %s is up and running", pName)
			return newProc
		}
		t.Logf("Process %s not found, retrying in 30 seconds...", pName)
		time.Sleep(pollInterval)
	}

	t.Fatalf("Failed to start process %s after polling for %v", pName, totalDuration)
	return nil // This line is technically unreachable due to t.Fatalf
}

// findProcessByName uses telemetry to collect and return the process information. It return nill if the process is not found.
func findProcessByName(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string) *oc.System_Process {
	startPollTime := time.Now()
	const maxProcessRestartTime = 10
	for time.Since(startPollTime) < maxProcessRestartTime*time.Minute {
		processStatePath := ocpath.Root().System().ProcessAny().State()
		gnmic := dut.RawAPIs().GNMI(t)
		yc, err := ygnmi.NewClient(gnmic)
		if err != nil {
			t.Logf("Could not create ygnmi.Client: %v", err)
			continue
		}
		values, err := ygnmi.LookupAll(ctx, yc, processStatePath)
		if err == nil && len(values) > 0 {
			break
		}
		t.Logf("Process list is empty, retrying in 30 seconds...")
		time.Sleep(30 * time.Second)
	}
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	for _, proc := range pList {
		if proc.GetName() == pName {
			t.Logf("Pid of daemon '%s' is '%d'", pName, proc.GetPid())
			return proc
		}
	}
	return nil

}

func setGrpcServername(t *testing.T, dut *ondatra.DUTDevice, grpcServerName string) {
	path := gnmi.OC().System().GrpcServer(grpcServerName).Name()
	gnmi.Update(t, dut, path.Config(), grpcServerName)
}

func getGrpcServername(t *testing.T, dut *ondatra.DUTDevice) string {

	data := gnmi.LookupAll(t, dut, gnmi.OC().System().GrpcServerAny().State())
	sysGrpcCont, pres := data[0].Val()
	if !pres {
		t.Fatalf("Got nil system grpc server data")
	}
	return sysGrpcCont.GetName()
}

func edtSystemAaaAuthz(t *testing.T, dut *ondatra.DUTDevice, subscribeMode gpb.SubscriptionMode, policyExpected authz.GrpcAuthzPolicy) *gnmi.Watcher[*oc.System_Aaa_Authorization] {
	t.Helper()
	path := gnmi.OC().System().Aaa().Authorization().State()
	t.Logf("TRY: subscribe ON_CHANGE to %s", path)
	count := 0
	edtWatch := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(subscribeMode), ygnmi.WithSampleInterval(10*time.Second)),
		path,
		time.Second*30,
		func(val *ygnmi.Value[*oc.System_Aaa_Authorization]) bool {
			data, present := val.Val()
			// subscription sample case
			if subscribeMode == gpb.SubscriptionMode_SAMPLE {
				if present && data.GetGrpcAuthzPolicyCreatedOn() == policyExpected.GrpcAuthzPolicyCreatedOn && data.GetGrpcAuthzPolicyVersion() == policyExpected.GrpcAuthzPolicyVersion {
					count += 1
				}
				return count >= 2
			}
			// subscription on-change case / target-defined
			return present && data.GetGrpcAuthzPolicyCreatedOn() == policyExpected.GrpcAuthzPolicyCreatedOn && data.GetGrpcAuthzPolicyVersion() == policyExpected.GrpcAuthzPolicyVersion
		})
	return edtWatch
}

func edtWatchSampleAuthzPolicyCountersRpc(t *testing.T, dut *ondatra.DUTDevice, grpcServerName, rpcPath string, counterBefore authz.GrpcCounter) *gnmi.Watcher[*oc.System_GrpcServer_AuthzPolicyCounters_Rpc] {
	t.Helper()
	path := gnmi.OC().System().GrpcServer(grpcServerName).AuthzPolicyCounters().Rpc(rpcPath).State()
	t.Logf("TRY: subscribe ON_CHANGE to %s", path)
	count := 0
	edtWatch := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(10*time.Second)),
		path,
		time.Second*30,
		func(val *ygnmi.Value[*oc.System_GrpcServer_AuthzPolicyCounters_Rpc]) bool {
			data, present := val.Val()
			if present && data.GetAccessAccepts() == counterBefore.AccessAccepts+1 && data.GetAccessRejects() == counterBefore.AccessRejects {
				count += 1
			}
			return count >= 2
		})
	return edtWatch
}

func edtWatchSampleAuthzPolicyCounters(t *testing.T, dut *ondatra.DUTDevice, grpcServerName, rpcPath string, counterBefore authz.GrpcCounter) *gnmi.Watcher[*oc.System_GrpcServer_AuthzPolicyCounters] {
	t.Helper()
	path := gnmi.OC().System().GrpcServer(grpcServerName).AuthzPolicyCounters().State()
	t.Logf("TRY: subscribe ON_CHANGE to %s", path)
	count := 0
	edtWatch := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(10*time.Second)),
		path,
		time.Second*30,
		func(val *ygnmi.Value[*oc.System_GrpcServer_AuthzPolicyCounters]) bool {
			data1, present := val.Val()
			data := data1.GetRpc(rpcPath)
			if present && data.GetAccessAccepts() == counterBefore.AccessAccepts+1 && data.GetAccessRejects() == counterBefore.AccessRejects {
				count += 1
			}
			return count >= 2
		})
	return edtWatch
}

func edtWatchSampleGrpcServer(t *testing.T, dut *ondatra.DUTDevice, grpcServerName, rpcPath string, counterBefore authz.GrpcCounter) *gnmi.Watcher[*oc.System_GrpcServer] {
	t.Helper()
	path := gnmi.OC().System().GrpcServer(grpcServerName).State()
	t.Logf("TRY: subscribe ON_CHANGE to %s", path)
	count := 0
	edtWatch := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(10*time.Second)),
		path,
		time.Second*30,
		func(val *ygnmi.Value[*oc.System_GrpcServer]) bool {
			data1, present := val.Val()
			data := data1.GetAuthzPolicyCounters().GetRpc(rpcPath)
			if present && data.GetAccessAccepts() == counterBefore.AccessAccepts+1 && data.GetAccessRejects() == counterBefore.AccessRejects {
				count += 1
			}
			return count >= 2
		})
	return edtWatch
}

func edtWatchSampleSystem(t *testing.T, dut *ondatra.DUTDevice, grpcServerName, rpcPath string, counterBefore authz.GrpcCounter) *gnmi.Watcher[*oc.System] {
	t.Helper()
	path := gnmi.OC().System().State()
	t.Logf("TRY: subscribe ON_CHANGE to %s", path)
	count := 0
	edtWatch := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(10*time.Second)),
		path,
		time.Second*30,
		func(val *ygnmi.Value[*oc.System]) bool {
			data1, present := val.Val()
			data := data1.GetGrpcServer(grpcServerName).GetAuthzPolicyCounters().GetRpc(rpcPath)
			if present && data.GetAccessAccepts() == counterBefore.AccessAccepts+1 && data.GetAccessRejects() == counterBefore.AccessRejects {
				count += 1
			}
			return count >= 2
		})
	return edtWatch
}
