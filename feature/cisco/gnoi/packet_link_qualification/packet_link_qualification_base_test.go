package packet_link_qualification_test

import (
	"context"
	"math"
	"testing"
	"time"

	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/gnoigo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/protobuf/types/known/durationpb"
)

type linkQualificationDuration struct {
	// time needed to complete preparation
	setupDuration time.Duration
	// time duration to wait before starting link qual preparation
	preSyncDuration time.Duration
	// packet linkqual duration
	testDuration time.Duration
	// time to wait post link-qual before starting teardown
	generatorPostSyncDuration time.Duration
	reflectorPostSyncDuration time.Duration
	// time required to bring the interface back to pre-test state
	tearDownDuration time.Duration
}

var plqDuration = &linkQualificationDuration{
	preSyncDuration:           30 * time.Second,
	setupDuration:             30 * time.Second,
	testDuration:              120 * time.Second,
	generatorPostSyncDuration: 5 * time.Second,
	reflectorPostSyncDuration: 10 * time.Second,
	tearDownDuration:          30 * time.Second,
}

var timing = &plqpb.QualificationConfiguration_Rpc{
	Rpc: &plqpb.RPCSyncedTiming{
		Duration:         durationpb.New(plqDuration.testDuration),
		PreSyncDuration:  durationpb.New(plqDuration.preSyncDuration),
		SetupDuration:    durationpb.New(plqDuration.setupDuration),
		PostSyncDuration: durationpb.New(plqDuration.reflectorPostSyncDuration),
		TeardownDuration: durationpb.New(plqDuration.tearDownDuration),
	},
}

func generatorCreateRequest(t *testing.T, testID string, dp *ondatra.Port, capResp *plqpb.CapabilitiesResponse) *plqpb.CreateRequest {
	t.Helper()
	qc := &plqpb.QualificationConfiguration{
		Id:            testID,
		InterfaceName: dp.Name(),
		Timing:        timing,
	}

	var maxPPS = capResp.GetGenerator().GetPacketGenerator().GetMaxPps()
	t.Logf("MaxPps: %v", maxPPS)
	var maxMTU = capResp.GetGenerator().GetPacketGenerator().GetMaxMtu()
	t.Logf("MaxMtu: %v", maxMTU)
	var pktRate uint64
	var pktSize uint32

	switch dp.Speed() {
	case ondatra.Speed10Gb:
		pktRate = uint64(13888)
		pktSize = maxMTU
	case ondatra.Speed100Gb:
		pktRate = maxPPS
		pktSize = uint32(256)
	case ondatra.Speed400Gb:
		pktRate = maxPPS
		pktSize = maxMTU
	default:
		pktRate = uint64(13888)
		pktSize = maxMTU
	}
	qc.EndpointType = &plqpb.QualificationConfiguration_PacketGenerator{
		PacketGenerator: &plqpb.PacketGeneratorConfiguration{
			PacketRate: pktRate,
			PacketSize: pktSize,
		},
	}
	gcr := &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{qc},
	}
	t.Logf("GeneratorCreateRequest: %v", gcr)
	return gcr
}

func reflectorCreateRequest(t *testing.T, testID string, dp *ondatra.Port) *plqpb.CreateRequest {
	t.Helper()
	qc := &plqpb.QualificationConfiguration{
		Id:            testID,
		InterfaceName: dp.Name(),
		Timing:        timing,
		EndpointType: &plqpb.QualificationConfiguration_PmdLoopback{
			PmdLoopback: &plqpb.PmdLoopbackConfiguration{},
		},
	}

	rcr := &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{qc},
	}
	t.Logf("ReflectorCreateRequest: %v", rcr)
	return rcr
}

func listAndDeleteResults(t *testing.T, client gnoigo.Clients, dut *ondatra.DUTDevice) {
	t.Helper()
	listResp, err := client.LinkQualification().List(context.Background(), &plqpb.ListRequest{})
	t.Logf("List reponse on %v: %v", dut.Name(), listResp)
	if err != nil {
		t.Fatalf("failed to handle gnoi LinkQualification().List() on %v: error: %v", dut, err)
	}
	if len(listResp.GetResults()) != 0 {
		var idList []string
		for _, result := range listResp.GetResults() {
			id := result.GetId()
			idList = append(idList, id)
		}
		_, err := client.LinkQualification().Delete(context.Background(), &plqpb.DeleteRequest{Ids: idList})
		if err != nil {
			t.Fatalf("failed to handle gnoi LinkQualification().Delete() on %v: error: %v", dut, err)
		}

		listResp, err = client.LinkQualification().List(context.Background(), &plqpb.ListRequest{})
		if err != nil {
			t.Fatalf("failed to handle gnoi LinkQualification().List() on %v: error: %v", dut, err)
		}
		if len(listResp.GetResults()) != 0 {
			t.Fatalf("results found on %v after Delete RPC", dut)
		}

	}
}

