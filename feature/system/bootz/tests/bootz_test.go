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

// Package bootz implement tests  authz-14.
package bootz

import (
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/bootz/dhcp"
	"github.com/openconfig/bootz/server/entitymanager/proto/entity"
	"github.com/openconfig/bootz/server/service"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	ov "github.com/openconfig/bootz/common/ownership_voucher"
	dhcpLease "github.com/openconfig/bootz/dhcp/plugins/slease"
	bootzSever "github.com/openconfig/bootz/server"
	bootzem "github.com/openconfig/bootz/server/entitymanager"

	bpb "github.com/openconfig/bootz/proto/bootz"
)

// var (
//
//	dhcpIntf        = flag.String("dhcp-intf", "eth1", "Interface that will be used by dhcp server to listen for dhcp requests")
//	bootzAddr       = flag.String("bootz_addr", "", "The ip:port to start the Bootz server. Ip must be specified and be reachable from the router.")
//	imageServerAddr = flag.String("img_srv_addr", "", "The ip:port to start the Image server. Ip must be specified and be reachable from the router.")
//	imagesDir       = flag.String("img_dir", "", "Directory where the images will be located.")
//	imageVersion    = flag.String("img_ver", "", "Version of the image to be loaded using bootz")
//	dhcpIP          = flag.String("dhcp_ip", "", "IP address in CIDR format that dhcp server will assign to the dut.")
//	dhcpGateway     = flag.String("dhcp_gateway", "", "Gateway IP that dhcp server will assign to DUT.")
//
// )
var (
	dhcpIntf        = flag.String("dhcp-intf", "ens224", "Interface that will be used by dhcp server to listen for dhcp requests")
	bootzAddr       = flag.String("bootz_addr", "5.38.4.124:15006", "The ip:port to start the Bootz server. Ip must be specefied and be reachable from the router.")
	imageServerAddr = flag.String("img_serv_addr", "5.38.4.124:15007", "The ip:port to start the Image server. Ip must be specefied and be reachable from the router.")
	imagesDir       = flag.String("img_dir", "/var/www/html/", "Directory where the images will be located.")
	imageVersion    = flag.String("img_ver", "24.2.1.17I", "Version of the image to be loaded using bootz")
	dhcpIP          = flag.String("dhcp_ip", "5.38.9.29/16", "IP address in CIDR format that dhcp server will assign to the dut.")
	dhcpGateway     = flag.String("dhcp_gateway", "5.38.0.1", "Gateway IP that dhcp server will assign to DUT.")
)

var (
	controlcardType       = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	chassisType           = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS
	isSetupDone           = false
	chassisSerial         = ""
	controllerCards       = []string{}
	controllerCardSerials = []string{}
	chassisBootzConfig    = &entity.Chassis{}
	hwAddrs               = []string{}
	secArtifacts          = &service.SecurityArtifacts{}
	em                    *bootzem.InMemoryEntityManager
	bServer               *bootzSever.Server
	chassisEntity         = &service.EntityLookup{}
	baseConfig            string
	bootzServerFailed     atomic.Bool
)

type bootzTest struct {
	Name            string
	VendorConfig    string
	OV              []byte
	Image           *bpb.SoftwareImage
	ExpectedFailure bool
}

