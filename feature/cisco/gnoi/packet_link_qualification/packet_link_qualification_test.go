package packet_link_qualification_test

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/gnoigo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGnmiSubscriptionDuringPlq(t *testing.T) {
	/*
		during test, t1 ping t2 traceroute
	*/
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	d1p := dut1.Port(t, "port1")
	d2p := dut2.Port(t, "port1")
	t.Logf("dut1: %v, dut2: %v", dut1.Name(), dut2.Name())
	t.Logf("dut1 dp1 name: %v, dut2 dp2 name : %v", d1p.Name(), d2p.Name())

	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	gnoiClient2 := dut2.RawAPIs().GNOI(t)
	clients := []gnoigo.Clients{gnoiClient1, gnoiClient2}
	duts := []*ondatra.DUTDevice{dut1, dut2}

	capResponse, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	t.Logf("LinkQualification().CapabilitiesResponse: %v", capResponse)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	}
	plqID := dut1.Name() + ":" + d1p.Name() + "<->" + dut2.Name() + ":" + d2p.Name()

	for i := 0; i < len(clients) && i < len(duts); i++ {
		listAndDeleteResults(t, clients[i], duts[i])
	}

	genCreateReq := generatorCreateRequest(t, plqID, d1p, capResponse)
	genCreateResp, err := gnoiClient1.LinkQualification().Create(context.Background(), genCreateReq)
	t.Logf("LinkQualification().Create() GeneratorCreateResponse: %v, err: %v", genCreateResp, err)
	if err != nil {
		t.Fatalf("Failed to handle generator LinkQualification().Create() for Generator: %v", err)
	}
	if got, want := genCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
		t.Errorf("generatorCreateResp: got %v, want %v", got, want)
	}

	refCreateReq := reflectorCreateRequest(t, plqID, d2p)
	refCreateResp, err := gnoiClient2.LinkQualification().Create(context.Background(), refCreateReq)
	t.Logf("LinkQualification().Create() ReflectorCreateResponse: %v, err: %v", refCreateResp, err)
	if err != nil {
		t.Fatalf("Failed to handle generator LinkQualification().Create() for Reflector: %v", err)
	}
	if got, want := refCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
		t.Errorf("reflectorCreateResponse: got %v, want %v", got, want)
	}

	sleepTime := 30 * time.Second
	minTestTime := plqDuration.testDuration + plqDuration.preSyncDuration + plqDuration.reflectorPostSyncDuration + plqDuration.setupDuration + plqDuration.tearDownDuration
	counter := int(minTestTime.Seconds())/int(sleepTime.Seconds()) + 2
	genRunning, refRunning := false, false // flags for Generator and Reflector test status
	for i := 0; i <= counter; i++ {
		t.Logf("Wait for %v seconds: %d/%d", sleepTime.Seconds(), i+1, counter)
		time.Sleep(sleepTime)
		t.Logf("Check client. Iteration: %d", i+1)
		generatorGetResponse, err := gnoiClient1.LinkQualification().Get(context.Background(), &plqpb.GetRequest{Ids: []string{plqID}})
		if err != nil {
			t.Fatalf("Failed to handle gnoi LinkQualification().Get() for testID %v on Generator: %v", plqID, err)
		}
		reflectorGetResponse, err := gnoiClient2.LinkQualification().Get(context.Background(), &plqpb.GetRequest{Ids: []string{plqID}})
		if err != nil {
			t.Fatalf("Failed to handle gnoi LinkQualification().Get() for testID %v on Reflector: %v", plqID, err)
		}
		if generatorGetResponse.GetResults()[plqID].GetState() == plqpb.QualificationState_QUALIFICATION_STATE_RUNNING {
			genRunning = true
		}
		if reflectorGetResponse.GetResults()[plqID].GetState() == plqpb.QualificationState_QUALIFICATION_STATE_RUNNING {
			refRunning = true
		}
		if genRunning && refRunning {
			// subscribe to interface path while test is running
			t.Run("Subscribe to interface path on Generator during PLQ", func(t *testing.T) {
				state := gnmi.OC().Interface(d1p.Name()).AdminStatus().State()
				stateGot := gnmi.Get(t, dut1, state)
				if got, want := stateGot.String(), "UP"; got != want {
					t.Errorf("Interface status via gnmi during PLQ. Got %v, Want %v on Generator", got, want)
				}
			})
			t.Run("Subscribe to interface path on Reflector during PLQ", func(t *testing.T) {
				state := gnmi.OC().Interface(d2p.Name()).AdminStatus().State()
				stateGot := gnmi.Get(t, dut2, state)
				if got, want := stateGot.String(), "UP"; got != want {
					t.Errorf("Interface status via gnmi during PLQ. Got %v, Want %v on Reflector", got, want)
				}
			})
			break
		}
	}
	if genRunning && refRunning == false {
		t.Fatalf("PLQ test did not reach the desired RUNNING state. genRunning: %v, refRunning: %v", genRunning, refRunning)
	}
}

