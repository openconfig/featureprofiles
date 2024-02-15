// Copyright 2022 Google LLC
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

package osinstall_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"flag"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	closer "github.com/openconfig/gocloser"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ospb "github.com/openconfig/gnoi/os"
	spb "github.com/openconfig/gnoi/system"
)

var packageReader func(context.Context) (io.ReadCloser, error) = func(ctx context.Context) (io.ReadCloser, error) {
	f, err := os.Open(*osFile)
	if err != nil {
		return nil, err
	}
	return f, nil
}

var (
	osFile    = flag.String("osfile", "", "Path to the OS image for the install operation")
	osVersion = flag.String("osver", "", "Version of the OS image for the install operation")

	timeout = flag.Duration("timeout", time.Minute*30, "Time to wait for reboot to complete")

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutAttrs = attrs.Attributes{
		Desc:    "To ATE",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	bgpGlobalAttrs = bgpAttrs{
		rplName:       "ALLOW",
		grRestartTime: 60,
		prefixLimit:   200,
		dutAS:         64500,
		ateAS:         64501,
		peerGrpNamev4: "BGP-PEER-GROUP-V4",
	}
	bgpNbr1 = bgpNeighbor{localAs: bgpGlobalAttrs.dutAS, peerAs: bgpGlobalAttrs.ateAS, pfxLimit: bgpGlobalAttrs.prefixLimit, neighborip: ateSrc.IPv4, isV4: true}
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
)

type bgpAttrs struct {
	rplName, peerGrpNamev4    string
	prefixLimit, dutAS, ateAS uint32
	grRestartTime             uint16
}
type bgpNeighbor struct {
	localAs, peerAs, pfxLimit uint32
	neighborip                string
	isV4                      bool
}
type testCase struct {
	dut *ondatra.DUTDevice
	// dualSup indicates if the DUT has a standby supervisor available.
	dualSup bool
	reader  io.ReadCloser

	osc ospb.OSClient
	sc  spb.SystemClient
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOSInstall(t *testing.T) {
	if *osFile == "" || *osVersion == "" {
		t.Fatal("Missing osfile or osver args")
	}

	t.Logf("Testing GNOI OS Install to version %s from file %q", *osVersion, *osFile)

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	reader, err := packageReader(ctx)
	if err != nil {
		t.Fatalf("Error creating package reader: %s", err)
	}

	tc := testCase{
		reader: reader,
		dut:    dut,
		osc:    dut.RawAPIs().GNOI(t).OS(),
		sc:     dut.RawAPIs().GNOI(t).System(),
	}
	noReboot := deviations.OSActivateNoReboot(dut)
	tc.fetchStandbySupervisorStatus(ctx, t)
	tc.transferOS(ctx, t, false)
	tc.activateOS(ctx, t, false, noReboot)

	if deviations.InstallOSForStandbyRP(dut) && tc.dualSup {
		tc.transferOS(ctx, t, true)
		tc.activateOS(ctx, t, true, noReboot)
	}

	if noReboot {
		tc.rebootDUT(ctx, t)
	}

	tc.verifyInstall(ctx, t)
}

func (tc *testCase) activateOS(ctx context.Context, t *testing.T, standby, noReboot bool) {
	t.Helper()
	if standby {
		t.Log("OS.Activate is started for standby RP.")
	} else {
		t.Log("OS.Activate is started for active RP.")
	}
	act, err := tc.osc.Activate(ctx, &ospb.ActivateRequest{
		StandbySupervisor: standby,
		Version:           *osVersion,
		NoReboot:          noReboot,
	})
	if err != nil {
		t.Fatalf("OS.Activate request failed: %s", err)
	}

	switch resp := act.Response.(type) {
	case *ospb.ActivateResponse_ActivateOk:
		if standby {
			t.Log("OS.Activate standby supervisor complete.")
		} else {
			t.Log("OS.Activate complete.")
		}
	case *ospb.ActivateResponse_ActivateError:
		actErr := resp.ActivateError
		t.Fatalf("OS.Activate error %s: %s", actErr.Type, actErr.Detail)
	default:
		t.Fatalf("OS.Activate unexpected response: got %v (%T)", resp, resp)
	}
}

// fetchStandbySupervisorStatus checks if the DUT has a standby supervisor available in a working state.
func (tc *testCase) fetchStandbySupervisorStatus(ctx context.Context, t *testing.T) {
	r, err := tc.osc.Verify(ctx, &ospb.VerifyRequest{})
	if err != nil {
		t.Fatal(err)
	}

	switch v := r.GetVerifyStandby().GetState().(type) {
	case *ospb.VerifyStandby_StandbyState:
		if v.StandbyState.GetState() == ospb.StandbyState_UNAVAILABLE {
			t.Fatal("OS.Verify RPC reports standby supervisor in UNAVAILABLE state.")
		}
		// All other supervisor states indicate this device does not support or have dual supervisors available.
		t.Log("DUT is detected as single supervisor.")
		tc.dualSup = false
	case *ospb.VerifyStandby_VerifyResponse:
		t.Log("DUT is detected as dual supervisor.")
		tc.dualSup = true
	default:
		t.Fatalf("Unexpected OS.Verify Standby State RPC Response: got %v (%T)", v, v)
	}
}

func (tc *testCase) rebootDUT(ctx context.Context, t *testing.T) {
	t.Log("Send DUT Reboot Request")
	_, err := tc.sc.Reboot(ctx, &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Force:   true,
		Message: "Apply GNOI OS Software Install",
	})
	if err != nil && status.Code(err) != codes.Unavailable {
		t.Fatalf("System.Reboot request failed: %s", err)
	}
}

func (tc *testCase) transferOS(ctx context.Context, t *testing.T, standby bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ic, err := tc.osc.Install(ctx)
	if err != nil {
		t.Fatalf("OS.Install client request failed: %s", err)
	}

	ireq := &ospb.InstallRequest{
		Request: &ospb.InstallRequest_TransferRequest{
			TransferRequest: &ospb.TransferRequest{
				Version:           *osVersion,
				StandbySupervisor: standby,
			},
		},
	}
	if err = ic.Send(ireq); err != nil {
		t.Fatalf("OS.Install error sending install request: %s", err)
	}

	iresp, err := ic.Recv()
	if err != nil {
		t.Fatalf("OS.Install error receiving: %s", err)
	}
	switch v := iresp.GetResponse().(type) {
	case *ospb.InstallResponse_TransferReady:
	case *ospb.InstallResponse_Validated:
		if standby {
			t.Log("DUT standby supervisor has valid preexisting image; skipping transfer.")
		} else {
			t.Log("DUT supervisor has valid preexisting image; skipping transfer.")
		}
		return
	case *ospb.InstallResponse_SyncProgress:
		if !tc.dualSup {
			t.Fatalf("Unexpected SyncProgress on single supervisor: got %v (%T)", v, v)
		}
		t.Logf("Sync progress: %v%% synced from supervisor", v.SyncProgress.GetPercentageTransferred())
	default:
		t.Fatalf("Expected TransferReady following TransferRequest: got %v (%T)", v, v)
	}

	awaitChan := make(chan error)
	go func() {
		err := watchStatus(t, ic, standby)
		awaitChan <- err
	}()

	if !standby {
		err = transferContent(ic, tc.reader)
		if err != nil {
			t.Fatalf("Error transferring content: %s", err)
		}
	}

	if err = <-awaitChan; err != nil {
		t.Fatalf("Transfer receive error: %s", err)
	}

	if standby {
		t.Log("OS.Install standby supervisor image transfer complete.")
	} else {
		t.Log("OS.Install supervisor image transfer complete.")
	}
}

// verifyInstall validates the OS.Verify RPC returns no failures and version numbers match the
// newly requested software version.
func (tc *testCase) verifyInstall(ctx context.Context, t *testing.T) {
	rebootWait := time.Minute
	deadline := time.Now().Add(*timeout)
	for time.Now().Before(deadline) {
		r, err := tc.osc.Verify(ctx, &ospb.VerifyRequest{})
		switch status.Code(err) {
		case codes.OK:
		case codes.Unavailable:
			t.Log("Reboot in progress.")
			time.Sleep(rebootWait)
			continue
		default:
			t.Fatalf("OS.Verify request failed: %v", err)
		}
		// when noreboot is set to false, the device returns "in-progress" before initiating the reboot
		if r.GetActivationFailMessage() == "in-progress" {
			t.Logf("Waiting for reboot to initiate.")
			time.Sleep(rebootWait)
			continue
		}
		if got, want := r.GetActivationFailMessage(), ""; got != want {
			t.Fatalf("OS.Verify ActivationFailMessage: got %q, want %q", got, want)
		}
		if got, want := r.GetVersion(), *osVersion; got != want {
			t.Logf("Reboot has not finished with the right version: got %s , want: %s.", got, want)
			time.Sleep(rebootWait)
			continue
		}

		dut := ondatra.DUT(t, "dut")
		if !deviations.SwVersionUnsupported(dut) {
			ver, ok := gnmi.Lookup(t, dut, gnmi.OC().System().SoftwareVersion().State()).Val()
			if !ok {
				t.Log("Reboot has not finished with the right version: couldn't get system/state/software-version")
				time.Sleep(rebootWait)
				continue
			}
			if got, want := ver, *osVersion; !strings.HasPrefix(got, want) {
				t.Logf("Reboot has not finished with the right version: got %s , want: %s.", got, want)
				time.Sleep(rebootWait)
				continue
			}
		}

		if tc.dualSup {
			if got, want := r.GetVerifyStandby().GetVerifyResponse().GetActivationFailMessage(), ""; got != want {
				t.Fatalf("OS.Verify Standby ActivationFailMessage: got %q, want %q", got, want)
			}

			if got, want := r.GetVerifyStandby().GetVerifyResponse().GetVersion(), *osVersion; got != want {
				t.Log("Standby not ready.")
				time.Sleep(rebootWait)
				continue
			}
		}

		t.Log("OS.Verify complete")
		return
	}

	t.Fatal("OS.Verify did not return the correct version before deadline.")
}

func transferContent(ic ospb.OS_InstallClient, reader io.ReadCloser) error {
	// The gNOI SetPackage operation sets the maximum chunk size at 64K,
	// so assuming the install operation allows for up to the same size.
	buf := make([]byte, 64*1024)
	defer closer.CloseAndLog(reader.Close, "error closing package file")
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			tc := &ospb.InstallRequest{
				Request: &ospb.InstallRequest_TransferContent{
					TransferContent: buf[0:n],
				},
			}
			if err := ic.Send(tc); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	te := &ospb.InstallRequest{
		Request: &ospb.InstallRequest_TransferEnd{
			TransferEnd: &ospb.TransferEnd{},
		},
	}
	return ic.Send(te)
}

func watchStatus(t *testing.T, ic ospb.OS_InstallClient, standby bool) error {
	var gotProgress bool

	for {
		iresp, err := ic.Recv()
		if err != nil {
			return err
		}

		switch v := iresp.GetResponse().(type) {
		case *ospb.InstallResponse_InstallError:
			errName := ospb.InstallError_Type_name[int32(v.InstallError.Type)]
			return fmt.Errorf("installation error %q: %s", errName, v.InstallError.GetDetail())
		case *ospb.InstallResponse_TransferProgress:
			if standby {
				return fmt.Errorf("unexpected TransferProgress: got %v, want SyncProgress", v)
			}
			t.Logf("Transfer progress: %v bytes received by DUT", v.TransferProgress.GetBytesReceived())
			gotProgress = true
		case *ospb.InstallResponse_SyncProgress:
			if !standby {
				return fmt.Errorf("unexpected SyncProgress: got %v, want TransferProgress", v)
			}
			t.Logf("Transfer progress: %v%% synced from supervisor", v.SyncProgress.GetPercentageTransferred())
			gotProgress = true
		case *ospb.InstallResponse_Validated:
			if !gotProgress {
				return fmt.Errorf("transfer completed without progress status")
			}
			if got, want := v.Validated.GetVersion(), *osVersion; got != want {
				return fmt.Errorf("mismatched validation software versions: got %s, want %s", got, want)
			}
			return nil
		default:
			return fmt.Errorf("unexpected client install response: got %v (%T)", v, v)
		}
	}
}

func TestPushAndVerifyInterfaceConfig(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	t.Logf("Create and push interface config to the DUT")
	dutPort := dut.Port(t, "port1")
	dutPortName := dutPort.Name()
	intf1 := dutAttrs.NewOCInterface(dutPortName, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intf1.GetName()).Config(), intf1)

	dc := gnmi.OC().Interface(dutPortName).Config()
	in := configInterface(dutPortName, dutAttrs.Desc, dutAttrs.IPv4, dutAttrs.IPv4Len, dut)
	fptest.LogQuery(t, fmt.Sprintf("%s to Replace()", dutPort), dc, in)
	gnmi.Replace(t, dut, dc, in)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		ocPortName := dut.Port(t, "port1").Name()
		fptest.AssignToNetworkInstance(t, dut, ocPortName, deviations.DefaultNetworkInstance(dut), 0)
	}

	t.Logf("Fetch interface config from the DUT using Get RPC and verify it matches with the config that was pushed earlier")
	if val, present := gnmi.LookupConfig(t, dut, dc).Val(); present {
		if reflect.DeepEqual(val, in) {
			t.Logf("Interface config Want and Got matched")
			fptest.LogQuery(t, fmt.Sprintf("%s from Get", dutPort), dc, val)
		} else {
			t.Errorf("Config %v Get() value not matching with what was Set()", dc)
		}
	} else {
		t.Errorf("Config %v Get() failed", dc)
	}
}