const (
	dhcpTimeout                = 30 * time.Minute // connection to dhcp after factory default
	bootzConnectionTimeout     = 5 * time.Minute  // request for bootstrap after dhcp
	bootzStatusTimeout         = 5 * time.Minute  // only ov + config
	fullBootzCompletionTimeout = 45 * time.Minute // image + ov + config
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func checkBootzStatus(t *testing.T, expectFailure bool) {
	if bootzServerFailed.Load() {
		t.Fatal("bootz server is down, check the test log for detailed error")
	}
	for _, ccSerial := range controllerCardSerials {
		err := awaitBootzStatus(ccSerial, bpb.ReportStatusRequest_BOOTSTRAP_STATUS_INITIATED, bootzStatusTimeout)
		if err != nil {
			t.Errorf("ReportStatusRequest_BOOTSTRAP_STATUS_INITIATED in not reported in %d minutes for controller card %s", bootzStatusTimeout, ccSerial)
		} else {
			t.Log("DUT reported ReportStatusRequest_BOOTSTRAP_STATUS_INITIATED to bootz server as expected")
		}
	}
	expectedCCstatus := bpb.ReportStatusRequest_BOOTSTRAP_STATUS_SUCCESS
	if expectFailure {
		expectedCCstatus = bpb.ReportStatusRequest_BOOTSTRAP_STATUS_FAILURE
	}
	for _, ccSerial := range controllerCardSerials {
		err := awaitBootzStatus(ccSerial, expectedCCstatus, bootzStatusTimeout)
		if err != nil {
			t.Errorf("Status %s is not reported as expected in %d minutes", expectedCCstatus.String(), bootzStatusTimeout)
		} else {
			t.Logf("DUT reported %s to bootz server as expected", expectedCCstatus.String())
		}
	}
}

func dutBootzStatus(t *testing.T, dut *ondatra.DUTDevice, maxRebootTime time.Duration) {
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())
		time.Sleep(1 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device bootzed successfully with received time: %v", currentTime)
			break
		}

		if time.Since(startReboot) > maxRebootTime {
			t.Logf("Check bootz time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
			break
		}
	}
	t.Logf("Device bootz time: %.2f minutes", time.Since(startReboot).Minutes())
	//TODO: add oc leaves check
}

func testSetup(t *testing.T, dut *ondatra.DUTDevice) {
	if !isSetupDone {
		baseConfig = getBaseConfig(t, dut)
		comps := components.FindComponentsByType(t, dut, chassisType)
		if len(comps) != 1 {
			t.Fatalf("Could not find the chassis in component list")
		}
		chassisSerial = gnmi.Get(t, dut, gnmi.OC().Component(comps[0]).SerialNo().State())

		controllerCards = components.FindComponentsByType(t, dut, controlcardType)
		for _, comp := range controllerCards {
			ccSerial := gnmi.Get(t, dut, gnmi.OC().Component(comp).SerialNo().State())
			controllerCardSerials = append(controllerCardSerials, ccSerial)
		}

		for _, ports := range dut.Ports() {
			// We assume the ports in binding are management interfaces, otherwise the test must fail.
			mac := gnmi.Get(t, dut, gnmi.OC().Interface(ports.Name()).Ethernet().HwMacAddress().State())
			if !gnmi.Get(t, dut, gnmi.OC().Interface(ports.Name()).Management().State()) {
				t.Fatalf("Ports are exepcted to be managment interfaces")
			}
			hwAddrs = append(hwAddrs, mac)
		}
		isSetupDone = true
	}

	loadSecArtifacts(t, "testdata/pdc.cert.pem", "testdata/pdc.key.pem")
	var err error
	em, err = bootzem.New("", secArtifacts)
	if err != nil {
		t.Fatalf("Could not initialize bootz inventory: %v", err)
	}
	prepareBootzConfig(t, dut)
	startBootzSever(t)

	err = startDhcpServer(*dhcpIntf, em, *bootzAddr)
	if err != nil {
		t.Fatalf("Could not start dhcp sever on interface %s, err: %v", *dhcpIntf, err)
	}
}

// loadOV load ovs from a specified file
func loadOV(t *testing.T, serialNumber string, pdc *x509.Certificate, verify bool) []byte {
	ovPath := fmt.Sprintf("testdata/%s.ov", serialNumber)
	ovByte, err := os.ReadFile(ovPath)
	if err != nil {
		t.Fatalf("Error opening key file %v", err)
	}
	parsedOV, err := ov.Unmarshal(ovByte, nil)
	if err != nil {
		t.Fatalf("unable to verify ownership voucher: %v", err)
	}

	// Verify the serial number for this OV
	t.Logf("Verifying the serial number for OV")
	got := parsedOV.OV.SerialNumber
	want := serialNumber
	if got != want {
		if verify {
			t.Fatalf("Serial number from OV does not match requested Serial Number, want %v, got %v", serialNumber, got)
		}
	}

	// ensure the cert in ov is valid.
	_, err = x509.ParseCertificate(parsedOV.OV.PinnedDomainCert)
	if err != nil {
		t.Fatalf("Unable to parse PDC DER to x509 certificate: %v", err)
	}
	if string(pdc.Raw) != string(parsedOV.OV.PinnedDomainCert) {
		if verify {
			t.Fatalf("The PDC from the ov does not match the expected pdc")
		}
	}
	return ovByte
}

