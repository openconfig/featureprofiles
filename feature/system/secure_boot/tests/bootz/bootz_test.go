// Copyright 2026 Google LLC
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

package bootz_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	ownercertificate "github.com/openconfig/bootz/common/owner_certificate"
	ownershipvoucher "github.com/openconfig/bootz/common/ownership_voucher"
	bootzpb "github.com/openconfig/bootz/proto/bootz"
	bootzsrv "github.com/openconfig/bootz/server"
	bootzconfig "github.com/openconfig/bootz/server/proto/config"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	frpb "github.com/openconfig/gnoi/factory_reset"
	authzpb "github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

var (
	dutID = flag.String("dut", "dut", "The DUT ID in the binding file.")

	bootzAddr = flag.String("bootz_addr", outboundIP().String(), "The Bootz server listen address. If no port is provided, a free port is assigned.")

	factoryReset = flag.Bool("factory_reset", true, "If true, initiate bootz via gNOI FactoryReset.")
	manualZTP    = flag.Bool("manual_ztp", false, "If true, do not initiate ZTP automatically.")
	zeroFill     = flag.Bool("zero_fill", false, "If true, perform a zero-fill factory reset.")

	chassisSerial         = flag.String("chassis_serial", "", "Optional chassis serial number. Otherwise fetched from gNMI.")
	controllerCardSerials = flag.String("cc_serials", "", "Optional comma-separated controller card serial numbers. Otherwise fetched from gNMI.")
	ovFromChassis         = flag.Bool("ov_from_chassis", false, "If true, generate the ownership voucher from the chassis serial instead of control-card serials.")

	mgmtIP             = flag.String("mgmt_ip", "", "Management IP/prefix for the DUT. If empty, fetched from gNMI.")
	mgmtGW             = flag.String("mgmt_gw", "", "Management gateway for the DUT. If empty, inferred from static routes.")
	godhcpdAddr        = flag.String("godhcpd_addr", "", "HTTP address of the godhcpd server. If empty, DHCP setup is skipped.")
	godhcpdBootzTunnel = flag.Bool("godhcpd_bootz_tunnel", false, "If true, request a godhcpd tunnel for the Bootz server.")
	manufacturer       = flag.String("manufacturer", "Cisco", "DUT manufacturer used for Bootz inventory lookup and OV vendor CA.")

	// Compatibility flags that will be used later as more tests are added
	_ = flag.Bool("use_masa", false, "Unused compatibility flag.")
	_ = flag.String("image_path", "", "Unused compatibility flag.")
	_ = flag.String("image_version", "", "Unused compatibility flag.")
	_ = flag.Bool("godhcpd_img_srv_tunnel", false, "Unused compatibility flag.")

	binding bindpb.Binding

	bootzAdvertisedAddr string
)

const (
	bootzStartTimeout    = 30 * time.Minute
	bootzCompleteTimeout = 60 * time.Minute
	bootupTimeout        = 30 * time.Minute

	authzPolicyVersion   = "20260508-bootz-authz"
	authzPolicyCreatedOn = uint64(1694813669807)
	authzPolicyJSON      = `{"name":"simple_authz_policy","allow_rules":[{"name":"allow_all","source":{"principals":["*"]},"request":{"paths":["*"]}}]}`
)

