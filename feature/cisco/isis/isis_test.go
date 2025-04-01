package basetest

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	ft "github.com/openconfig/featureprofiles/tools/inputcisco/feature"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestISISState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	ate := ondatra.ATE(t, ate)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	inputObj.ConfigInterfaces(dut)
	time.Sleep(10 * time.Second)
	inputObj.StartAteProtocols(ate)
	time.Sleep(15 * time.Second)
	isis := inputObj.Device(dut).Features().Isis[0]
	peerIsis := inputObj.ATE(ate).Features().Isis[0]
	isisPath := gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isis.Name).Isis()
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/state/level-number", func(t *testing.T) {
		state := isisPath.Level(uint8(ft.GetIsisLevelType(isis.Level))).LevelNumber()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != uint8(ft.GetIsisLevelType(isis.Level)) {
			t.Errorf("ISIS Level: got %d, want %d", val, ft.GetIsisLevelType(isis.Level))
		}
	})
	intf := isis.Interface[0]
	isisadjPath := isisPath.Interface(intf.Name).Level(uint8(ft.GetIsisLevelType(intf.CircuitType))).Adjacency(peerIsis.Systemid)

	/*
		t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/dis-system-id", func(t *testing.T) {
			state := isisadjPath.DisSystemId()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val != peerIsis.EisrsystemId {
				t.Errorf("ISIS Adj DisSystemId: got %s, want %s", val, "''")
			}
		})
	*/

	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/system-id", func(t *testing.T) {
		state := isisadjPath.SystemId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != peerIsis.Systemid {
			t.Errorf("ISIS Adj SystemId: got %s, want %s", val, peerIsis.Systemid)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-snpa", func(t *testing.T) {
		state := isisadjPath.NeighborSnpa()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("ISIS Adj NeighborsSNPA: got %s, want !=%s", val, "''")
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-status", func(t *testing.T) {
		state := isisadjPath.RestartStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("ISIS Adj RestartStatus: got %t, want %t", val, false)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-support", func(t *testing.T) {
		state := isisadjPath.RestartSupport()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("ISIS Adj RestartSupport: got %t, want %t", val, false)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/restart-suppress", func(t *testing.T) {
		state := isisadjPath.RestartSuppress()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("ISIS Adj RestartSuppress: got %t, want %t", val, false)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/multi-topology", func(t *testing.T) {
		state := isisadjPath.MultiTopology()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("ISIS Adj MultiTopology: got %t, want %t", val, false)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/adjacency-state", func(t *testing.T) {
		state := isisadjPath.AdjacencyState()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.Isis_IsisInterfaceAdjState_UP {
			t.Errorf("ISIS Adj State: got %v, want %v", val, oc.Isis_IsisInterfaceAdjState_UP)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-circuit-type", func(t *testing.T) {
		state := isisadjPath.NeighborCircuitType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.Isis_LevelType_LEVEL_2 {
			t.Errorf("ISIS Adj NeighborCircuitType: got %v, want %v", val, oc.Isis_LevelType_LEVEL_2)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/nlpid", func(t *testing.T) {
		state := isisadjPath.Nlpid()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if cmp.Diff(val, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}) != "" {
			t.Errorf("ISIS Adj Nlpid: got %v, want %v", val, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6})
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/priority", func(t *testing.T) {
		state := isisadjPath.Priority()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS Adj Priority: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/up-timestamp", func(t *testing.T) {
		state := isisadjPath.UpTimestamp()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS Adj UpTimeStamp: got %d, want non-zero value", val)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/local-extended-circuit-id", func(t *testing.T) {
		state := isisadjPath.LocalExtendedCircuitId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS Adj LocalExtendedCircuitId: got %d, want !=%d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-extended-circuit-id", func(t *testing.T) {
		state := isisadjPath.NeighborExtendedCircuitId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS Adj NeighborExtendedCircuitId: got %d, want !=%d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/topology", func(t *testing.T) {
		state := isisadjPath.Topology()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if cmp.Diff(val, []oc.E_IsisTypes_AFI_SAFI_TYPE{oc.IsisTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.IsisTypes_AFI_SAFI_TYPE_IPV6_UNICAST}) != "" {
			t.Errorf("ISIS Adj Topology: got %v, want %v", val, []oc.E_IsisTypes_AFI_SAFI_TYPE{oc.IsisTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.IsisTypes_AFI_SAFI_TYPE_IPV6_UNICAST})
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/area-address", func(t *testing.T) {
		state := isisadjPath.AreaAddress()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		found := false
		for _, v := range val {
			if v == "49.0001" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ISIS Adj AreaAddress: got %v, want should contain %v", val, "49.0001")
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv6-address", func(t *testing.T) {
		state := isisadjPath.NeighborIpv6Address()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != "::" {
			t.Errorf("ISIS Adj NeighborIpv6Address: got %s, want %s", val, "::")
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/adjacencies/adjacency/state/neighbor-ipv4-address", func(t *testing.T) {
		state := isisadjPath.NeighborIpv4Address()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != isis.Neighboraddress {
			t.Errorf("ISIS Adj NeighborIpv4Address: got %s, want %s", val, "::")
		}
	})
	for _, afisafi := range intf.Afisafi {
		afiType, safiType := ft.GetIsisAfiSafiname(afisafi.Type)
		afisafiPath := isisPath.Interface(intf.Name).Level(uint8(ft.GetIsisLevelType(intf.CircuitType))).Af(afiType, safiType)
		t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/afi-name", func(t *testing.T) {
			state := afisafiPath.AfiName()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val != afiType {
				t.Errorf("ISIS AfiSafi AfiName: got %v, want %v", val, afiType)
			}
		})
		t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/safi-name", func(t *testing.T) {
			state := afisafiPath.SafiName()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val != safiType {
				t.Errorf("ISIS AfiSafi SafiName: got %v, want %v", val, safiType)
			}
		})
		t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/afi-safi/af/state/metric", func(t *testing.T) {
			state := afisafiPath.Metric()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val != uint32(afisafi.Metric) {
				t.Errorf("ISIS AfiSafi Metric: got %d, want %d", val, afisafi.Metric)
			}
		})

	}
	csnpCounterPath := isisPath.Interface(intf.Name).Level(uint8(ft.GetIsisLevelType(intf.CircuitType))).PacketCounters().Csnp()
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/csnp/state/dropped", func(t *testing.T) {
		state := csnpCounterPath.Dropped()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS Csnp Counter Dropped: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/csnp/state/retransmit", func(t *testing.T) {
		state := csnpCounterPath.Retransmit()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS Csnp Counter Retransmit: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/csnp/state/processed", func(t *testing.T) {
		state := csnpCounterPath.Processed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS Csnp Counter Processed: got %d, want !=%d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/csnp/state/received", func(t *testing.T) {
		state := csnpCounterPath.Received()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS Csnp Counter Received: got %d, want !=%d", val, 0)
		}
	})
	psnpCounterPath := isisPath.Interface(intf.Name).Level(uint8(ft.GetIsisLevelType(intf.CircuitType))).PacketCounters().Psnp()
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/dropped", func(t *testing.T) {
		state := psnpCounterPath.Dropped()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS Psnp Counter Dropped: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/retransmit", func(t *testing.T) {
		state := psnpCounterPath.Retransmit()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS Psnp Counter Retransmit: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/processed", func(t *testing.T) {
		state := psnpCounterPath.Processed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS Psnp Counter Processed: got %d, want !=%d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/received", func(t *testing.T) {
		state := psnpCounterPath.Received()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS Psnp Counter Received: got %d, want !=%d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/psnp/state/sent", func(t *testing.T) {
		state := psnpCounterPath.Sent()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS Psnp Counter Sent: got %d, want !=%d", val, 0)
		}
	})
	lspCounterPath := isisPath.Interface(intf.Name).Level(uint8(ft.GetIsisLevelType(intf.CircuitType))).PacketCounters().Lsp()
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/dropped", func(t *testing.T) {
		state := lspCounterPath.Dropped()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS lsp Counter Dropped: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/retransmit", func(t *testing.T) {
		state := lspCounterPath.Retransmit()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS lsp Counter Retransmit: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/processed", func(t *testing.T) {
		state := lspCounterPath.Processed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS lsp Counter Processed: got %d, want !=%d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/received", func(t *testing.T) {
		state := lspCounterPath.Received()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS lsp Counter Received: got %d, want !=%d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/lsp/state/sent", func(t *testing.T) {
		state := lspCounterPath.Sent()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS lsp Counter Sent: got %d, want !=%d", val, 0)
		}
	})
	iihCounterPath := isisPath.Interface(intf.Name).Level(uint8(ft.GetIsisLevelType(intf.CircuitType))).PacketCounters().Iih()
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/dropped", func(t *testing.T) {
		state := iihCounterPath.Dropped()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS iih Counter Dropped: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/retransmit", func(t *testing.T) {
		state := iihCounterPath.Retransmit()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS lsp Counter Retransmit: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/processed", func(t *testing.T) {
		state := iihCounterPath.Processed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS iih Counter Processed: got %d, want !=%d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/received", func(t *testing.T) {
		state := iihCounterPath.Received()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS iih Counter Received: got %d, want !=%d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/levels/level/packet-counters/iih/state/sent", func(t *testing.T) {
		state := iihCounterPath.Sent()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 {
			t.Errorf("ISIS iih Counter Sent: got %d, want !=%d", val, 0)
		}
	})
	systemLevelCountersPath := isisPath.Level(uint8(ft.GetIsisLevelType(isis.Level))).SystemLevelCounters()
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/system-level-counters", func(t *testing.T) {
		state := systemLevelCountersPath.AuthFails()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters AuthFails: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/auth-type-fails", func(t *testing.T) {
		state := systemLevelCountersPath.AuthTypeFails()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters AuthTypeFails: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/manual-address-drop-from-areas", func(t *testing.T) {
		state := systemLevelCountersPath.ManualAddressDropFromAreas()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters ManualAddressDropFromAreas: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/part-changes", func(t *testing.T) {
		state := systemLevelCountersPath.PartChanges()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters PartChanges: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/corrupted-lsps", func(t *testing.T) {
		state := systemLevelCountersPath.CorruptedLsps()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters CorruptedLsps: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/spf-runs", func(t *testing.T) {
		state := systemLevelCountersPath.ExceedMaxSeqNums()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters ExceedMaxSeqNums: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/id-len-mismatch", func(t *testing.T) {
		state := systemLevelCountersPath.IdLenMismatch()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters IdLenMismatch: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/lsp-errors", func(t *testing.T) {
		state := systemLevelCountersPath.LspErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters LspErrors: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/max-area-address-mismatches", func(t *testing.T) {
		state := systemLevelCountersPath.MaxAreaAddressMismatches()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters MaxAreaAddressMismatches: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/own-lsp-purges", func(t *testing.T) {
		state := systemLevelCountersPath.OwnLspPurges()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != 0 {
			t.Errorf("ISIS System Level Counters OwnLspPurges: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/seq-num-skips", func(t *testing.T) {
		state := systemLevelCountersPath.SeqNumSkips()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 || val > 0 {
			t.Logf("Got correct value for ISIS System Level Counters SeqNumSkips")
		} else {
			t.Errorf("ISIS System Level Counters SeqNumSkips: got %d, want %d", val, 0)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/system-level-counters/state/spf-runs", func(t *testing.T) {
		state := systemLevelCountersPath.SpfRuns()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		gnmi.Get(t, dut, state.State())
	})
	//store initial values of CircuitCounters
	iCCPath := isisPath.Interface(intf.Name).CircuitCounters()
	iCC := gnmi.Get(t, dut, iCCPath.State())
	flapInterface(t, dut, intf.Name, 30)
	circuitCounters := isisPath.Interface(intf.Name).CircuitCounters()
	time.Sleep(20 * time.Second)
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/adj-changes", func(t *testing.T) {
		state := circuitCounters.AdjChanges()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != iCC.GetAdjChanges()+3 {
			t.Errorf("ISIS CircuitCounters Counters AdjChanges: got %d, want %d", val, iCC.GetAdjChanges()+3)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/adj-number", func(t *testing.T) {
		state := circuitCounters.AdjNumber()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != iCC.GetAdjNumber() {
			t.Errorf("ISIS CircuitCounters AdjNumber: got %d, want %d", val, iCC.GetAdjNumber())
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/state/auth-fails", func(t *testing.T) {
		state := circuitCounters.AuthFails()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != iCC.GetAuthFails() {
			t.Errorf("ISIS CircuitCounters AuthFails: got %d, want %d", val, iCC.GetAuthFails())
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/auth-type-fails", func(t *testing.T) {
		state := circuitCounters.AuthTypeFails()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != iCC.GetAuthTypeFails() {
			t.Errorf("ISIS CircuitCounters AuthTypeFails: got %d, want %d", val, iCC.GetAuthTypeFails())
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/id-field-len-mismatches", func(t *testing.T) {
		state := circuitCounters.IdFieldLenMismatches()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != iCC.GetIdFieldLenMismatches() {
			t.Errorf("ISIS CircuitCounters IdFieldLenMismatches: got %d, want %d", val, iCC.GetIdFieldLenMismatches())
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/init-fails", func(t *testing.T) {
		state := circuitCounters.InitFails()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != iCC.GetInitFails() {
			t.Errorf("ISIS CircuitCounters InitFails: got %d, want %d", val, iCC.GetInitFails())
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/lan-dis-changes", func(t *testing.T) {
		state := circuitCounters.LanDisChanges()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != iCC.GetLanDisChanges() {
			t.Errorf("ISIS CircuitCounters LanDisChanges: got %d, want %d", val, iCC.GetLanDisChanges())
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/max-area-address-mismatches", func(t *testing.T) {
		state := circuitCounters.MaxAreaAddressMismatches()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != iCC.GetMaxAreaAddressMismatches() {
			t.Errorf("ISIS CircuitCounters MaxAreaAddressMismatches: got %d, want %d", val, iCC.GetMaxAreaAddressMismatches())
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/interfaces/interface/circuit-counters/state/rejected-adj", func(t *testing.T) {
		state := circuitCounters.RejectedAdj()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != iCC.GetRejectedAdj() {
			t.Errorf("ISIS CircuitCounters RejectedAdj: got %d, want %d", val, iCC.GetRejectedAdj())
		}
	})

	lsp := isisPath.Level(uint8(2)).
		Lsp(peerIsis.LspId)
	tlvExtv6Prefix := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).Ipv6Reachability().Prefix(peerIsis.Connectedv6Prefix)
	//Subscribe at a higher YANG path level as per CSCwb73158
	gnmi.Get(t, dut, lsp.State())

	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/metric", func(t *testing.T) {
		state := tlvExtv6Prefix.Metric()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != uint32(peerIsis.Iprmetric) {
			t.Errorf("ISIS tlvExtv6Prefix metric: got %d, want %d", val, uint32(peerIsis.Iprmetric))
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/prefix", func(t *testing.T) {
		state := tlvExtv6Prefix.Prefix()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != peerIsis.Connectedv6Prefix {
			t.Errorf("ISIS tlvExtv6Prefix prefix: got %s, want %s", val, peerIsis.Connectedv6Prefix)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/x-bit", func(t *testing.T) {
		state := tlvExtv6Prefix.XBit()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("ISIS tlvExtv6Prefix Xbit:: got %t, want %t", val, false)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/s-bit", func(t *testing.T) {
		state := tlvExtv6Prefix.SBit()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("ISIS tlvExtv6Prefix Sbit: got %t, want %t", val, false)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/state/up-down", func(t *testing.T) {
		state := tlvExtv6Prefix.UpDown()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("ISIS tlvExtv6Prefix up-down: got %t, want %t", val, false)
		}
	})
	tlvExtv4Prefix := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).
		ExtendedIpv4Reachability().Prefix(peerIsis.Connectedv4Prefix)
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/metric", func(t *testing.T) {
		state := tlvExtv4Prefix.Metric()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != uint32(peerIsis.Prefixmetric) {
			t.Errorf("ISIS tlvExtv4Prefix metric: got %d, want %d", val, uint32(peerIsis.Prefixmetric))
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/prefix", func(t *testing.T) {
		state := tlvExtv4Prefix.Prefix()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != peerIsis.Connectedv4Prefix {
			t.Errorf("ISIS tlvExtv4Prefix prefix: got %s, want %s", val, peerIsis.Connectedv4Prefix)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/s-bit", func(t *testing.T) {
		state := tlvExtv4Prefix.SBit()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("ISIS tlvExtv4Prefix Sbit: got %t, want %t", val, false)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/state/up-down", func(t *testing.T) {
		state := tlvExtv4Prefix.UpDown()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("ISIS tlvExtv4Prefix up-down: got %t, want %t", val, false)
		}
	})

	id := uint64(0)
	neighbor := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IS_REACHABILITY).ExtendedIsReachability().Neighbor(peerIsis.EisrsystemId)
	val := gnmi.Get(t, dut, neighbor.State())
	if val != nil {
		for _, y := range val.Instance {
			id = *y.Id
		}
	}

	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-is-reachability/neighbors/neighbor/instances/instance/state/metric", func(t *testing.T) {
		state := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IS_REACHABILITY).ExtendedIsReachability().Neighbor(peerIsis.EisrsystemId).Instance(id).Metric()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != uint32(peerIsis.Prefixmetric) {
			t.Errorf("ISIS tlvExtv4Prefix instsance metric: got %d, want %d", val, peerIsis.Prefixmetric)
		}
	})
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-interfaces-addresses/state/ipv6-interface-addresses", func(t *testing.T) {
		state := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_INTERFACE_ADDRESSES).Ipv6InterfaceAddresses().Address()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if cmp.Diff(val, []string{"2000::100:120:1:2"}) != "" {
			t.Errorf("ISIS tlvExtv6Prefix Address: got %v, want %v", val, []string{"2000::100:120:1:2"})
		}
	})
	/*
		t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/subtlvs/subtlv/ipv4-source-router-id/state/ipv4-source-router-id", func(t *testing.T) {
			state := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).
				Ipv6Reachability().Prefix(peerIsis.GetConnectedv6Prefix()).Subtlv(oc.IsisLsdbTypes_ISIS_SUBTLV_TYPE_IP_REACHABILITY_IPV4_ROUTER_ID).Ipv4SourceRouterId().RouterId()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val == "" {
				t.Errorf("ISIS tlvExtv6Prefix Address: got %s, want !=%s", val, "''")
			}
		})
		t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/subtlvs/subtlv/ipv4-source-router-id/state/ipv4-source-router-id", func(t *testing.T) {
			state := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).
				Ipv6Reachability().Prefix(peerIsis.GetConnectedv6Prefix()).Subtlv(oc.IsisLsdbTypes_ISIS_SUBTLV_TYPE_IP_REACHABILITY_IPV4_ROUTER_ID).Ipv4SourceRouterId().RouterId()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val == "" {
				t.Errorf("ISIS tlv IPv6 Reachability Prefix Subtlv IPv4 Router ID: got %s, want !=%s", val, "''")
			}
		})
		t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/subtlvs/subtlv/ipv6-source-router-id/state/ipv4-source-router-id", func(t *testing.T) {
			state := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).
				Ipv6Reachability().Prefix(peerIsis.GetConnectedv6Prefix()).Subtlv(oc.IsisLsdbTypes_ISIS_SUBTLV_TYPE_IP_REACHABILITY_IPV6_ROUTER_ID).Ipv6SourceRouterId().RouterId()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val == "" {
				t.Errorf("ISIS tlv IPv6 Reachability Prefix Subtlv IPv6 Router ID: got %s, want !=%s", val, "''")
			}
		})
		t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/ipv6-reachability/prefixes/prefix/subtlvs/subtlv/flags/state/flags", func(t *testing.T) {
			state := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).
				Ipv6Reachability().Prefix(peerIsis.GetConnectedv6Prefix()).Subtlv(oc.IsisLsdbTypes_ISIS_SUBTLV_TYPE_IP_REACHABILITY_PREFIX_FLAGS).Flags().Flags()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if cmp.Diff(val, []oc.E_Flags_Flags{oc.Flags_Flags_EXTERNAL_FLAG, oc.Flags_Flags_READVERTISEMENT_FLAG, oc.Flags_Flags_NODE_FLAG}) != "" {
				t.Errorf("ISIS tlv IPv6 Reachability Prefix Subtlv IPv6 Router ID: got %v, want %v", val, []oc.E_Flags_Flags{oc.Flags_Flags_EXTERNAL_FLAG, oc.Flags_Flags_READVERTISEMENT_FLAG, oc.Flags_Flags_NODE_FLAG})
			}
		})
	*/
	t.Run("Subscribe//network-instances/network-instance/protocols/protocol/isis/levels/level/link-state-database/lsp/tlvs/tlv/extended-ipv4-reachability/prefixes/prefix/subtlvs/subtlv/flags/state/flags", func(t *testing.T) {
		state := lsp.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).
			ExtendedIpv4Reachability().Prefix(peerIsis.V4Prefix).Subtlv(oc.IsisLsdbTypes_ISIS_SUBTLV_TYPE_IP_REACHABILITY_PREFIX_FLAGS).Flags()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		t.Log(val.Type.String())
	})

}