// basePlqSingleInterface checks PLQ for a Generator-Reflector pair with the following operations
// List & Delete; Create; Get; Verify traffic;
func basePlqSingleInterface(t *testing.T, dut1, dut2 *ondatra.DUTDevice, gnoiClient1, gnoiClient2 gnoigo.Clients, d1p, d2p *ondatra.Port, plqID string) {
	/*
		list and delete on gen and ref
		create req on gen and ref
		get on both devices
		verify interface status
		check for control plane
		verify traffic loss
	*/
	t.Helper()
	clients := []gnoigo.Clients{gnoiClient1, gnoiClient2}
	duts := []*ondatra.DUTDevice{dut1, dut2}
	// Check for existing PLQ results, and if present, delete them on both Generator and Reflector
	for i := 0; i < len(clients) && i < len(duts); i++ {
		listAndDeleteResults(t, clients[i], duts[i])
	}
	checkInterfaceStatusUp(t, dut1, d1p)
	capResponse, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	t.Logf("LinkQualification().CapabilitiesResponse: %v", capResponse)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	}
	var maxPPS = capResponse.GetGenerator().GetPacketGenerator().GetMaxPps()
	t.Logf("MaxPps supported by Generator: %v", maxPPS)
	var maxMTU = capResponse.GetGenerator().GetPacketGenerator().GetMaxMtu()
	t.Logf("MaxMtu supported by Generator: %v", maxMTU)

	genCreateReq := generatorCreateRequest(t, plqID, d1p, capResponse)
	genCreateResp, err := gnoiClient1.LinkQualification().Create(context.Background(), genCreateReq)
	t.Logf("LinkQualification().Create() GeneratorCreateResponse: %v, err: %v", genCreateResp, err)
	if err != nil {
		t.Fatalf("Failed to handle generator LinkQualification().Create() for Generator: %v", err)
	}
	if got, want := genCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
		t.Fatalf("generatorCreateResp: got %v, want %v", got, want)
	}

	refCreateReq := reflectorCreateRequest(t, plqID, d2p)
	refCreateResp, err := gnoiClient2.LinkQualification().Create(context.Background(), refCreateReq)
	t.Logf("LinkQualification().Create() ReflectorCreateResponse: %v, err: %v", refCreateResp, err)
	if err != nil {
		t.Fatalf("Failed to handle generator LinkQualification().Create() for Reflector: %v", err)
	}
	if got, want := refCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
		checkInterfaceStatusUp(t, dut2, d2p)
		t.Fatalf("reflectorCreateResponse: got %v, want %v", got, want)
	}

	sleepTime := 30 * time.Second
	minTestTime := plqDuration.testDuration + plqDuration.preSyncDuration + plqDuration.reflectorPostSyncDuration + plqDuration.setupDuration + plqDuration.tearDownDuration
	counter := int(minTestTime.Seconds())/int(sleepTime.Seconds()) + 2
	for i := 0; i <= counter; i++ {
		t.Logf("Wait for %v seconds: %d/%d", sleepTime.Seconds(), i+1, counter)
		time.Sleep(sleepTime)
		testDone := true
		for i, client := range []gnoigo.Clients{gnoiClient1, gnoiClient2} {
			t.Logf("Check client: %d", i+1)

			listResp, err := client.LinkQualification().List(context.Background(), &plqpb.ListRequest{})
			t.Logf("LinkQualification().List(): %v, err: %v", listResp, err)
			if err != nil {
				t.Fatalf("Failed to handle gnoi LinkQualification().List(): %v", err)
			}

			for j := 0; j < len(listResp.GetResults()); j++ {
				if listResp.GetResults()[j].GetState() != plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED {
					testDone = false
				}
			}
			if len(listResp.GetResults()) == 0 {
				testDone = false
			}
		}
		if testDone {
			t.Logf("Detected QualificationState_QUALIFICATION_STATE_COMPLETED.")
			break
		}
	}

	getRequest := &plqpb.GetRequest{
		Ids: []string{plqID},
	}
	// NOTE: interface OperStatus as TESTING during PLQ is not supported by Cisco as of January 2024 - CD25

	var generatorPktsSent, generatorPktsRxed uint64
	for i, client := range []gnoigo.Clients{gnoiClient1, gnoiClient2} {
		t.Logf("Check client: %d", i+1)
		getResp, err := client.LinkQualification().Get(context.Background(), getRequest)
		t.Logf("LinkQualification().Get(): %v, err: %v", getResp, err)
		if err != nil {
			t.Fatalf("Failed to handle LinkQualification().Get(): %v", err)
		}

		result := getResp.GetResults()[plqID]
		if got, want := result.GetStatus().GetCode(), int32(0); got != want {
			t.Errorf("result.GetStatus().GetCode(): got %v, want %v", got, want)
		}
		if got, want := result.GetState(), plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED; got != want {
			t.Errorf("result.GetState(): got %v, want %v", got, want)
		}
		if got, want := result.GetPacketsError(), uint64(0); got != want {
			t.Errorf("result.GetPacketsError(): got %v, want %v", got, want)
		}
		if got, want := result.GetPacketsDropped(), uint64(0); got != want {
			t.Errorf("result.GetPacketsDropped(): got %v, want %v", got, want)
		}

		if client == gnoiClient1 {
			generatorPktsSent = result.GetPacketsSent()
			generatorPktsRxed = result.GetPacketsReceived()
		}
	}
	// The packet counters between Generator sent & received mismatch tolerance level in percentage
	var tolerance = 0.0001
	if (math.Abs(float64(generatorPktsSent)-float64(generatorPktsRxed))/float64(generatorPktsSent))*100.00 > tolerance {
		t.Errorf("The difference between packets received count and packets sent count Generator is greater than %0.4f percent: generatorPktsSent %v, generatorPktsRxed %v", tolerance, generatorPktsSent, generatorPktsRxed)
	} else {
		portPair := dut1.Name() + ":" + d1p.Name() + "<->" + dut2.Name() + ":" + d2p.Name()
		t.Logf("Packet Link Qualification successful between the ports %v", portPair)
	}
}

func checkInterfaceStatusUp(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	t.Helper()
	_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), 1*time.Minute, func(y *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		operState, ok := y.Val()
		return ok && operState == oc.Interface_OperStatus_UP
	}).Await(t)
	if !ok {
		t.Fatalf("Interface is not UP between Generator and Reflector")
	}
}
