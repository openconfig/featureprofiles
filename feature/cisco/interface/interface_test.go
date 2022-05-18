package basetest

import (
	"strings"
	"testing"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestInterfaceCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	t.Run("configInterface", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name())
		obj := &oc.Interface{
			Name:        ygot.String(iut.Name()),
			Description: ygot.String("randstr"),
		}
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, obj)

	})

	t.Run("updateconfig//interfaces/interface/config/name", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, iut.Name())

	})

	t.Run("replaceconfig//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Update(t, "desc1")

	})
	t.Run("updateconfig//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Replace(t, "desc2")

	})
	t.Run("deleteconfig//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "DELETE", path)
		path.Delete(t)

	})
	t.Run("updateconfig//interfaces/interface/config/mtu", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, 600)

	})
	t.Run("replaceconfig//interfaces/interface/config/mtu", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, 1200)

	})
	t.Run("deleteconfig//interfaces/interface/config/mtu", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "DELETE", path)
		path.Delete(t)

	})
	member := iut.Members()[0]
	macAdd := "78:2a:67:b6:a8:08"
	t.Run("replaceconfig//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(member).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, macAdd)

	})
	t.Run("replaceconfig//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(member).Type()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	})
	t.Run("replaceconfig//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, iut.Name())

	})
	t.Run("updateconfig//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(member).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, macAdd)

	})
	t.Run("updateconfig//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(member).Type()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	})
	t.Run("updateconfig//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, iut.Name())

	})

}

func TestInterfaceIPCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	vlanid := uint32(0)
	t.Run("configInterfaceIP", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Subinterface(vlanid)
		obj := &oc.Interface_Subinterface{
			Index: ygot.Uint32(0),
			Ipv6: &oc.Interface_Subinterface_Ipv6{
				Address: map[string]*oc.Interface_Subinterface_Ipv6_Address{
					iut.Ipv6Address(): {
						Ip:           ygot.String(iut.Ipv6Address()),
						PrefixLength: ygot.Uint8(iut.Ipv6PrefixLength()),
					},
				},
			},
			Ipv4: &oc.Interface_Subinterface_Ipv4{
				Address: map[string]*oc.Interface_Subinterface_Ipv4_Address{
					iut.Ipv4Address(): {
						Ip:           ygot.String(iut.Ipv4Address()),
						PrefixLength: ygot.Uint8(iut.Ipv4PrefixLength()),
					},
				},
			},
		}

		defer observer.RecordYgot(t, "REPLACE", path)
		defer observer.RecordYgot(t, "REPLACE", path.Ipv4().Address(iut.Ipv4Address()).Ip())
		defer observer.RecordYgot(t, "REPLACE", path.Ipv4().Address(iut.Ipv4Address()).PrefixLength())
		defer observer.RecordYgot(t, "REPLACE", path.Ipv6().Address(iut.Ipv6Address()).Ip())
		defer observer.RecordYgot(t, "REPLACE", path.Ipv6().Address(iut.Ipv6Address()).PrefixLength())
		path.Replace(t, obj)

	})
	path := dut.Config().Interface(iut.Name()).Subinterface(vlanid)
	obj := &oc.Interface_Subinterface{
		Index: ygot.Uint32(0),
		Ipv6: &oc.Interface_Subinterface_Ipv6{
			Address: map[string]*oc.Interface_Subinterface_Ipv6_Address{
				iut.Ipv6Address(): {
					Ip:           ygot.String(iut.Ipv6Address()),
					PrefixLength: ygot.Uint8(iut.Ipv6PrefixLength()),
				},
			},
		},
		Ipv4: &oc.Interface_Subinterface_Ipv4{
			Address: map[string]*oc.Interface_Subinterface_Ipv4_Address{
				iut.Ipv4Address(): {
					Ip:           ygot.String(iut.Ipv4Address()),
					PrefixLength: ygot.Uint8(iut.Ipv4PrefixLength()),
				},
			},
		},
	}

	defer observer.RecordYgot(t, "UPDATE", path)
	defer observer.RecordYgot(t, "UPDATE", path.Ipv4().Address(iut.Ipv4Address()).Ip())
	defer observer.RecordYgot(t, "UPDATE", path.Ipv4().Address(iut.Ipv4Address()).PrefixLength())
	defer observer.RecordYgot(t, "UPDATE", path.Ipv6().Address(iut.Ipv6Address()).Ip())
	defer observer.RecordYgot(t, "UPDATE", path.Ipv6().Address(iut.Ipv6Address()).PrefixLength())
	path.Update(t, obj)

}