func TestPlqInvalidEndpoints(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	d1p := dut1.Port(t, "port1")
	d2p := dut2.Port(t, "port1")
	t.Logf("dut1: %v, dut2: %v", dut1.Name(), dut2.Name())
	t.Logf("dut1 dp1 name: %v, dut2 dp2 name : %v", d1p.Name(), d2p.Name())

	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	gnoiClient2 := dut2.RawAPIs().GNOI(t)
	clients := []gnoigo.Clients{gnoiClient1, gnoiClient2}
	duts := []*ondatra.DUTDevice{dut1, dut2}

	var wg sync.WaitGroup
	for i := 0; i < len(clients) && i < len(duts); i++ {
		wg.Add(1)
		go func(client gnoigo.Clients, dev *ondatra.DUTDevice) {
			defer wg.Done()
			listAndDeleteResults(t, client, dev)
		}(clients[i], duts[i])
	}
	wg.Wait()

	capResponse, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	t.Logf("LinkQualification().CapabilitiesResponse: %v", capResponse)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	}
	t.Logf("LinkQualification().CapabilitiesResponse: %v", capResponse)
	plqID := dut1.Name() + ":" + d1p.Name() + "<->" + dut2.Name() + ":" + d2p.Name()

	t.Run("Test Generator as Packet Injector", func(t *testing.T) {
		_qc := &plqpb.QualificationConfiguration{
			Id:            plqID,
			InterfaceName: d1p.Name(),
			Timing:        timing,
		}
		_qc.EndpointType = &plqpb.QualificationConfiguration_PacketInjector{
			PacketInjector: &plqpb.PacketInjectorConfiguration{
				PacketSize:   uint32(256),
				PacketCount:  uint32(1000),
				LoopbackMode: &plqpb.PacketInjectorConfiguration_PmdLoopback{PmdLoopback: &plqpb.PmdLoopbackConfiguration{}},
			},
		}
		gcr := &plqpb.CreateRequest{
			Interfaces: []*plqpb.QualificationConfiguration{_qc},
		}
		t.Logf("generatorCreateRequest: %v", gcr)

		generatorCreateResp, err := gnoiClient1.LinkQualification().Create(context.Background(), gcr)
		t.Logf("LinkQualification().Create() generatorCreateResp: %v, err: %v", generatorCreateResp, err)
		if err != nil {
			t.Fatalf("Failed to handle generator LinkQualification().Create(): %v", err)
		}
		if generatorCreateResp.GetStatus()[plqID].GetCode() != int32(3) {
			t.Errorf("Invalid Argument is expected as Packet Injector generator mode is not supported")
		}

	})

	t.Run("Test Reflector as ASIC Loopback", func(t *testing.T) {
		rc := &plqpb.QualificationConfiguration{
			Id:            plqID,
			InterfaceName: d2p.Name(),
			Timing:        timing,
			EndpointType:  &plqpb.QualificationConfiguration_AsicLoopback{AsicLoopback: &plqpb.AsicLoopbackConfiguration{}},
		}
		rcr := &plqpb.CreateRequest{
			Interfaces: []*plqpb.QualificationConfiguration{rc},
		}
		t.Logf("ReflectorCreateRequest: %v", rcr)
		reflectorCreateResp, err := gnoiClient2.LinkQualification().Create(context.Background(), rcr)
		t.Logf("LinkQualification().Create() reflectorCreateResp: %v, err: %v", reflectorCreateResp, err)
		if err != nil {
			t.Fatalf("Failed to handle reflector LinkQualification().Create(): %v", err)
		}
		if reflectorCreateResp.GetStatus()[plqID].GetCode() != int32(3) {
			t.Errorf("Invalid Argument is expected as ASIC loopback mode reflector is not supported")
		}
	})
}