func setupEnvironment() {
	if _, _, err := net.SplitHostPort(*bootzAddr); err != nil {
		port, err := freePort()
		if err != nil {
			panic(fmt.Sprintf("failed to find a free Bootz port: %v", err))
		}
		*bootzAddr = net.JoinHostPort(*bootzAddr, strconv.Itoa(port))
	}
	bootzAdvertisedAddr = *bootzAddr

	if bindingFlag := flag.Lookup("binding"); bindingFlag != nil && bindingFlag.Value.String() != "" {
		in, err := os.ReadFile(bindingFlag.Value.String())
		if err != nil {
			panic(fmt.Sprintf("failed to read binding file: %v", err))
		}
		if err := prototext.Unmarshal(in, &binding); err != nil {
			panic(fmt.Sprintf("failed to parse binding file: %v", err))
		}
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	setupEnvironment()
	fptest.RunTests(m)
}

// TestBootz1_3ValidMinimumConfig verifies bootz-1.3 from README.md.
func TestBootz1_3ValidMinimumConfig(t *testing.T) {
	runBootzPositiveTest(t, nil)
}

func runBootzPositiveTest(t *testing.T, postBootz func(*testing.T, *ondatra.DUTDevice, *statusTracker)) {
	// Step 1: The test initializes the bootz server with a bootstrap response based on the test scenario.
	dut := ondatra.DUT(t, *dutID)

	chassis := getChassisSerial(t, dut)
	if *godhcpdAddr != "" {
		configureDHCPForDUT(t, dut, chassis)
		t.Cleanup(func() {
			deleteDHCPRecord(t, *godhcpdAddr, chassis)
		})
	}

	controlCards := getControllerCardSerials(t, dut)
	serverConfig := newBootzServerConfig(t, dut.ID(), chassis, controlCards)

	preVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	preLastAttempt, _ := gnmi.Lookup(t, dut, gnmi.OC().System().Bootz().LastBootAttempt().State()).Val()

	tracker := newStatusTracker(controlCards)
	srv, err := bootzsrv.NewServer(
		serverConfig,
		&bootzsrv.InterceptorOpts{BootzInterceptor: tracker.interceptor()},
	)
	if err != nil {
		t.Fatalf("failed to create Bootz server: %v", err)
	}
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Bootz server stopped: %v", err)
		}
	}()
	t.Cleanup(srv.Stop)

	// Step 2: The test initiates bootz via a gNOI factory reset request.
	initiateBootz(t, dut)

	// Step 3: The test validates that the device has sent a BootStrapRequest to the server.
	tracker.awaitBootstrapRequest(t, bootzStartTimeout)
	// Step 4: The test validates that the server has received ReportStatusRequest(s) with BOOTSTRAP_STATUS_INITIATED and CONTROL_CARD_STATUS_UNINITIALIZED for each controller card.
	tracker.awaitBootstrapStatus(t, bootzpb.ReportStatusRequest_BOOTSTRAP_STATUS_INITIATED, bootzStartTimeout)
	tracker.awaitControlCardStatus(t, bootzpb.ControlCardState_CONTROL_CARD_STATUS_NOT_INITIALIZED, bootzStartTimeout)
	// Step 5: The test validates that the server has received ReportStatusRequest(s) with BOOTSTRAP_STATUS_SUCCESS.
	tracker.awaitBootstrapStatus(t, bootzpb.ReportStatusRequest_BOOTSTRAP_STATUS_SUCCESS, bootzCompleteTimeout)
	tracker.awaitControlCardStatus(t, bootzpb.ControlCardState_CONTROL_CARD_STATUS_INITIALIZED, bootzCompleteTimeout)

	// Step 6: The test validates the telemetry.
	awaitBootzStatus(t, dut, oc.Bootz_Status_BOOTZ_OK, bootupTimeout)
	validateBootzTelemetry(t, dut, preLastAttempt, tracker.bootstrapDataChecksum())
	// Step 7: The test validates the software image version against the expected version.
	validateSoftwareVersion(t, dut, preVersion)
	if postBootz != nil {
		postBootz(t, dut, tracker)
	}
}