func TestInterfaceCountersState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	// cli_handle := dut.RawAPIs().CLI(t)
	// cli_handle.Stdin().Write([]byte("clear counters\n"))
	// cli_handle.Stdin().Write([]byte("\n"))
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	state := dut.Telemetry().Interface(iut.Members()[0]).Counters()
	t.Run("state//interfaces/interface/state/counters/in-broadcast-pkts", func(t *testing.T) {
		state := state.InBroadcastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InBroadcastPkts: got %d, want %d", counter, 0)

		}
	})
	t.Run("state//interfaces/interface/state/counters/in-errors", func(t *testing.T) {
		state := state.InErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InErrors: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/in-discards", func(t *testing.T) {
		state := state.InDiscards()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InDiscards: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/in-multicast-pkts", func(t *testing.T) {
		state := state.InMulticastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InMulticastPkts: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/in-octets", func(t *testing.T) {
		state := state.InOctets()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InOctets: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/in-unicast-pkts", func(t *testing.T) {
		state := state.InUnicastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InUnicastPkts: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/in-unknown-protos", func(t *testing.T) {
		state := state.InUnknownProtos()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InUnknownProtos: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/in-pkts", func(t *testing.T) {
		state := state.InPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InPkts: got %d, want %d", counter, 0)

		}

	})
	t.Run("state///interfaces/interface/state/counters/out-broadcast-pkts", func(t *testing.T) {
		state := state.OutBroadcastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface OutBroadcastPkts: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/out-discards", func(t *testing.T) {
		state := state.OutDiscards()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface OutDiscards: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/out-errorse", func(t *testing.T) {
		state := state.OutErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface OutErrors: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/out-multicast-pkts", func(t *testing.T) {
		state := state.OutMulticastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface OutMulticastPkts: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/out-octets", func(t *testing.T) {
		state := state.OutOctets()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface OutOctets: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/out-pkts", func(t *testing.T) {
		state := state.OutPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface OutPkts: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/out-unicast-pkts", func(t *testing.T) {
		state := state.OutUnicastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface OutUnicastPkts: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/state/counters/in-fcs-errors", func(t *testing.T) {
		state := state.InFcsErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InFcsErrors: got %d, want %d", counter, 0)

		}

	})
	member := iut.Members()[0]
	t.Run("state//interfaces/interface/ethernet/state/counters/in-mac-pause-frames", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().Counters().InMacPauseFrames()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface InMacPauseFrames: got %d, want %d", counter, 0)

		}

	})
	t.Run("state//interfaces/interface/ethernet/state/counters/out-mac-pause-frames", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().Counters().OutMacPauseFrames()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter != 0 {
			t.Errorf("Interface OutMacPauseFrames: got %d, want %d", counter, 0)

		}

	})
}
func TestInterfaceState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	randstr := "random string"
	randmtu := ygot.Uint16(1200)
	path := dut.Config().Interface(iut.Name())
	obj := &oc.Interface{
		Name:        ygot.String(iut.Name()),
		Description: ygot.String(randstr),
		Mtu:         randmtu,
	}
	path.Replace(t, obj)

	t.Run("state//interfaces/interface/state/name", func(t *testing.T) {
		state := dut.Telemetry().Interface(iut.Name()).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		name := state.Get(t)
		if name != iut.Name() {
			t.Errorf("Interface Name: got %s, want %s", name, iut.Name())

		}

	})

	t.Run("state//interfaces/interface/state/description", func(t *testing.T) {
		state := dut.Telemetry().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		description := state.Get(t)
		if description != randstr {
			t.Errorf("Interface Description: got %s, want %s", description, randstr)
		}

	})
	t.Run("state//interfaces/interface/state/mtu", func(t *testing.T) {
		state := dut.Telemetry().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		mtu := state.Get(t)
		if mtu != *randmtu {
			t.Errorf("Interface Mtu: got %d, want %d", mtu, *randmtu)
		}

	})
	vlanid := uint32(0)
	spath := dut.Config().Interface(iut.Name()).Subinterface(vlanid)
	sobj := &oc.Interface_Subinterface{
		Index: ygot.Uint32(0),
		Ipv6: &oc.Interface_Subinterface_Ipv6{
			Address: map[string]*oc.Interface_Subinterface_Ipv6_Address{
				iut.Ipv6Address(): {
					Ip:           ygot.String(iut.Ipv6Address()),
					PrefixLength: ygot.Uint8(iut.Ipv6PrefixLength()),
				},
			},
		},
		Ipv4: &oc.Interface_Subinterface_Ipv4{
			Address: map[string]*oc.Interface_Subinterface_Ipv4_Address{
				iut.Ipv4Address(): {
					Ip:           ygot.String(iut.Ipv4Address()),
					PrefixLength: ygot.Uint8(iut.Ipv4PrefixLength()),
				},
			},
		},
	}
	spath.Update(t, sobj)

	state := dut.Telemetry().Interface(iut.Members()[0])
	path = dut.Config().Interface(iut.Name())
	obj = &oc.Interface{
		Name:        ygot.String(iut.Name()),
		Description: ygot.String("randstr"),
		Mtu:         ygot.Uint16(1200),
	}
	path.Update(t, obj)
	t.Run("state//interfaces/interface/state/admin-status", func(t *testing.T) {
		state := state.AdminStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		status := state.Get(t)
		if status != oc.Interface_AdminStatus_UP {
			t.Errorf("Interface InFcsErrors: got %v, want %v", status, oc.Interface_AdminStatus_UP)

		}
	})
	t.Run("state//interfaces/interface/state/type", func(t *testing.T) {
		state := state.Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		status := state.Get(t)
		if status != oc.IETFInterfaces_InterfaceType_ieee8023adLag {
			t.Errorf("Interface InFcsErrors: got %v, want %v", status, oc.Interface_AdminStatus_UP)

		}
	})
	t.Run("state//interfaces/interface/state/oper-status", func(t *testing.T) {
		state := state.OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		status := state.Get(t)
		if status != oc.Interface_OperStatus_UP {
			t.Errorf("Interface InFcsErrors: got %v, want %v", status, oc.Interface_AdminStatus_UP)

		}
	})
	t.Run("state//interfaces/interface/aggregation/state/member", func(t *testing.T) {
		state := dut.Telemetry().Interface(iut.Name()).Aggregation().Member()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		members := state.Get(t)
		if sliceEqual(members, iut.Members()) {
			t.Errorf("Interface InFcsErrors: got %v, want %v", members, iut.Members())

		}
	})
	member := iut.Members()[0]
	reqspeed := oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	if strings.Contains(member, "FourHun") {
		reqspeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
	}
	t.Run("state//interfaces/interface/ethernet/state/port-speed", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().PortSpeed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		speed := state.Get(t)
		if speed != reqspeed {
			t.Errorf("Interface InFcsErrors: got %v, want %v", speed, reqspeed)

		}
	})
	t.Run("state//interfaces/interface/ethernet/state/negotiated-port-speeds", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().NegotiatedPortSpeed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		speed := state.Get(t)
		if speed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
			t.Errorf("Interface InFcsErrors: got %v, want %v", speed, oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN)

		}
	})
	t.Run("updateconfig//interfaces/interface/ethernet/config/aggregate-id", func(t *testing.T) {
		path := dut.Config().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, iut.Name())

	})
	t.Run("state//interfaces/interface/ethernet/state/aggregate-id", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		id := state.Get(t)
		if id == "" {
			t.Errorf("Interface InFcsErrors: got %s, want !=%s", id, "''")

		}
	})
	macAdd := "78:2a:67:b6:a8:08"
	t.Run("updateconfig//interfaces/interface/ethernet/config/mac-addres", func(t *testing.T) {
		path := dut.Config().Interface(member).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, macAdd)

	})
	t.Run("state//interfaces/interface/ethernet/state/mac-address", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		macadd := state.Get(t)
		if macadd != macAdd {
			t.Errorf("Interface InFcsErrors: got %s, want !=%s", macadd, macAdd)

		}
	})
	t.Run("updateconfig//interfaces/interface/config/type", func(t *testing.T) {
		path := dut.Config().Interface(member).Type()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	})
	t.Run("state//interfaces/interface/state/type", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		_type := state.Get(t)
		if _type != oc.IETFInterfaces_InterfaceType_ethernetCsmacd {
			t.Errorf("Interface InFcsErrors: got %v, want !=%v", _type, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

		}
	})

}