func TestPlqGeneratorRequest(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	d1p := dut1.Port(t, "port1")
	d2p := dut2.Port(t, "port2")
	t.Logf("dut1: %v, dut2: %v", dut1.Name(), dut2.Name())
	t.Logf("dut1 dp1 name: %v, dut2 dp2 name : %v", d1p.Name(), d2p.Name())
	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	//plqID := dut1.Name() + ":" + d1p.Name() + "<->" + dut2.Name() + ":" + d2p.Name()

	interfaceNameCases := []struct {
		desc          string
		plqID         string
		interfaceName string
		want          int32
	}{
		{
			desc:          "Generator Create with unsupported interface Bundle-Ether600",
			plqID:         "wrongIf1",
			interfaceName: "Bundle-Ether600",
			want:          int32(3),
		},
		{
			desc:          "Generator Create with unsupported interface MgmtEth0/RP0/CPU0/0",
			plqID:         "wrongIf2",
			interfaceName: "MgmtEth0/RP0/CPU0/0",
			want:          int32(3),
		},
		{
			desc:          "Generator Create with unsupported interface name Loopback0",
			plqID:         "wrongIf3",
			interfaceName: "Loopback0",
			want:          int32(3),
		},
		{
			desc:          "Generator Create with non-existing interface name HundredGigE0/20/0/19",
			plqID:         "wrongIf4",
			interfaceName: "HundredGigE0/20/0/19",
			want:          int32(3),
		},
	}

	// check if interface name is valid
	for _, tc := range interfaceNameCases {
		t.Run(tc.desc, func(t *testing.T) {
			_generatorCreateRequest := &plqpb.CreateRequest{
				Interfaces: []*plqpb.QualificationConfiguration{
					{
						Id:            tc.plqID,
						InterfaceName: tc.interfaceName,
						EndpointType: &plqpb.QualificationConfiguration_PacketGenerator{
							PacketGenerator: &plqpb.PacketGeneratorConfiguration{
								PacketRate: uint64(138888),
								PacketSize: uint32(256),
							},
						},
						Timing: &plqpb.QualificationConfiguration_Rpc{
							Rpc: &plqpb.RPCSyncedTiming{
								Duration:         durationpb.New(plqDuration.testDuration),
								PreSyncDuration:  durationpb.New(plqDuration.preSyncDuration),
								SetupDuration:    durationpb.New(plqDuration.setupDuration),
								PostSyncDuration: durationpb.New(plqDuration.generatorPostSyncDuration),
								TeardownDuration: durationpb.New(plqDuration.tearDownDuration),
							},
						},
					},
				},
			}
			_generatorCreateResp, err := gnoiClient1.LinkQualification().Create(context.Background(), _generatorCreateRequest)
			t.Logf("LinkQualification().Create() generatorCreateResp: %v, err: %v", _generatorCreateResp, err)
			if err != nil {
				t.Fatalf("Failed to handle generator LinkQualification().Create(): %v", err)
			}
			_generatorGetResponse, err := gnoiClient1.LinkQualification().Get(context.Background(), &plqpb.GetRequest{Ids: []string{tc.plqID}})
			if err != nil {
				t.Fatalf("Failed to handle gnoi LinkQualification().Get() on Generator: %v", err)
			}
			t.Logf("PLQ status is %v", _generatorGetResponse.GetResults()[tc.plqID].GetState())
		})
	}
}

