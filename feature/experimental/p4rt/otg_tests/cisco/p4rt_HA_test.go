package cisco_p4rt_test

import (
	"context"
	"fmt"
	"testing"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
)

var (
	P4RTHATestcase = []Testcase{
		{
			name: "Process restart sysdb",
			desc: "P4RT HA: Sysdb Restart",
			fn:   makeProcessRestartTestcases(testHAProcessRestart, "sysdb_mc"),
		},
		{
			name: "Process restart netio",
			desc: "P4RT HA: netio Restart",
			fn:   makeProcessRestartTestcases(testHAProcessRestart, "netio"),
		},
		{
			name: "Process restart lpts",
			desc: "P4RT HA: lpts Restart",
			fn:   makeProcessRestartTestcases(testHAProcessRestart, "lpts_fm"),
		},
		{
			name: "Process restart spio",
			desc: "P4RT HA: spio Restart",
			fn:   makeProcessRestartTestcases(testHAProcessRestart, "spio_ea"),
		},
		{
			name: "Process restart ifmgr",
			desc: "P4RT HA: ifmgr Restart",
			fn:   makeProcessRestartTestcases(testHAProcessRestart, "ifmgr"),
		},
		{
			name: "Process restart emsd",
			desc: "P4RT HA: emsd Restart",
			fn:   makeProcessRestartTestcases(testHAProcessRestart, "emsd"),
		},
		// {
		// 	name: "Process restart emsd",
		// 	desc: "P4RT HA: emsd Restart",
		// 	fn:   testHAProcessRestartEmsd,
		// },
	}
)

func makeProcessRestartTestcases(fn func(context.Context, *testing.T, string, *testArgs), processName string) func(context.Context, *testing.T, *testArgs) {
	return func(ctx context.Context, t *testing.T, args *testArgs) { fn(ctx, t, processName, args) }
}

// common HA Process Restart
func testHAProcessRestart(ctx context.Context, t *testing.T, processName string, args *testArgs) {
	backupClients := []*p4rt_client.P4RTClient{args.p4rtClientB, args.p4rtClientC, args.p4rtClientD}

	// Apply Process restart
	cli := fmt.Sprint("process restart ", processName)
	config.CMDViaGNMI(ctx, t, args.dut, cli)

	psp := p4rt_client.P4RTStreamParameters{
		Name:        fmt.Sprint(deviceID),
		DeviceId:    deviceID,
		ElectionIdH: 0,
		ElectionIdL: electionID,
	}

	t.Logf("Verify SetForwardingPipeline.")
	for i := 0; i < len(args.npus); i++ {
		psp.DeviceId = deviceID + uint64(i)
		psp.Name = fmt.Sprint(psp.DeviceId)
		if err := setupForwardingPipeline(ctx, t, psp, args.p4rtClientA); err != nil {
			t.Fatalf("When SetForwardingPipeline on primary client, there is error after process restart, %v, device-id: %v", err, psp.DeviceId)
		}
		for j, client := range backupClients {
			psp.ElectionIdL = psp.ElectionIdL - uint64(j+1)
			if err := setupForwardingPipeline(ctx, t, psp, client); err == nil {
				t.Fatalf("When SetForwardingPipeline on backup client, there is error after process restart, %v, device-id: %v", err, psp.DeviceId)
			}
			psp.ElectionIdL = electionID
		}
	}

	// Apply Process restart
	config.CMDViaGNMI(ctx, t, args.dut, cli)

	t.Logf("Verify Write RPC.")
	for i := 0; i < len(args.npus); i++ {
		psp.DeviceId = deviceID + uint64(i)
		psp.Name = fmt.Sprint(psp.DeviceId)
		if err := programmGDPMatchEntryWithStreamParameter(psp, args.p4rtClientA, false); err != nil {
			t.Fatalf("When Write on primary client, there is error after process restart, %v, %v", err, psp.DeviceId)
		}
		defer programmGDPMatchEntryWithStreamParameter(psp, args.p4rtClientA, true)
		for j, client := range backupClients {
			psp.ElectionIdL = psp.ElectionIdL - uint64(j+1)
			if err := programmGDPMatchEntryWithStreamParameter(psp, client, false); err == nil {
				t.Fatalf("When Write on backup client, there is error after process restart, %v", err)
			}
			psp.ElectionIdL = electionID
		}
	}

	// Apply Process restart
	config.CMDViaGNMI(ctx, t, args.dut, cli)

	t.Logf("Verify Read RPC.")
	for i := 0; i < len(args.npus); i++ {
		for _, client := range append([]*p4rt_client.P4RTClient{args.p4rtClientA}, backupClients...) {
			entries, err := readProgrammedEntry(ctx, t, deviceID+uint64(i), client)
			if err != nil {
				t.Fatalf("When performing Read RPC, there is an error %v", err)
			}
			for _, entry := range entries {
				if !compareEntry(ctx, t, gdpTableEntry, entry) {
					t.Fatalf("The entry is not seen in the response %v", entry.String())
				}
			}

		}
	}
}