// load sec artifacts
func loadSecArtifacts(t *testing.T, pdcCertPEM, pdcKeyPEM string) {
	serials := []string{chassisSerial}
	if len(controllerCardSerials) >= 1 { // modular chassis
		serials = controllerCardSerials
	}
	pdcKey, pdcCert := loadKeyPair(t, pdcKeyPEM, pdcCertPEM)
	os.Remove("testdata/tls.key.pem")
	os.Remove("testdata/tls.cert.pem")
	ip := strings.Split(*bootzAddr, ":")[0]
	anchorCert := generateCert(t, pdcCert, pdcKey, ip, "bootz server")

	sa := &service.SecurityArtifacts{
		PDC:                 pdcCert,
		OwnerCert:           pdcCert,
		OwnerCertPrivateKey: pdcKey,
		TLSKeypair:          anchorCert,
		OV:                  service.OVList{},
		TrustAnchor:         pdcCert,
	}
	for _, serial := range serials {
		sa.OV[serial] = loadOV(t, serial, pdcCert, true)
	}
	secArtifacts = sa
}

func prepareBootzConfig(t *testing.T, dut *ondatra.DUTDevice) {
	caser := cases.Title(language.English)
	chassisEntity = &service.EntityLookup{SerialNumber: chassisSerial, Manufacturer: caser.String(dut.Vendor().String())}
	chassisBootzConfig = &entity.Chassis{
		SerialNumber:  chassisSerial,
		SoftwareImage: nil,
		Manufacturer:  caser.String(dut.Vendor().String()),
		BootMode:      bpb.BootMode_BOOT_MODE_SECURE,
		Config: &entity.Config{
			BootConfig: &entity.BootConfig{
				VendorConfig: []byte(getBaseConfig(t, dut)),
			},
			GnsiConfig: &entity.GNSIConfig{
				AuthzUpload: &authz.UploadRequest{
					Version:   "0.0",
					CreatedOn: uint64(time.Now().UnixMilli()),
					Policy:    "{}", // TODO: add authz policy here
				},
			},
		},
		DhcpConfig: &entity.DHCPConfig{
			HardwareAddress: chassisSerial,
			IpAddress:       *dhcpIP,
			Gateway:         *dhcpGateway,
		},
	}
	for i, cc := range controllerCardSerials {
		ccConfig := &entity.ControlCard{
			SerialNumber: cc,
			DhcpConfig: &entity.DHCPConfig{
				HardwareAddress: hwAddrs[i],
				IpAddress:       *dhcpIP,
				Gateway:         *dhcpGateway,
			},
		}
		chassisBootzConfig.ControllerCards = append(chassisBootzConfig.ControllerCards, ccConfig)
	}
	em.ReplaceDevice(chassisEntity, chassisBootzConfig)
}

func startBootzSever(t *testing.T) {
	err := em.ReplaceDevice(chassisEntity, chassisBootzConfig)
	if err != nil {
		t.Fatalf("Could not add chassis config to entitymanager config: %v", err)
	}
	imgaeServOpts := &bootzSever.ImgSrvOpts{
		ImagesLocation: *imagesDir,
		Address:        *imageServerAddr,
		CertFile:       "testdata/tls.cert.pem",
		KeyFile:        "testdata/tls.key.pem",
	}
	interceptor := &bootzSever.InterceptorOpts{
		BootzInterceptor: bootzInterceptor,
	}
	bServer, err = bootzSever.NewServer(*bootzAddr, em, secArtifacts, imgaeServOpts, interceptor)
	if err != nil {
		t.Fatalf("Could not initiate bootz server %v", err)
	}
	bootzServerFailed.Store(false)
	go func() {
		err := bServer.Start()
		if err != nil {
			t.Logf("Unexpected Bootz server error %v, test will be terminated ASAP", err)
			bootzServerFailed.Store(true)
		}
	}()
}