func TestPlqDeletePlqTestDuringQualification(t *testing.T) {
	/*
		create plq request
		delete on generator
		create same request again
		delete on reflector
		create request and ensure it is working
	*/
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	d1p := dut1.Port(t, "port1")
	d2p := dut2.Port(t, "port1")
	t.Logf("dut1: %v, dut2: %v", dut1.Name(), dut2.Name())
	t.Logf("dut1 dp1 name: %v, dut2 dp2 name : %v", d1p.Name(), d2p.Name())

	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	gnoiClient2 := dut2.RawAPIs().GNOI(t)
	clients := []gnoigo.Clients{gnoiClient1, gnoiClient2}
	duts := []*ondatra.DUTDevice{dut1, dut2}

	var wg sync.WaitGroup
	for i := 0; i < len(clients) && i < len(duts); i++ {
		wg.Add(1)
		go func(client gnoigo.Clients, dev *ondatra.DUTDevice) {
			defer wg.Done()
			listAndDeleteResults(t, client, dev)
		}(clients[i], duts[i])
	}
	wg.Wait()

	capResponse, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	t.Logf("LinkQualification().CapabilitiesResponse: %v", capResponse)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	}
	plqID := dut1.Name() + ":" + d1p.Name() + "<->" + dut2.Name() + ":" + d2p.Name()

	t.Run("Delete during qualification on Generator", func(t *testing.T) {
		genCreateReq := generatorCreateRequest(t, plqID, d1p, capResponse)
		genCreateResp, err := gnoiClient1.LinkQualification().Create(context.Background(), genCreateReq)
		t.Logf("LinkQualification().Create() GeneratorCreateResponse: %v, err: %v", genCreateResp, err)
		if err != nil {
			t.Fatalf("Failed to handle generator LinkQualification().Create() for Generator: %v", err)
		}
		if got, want := genCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
			t.Errorf("generatorCreateResp: got %v, want %v", got, want)
		}

		refCreateReq := reflectorCreateRequest(t, plqID, d2p)
		refCreateResp, err := gnoiClient2.LinkQualification().Create(context.Background(), refCreateReq)
		t.Logf("LinkQualification().Create() ReflectorCreateResponse: %v, err: %v", refCreateResp, err)
		if err != nil {
			t.Fatalf("Failed to handle generator LinkQualification().Create() for Reflector: %v", err)
		}
		if got, want := refCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
			t.Errorf("reflectorCreateResponse: got %v, want %v", got, want)
		}

		sleepTime := 30 * time.Second
		minTestTime := plqDuration.testDuration + plqDuration.preSyncDuration + plqDuration.reflectorPostSyncDuration + plqDuration.setupDuration + plqDuration.tearDownDuration
		counter := int(minTestTime.Seconds())/int(sleepTime.Seconds()) + 2
		for i := 0; i <= counter; i++ {
			t.Logf("Wait for %v seconds: %d/%d", sleepTime.Seconds(), i+1, counter)
			time.Sleep(sleepTime)
			t.Logf("Check client: %d", i+1)

			listResp, err := gnoiClient1.LinkQualification().List(context.Background(), &plqpb.ListRequest{})
			t.Logf("LinkQualification().List(): %v, err: %v", listResp, err)
			if err != nil {
				t.Fatalf("Failed to handle gnoi LinkQualification().List(): %v", err)
			}
			if listResp.GetResults()[0].GetState() == plqpb.QualificationState_QUALIFICATION_STATE_RUNNING {
				// Test is underway. Delete the test on Generator
				deleteResp, err := gnoiClient1.LinkQualification().Delete(context.Background(), &plqpb.DeleteRequest{Ids: []string{plqID}})
				t.Logf("LinkQualification().Delete(): %v, err: %v", deleteResp, err)
				if err != nil {
					t.Fatalf("Failed to handle gnoi LinkQualification().Delete() on Generator: %v", err)
				}
				break
			}
		}
	})

	t.Run("Delete during qualification on Reflector", func(t *testing.T) {
		// Delete the previous entries on Generator and Reflector
		var wg sync.WaitGroup
		for i := 0; i < len(clients) && i < len(duts); i++ {
			wg.Add(1)
			go func(client gnoigo.Clients, dev *ondatra.DUTDevice) {
				defer wg.Done()
				listAndDeleteResults(t, client, dev)
			}(clients[i], duts[i])
		}
		wg.Wait()

		time.Sleep(5 * time.Second)

		refCreateReq := reflectorCreateRequest(t, plqID, d2p)
		refCreateResp, err := gnoiClient2.LinkQualification().Create(context.Background(), refCreateReq)
		t.Logf("LinkQualification().Create() ReflectorCreateResponse: %v, err: %v", refCreateResp, err)
		if err != nil {
			t.Fatalf("Failed to handle generator LinkQualification().Create() for Reflector: %v", err)
		}
		if got, want := refCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
			t.Errorf("reflectorCreateResponse: got %v, want %v", got, want)
		}

		genCreateReq := generatorCreateRequest(t, plqID, d1p, capResponse)
		genCreateResp, err := gnoiClient1.LinkQualification().Create(context.Background(), genCreateReq)
		t.Logf("LinkQualification().Create() GeneratorCreateResponse: %v, err: %v", genCreateResp, err)
		if err != nil {
			t.Fatalf("Failed to handle generator LinkQualification().Create() for Generator: %v", err)
		}
		if got, want := genCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
			t.Errorf("generatorCreateResp: got %v, want %v", got, want)
		}

		sleepTime := 30 * time.Second
		minTestTime := plqDuration.testDuration + plqDuration.preSyncDuration + plqDuration.reflectorPostSyncDuration + plqDuration.setupDuration + plqDuration.tearDownDuration
		counter := int(minTestTime.Seconds())/int(sleepTime.Seconds()) + 2
		for i := 0; i <= counter; i++ {
			t.Logf("Wait for %v seconds: %d/%d", sleepTime.Seconds(), i+1, counter)
			time.Sleep(sleepTime)
			t.Logf("Check client: %d", i+1)

			listResp, err := gnoiClient2.LinkQualification().List(context.Background(), &plqpb.ListRequest{})
			t.Logf("LinkQualification().List(): %v, err: %v", listResp, err)
			if err != nil {
				t.Fatalf("Failed to handle gnoi LinkQualification().List(): %v", err)
			}
			if listResp.GetResults()[0].GetState() == plqpb.QualificationState_QUALIFICATION_STATE_RUNNING {
				// Test is underway. Delete the test on Reflector
				deleteResp, err := gnoiClient2.LinkQualification().Delete(context.Background(), &plqpb.DeleteRequest{Ids: []string{plqID}})
				t.Logf("LinkQualification().Delete(): %v, err: %v", deleteResp, err)
				if err != nil {
					t.Fatalf("Failed to handle gnoi LinkQualification().Delete() on Generator: %v", err)
				}
				break
			}
		}
	})

	basePlqSingleInterface(t, dut1, dut2, gnoiClient1, gnoiClient2, d1p, d2p, plqID)

}