func newBootzServerConfig(t *testing.T, dutID, chassisSerial string, controlCards []string) *bootzconfig.Config {
	t.Helper()

	pdc, pdcKey, err := ownercertificate.NewRSACertificate("Pinned Domain Certificate", "", nil, nil)
	if err != nil {
		t.Fatalf("failed to generate pinned domain certificate: %v", err)
	}
	ownerCert, ownerKey, err := ownercertificate.NewRSACertificate("Owner Certificate", "", pdc, pdcKey)
	if err != nil {
		t.Fatalf("failed to generate owner certificate: %v", err)
	}
	vendorCA, vendorCAKey, err := ownercertificate.NewRSACertificate("Vendor Certificate Authority", "", nil, nil)
	if err != nil {
		t.Fatalf("failed to generate vendor CA: %v", err)
	}
	trustAnchor, trustAnchorKey, err := ownercertificate.NewRSACertificate("Trust Anchor", "", nil, nil)
	if err != nil {
		t.Fatalf("failed to generate trust anchor: %v", err)
	}

	serverSerials := controlCards
	if *ovFromChassis {
		serverSerials = []string{chassisSerial}
	}
	bootzControlCards := make([]*bootzconfig.ControlCard, 0, len(serverSerials))
	for _, serial := range serverSerials {
		ov, err := ownershipvoucher.NewOwnershipVoucher("json", serial, pdc, vendorCA, vendorCAKey)
		if err != nil {
			t.Fatalf("failed to generate ownership voucher for %s: %v", serial, err)
		}
		bootzControlCards = append(bootzControlCards, &bootzconfig.ControlCard{
			SerialNumber:     serial,
			OwnershipVoucher: base64.StdEncoding.EncodeToString(ov),
		})
	}

	return &bootzconfig.Config{
		ServerAddress:    bootzServerAddress(t),
		TrustAnchor:      certKeyPair(t, trustAnchor, trustAnchorKey),
		OwnerCertificate: certKeyPair(t, ownerCert, ownerKey),
		VendorCaCerts:    []string{base64.StdEncoding.EncodeToString(vendorCA.Raw)},
		Chassis: []*bootzconfig.Chassis{{
			Manufacturer: *manufacturer,
			ControlCards: bootzControlCards,
			BootConfig: &bootzpb.BootConfig{
				VendorConfig: []byte(vendorConfig(t, dutID)),
			},
			// The public Bootz server requires AuthZ inventory to be present when
			// resolving bootstrap data, even for this minimum-config workflow.
			Authz: bootzAuthzUpload(),
		}},
	}
}

func bootzServerAddress(t *testing.T) string {
	t.Helper()

	_, listenPort, err := net.SplitHostPort(*bootzAddr)
	if err != nil {
		t.Fatalf("failed to parse Bootz listen address %q: %v", *bootzAddr, err)
	}
	// Bootz v0.7.1 uses the address host for the TLS certificate and its port
	// for the local listener. Preserve the local port when a DHCP tunnel changes
	// only the address advertised to the DUT.
	return net.JoinHostPort(hostFromAddress(bootzAdvertisedAddr), listenPort)
}

func bootzAuthzUpload() *authzpb.UploadRequest {
	return &authzpb.UploadRequest{
		Version:   authzPolicyVersion,
		CreatedOn: authzPolicyCreatedOn,
		Policy:    authzPolicyJSON,
	}
}

func certKeyPair(t *testing.T, cert *x509.Certificate, key crypto.PrivateKey) *bootzconfig.CertKeyPair {
	t.Helper()

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	return &bootzconfig.CertKeyPair{
		Cert: base64.StdEncoding.EncodeToString(cert.Raw),
		Key:  base64.StdEncoding.EncodeToString(keyDER),
	}
}

type statusTracker struct {
	mu       sync.Mutex
	ccSerial []string

	bootstrapRequests int
	reports           []*bootzpb.ReportStatusRequest
	checksum          string
	notify            chan struct{}
}

func newStatusTracker(controlCards []string) *statusTracker {
	return &statusTracker{
		ccSerial: append([]string(nil), controlCards...),
		notify:   make(chan struct{}),
	}
}

func (s *statusTracker) interceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			return resp, err
		}

		s.mu.Lock()
		defer s.mu.Unlock()
		switch r := req.(type) {
		case *bootzpb.GetBootstrapDataRequest:
			s.bootstrapRequests++
			if br, ok := resp.(*bootzpb.GetBootstrapDataResponse); ok {
				serialized := br.GetSerializedBootstrapData()
				if len(serialized) > 0 {
					sum := sha256.Sum256(serialized)
					s.checksum = fmt.Sprintf("%x", sum[:])
				}
			}
			s.signalLocked()
		case *bootzpb.ReportStatusRequest:
			s.reports = append(s.reports, proto.Clone(r).(*bootzpb.ReportStatusRequest))
			s.signalLocked()
		}
		return resp, nil
	}
}

