package healthz_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	hpb "github.com/openconfig/gnoi/healthz"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestInvalidGetRpc(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)
	//platform := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	componentNames := []map[string]string{{"name": "0/RP3/CPU0-CHAS_INLET_T_I_T"}, {"name": "0/PM4-CHASSIS"}}
	for _, componentName := range componentNames {
		req := &hpb.GetRequest{
			Path: &tpb.Path{
				Origin: "openconfig",
				Elem: []*tpb.PathElem{
					{Name: "components"},
					{Name: "component", Key: componentName},
				},
			},
		}
		getResp, err := gnoiClient.Healthz().Get(context.Background(), req)
		t.Logf("Get response: %v", getResp)
		if err != nil {
			t.Errorf("Error on Get(%q): %v", componentName, err)
		}
	}
}

func TestInvalidListRpc(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)
	componentNames := []map[string]string{{"name": "0/RP3/CPU0-CHAS_INLET_T_I_T"}, {"name": "0/PM4-CHASSIS"}}
	for _, componentName := range componentNames {
		listReq := &hpb.ListRequest{
			Path: &tpb.Path{
				Elem: []*tpb.PathElem{
					{Name: "components"},
					{Name: "component", Key: componentName},
				},
			},
		}
		listResp, err := gnoiClient.Healthz().List(context.Background(), listReq)
		t.Logf("List response: %v", listResp)
		if err == nil {
			t.Errorf("Error on List(%q): %v", componentName, err)
		}
	}
}

func TestInvalidCheckRpc(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnoiClient := dut.RawAPIs().GNOI(t)
	// crash a process to create an event on active RP
	killProcessRequest := &spb.KillProcessRequest{
		Signal:  spb.KillProcessRequest_SIGNAL_KILL,
		Name:    "ifmgr",
		Pid:     findProcessByName(t, dut, "ifmgr"),
		Restart: true,
	}
	killProcessResp, err := gnoiClient.System().KillProcess(context.Background(), killProcessRequest)
	t.Logf("KillProcess response: %v", killProcessResp)
	if err != nil {
		t.Fatalf("Failed to restart process: %v", err)
	}

	var activeRp string
	rpList := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	t.Logf("Found RP list: %v", rpList)
	if len(rpList) < 2 {
		activeRp = "0/RP0/CPU0"
	} else {
		standbyRpName, activeRpName := components.FindStandbyRP(t, dut, rpList)
		t.Logf("Standby RP: %v, Active RP: %v", standbyRpName, activeRpName)
		activeRp = activeRpName
	}

	t.Run("Test Check RPC with invalid component and valid event-id", func(t *testing.T) {
		t.Skip() // need to check if this is supported today
		if len(rpList) != 2 {
			t.Skipf("Need 2 Route Processors to test with ")
		}
		// get the event-id for the above crash
		getReq := &hpb.GetRequest{
			Path: &tpb.Path{
				Origin: "openconfig",
				Elem: []*tpb.PathElem{
					{
						Name: "components",
					},
					{
						Name: "component",
						Key:  map[string]string{"name": activeRp},
					},
				},
			},
		}
		getResp, err := gnoiClient.Healthz().Get(context.Background(), getReq)
		t.Logf("Get response: %v", getResp)
		if err != nil {
			t.Errorf("Error on Get(%q): %v", activeRp, err)
		}
		eventId := getResp.GetComponent().GetId()
		if eventId == "" {
			t.Errorf("Get returned an empty event_id")
		}
		componentNames := []map[string]string{{"name": "0/RP3/CPU0-CHASSIS"}}
		for _, componentName := range componentNames {
			checkReq := &hpb.CheckRequest{
				Path: &tpb.Path{
					Origin: "openconfig",
					Elem: []*tpb.PathElem{
						{Name: "components"},
						{Name: "component", Key: componentName},
					},
				},
				EventId: eventId,
			}
			checkResp, err := gnoiClient.Healthz().Check(context.Background(), checkReq)
			t.Logf("Check response: %v", checkResp)
			if err == nil {
				t.Errorf("Error on Check(%q): %v", componentName, err)
			}
		}
	})

	t.Run("Test Check RPC with invalid event-id on valid component", func(t *testing.T) {
		componentNames := []map[string]string{{"name": activeRp}}
		for _, componentName := range componentNames {
			checkReq := &hpb.CheckRequest{
				Path: &tpb.Path{
					Elem: []*tpb.PathElem{
						{Name: "components"},
						{Name: "component", Key: componentName},
					},
				},
				EventId: "123478097116782", // random/incorrect event_id
			}
			checkResp, err := gnoiClient.Healthz().Check(context.Background(), checkReq)
			t.Logf("Check response: %v", checkResp)
			if err == nil {
				t.Errorf("Error on Check(%q): %v", componentName, err)
			}
		}
	})
}

// findProcessByName uses telemetry to find out the PID of a process
func findProcessByName(t *testing.T, dut *ondatra.DUTDevice, pName string) uint32 {
	t.Helper()
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	var pID uint32
	for _, proc := range pList {
		if proc.GetName() == pName {
			pID = uint32(proc.GetPid())
			t.Logf("Pid of daemon '%s' is '%d'", pName, pID)
		}
	}
	return pID
}