// ### bootz-1: Validate minimum necessary bootz configuration
func TestBootz1(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	testSetup(t, dut)
	defer bServer.Stop()
	defer dhcp.Stop()

	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	bootzStarted := false

	bootz1 := []bootzTest{
		{
			Name:            "Bootz-1.1: Missing config",
			VendorConfig:    "",
			ExpectedFailure: true,
		},
		{
			Name:            "Bootz-1.2: Invalid config",
			VendorConfig:    "invalid config",
			ExpectedFailure: true,
		},
		{
			Name:            "Bootz-1.3: Valid config",
			VendorConfig:    baseConfig,
			ExpectedFailure: false,
		},
	}
	t.Run("Running Bootz1 Test to Validate minimum necessary bootz configuration", func(t *testing.T) {
		for _, tt := range bootz1 {
			t.Run(tt.Name, func(t *testing.T) {
				if bootzServerFailed.Load() {
					t.Fatal("bootz server is down, check the test log for detailed error")
				}
				// reset bootz logs
				bootzStatusLogs = bootzStatus{}
				bootzReqLogs = bootzLogs{}
				//ensure no old dhcp log causing an issue
				dhcpLease.CleanLog()

				chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(tt.VendorConfig)
				em.ReplaceDevice(chassisEntity, chassisBootzConfig)
				if !bootzStarted {
					factoryReset(t, dut)
					bootzStarted = true
				}
				dhcpIDs := []string{chassisSerial}
				dhcpIDs = append(dhcpIDs, hwAddrs...)
				err := awaitDHCPCompletion(dhcpIDs, dhcpTimeout)
				if err != nil {
					t.Errorf("DUT connection to DHCP server was not successful in %d minutes", dhcpTimeout)
				} else {
					t.Logf("DUT connection to DHCP server was  successful")
				}
				err = awaitBootzConnection(*chassisEntity, bootzConnectionTimeout)
				if err != nil {
					t.Errorf("DUT connection to bootz server was not successful in %d minutes", bootzConnectionTimeout)
				} else {
					t.Log("DUT is connected to bootz server")
				}
				checkBootzStatus(t, tt.ExpectedFailure)
			})
		}
		dutBootzStatus(t, dut, 5*time.Second)
		dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
		if dutPreTestVersion != dutPostTestVersion {
			t.Fatalf("DUT software versions do not match, pretest: %s , posttest: %s ", dutPreTestVersion, dutPostTestVersion)
		}

	})
}

// ### bootz-2: Validate Software image in bootz configuration
func TestBootz2(t *testing.T) {

	dut := ondatra.DUT(t, "dut")

	testSetup(t, dut)
	defer bServer.Stop()
	defer dhcp.Stop()

	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	bootzStarted := false

	bootz2 := []bootzTest{
		{
			Name:         "Bootz-2.1 Invalid software image ",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "badimage.iso",
				Url:           fmt.Sprintf("https://%s/badimage.iso", *imageServerAddr),
				HashAlgorithm: "sha256",
				OsImageHash:   getImageHash(t, fmt.Sprintf("%s/badimage.iso", *imagesDir)),
				Version:       "99999",
			},
			ExpectedFailure: true,
		},
		{
			Name:         "Bootz-2.2: Software version is different",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "goodimage.iso",
				Url:           fmt.Sprintf("https://%s/goodimage.iso", *imageServerAddr),
				HashAlgorithm: "sha256",
				OsImageHash:   getImageHash(t, fmt.Sprintf("%s/goodimage.iso", *imagesDir)),
				Version:       *imageVersion,
			},
			ExpectedFailure: false,
		},
	}

	t.Run("Running Bootz2 Test to Validate Software image in bootz configuration", func(t *testing.T) {
		for _, tt := range bootz2 {
			t.Run(tt.Name, func(t *testing.T) {
				if bootzServerFailed.Load() {
					t.Fatal("bootz server is down, check the test log for detailed error")
				}
				// reset bootz logs
				bootzStatusLogs = bootzStatus{}
				bootzReqLogs = bootzLogs{}
				//ensure no old dhcp log causing an issue
				dhcpLease.CleanLog()

				chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(tt.VendorConfig)
				chassisBootzConfig.SoftwareImage = tt.Image
				em.ReplaceDevice(chassisEntity, chassisBootzConfig)
				if !bootzStarted {
					factoryReset(t, dut)
					bootzStarted = true
				}
				dhcpIDs := []string{chassisSerial}
				dhcpIDs = append(dhcpIDs, hwAddrs...)
				err := awaitDHCPCompletion(dhcpIDs, dhcpTimeout)
				if err != nil {
					t.Errorf("DUT connection to DHCP server was not successful in %d minutes", dhcpTimeout)
				} else {
					t.Logf("DUT connection to DHCP server was  successful")
				}
				err = awaitBootzConnection(*chassisEntity, bootzConnectionTimeout)
				if err != nil {
					t.Errorf("DUT connection to bootz server was not successful in %d minutes", bootzConnectionTimeout)
				} else {
					t.Log("DUT is connected to bootz server")
				}
				checkBootzStatus(t, tt.ExpectedFailure)
			})
		}
		dutBootzStatus(t, dut, fullBootzCompletionTimeout)
		dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
		if dutPostTestVersion != *imageVersion {
			t.Fatalf("DUT software versions do not match, pretest: %s , posttest: %s ", dutPreTestVersion, dutPostTestVersion)
		}
	})

}