// signalLocked wakes all current waiters. The caller must hold s.mu.
func (s *statusTracker) signalLocked() {
	close(s.notify)
	s.notify = make(chan struct{})
}

func (s *statusTracker) bootstrapDataChecksum() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checksum
}

func (s *statusTracker) awaitBootstrapRequest(t *testing.T, timeout time.Duration) {
	t.Helper()
	s.await(t, timeout, "bootstrap request", func() bool {
		return s.bootstrapRequests > 0
	})
}

func (s *statusTracker) awaitBootstrapStatus(t *testing.T, want bootzpb.ReportStatusRequest_BootstrapStatus, timeout time.Duration) {
	t.Helper()
	s.await(t, timeout, want.String(), func() bool {
		for _, report := range s.reports {
			if report.GetStatus() == want {
				return true
			}
		}
		return false
	})
}

func (s *statusTracker) awaitControlCardStatus(t *testing.T, want bootzpb.ControlCardState_ControlCardStatus, timeout time.Duration) {
	t.Helper()
	s.await(t, timeout, want.String(), func() bool {
		seen := map[string]bool{}
		for _, report := range s.reports {
			for _, state := range report.GetStates() {
				if state.GetStatus() == want {
					seen[state.GetSerialNumber()] = true
				}
			}
		}
		for _, serial := range s.ccSerial {
			if !seen[serial] {
				return false
			}
		}
		return true
	})
}

func (s *statusTracker) await(t *testing.T, timeout time.Duration, desc string, ok func() bool) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		s.mu.Lock()
		done := ok()
		notify := s.notify
		s.mu.Unlock()
		if done {
			return
		}
		select {
		case <-notify:
		case <-timer.C:
			s.mu.Lock()
			if ok() {
				s.mu.Unlock()
				return
			}
			reports := append([]*bootzpb.ReportStatusRequest(nil), s.reports...)
			s.mu.Unlock()
			t.Fatalf("timed out waiting for %s; observed reports: %v", desc, reports)
		}
	}
}

func initiateBootz(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	if *factoryReset {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(ctx)
		if err != nil {
			t.Fatalf("failed to dial gNOI: %v", err)
		}
		if _, err := gnoiClient.FactoryReset().Start(ctx, &frpb.StartRequest{FactoryOs: false, ZeroFill: *zeroFill}); err != nil {
			t.Fatalf("failed to initiate factory reset: %v", err)
		}
		return
	}
	if *manualZTP {
		t.Log("manual_ztp is set; waiting for external ZTP initiation")
		return
	}
	if dut.Vendor() != ondatra.CISCO {
		t.Fatalf("automatic ZTP initiation is only implemented for Cisco DUTs; use factory_reset or manual_ztp")
	}

	cli := dut.RawAPIs().CLI(t)
	if res, err := cli.RunCommand(context.Background(), "ztp terminate noprompt"); err != nil {
		t.Fatalf("failed to terminate existing ZTP: %v", err)
	} else {
		t.Log(res.Output())
	}
	time.Sleep(30 * time.Second)
	if res, err := cli.RunCommand(context.Background(), "ztp clean noprompt"); err != nil {
		t.Fatalf("failed to clean ZTP state: %v", err)
	} else {
		t.Log(res.Output())
	}
	time.Sleep(30 * time.Second)
	if res, err := cli.RunCommand(context.Background(), "run rm -rf /var/log/ztp.log\nztp initiate management noprompt"); err != nil {
		t.Fatalf("failed to initiate ZTP: %v", err)
	} else if strings.Contains(strings.ToLower(res.Output()), "error") {
		t.Fatalf("ZTP initiation returned error output: %s", res.Output())
	}
}

func awaitBootzStatus(t *testing.T, dut *ondatra.DUTDevice, want oc.E_Bootz_Status, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var got oc.E_Bootz_Status
		if errMsg := testt.CaptureFatal(t, func(tb testing.TB) {
			got = gnmi.Get(tb, dut, gnmi.OC().System().Bootz().Status().State())
		}); errMsg == nil {
			t.Logf("Observed Bootz status: %s", got)
			if got == want {
				return
			}
		} else {
			t.Logf("Bootz status lookup failed while waiting for %s: %s", want, *errMsg)
		}
		time.Sleep(15 * time.Second)
	}
	t.Fatalf("timed out waiting for Bootz status %s", want)
}