// configInterface generates an interface's configuration based on the the attributes given.
func configInterface(name, desc, ipv4 string, prefixlen uint8, dut *ondatra.DUTDevice) *oc.Interface {
	i := &oc.Interface{}
	i.Name = ygot.String(name)
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()

	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}

	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return i
}

func TestPushAndVerifyBGPConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	t.Logf("Create and push BGP config to the DUT")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := bgpCreateNbr(dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

	t.Logf("Fetch BGP config from the DUT using Get RPC and verify it matches with the config that was pushed earlier")
	if val, present := gnmi.LookupConfig(t, dut, dutConfPath.Config()).Val(); present {
		if reflect.DeepEqual(val, dutConf) {
			t.Logf("BGP config Want and Got matched")
			fptest.LogQuery(t, "BGP fetched from DUT using Get()", dutConfPath.Config(), val)
		} else {
			t.Errorf("Config %v Get() value not matching with what was Set()", dutConfPath.Config())
		}
	} else {
		t.Errorf("Config %v Get() failed", dutConfPath.Config())
	}
}

// bgpCreateNbr creates a BGP object with neighbor pointing to ateSrc
func bgpCreateNbr(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(uint32(bgpGlobalAttrs.dutAS))
	global.RouterId = ygot.String(dutSrc.IPv4)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	pgv4 := bgp.GetOrCreatePeerGroup(bgpGlobalAttrs.peerGrpNamev4)
	pgv4.PeerAs = ygot.Uint32(uint32(bgpGlobalAttrs.ateAS))
	pgv4.PeerGroupName = ygot.String(bgpGlobalAttrs.peerGrpNamev4)
	nbr := bgpNbr1
	nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
	nv4.PeerAs = ygot.Uint32(nbr.peerAs)
	nv4.Enabled = ygot.Bool(true)
	nv4.PeerGroup = ygot.String(bgpGlobalAttrs.peerGrpNamev4)
	return niProto
}
