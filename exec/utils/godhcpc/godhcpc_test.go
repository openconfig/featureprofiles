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
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

var (
	addr    = flag.String("addr", "", "godhcpd server address")
	gateway = flag.String("dhcp_gw", "", "dhcp gateway")

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

	mgmtIpAddress := ""
	mgmtPrefixLength := uint8(0)
	intfs := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	for _, intf := range intfs {
		if intf.GetManagement() {
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

	gw := *gateway
	if gw != "" {
		t.Logf("Using configured gateway: %s", gw)
	} else {
		t.Logf("No gateway configured, searching for longest prefix match")

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

func doRequestWithRetry(t *testing.T, desc string, backoffs []time.Duration, build func() (*http.Request, error)) (*http.Response, []byte) {
	var lastStatus string
	var lastBody []byte
	for attempt := 0; attempt < len(backoffs)+1; attempt++ {
		req, err := build()
		if err != nil {
			lastStatus = fmt.Sprintf("build error: %v", err)
		} else {
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				lastStatus = err.Error()
			} else {
				b, _ := io.ReadAll(resp.Body)
				if resp.StatusCode < 300 {
					return resp, b // caller will close
				}
				// non-success; capture and close
				lastStatus = resp.Status
				lastBody = b
				resp.Body.Close()
			}
		}
		if attempt < len(backoffs) {
			wait := backoffs[attempt]
			t.Logf("%s attempt %d failed (%s). Retrying in %s", desc, attempt+1, lastStatus, wait)
			time.Sleep(wait)
		}
	}
	t.Fatalf("%s failed after retries: %s\n%s", desc, lastStatus, string(lastBody))
	return nil, nil
}

func backoffFn(total, interval time.Duration) []time.Duration {
	if total <= 0 || interval <= 0 {
		return nil
	}
	n := int(total / interval)
	if n == 0 {
		n = 1
	}
	backs := make([]time.Duration, n)
	for i := range backs {
		backs[i] = interval
	}
	return backs
}

func deleteDHCPEntry(t *testing.T, id string) {
	backoffs := backoffFn(5*time.Minute, 30*time.Second)
	resp, body := doRequestWithRetry(t, fmt.Sprintf("DELETE /records/%s", id), backoffs, func() (*http.Request, error) {
		return http.NewRequest("DELETE", fmt.Sprintf("%s/records/%s", *addr, id), nil)
	})
	defer resp.Body.Close()
	t.Logf("DELETE /records/%s response: %s", id, string(body))
}

func addDHCPEntry(t *testing.T, id, ip, gw, bootzUrl string) {
	req := map[string]any{"id": id, "ip": ip, "gw": gw, "bootz_urls": []string{bootzUrl}}
	body, _ := json.Marshal(req)
	t.Logf("POST /records body: %s", string(body))
	// Retry for up to ~5 minutes with 30s intervals
	backoffs := backoffFn(5*time.Minute, 30*time.Second)
	resp, respBody := doRequestWithRetry(t, "POST /records", backoffs, func() (*http.Request, error) {
		return http.NewRequest("POST", fmt.Sprintf("%s/records", *addr), bytes.NewReader(body))
	})
	defer resp.Body.Close()
	t.Logf("POST /records response: %s", string(respBody))
}