func validateBootzTelemetry(t *testing.T, dut *ondatra.DUTDevice, preLastAttempt uint64, wantChecksum string) {
	t.Helper()

	lastAttempt := gnmi.Get(t, dut, gnmi.OC().System().Bootz().LastBootAttempt().State())
	if lastAttempt <= preLastAttempt {
		t.Fatalf("last boot attempt did not advance: got %d, previous %d", lastAttempt, preLastAttempt)
	}

	if checksum, ok := gnmi.Lookup(t, dut, gnmi.OC().System().Bootz().Checksum().State()).Val(); ok && checksum != "" && wantChecksum != "" {
		if checksum != wantChecksum {
			t.Fatalf("Bootz checksum mismatch: got %q, want %q", checksum, wantChecksum)
		}
	}
}

func validateSoftwareVersion(t *testing.T, dut *ondatra.DUTDevice, want string) {
	t.Helper()
	got := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	if got != want {
		t.Fatalf("software version changed: got %q, want %q", got, want)
	}
}

func getChassisSerial(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	if *chassisSerial != "" {
		return *chassisSerial
	}
	comps := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS)
	if len(comps) != 1 {
		t.Fatalf("expected one chassis component, got %d: %v", len(comps), comps)
	}
	serial := gnmi.Get(t, dut, gnmi.OC().Component(comps[0]).SerialNo().State())
	if serial == "" {
		t.Fatalf("chassis serial is empty")
	}
	return serial
}

func getControllerCardSerials(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	if *controllerCardSerials != "" {
		return strings.Split(*controllerCardSerials, ",")
	}
	controllerCards := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	serials := make([]string, 0, len(controllerCards))
	for _, comp := range controllerCards {
		serial := gnmi.Get(t, dut, gnmi.OC().Component(comp).SerialNo().State())
		if serial == "" {
			t.Fatalf("controller card %s has empty serial", comp)
		}
		serials = append(serials, serial)
	}
	if len(serials) == 0 {
		t.Fatalf("no controller-card components found")
	}
	return serials
}

func vendorConfig(t *testing.T, dutID string) string {
	t.Helper()
	baseConfig := vendorConfigFromBinding(t, dutID)
	if strings.Contains(baseConfig, "\nend\n") {
		return baseConfig
	}
	return baseConfig + "\n"
}

func vendorConfigFromBinding(t *testing.T, dutID string) string {
	t.Helper()
	for _, dut := range binding.GetDuts() {
		if dut.GetId() != dutID {
			continue
		}
		if dut.GetConfig() == nil || len(dut.GetConfig().GetGnmiSetFile()) == 0 {
			t.Fatalf("binding DUT %s has no GNMI set file", dutID)
		}
		data, err := os.ReadFile(dut.GetConfig().GetGnmiSetFile()[0])
		if err != nil {
			t.Fatalf("failed to read GNMI set file: %v", err)
		}
		var setReq gnmipb.SetRequest
		if err := prototext.Unmarshal(data, &setReq); err != nil {
			t.Fatalf("failed to parse GNMI set file: %v", err)
		}
		for _, replace := range setReq.GetReplace() {
			if replace.GetPath().GetOrigin() == "cli" {
				return replace.GetVal().GetAsciiVal()
			}
		}
		t.Fatalf("binding DUT %s GNMI set file has no CLI vendor config", dutID)
	}
	t.Fatalf("DUT %s not found in binding", dutID)
	return ""
}

func configureDHCPForDUT(t *testing.T, dut *ondatra.DUTDevice, chassisSerial string) {
	t.Helper()

	ip, prefixLen, gw := managementAddress(t, dut)
	bootzURL := fmt.Sprintf("bootz://%s/grpc", *bootzAddr)
	ports := addDHCPRecord(t, *godhcpdAddr, chassisSerial, fmt.Sprintf("%s/%d", ip, prefixLen), gw, bootzURL, *godhcpdBootzTunnel)
	if *godhcpdBootzTunnel {
		if advertised, ok := ports[*bootzAddr]; ok {
			bootzAdvertisedAddr = advertised
		}
	}
}