func TestPlqSingleInterface(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	dp1 := dut1.Port(t, "port1")
	dp2 := dut2.Port(t, "port1")
	t.Logf("dut1: %v, dut2: %v", dut1.Name(), dut2.Name())
	t.Logf("dut1 dp1 name: %v, dut2 dp2 name : %v", dp1.Name(), dp2.Name())

	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	gnoiClient2 := dut2.RawAPIs().GNOI(t)
	clients := []gnoigo.Clients{gnoiClient1, gnoiClient2}
	duts := []*ondatra.DUTDevice{dut1, dut2}

	var wg sync.WaitGroup
	for i := 0; i < len(clients) && i < len(duts); i++ {
		wg.Add(1)
		go func(client gnoigo.Clients, dev *ondatra.DUTDevice) {
			defer wg.Done()
			listAndDeleteResults(t, client, dev)
		}(clients[i], duts[i])
	}
	wg.Wait()

	capResponse, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("Failed to handle generator LinkQualification().Capabilities() for Generator: %v", err)
	}
	t.Logf("LinkQualification().CapabilitiesResponse: %v", capResponse)
	plqID := dut1.Name() + ":" + dp1.Name() + "<->" + dut2.Name() + ":" + dp2.Name()

	genCreateReq := generatorCreateRequest(t, plqID, dp1, capResponse)
	genCreateResp, err := gnoiClient1.LinkQualification().Create(context.Background(), genCreateReq)
	t.Logf("LinkQualification().Create() GeneratorCreateResponse: %v, err: %v", genCreateResp, err)
	if err != nil {
		t.Fatalf("Failed to handle generator LinkQualification().Create() for Generator: %v", err)
	}

	refCreateReq := reflectorCreateRequest(t, plqID, dp2)
	refCreateResp, err := gnoiClient2.LinkQualification().Create(context.Background(), refCreateReq)
	t.Logf("LinkQualification().Create() ReflectorCreateResponse: %v, err: %v", refCreateResp, err)
	if err != nil {
		t.Fatalf("Failed to handle generator LinkQualification().Create() for Reflector: %v", err)
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

	var generatorPktsSent, generatorPktsRxed uint64
	var dev string
	for i, client := range []gnoigo.Clients{gnoiClient1, gnoiClient2} {
		if client == gnoiClient1 {
			dev = "DUT1 Generator"
		} else {
			dev = "DUT2 Reflector"
		}
		t.Logf("Check client: %d", i+1)
		getResp, err := client.LinkQualification().Get(context.Background(), getRequest)
		if err != nil {
			t.Fatalf("Failed to handle LinkQualification().Get(): %v", err)
		}

		result := getResp.GetResults()[plqID]
		if got, want := result.GetStatus().GetCode(), int32(0); got != want {
			t.Errorf("result.GetStatus().GetCode(): got %v, want %v", got, want)
		}
		if got, want := result.GetState(), plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED; got != want {
			t.Logf("result.GetState() on %v = %v", dev, result.GetState().String())
			t.Errorf("result.GetState() failed on %v: got %v, want %v", dev, got, want)
		}
		if got, want := result.GetPacketsError(), uint64(0); got != want {
			t.Errorf("result.GetPacketsError() failed on %v: got %v, want %v", dev, got, want)
		}
		if got, want := result.GetPacketsDropped(), uint64(0); got != want {
			t.Errorf("result.GetPacketsDropped() failed on %v: got %v, want %v", dev, got, want)
		}

		if client == gnoiClient1 {
			generatorPktsSent = result.GetPacketsSent()
			generatorPktsRxed = result.GetPacketsReceived()
		}
	}

	// The difference in counters between Generator sent packets & received packets
	// mismatch tolerance level in percentage
	var tolerance = 0.0001
	if (math.Abs(float64(generatorPktsSent)-float64(generatorPktsRxed))/float64(generatorPktsSent))*100.00 > tolerance {
		t.Errorf("The difference between packets received count and packets sent count of Generator is greater than %0.4f percent: generatorPktsSent %v, generatorPktsRxed %v", tolerance, generatorPktsSent, generatorPktsRxed)
	}
}
