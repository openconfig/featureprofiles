package basetest

import (
	"strings"
	"testing"
	"time"

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
	iute := dut.Port(t, "port8")

	t.Run("configInterface", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name())
		obj := &oc.Interface{
			Name:        ygot.String(iut.Name()),
			Description: ygot.String("randstr"),
		}
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, obj)

	})

	t.Run("Update//interfaces/interface/config/name", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, iut.Name())

	})

	t.Run("Replace//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Update(t, "desc1")

	})
	t.Run("Update//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Replace(t, "desc2")

	})
	t.Run("Delete//interfaces/interface/config/description", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "DELETE", path)
		path.Delete(t)

	})
	t.Run("Update//interfaces/interface/config/mtu", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, 600)

	})
	t.Run("Replace//interfaces/interface/config/mtu", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, 1200)

	})
	t.Run("Delete//interfaces/interface/config/mtu", func(t *testing.T) {
		path := dut.Config().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "DELETE", path)
		path.Delete(t)

	})

	member := iut.Members()[0]
	macAdd := "78:2a:67:b6:a8:08"
	t.Run("Replace//interfaces/interface/ethernet/config/mac-address", func(t *testing.T) {
		path := dut.Config().Interface(iute.Name()).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, macAdd)

	})
	t.Run("Replace//interfaces/interface/config/type", func(t *testing.T) {
		path := dut.Config().Interface(iute.Name()).Type()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	})
	t.Run("Replace//interfaces/interface/ethernet/config/aggregate-id", func(t *testing.T) {
		path := dut.Config().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, iut.Name())

	})
	t.Run("Update//interfaces/interface/ethernet/config/mac-address", func(t *testing.T) {
		path := dut.Config().Interface(iute.Name()).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, macAdd)

	})
	t.Run("Update//interfaces/interface/config/type", func(t *testing.T) {
		path := dut.Config().Interface(iute.Name()).Type()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	})
	t.Run("Update//interfaces/interface/ethernet/config/aggregate-id", func(t *testing.T) {
		path := dut.Config().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, iut.Name())

	})
	// port-speed and duplex-mode supported for GigabitEthernet/FastEthernet type interfaces
	t.Run("Replace//interfaces/interface/ethernet/config/port-speed", func(t *testing.T) {
		path := dut.Config().Interface("GigabitEthernet0/0/0/1").Ethernet().PortSpeed()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, oc.IfEthernet_ETHERNET_SPEED_SPEED_1GB)

	})
	t.Run("Replace//interfaces/interface/ethernet/config/duplex-mode", func(t *testing.T) {
		path := dut.Config().Interface("GigabitEthernet0/0/0/1").Ethernet().DuplexMode()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, oc.Ethernet_DuplexMode_FULL)

	})

	t.Run("Update//interfaces/interface/ethernet/config/port-speed", func(t *testing.T) {
		path := dut.Config().Interface("GigabitEthernet0/0/0/1").Ethernet().PortSpeed()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.IfEthernet_ETHERNET_SPEED_SPEED_1GB)

	})
	t.Run("Update//interfaces/interface/ethernet/config/duplex-mode", func(t *testing.T) {
		path := dut.Config().Interface("GigabitEthernet0/0/0/1").Ethernet().DuplexMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.Ethernet_DuplexMode_FULL)

	})
	t.Run("Delete//interfaces/interface/ethernet/config/port-speed", func(t *testing.T) {
		path := dut.Config().Interface("GigabitEthernet0/0/0/1").Ethernet().PortSpeed()
		defer observer.RecordYgot(t, "DELETE", path)
		path.Delete(t)

	})
	t.Run("Delete//interfaces/interface/ethernet/config/duplex-mode", func(t *testing.T) {
		path := dut.Config().Interface("GigabitEthernet0/0/0/1").Ethernet().DuplexMode()
		defer observer.RecordYgot(t, "DELETE", path)
		path.Delete(t)

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
	t.Run("Subscribe//interfaces/interface/state/counters/in-broadcast-pkts", func(t *testing.T) {
		state := state.InBroadcastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InBroadcastPkts")
		} else {
			t.Errorf("Interface InBroadcastPkts: got %d, want equal to greater than %d", counter, 0)

		}
	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-errors", func(t *testing.T) {
		state := state.InErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InErrors")
		} else {
			t.Errorf("Interface InErrors: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-discards", func(t *testing.T) {
		state := state.InDiscards()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InDiscards")
		} else {
			t.Errorf("Interface InDiscards: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-multicast-pkts", func(t *testing.T) {
		state := state.InMulticastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InMulticastPkts")
		} else {
			t.Errorf("Interface InMulticastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-octets", func(t *testing.T) {
		state := state.InOctets()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InOctets")
		} else {
			t.Errorf("Interface InOctets: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-unicast-pkts", func(t *testing.T) {
		state := state.InUnicastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InUnicastPkts")
		} else {
			t.Errorf("Interface InUnicastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-unknown-protos", func(t *testing.T) {
		state := state.InUnknownProtos()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InUnknownProtos")
		} else {
			t.Errorf("Interface InUnknownProtos: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-pkts", func(t *testing.T) {
		state := state.InPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InPkts")
		} else {
			t.Errorf("Interface InPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe///interfaces/interface/state/counters/out-broadcast-pkts", func(t *testing.T) {
		state := state.OutBroadcastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutBroadcastPkts")
		} else {
			t.Errorf("Interface OutBroadcastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-discards", func(t *testing.T) {
		state := state.OutDiscards()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutDiscards")
		} else {
			t.Errorf("Interface OutDiscards: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-errorse", func(t *testing.T) {
		state := state.OutErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutErrors")
		} else {
			t.Errorf("Interface OutErrors: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-multicast-pkts", func(t *testing.T) {
		state := state.OutMulticastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutMulticastPkts")
		} else {
			t.Errorf("Interface OutMulticastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-octets", func(t *testing.T) {
		state := state.OutOctets()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutOctets")
		} else {
			t.Errorf("Interface OutOctets: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-pkts", func(t *testing.T) {
		state := state.OutPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutPkts")
		} else {
			t.Errorf("Interface OutPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-unicast-pkts", func(t *testing.T) {
		state := state.OutUnicastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutUnicastPkts")
		} else {
			t.Errorf("Interface OutUnicastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-fcs-errors", func(t *testing.T) {
		state := state.InFcsErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InFcsErrors")
		} else {
			t.Errorf("Interface InFcsErrors: got %d, want equal to greater than %d", counter, 0)

		}

	})
	member := iut.Members()[0]
	t.Run("Subscribe//interfaces/interface/ethernet/state/counters/in-mac-pause-frames", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().Counters().InMacPauseFrames()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InMacPauseFrames")
		} else {
			t.Errorf("Interface InMacPauseFrames: got %d, want  equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/ethernet/state/counters/out-mac-pause-frames", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().Counters().OutMacPauseFrames()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := state.Get(t)
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutMacPauseFrames")
		} else {
			t.Errorf("Interface OutMacPauseFrames: got %d, want equal to greater than %d", counter, 0)

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

	t.Run("Subscribe//interfaces/interface/state/name", func(t *testing.T) {
		state := dut.Telemetry().Interface(iut.Name()).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		name := state.Get(t)
		if name != iut.Name() {
			t.Errorf("Interface Name: got %s, want %s", name, iut.Name())

		}

	})

	t.Run("Subscribe//interfaces/interface/state/description", func(t *testing.T) {
		state := dut.Telemetry().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		description := state.Get(t)
		if description != randstr {
			t.Errorf("Interface Description: got %s, want %s", description, randstr)
		}

	})
	t.Run("Subscribe//interfaces/interface/state/mtu", func(t *testing.T) {
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
	t.Run("Subscribe//interfaces/interface/state/admin-status", func(t *testing.T) {
		state := state.AdminStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		status := state.Get(t)
		if status != oc.Interface_AdminStatus_UP {
			t.Errorf("Interface AdminStatus: got %v, want %v", status, oc.Interface_AdminStatus_UP)

		}
	})
	t.Run("Subscribe//interfaces/interface/state/type", func(t *testing.T) {
		state := state.Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		status := dut.Telemetry().Interface(iut.Name()).Type().Get(t)
		if status != oc.IETFInterfaces_InterfaceType_ieee8023adLag {
			t.Errorf("Interface Type: got %v, want %v", status, oc.IETFInterfaces_InterfaceType_ieee8023adLag)

		}
	})
	t.Run("Subscribe//interfaces/interface/state/oper-status", func(t *testing.T) {
		state := state.OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		status := state.Get(t)
		if status != oc.Interface_OperStatus_UP {
			t.Errorf("Interface OperStatus: got %v, want %v", status, oc.Interface_OperStatus_UP)

		}
	})
	t.Run("Subscribe//interfaces/interface/aggregation/state/member", func(t *testing.T) {
		state := dut.Telemetry().Interface(iut.Name()).Aggregation().Member()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		members := state.Get(t)
		if sliceEqual(members, iut.Members()) {
			t.Logf("Got correct Interface Aggregation Member Value")
		} else {
			t.Errorf("Interface Aggregation Member: got %v, want %v", members, iut.Members())

		}
	})
	member := iut.Members()[0]
	reqspeed := oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	if strings.Contains(member, "FourHun") {
		reqspeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
	}
	t.Run("Subscribe//interfaces/interface/ethernet/state/port-speed", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().PortSpeed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		speed := state.Get(t)
		if speed != reqspeed {
			t.Errorf("Interface PortSpeed: got %v, want %v", speed, reqspeed)

		}
	})
	t.Run("Subscribe//interfaces/interface/ethernet/state/negotiated-port-speeds", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().NegotiatedPortSpeed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		speed := state.Get(t)
		if speed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
			t.Errorf("Interface InFcsErrors: got %v, want %v", speed, oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN)

		}
	})
	t.Run("Update//interfaces/interface/ethernet/config/aggregate-id", func(t *testing.T) {
		path := dut.Config().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, iut.Name())

	})
	t.Run("Subscribe//interfaces/interface/ethernet/state/aggregate-id", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		id := state.Get(t)
		if id == "" {
			t.Errorf("Interface AggregateId: got %s, want !=%s", id, "''")

		}
	})
	iute := dut.Port(t, "port8")
	macAdd := "78:2a:67:b6:a8:08"
	t.Run("Update//interfaces/interface/ethernet/config/mac-addres", func(t *testing.T) {
		path := dut.Config().Interface(iute.Name()).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, macAdd)

	})
	t.Run("Subscribe//interfaces/interface/ethernet/state/mac-address", func(t *testing.T) {
		state := dut.Telemetry().Interface(iute.Name()).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		macadd := state.Get(t)
		if macadd != macAdd {
			t.Errorf("Interface MacAddress: got %s, want !=%s", macadd, macAdd)

		}
	})
	t.Run("Update//interfaces/interface/config/type", func(t *testing.T) {
		path := dut.Config().Interface(iute.Name()).Type()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	})
	t.Run("Subscribe//interfaces/interface/state/type", func(t *testing.T) {
		state := dut.Telemetry().Interface(iute.Name()).Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		_type := state.Get(t)
		if _type != oc.IETFInterfaces_InterfaceType_ethernetCsmacd {
			t.Errorf("Interface Type: got %v, want !=%v", _type, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

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
	t.Run("Update//interfaces/interface/hold-time/config/up", func(t *testing.T) {
		config := dut.Config().Interface(member).HoldTime().Up()
		defer observer.RecordYgot(t, "UPDATE", config)
		config.Update(t, hlt)
	})
	t.Run("Subscribe//interfaces/interface/hold-time/state/up", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).HoldTime().Up()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		holdtime := state.Get(t)
		if holdtime != hlt {
			t.Errorf("Interface HoldTime Up: got %d, want %d", holdtime, hlt)

		}

	})
	t.Run("Update//interfaces/interface/hold-time/config/down", func(t *testing.T) {
		config := dut.Config().Interface(member).HoldTime().Down()
		defer observer.RecordYgot(t, "UPDATE", config)
		config.Update(t, hlt)
	})
	t.Run("Subscribe//interfaces/interface/hold-time/state/down", func(t *testing.T) {
		state := dut.Telemetry().Interface(member).HoldTime().Down()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		holdtime := state.Get(t)
		if holdtime != hlt {
			t.Errorf("Interface HoldTime Down: got %d, want %d", holdtime, hlt)

		}

	})
	t.Run("Delete//interfaces/interface/hold-time/config/down", func(t *testing.T) {
		config := dut.Config().Interface(member).HoldTime().Down()
		defer observer.RecordYgot(t, "DELETE", config)
		config.Delete(t)
	})
	t.Run("Delete//interfaces/interface/hold-time/config/up", func(t *testing.T) {
		config := dut.Config().Interface(member).HoldTime().Up()
		defer observer.RecordYgot(t, "DELETE", config)
		config.Delete(t)
	})
}

func TestInterfaceTelemetry(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")

	//Default susbcription rate is 30 seconds.
	subscriptionDuration := 50 * time.Second
	triggerDelay := 20 * time.Second
	postInterfaceEventWait := 10 * time.Second
	expectedEntries := 2

	t.Run("Subscribe//interfaces/interface/state/oper-status", func(t *testing.T) {

		configPath := dut.Config().Interface(iut.Name()).Enabled()
		statePath := dut.Telemetry().Interface(iut.Name()).OperStatus()

		//initialise OperStatus
		configPath.Update(t, true)
		time.Sleep(postInterfaceEventWait)
		t.Logf("Updated interface oper status: %s", statePath.Get(t))

		//delay triggering OperStatus change
		go func(t *testing.T) {
			time.Sleep(triggerDelay)
			configPath.Update(t, false)
			t.Log("Triggered oper-status change")
		}(t)

		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)
		got := statePath.Collect(t, subscriptionDuration).Await(t)
		gotEntries := len(got)

		if gotEntries < expectedEntries {
			t.Errorf("Oper Status subscription samples: got %d, want %d", gotEntries, expectedEntries)
		}
		//verify last sample has event trigger recorded.
		if got[gotEntries-1].Val(t) != oc.Interface_OperStatus_DOWN {
			t.Errorf("Interface OperStatus change event was not recorded: got %s, want %s", got[gotEntries-1].Val(t), oc.Interface_OperStatus_DOWN)
		}
	})

	t.Run("Subscribe//interfaces/interface/state/admin-status", func(t *testing.T) {

		configPath := dut.Config().Interface(iut.Name()).Enabled()
		statePath := dut.Telemetry().Interface(iut.Name()).AdminStatus()

		//initialise OperStatus to change admin-status
		configPath.Update(t, true)
		time.Sleep(postInterfaceEventWait)
		t.Logf("Updated interface admin status: %s", statePath.Get(t))

		//delay triggering OperStatus change
		go func(t *testing.T) {
			time.Sleep(triggerDelay)
			configPath.Update(t, false)
			t.Log("Triggered oper-status change to change admin-status")
		}(t)

		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)
		got := statePath.Collect(t, subscriptionDuration).Await(t)
		t.Logf("Collected samples for admin-status: %v", got)
		gotEntries := len(got)

		if gotEntries < expectedEntries {
			t.Errorf("Admin Status subscription samples: got %d, want %d", gotEntries, expectedEntries)
		}
		//verify last sample has trigger event recorded.
		if got[gotEntries-1].Val(t) != oc.Interface_AdminStatus_DOWN {
			t.Errorf("Interface AdminStatus change event was not recorded: got %s, want %s", got[gotEntries-1].Val(t), oc.Interface_AdminStatus_DOWN)
		}
	})

}