// ### bootz-3: Validate Ownership Voucher in bootz configuration
func TestBootz3(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	testSetup(t, dut)
	defer bServer.Stop()
	defer dhcp.Stop()

	bootz3 := []bootzTest{
		// need to discuss and remove
		/*bootzTest{
			// TODO: clarify this since in secure mode we may not support this
			Name:            "Bootz-3.1: No ownership voucher ",
			VendorConfig:    baseConfig,
			ExpectedFailure: false,
			OV: []byte(""),
		},*/
		{
			Name:            "Bootz-3.2 Invalid OV",
			VendorConfig:    baseConfig,
			ExpectedFailure: true,
			OV:              []byte("invalid ov"),
		},
		{
			Name:            "bootz-3.3  Valid OV format but for differnt device",
			VendorConfig:    baseConfig,
			ExpectedFailure: true,
			OV:              loadOV(t, "wrongserial", secArtifacts.PDC, false), // get serail as flasg
		},
		{
			Name:            "bootz-3.4 Valid OV",
			VendorConfig:    baseConfig,
			ExpectedFailure: false,
		},
	}

	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	bootzStarted := false

	t.Run("Running Bootz3 Validate Ownership Voucher in bootz configuration", func(t *testing.T) {
		for _, tt := range bootz3 {
			t.Run(tt.Name, func(t *testing.T) {
				if bootzServerFailed.Load() {
					t.Fatal("bootz server is down, check the test log for detailed error")
				}
				// reset bootz logs
				bootzStatusLogs = bootzStatus{}
				bootzReqLogs = bootzLogs{}
				//ensure no old dhcp log causing an issue
				dhcpLease.CleanLog()

				chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(tt.VendorConfig)
				chassisBootzConfig.SoftwareImage = &bpb.SoftwareImage{}
				for k := range secArtifacts.OV {
					secArtifacts.OV[k] = tt.OV
				}
				if len(tt.OV) == 0 { // load the valid ovs
					for _, cc := range controllerCardSerials {
						secArtifacts.OV[cc] = loadOV(t, cc, secArtifacts.PDC, true)
					}
				}
				em.ReplaceDevice(chassisEntity, chassisBootzConfig)
				if !bootzStarted {
					factoryReset(t, dut)
					bootzStarted = true
				}
				dhcpIDs := []string{chassisSerial}
				dhcpIDs = append(dhcpIDs, hwAddrs...)
				err := awaitDHCPCompletion(dhcpIDs, dhcpTimeout)
				if err != nil {
					t.Errorf("DUT connection to DHCP server was not successful in %d minutes", dhcpTimeout)
				} else {
					t.Logf("DUT connection to DHCP server was  successful")
				}
				err = awaitBootzConnection(*chassisEntity, bootzConnectionTimeout)
				if err != nil {
					t.Errorf("DUT connection to bootz server was not successful in %d minutes", bootzConnectionTimeout)
				} else {
					t.Log("DUT is connected to bootz server")
				}
				if !tt.ExpectedFailure { // when OV validation fails, device has no secure way to connect and report the status
					checkBootzStatus(t, tt.ExpectedFailure)
				}
			})
		}
		dutBootzStatus(t, dut, 5*time.Second)
		dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
		if dutPreTestVersion != dutPreTestVersion {
			t.Fatalf("DUT Software Versions do not match, pretest: %s , posttest: %s ", dutPreTestVersion, dutPostTestVersion)
		}
	})
}