func managementAddress(t *testing.T, dut *ondatra.DUTDevice) (string, uint8, string) {
	t.Helper()

	if *mgmtIP != "" {
		ip, ipnet, err := net.ParseCIDR(*mgmtIP)
		if err != nil {
			t.Fatalf("failed to parse mgmt_ip %q: %v", *mgmtIP, err)
		}
		ones, _ := ipnet.Mask.Size()
		return ip.String(), uint8(ones), *mgmtGW
	}

	var ip string
	var prefixLen uint8
	for _, intf := range gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State()) {
		if !intf.GetManagement() {
			continue
		}
		for _, subintf := range intf.GetOrCreateSubinterfaceMap() {
			for _, addr := range subintf.GetOrCreateIpv4().GetOrCreateAddressMap() {
				ip = addr.GetIp()
				prefixLen = addr.GetPrefixLength()
				break
			}
			if ip != "" {
				break
			}
		}
		if ip != "" {
			break
		}
	}
	if ip == "" {
		t.Fatalf("failed to find management IPv4 address")
	}
	gw := *mgmtGW
	if gw == "" {
		gw = managementGateway(t, dut, ip)
	}
	return ip, prefixLen, gw
}

func managementGateway(t *testing.T, dut *ondatra.DUTDevice, mgmtIP string) string {
	t.Helper()
	statics := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).StaticMap().State())
	ip := net.ParseIP(mgmtIP)
	bestMask := -1
	var gw string
	for prefix, st := range statics {
		_, ipnet, err := net.ParseCIDR(prefix)
		if err != nil || !ipnet.Contains(ip) {
			continue
		}
		ones, _ := ipnet.Mask.Size()
		if ones <= bestMask {
			continue
		}
		for _, nh := range st.GetOrCreateNextHopMap() {
			if v, ok := nh.GetNextHop().(oc.UnionString); ok {
				bestMask = ones
				gw = string(v)
				break
			}
		}
	}
	if gw == "" {
		t.Fatalf("failed to infer management gateway for %s; set --mgmt_gw", mgmtIP)
	}
	return gw
}

type portMap map[string]string

func addDHCPRecord(t *testing.T, addr, id, ip, gw, bootzURL string, tunnelBootz bool) portMap {
	t.Helper()
	req := map[string]any{
		"id":        id,
		"ip":        ip,
		"gw":        gw,
		"bootz_url": bootzURL,
	}
	if tunnelBootz {
		req["tunnel_bootz"] = true
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal DHCP record: %v", err)
	}

	respBody := doHTTPRequest(t, "POST /records", func() (*http.Request, error) {
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/records", addr), bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})

	result := struct {
		Ports map[string]string `json:"ports"`
	}{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Logf("DHCP server response was not a tunnel map: %s", string(respBody))
	}
	return result.Ports
}

func deleteDHCPRecord(t *testing.T, addr, id string) {
	t.Helper()
	_ = doHTTPRequest(t, "DELETE /records/"+id, func() (*http.Request, error) {
		return http.NewRequest("DELETE", fmt.Sprintf("%s/records/%s", addr, id), nil)
	})
}

func doHTTPRequest(t *testing.T, desc string, build func() (*http.Request, error)) []byte {
	t.Helper()
	var lastErr string
	for attempt := 0; attempt < 11; attempt++ {
		req, err := build()
		if err != nil {
			lastErr = err.Error()
		} else {
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				lastErr = err.Error()
			} else {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode < 300 {
					return body
				}
				lastErr = fmt.Sprintf("%s: %s", resp.Status, string(body))
			}
		}
		if attempt < 10 {
			time.Sleep(2 * time.Second)
		}
	}
	t.Fatalf("%s failed after retries: %s", desc, lastErr)
	return nil
}

func outboundIP() net.IP {
	conn, err := net.Dial("udp", "192.0.2.1:80")
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return addr.IP
		}
	}
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
				continue
			}
			return ipnet.IP
		}
	}
	return net.IPv4(127, 0, 0, 1)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func hostFromAddress(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
