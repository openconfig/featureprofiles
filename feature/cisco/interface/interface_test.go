package basetest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
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
func TestForwardingUnviableFP(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut1 := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	iut2 := inputObj.Device(dut).GetInterface("Bundle-Ether121")
	nonBundleMember := iut2.Members()[0]
	bundleMember := iut1.Members()[0]
	outPktsBefore := dut.Telemetry().Interface(bundleMember).Counters().OutPkts().Get(t)

	t.Run("Configure forwarding-unviable on bundle member ", func(t *testing.T) {
		verifyForwardingViable(t, dut, bundleMember)
		t.Log("Sleep after for 10s after configuring bundle member")
		time.Sleep(30 * time.Second)
	})

	t.Run("Counters checked after forwarding-unviable configured on bundle-member", func(t *testing.T) {
		outPktsAfterBundleMember := dut.Telemetry().Interface(bundleMember).Counters().OutPkts().Get(t)
		outPktsAfterBundle := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		if (outPktsAfterBundle > outPktsBefore) && (outPktsAfterBundleMember > outPktsBefore) {
			t.Logf("Counters before forward-unviable config: %v , Counters after forward-unviable config on Bundle interface  %v", outPktsBefore, outPktsAfterBundle)
			t.Logf("Counters before forward-unviable config: %v , Counters after forward-unviable config on Bundle Member interface  %v", outPktsBefore, outPktsAfterBundleMember)
			t.Errorf("Out pkts are increasing with forwarding-unviable are not as expected")
		}
	})

	t.Run("Configure forwarding-unviable and check if bundle interface status is DOWN", func(t *testing.T) {
		stateBundleInterface := dut.Telemetry().Interface(iut1.Name()).OperStatus().Get(t).String()
		stateBundleMemberInterface := dut.Telemetry().Interface(bundleMember).OperStatus().Get(t).String()
		if (stateBundleInterface != "DOWN") && (stateBundleMemberInterface != "UP") {
			t.Logf("Bunde interface state %v, got %v , want DOWN", iut1.Name(), stateBundleInterface)
			t.Logf("Bundle member interface state %v, got %v, want UP ", bundleMember, stateBundleMemberInterface)
			t.Errorf("Interface state is not expected ")
		}

	})
	t.Run("Flap interfaces and check counter values", func(t *testing.T) {
		pktsBundleMemberBefore := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		pktsBundleBefore := dut.Telemetry().Interface(bundleMember).Counters().OutPkts().Get(t)
		for i := 0; i < 4; i++ {
			util.FlapInterface(t, dut, bundleMember, 10*time.Second)
			util.FlapInterface(t, dut, iut1.Name(), 10*time.Second)
		}
		time.Sleep(30 * time.Second)
		pktsBundleMemberAfter := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		pktsBundleAfter := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		if (pktsBundleMemberAfter > pktsBundleMemberBefore) && (pktsBundleAfter > pktsBundleBefore) {
			t.Logf("Counters before flap: %v , Counters after flap on Bundle interface  %v", pktsBundleBefore, pktsBundleAfter)
			t.Logf("Counters before flap config: %v , Counters after flap on Bundle Member interface   %v", pktsBundleMemberBefore, pktsBundleMemberAfter)
			t.Errorf("Out pkts are increasing with forwarding-unviable upon flapping")
		}

	})
	t.Run("Testing forwarding-unviable on non-bundle interface", func(t *testing.T) {
		member := dut.Config().Interface(nonBundleMember)
		member.Delete(t)
		defer member.Delete(t)
		pktsBefore := dut.Telemetry().Interface(nonBundleMember).Counters().OutPkts().Get(t)
		t.Log(pktsBefore)
		verifyForwardingViable(t, dut, nonBundleMember)
		time.Sleep(30 * time.Second)
		pktsAfter := dut.Telemetry().Interface(nonBundleMember).Counters().OutPkts().Get(t)
		t.Log(pktsAfter)
		if pktsAfter < pktsBefore {
			t.Errorf("Pkts after configuring forwarding-unviable on non bundle interface are expected to increase , Got pkts before configuring %v and after %v", pktsBefore, pktsAfter)
		}
		for i := 0; i < 4; i++ {
			util.FlapInterface(t, dut, nonBundleMember, 10*time.Second)
		}
		time.Sleep(30 * time.Second)
		pktsAfterFlap := dut.Telemetry().Interface(nonBundleMember).Counters().OutPkts().Get(t)
		t.Log(pktsAfterFlap)
		if pktsAfterFlap < pktsAfter {
			t.Errorf("Pkts after configuring forwarding-unviable on non bundle interface are expected to increase even after interface flap, Got pkts before configuring %v and after %v", pktsBefore, pktsAfter)
		}

	})

	t.Run("Configure 2 bundle members with one viable and other unviable", func(t *testing.T) {
		member := dut.Config().Interface(nonBundleMember)
		member.Delete(t)
		pktsBundleBefore := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)

		t.Log(pktsBundleBefore)
		members := dut.Config().Interface(nonBundleMember).Ethernet().AggregateId()
		members.Update(t, iut1.Name())
		defer members.Update(t, iut2.Name())
		defer member.Delete(t)
		t.Logf("Interface %v is forwarding-viable and Interface %v is forwarding-unviable", nonBundleMember, bundleMember)
		time.Sleep(10 * time.Second)
		bundleStatus := dut.Telemetry().Interface(iut1.Name()).OperStatus().Get(t).String()
		t.Log((bundleStatus))
		if bundleStatus != "UP" {
			t.Errorf("Expected Bundle interface %v to be UP as its member %v is forwading-viable ", bundleStatus, nonBundleMember)
		}
		time.Sleep(60 * time.Second)

		pktsBundleAfter := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		t.Log(pktsBundleAfter)
		if pktsBundleAfter < pktsBundleBefore {
			t.Errorf("Counters is not increasing as Interface is UP and bundle member is viable ")
		}
		verifyForwardingViable(t, dut, nonBundleMember)
		time.Sleep(10 * time.Second)
		bundleStatusAfter := dut.Telemetry().Interface(iut1.Name()).OperStatus().Get(t).String()
		time.Sleep(30 * time.Second)
		if bundleStatusAfter != "DOWN" {
			t.Errorf("Expected Bundle interface %v to be down as both its members are forwarding-unviable ", iut1.Name())
		}

		pktsBundleAfter2 := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		t.Log(pktsBundleAfter2)
		if pktsBundleAfter2 == pktsBundleAfter {
			t.Logf("Pkts before %v Pkts after %v ", pktsBundleAfter, pktsBundleAfter2)
		} else {
			t.Error("Outgoing pkts are increasing with forwarding unviable configured on the box ")
		}

	})

	t.Run("Process restart the router and check for counters ", func(t *testing.T) {

		pktsBundleMemberBefore := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		pktsBundleBefore := dut.Telemetry().Interface(bundleMember).Counters().OutPkts().Get(t)
		processList := [4]string{"ether_mgbl", "ifmgr", " bundlemgr_distrib", "bundlemgr_local"}
		for _, process := range processList {
			config.CMDViaGNMI(context.Background(), t, dut, fmt.Sprintf("process restart %v", process))
		}
		time.Sleep(30 * time.Second)
		pktsBundleMemberAfter := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		pktsBundleAfter := dut.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		if (pktsBundleMemberAfter > pktsBundleMemberBefore) && (pktsBundleAfter != pktsBundleBefore) {
			t.Logf("Counters before flap: %v , Counters after flap on Bundle interface  %v", pktsBundleBefore, pktsBundleAfter)
			t.Logf("Counters before flap config: %v , Counters after flap on Bundle Member interface   %v", pktsBundleMemberBefore, pktsBundleMemberAfter)
			t.Errorf("Out pkts are increasing with forwarding-unviable upon flapping")
		}

	})

	t.Run("Reload the router and check for counters", func(t *testing.T) {
		util.ReloadDUT(t, dut)
		dutR := ondatra.DUT(t, device1)
		pktsBundleMemberAfter := dutR.Telemetry().Interface(iut1.Name()).Counters().OutPkts().Get(t)
		pktsBundleAfter := dutR.Telemetry().Interface(bundleMember).Counters().OutPkts().Get(t)
		if (pktsBundleMemberAfter == 0) && (pktsBundleAfter == 0) {
			t.Logf(" Counters after flap on Bundle interface not expected: got %v, got 0 ", pktsBundleAfter)
			t.Logf(" Counters after flap on Bundle Member interface not expected: got %v, want 0 ", pktsBundleMemberAfter)
			t.Errorf("Out pkts are increasing with forwarding-unviable upon flapping")
		}

	})

}
func TestForwardViableSDN(t *testing.T) {
	t.Skip(t) // Run when SDN support comes in
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut2 := inputObj.Device(dut).GetInterface("Bundle-Ether121")
	bundleMember := iut2.Members()[0]
	interfaceContainer := &oc.Interface{ForwardingViable: ygot.Bool(false)}
	t.Run(fmt.Sprintf("Update//interface[%v]/config/forward-viable", bundleMember), func(t *testing.T) {
		path := dut.Config().Interface(bundleMember).ForwardingViable()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, *ygot.Bool(false))
	})
	t.Run(fmt.Sprintf("Get//interface[%v]/config/forward-viable", bundleMember), func(t *testing.T) {
		configContainer := dut.Config().Interface(bundleMember).ForwardingViable()
		defer observer.RecordYgot(t, "SUBSCRIBE", configContainer)
		forwardUnviable := configContainer.Get(t)
		if forwardUnviable != *ygot.Bool(false) {
			t.Errorf("Update for forward-unviable failed got %v , want false", forwardUnviable)
		}
	})
	t.Run(fmt.Sprintf("Subscribe//interface[%v]/state/forward-viable", bundleMember), func(t *testing.T) {
		stateContainer := dut.Telemetry().Interface(bundleMember).ForwardingViable()
		defer observer.RecordYgot(t, "SUBSCRIBE", stateContainer)
		forwardUnviable := stateContainer.Get(t)
		if forwardUnviable != *ygot.Bool(false) {
			t.Errorf("Update for forward-unviable failed got %v , want false", forwardUnviable)
		}
	})
	t.Run(fmt.Sprintf("Delete//interface[%v]/config/forward-viable", bundleMember), func(t *testing.T) {
		path := dut.Config().Interface(bundleMember).ForwardingViable()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Delete(t)
	})
	t.Run(fmt.Sprintf("Update//interface[%v]/", bundleMember), func(t *testing.T) {
		path := dut.Config().Interface(bundleMember)
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, interfaceContainer)
	})
	t.Run(fmt.Sprintf("Get//interface[%v]/", bundleMember), func(t *testing.T) {
		configContainer := dut.Config().Interface(bundleMember)
		defer observer.RecordYgot(t, "SUBSCRIBE", configContainer)
		forwardUnviable := configContainer.Get(t)
		if *forwardUnviable.ForwardingViable != *ygot.Bool(false) {
			t.Errorf("Update for forward-unviable failed got %v , want false", forwardUnviable)
		}
	})
	t.Run(fmt.Sprintf("Delete//interface[%v]/config/forward-viable", bundleMember), func(t *testing.T) {
		path := dut.Config().Interface(bundleMember)
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Delete(t)
	})
}