func TestInterfaceHoldTime(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	hlt := uint32(30)
	member := iut.Members()[0]
	t.Run("configupdate//interfaces/interface/hold-time/config/up", func(t *testing.T) {
		config := dut.Config().Interface(member).HoldTime().Up()
		defer observer.RecordYgot(t, "UPDATE", config)
		config.Update(t, hlt)
	})
	t.Run("state//interfaces/interface/hold-time/state/up", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).HoldTime().Up()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		holdtime := state.Get(t)
		if holdtime != hlt {
			t.Errorf("Interface OutMacPauseFrames: got %d, want %d", holdtime, hlt)

		}

	})
	t.Run("configupdate//interfaces/interface/hold-time/config/down", func(t *testing.T) {
		config := dut.Config().Interface(member).HoldTime().Down()
		defer observer.RecordYgot(t, "UPDATE", config)
		config.Update(t, hlt)
	})
	t.Run("state//interfaces/interface/hold-time/state/down", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).HoldTime().Down()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		holdtime := state.Get(t)
		if holdtime != hlt {
			t.Errorf("Interface OutMacPauseFrames: got %d, want %d", holdtime, hlt)

		}

	})
	t.Run("configdelete//interfaces/interface/hold-time/config/down", func(t *testing.T) {
		config := dut.Config().Interface(member).HoldTime().Down()
		defer observer.RecordYgot(t, "DELETE", config)
		config.Delete(t)
	})
	t.Run("configdelete//interfaces/interface/hold-time/config/up", func(t *testing.T) {
		config := dut.Config().Interface(member).HoldTime().Up()
		defer observer.RecordYgot(t, "DELETE", config)
		config.Delete(t)
	})
}