// Comment out those 2 functions to meet StaticAnalysis
// HA Process Restart emsd
// func testHAProcessRestartEmsd(ctx context.Context, t *testing.T, args *testArgs) {
// 	backupClients := []*p4rt_client.P4RTClient{args.p4rtClientB, args.p4rtClientC, args.p4rtClientD}

// 	// Apply Process restart
// 	cli := fmt.Sprint("process restart emsd")
// 	config.CMDViaGNMI(ctx, t, args.dut, cli)

// 	if err := recoverP4RTClients(ctx, t, args); err != nil {
// 		t.Fatalf("Could not recover p4rt client: %v", err)
// 	}

// 	psp := p4rt_client.P4RTStreamParameters{
// 		Name:        fmt.Sprint(deviceID),
// 		DeviceId:    deviceID,
// 		ElectionIdH: 0,
// 		ElectionIdL: electionID,
// 	}

// 	for i := 0; i < len(args.npus); i++ {
// 		psp.DeviceId = deviceID + uint64(i)
// 		psp.Name = fmt.Sprint(psp.DeviceId)
// 		if err := setupForwardingPipeline(ctx, t, psp, args.p4rtClientA); err != nil {
// 			t.Fatalf("When SetForwardingPipeline on primary client, there is error after process restart, %v, device-id: %v", err, psp.DeviceId)
// 		}
// 		for j, client := range backupClients {
// 			psp.ElectionIdL = psp.ElectionIdL - uint64(j+1)
// 			if err := setupForwardingPipeline(ctx, t, psp, client); err == nil {
// 				t.Fatalf("When SetForwardingPipeline on backup client, there is error after process restart, %v, device-id: %v", err, psp.DeviceId)
// 			}
// 			psp.ElectionIdL = electionID
// 		}
// 	}
// }

// func recoverP4RTClients(ctx context.Context, t *testing.T, args *testArgs) error {
// 	args.p4rtClientA.ServerDisconnect()
// 	args.p4rtClientB.ServerDisconnect()
// 	args.p4rtClientC.ServerDisconnect()
// 	args.p4rtClientD.ServerDisconnect()

// 	p4rtClientA := p4rt_client.P4RTClient{}
// 	if err := p4rtClientA.P4rtClientSet(args.dut.RawAPIs().P4RT(t)); err != nil {
// 		return err
// 	}

// 	p4rtClientB := p4rt_client.P4RTClient{}
// 	if err := p4rtClientB.P4rtClientSet(args.dut.RawAPIs().P4RT(t)); err != nil {
// 		return err
// 	}

// 	p4rtClientC := p4rt_client.P4RTClient{}
// 	if err := p4rtClientC.P4rtClientSet(args.dut.RawAPIs().P4RT(t)); err != nil {
// 		return err
// 	}

// 	p4rtClientD := p4rt_client.P4RTClient{}
// 	if err := p4rtClientD.P4rtClientSet(args.dut.RawAPIs().P4RT(t)); err != nil {
// 		return err
// 	}

// 	args.p4rtClientA = &p4rtClientA
// 	args.p4rtClientB = &p4rtClientB
// 	args.p4rtClientC = &p4rtClientC
// 	args.p4rtClientD = &p4rtClientD

// 	if err := setupScaleP4RTClient(ctx, t, args.npus, args); err != nil {
// 		return err
// 	}

// 	return nil
// }
