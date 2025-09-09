package godhcpc_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

var (
	addr = flag.String("addr", "", "godhcpd server address")

	chassisType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestAddDHCPEntry(t *testing.T) {
	if *addr == "" {
		t.Fatalf("godhcpd server address must be specified")
	}
	t.Logf("godhcpd server address: %s", *addr)

	dut := ondatra.DUT(t, "dut")
	comps := components.FindComponentsByType(t, dut, chassisType)
	if len(comps) != 1 {
		t.Fatalf("Could not find the chassis in component list")
	}

	chassisSerial := gnmi.Get(t, dut, gnmi.OC().Component(comps[0]).SerialNo().State())
	t.Logf("Chassis serial number: %s", chassisSerial)

	// get management interface ip address using gnmi
	// mgmtMacAddress := ""
	mgmtIpAddress := ""
	mgmtPrefixLength := uint8(0)
	intfs := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	for _, intf := range intfs {
		if intf.GetManagement() {
			// mgmtMacAddress = intf.GetEthernet().GetHwMacAddress()
			// t.Logf("Management interface: %s, MAC: %s", intf.GetName(), mgmtMacAddress)

			for _, subIntf := range intf.GetOrCreateSubinterfaceMap() {
				for _, addr := range subIntf.GetOrCreateIpv4().GetOrCreateAddressMap() {
					mgmtIpAddress = addr.GetIp()
					mgmtPrefixLength = addr.GetPrefixLength()
				}
			}
		}
	}

	if mgmtIpAddress == "" {
		t.Fatalf("Management IP address not found")
	}
	t.Logf("Management IP address: %s", mgmtIpAddress)

	gw := ""
	sm := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).StaticMap().State())

	longestPrefix := ""
	bestMask := -1
	var bestStatic *oc.NetworkInstance_Protocol_Static

	ip := net.ParseIP(mgmtIpAddress)
	if ip == nil {
		t.Logf("Management IP not found or invalid; skipping longest prefix search")
		return
	}
	for prefix, st := range sm {
		_, ipnet, err := net.ParseCIDR(prefix)
		if err != nil {
			continue
		}
		if ipnet.Contains(ip) {
			ones, _ := ipnet.Mask.Size()
			if ones > bestMask {
				bestMask = ones
				longestPrefix = prefix
				bestStatic = st
			}
		}
	}

	if bestStatic != nil {
		for _, nh := range bestStatic.GetOrCreateNextHopMap() {
			switch v := nh.GetNextHop().(type) {
			case oc.UnionString:
				gw = string(v)
			}
			if gw != "" {
				break
			}
		}
		t.Logf("Mgmt IP %s longest match %s (mask %d) gateway %s", mgmtIpAddress, longestPrefix, bestMask, gw)
	} else {
		t.Fatalf("No static route matched management IP %s", mgmtIpAddress)
	}

	localIp := util.GetOutboundIP(t)
	t.Logf("Local ip address: %s", localIp)

	deleteDHCPEntry(t, chassisSerial)
	addDHCPEntry(t, chassisSerial, fmt.Sprintf("%s/%d", mgmtIpAddress, mgmtPrefixLength),
		gw, fmt.Sprintf("bootz://%s:%d/%s", localIp, 15006, "grpc"))
}

func TestDeleteDHCPEntry(t *testing.T) {
	if *addr == "" {
		t.Fatalf("godhcpd server address must be specified")
	}
	t.Logf("godhcpd server address: %s", *addr)

	dut := ondatra.DUT(t, "dut")
	comps := components.FindComponentsByType(t, dut, chassisType)
	if len(comps) != 1 {
		t.Fatalf("Could not find the chassis in component list")
	}

	chassisSerial := gnmi.Get(t, dut, gnmi.OC().Component(comps[0]).SerialNo().State())
	t.Logf("Chassis serial number: %s", chassisSerial)
	deleteDHCPEntry(t, chassisSerial)
}

func deleteDHCPEntry(t *testing.T, id string) {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/records/%s", *addr, id), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /records/%s failed: %v", id, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		t.Fatalf("DELETE /records/%s failed: %s\n%s", id, resp.Status, string(b))
	}
	t.Logf("DELETE /records/%s response: %s", id, string(b))
}

func addDHCPEntry(t *testing.T, id, ip, gw, bootzUrl string) {
	req := map[string]any{"id": id, "ip": ip, "gw": gw, "bootz_urls": []string{bootzUrl}}
	body, _ := json.Marshal(req)
	t.Logf("POST /records body: %s", string(body))
	resp, err := http.Post(fmt.Sprintf("%s/records", *addr), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /records failed: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		t.Fatalf("POST /records failed: %s\n%s", resp.Status, string(out))
	}
	t.Logf("POST /records response: %s", string(out))
}