// ### bootz-4: Validate device properly resets if provided invalid image
func TestBootz4(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	testSetup(t, dut)
	defer bServer.Stop()
	defer dhcp.Stop()

	bootz4 := []bootzTest{
		{
			Name:         "Bootz-4.1 Invalid OS image provided",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "badimage.iso",
				Url:           fmt.Sprintf("https://%s/badimage.iso", *imageServerAddr),
				HashAlgorithm: "SHA256",
				OsImageHash:   getImageHash(t, fmt.Sprintf("%s/badimage.iso", *imagesDir)),
				Version:       "999",
			},
			ExpectedFailure: true,
		}, //gets covered as a part of Bootz2.2
		{
			Name:         "Bootz-4.2 Failed to fetch image from remote URL",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "badimage.iso",
				Url:           fmt.Sprintf("https://%s/goodimage.isoinvalidUrl", *imageServerAddr),
				HashAlgorithm: "SHA256",
				OsImageHash:   getImageHash(t, fmt.Sprintf("%s/goodimage.iso", *imagesDir)),
				Version:       "999",
			},
			ExpectedFailure: true,
		},
		{
			Name:         "Bootz-4.3 OS Checksum Doesn't Match",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "goodimage.iso",
				Url:           fmt.Sprintf("https://%s/goodimage.iso", *imageServerAddr),
				HashAlgorithm: "SHA256",
				OsImageHash:   "Invalid Hash",
				Version:       "999",
			},
			ExpectedFailure: true,
		},
		{
			Name:            "Bootz-4.4: No OS Provided",
			VendorConfig:    baseConfig,
			Image:           &bpb.SoftwareImage{},
			ExpectedFailure: false,
		},
	}
	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	bootzStarted := false

	t.Run("Running Bootz4 Test to Validate Software image in bootz configuration", func(t *testing.T) {
		for _, tt := range bootz4 {
			t.Run(tt.Name, func(t *testing.T) {
				if bootzServerFailed.Load() {
					t.Fatal("bootz server is down, check the test log for detailed error")
				}
				// reset bootz logs
				bootzStatusLogs = bootzStatus{}
				bootzReqLogs = bootzLogs{}
				//ensure no old dhcp log causing an issue
				dhcpLease.CleanLog()

				chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(tt.VendorConfig)
				chassisBootzConfig.SoftwareImage = tt.Image
				em.ReplaceDevice(chassisEntity, chassisBootzConfig)
				if !bootzStarted {
					factoryReset(t, dut)
					bootzStarted = true
				}
				dhcpIDs := []string{chassisSerial}
				dhcpIDs = append(dhcpIDs, hwAddrs...)
				err := awaitDHCPCompletion(dhcpIDs, dhcpTimeout)
				if err != nil {
					t.Errorf("DUT connection to DHCP server was not successful in %d minutes", dhcpTimeout)
				} else {
					t.Logf("DUT connection to DHCP server was  successful")
				}
				err = awaitBootzConnection(*chassisEntity, bootzConnectionTimeout)
				if err != nil {
					t.Errorf("DUT connection to bootz server was not successful in %d minutes", bootzConnectionTimeout)
				} else {
					t.Log("DUT is connected to bootz server")
				}
				checkBootzStatus(t, tt.ExpectedFailure)
			})
		}
		dutBootzStatus(t, dut, 5*time.Second)
		dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
		if dutPreTestVersion != dutPreTestVersion {
			t.Fatalf("DUT Software Versions do not match, pretest: %s , posttest: %s ", dutPreTestVersion, dutPostTestVersion)
		}
	})
}
