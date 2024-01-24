package basetest

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	"github.com/openconfig/ygot/ygot"

	"context"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:db8::1",
		IPv4Len: 24,
		IPv6Len: 64,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "203.0.113.2",
		IPv6:    "2001:db8::2",
		IPv4Len: 24,
		IPv6Len: 64,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "204.0.114.1",
		IPv6:    "2002:db8::1",
		IPv4Len: 24,
		IPv6Len: 64,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:02:02:01",
		IPv4:    "204.0.114.2",
		IPv6:    "2002:db8::2",
		IPv4Len: 24,
		IPv6Len: 64,
	}
)

// configureOTG configures port1 and port2 on the ATE.
func configureOTG(t *testing.T,
	ate *ondatra.ATEDevice,
	breakoutspeed *oc.E_IfEthernet_ETHERNET_SPEED,
	ateIpv4Subnets []string,
	Dutipv4Subnets []string,
	numbreakouts int) gosnappi.Config {

	top := gosnappi.NewConfig()
	ports := ate.Ports()

	// order the ports from 1 to 8
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})

	// based on setup 100G breakout is on ports 1 to 4 and 10G is ports 5 to 8
	if *breakoutspeed == oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB {
		t.Logf("speed is %v", *breakoutspeed)
	} else if *breakoutspeed == oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB {
		t.Logf("speed is needed to start port assignment on port5 as that is "+
			"where 10G ports are in setup %v", *breakoutspeed)
		ports = ports[4:]
	}

	for i, port := range ports {

		// remove the subnet mask from the ipv4 address
		ip, _, err := net.ParseCIDR(ateIpv4Subnets[i])
		ateIpAddress := ip.String()

		if err != nil {
			t.Fatalf("Invalid IP address: %v", err)
		}

		gwIp, _, err := net.ParseCIDR(Dutipv4Subnets[i])
		dutIpAddress := gwIp.String()

		if err != nil {
			t.Fatalf("Invalid IP address: %v", err)
		}
		t.Logf("Port Name: %s\n", port.Name())
		t.Logf("Port ID: %s\n", port.ID())

		t.Logf("ATE IPv4 Add is : %v and port is %v", ateIpAddress, port.ID())
		t.Logf("ATE IPV4 GW IS DUT IP OF : %v and port is %v", dutIpAddress, port.ID())

		top.Ports().Add().SetName(ate.Port(t, port.ID()).ID())
		i1 := top.Devices().Add().SetName(ate.Port(t, port.ID()).ID())
		macAddress := fmt.Sprintf("02:00:01:0%v:01:01", i+1)
		eth1 := i1.Ethernets().Add().SetName(port.ID() + ".Eth").SetMac(macAddress)
		eth1.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(i1.Name())
		eth1.Ipv4Addresses().Add().SetName(port.ID() + ateIpAddress).
			SetAddress(ateIpAddress).SetGateway(dutIpAddress).
			SetPrefix(uint32(atePort1.IPv4Len))
		// exit loop when we reached the number of breakout ports
		if i == numbreakouts-1 {
			break
		}
	}
	// Show the OTG Config
	t.Log("Complete configuration:", top.String())
	ate.OTG().PushConfig(t, top)
	time.Sleep(30)
	ate.OTG().StartProtocols(t)

	return top
}

func TestPlatformBreakoutConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var Dutipv4Subnets []string
	var Ateipv4Subnets []string

	// Flag is used for older schema of 0 was original flag value
	var schemaValue uint8
	if uint8(deviations.BreakOutSchemaValueFlag(dut)) == 1 {
		schemaValue = uint8(1)
	} else {
		schemaValue = uint8(0)
	}
	t.Log(schemaValue)

	compPorts := oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT
	compPortsList := components.FindComponentsByType(t, dut, compPorts)

	cases := []struct {
		numbreakouts  uint8
		breakoutspeed oc.E_IfEthernet_ETHERNET_SPEED
	}{
		{
			numbreakouts:  4,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
		},
		{
			numbreakouts:  2,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
		},
		{
			numbreakouts:  4,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
		},
	}
	gnoiClient := dut.RawAPIs().GNOI(t)

	ate := ondatra.ATE(t, "ate")
	for _, tc := range cases {

		originalPortName := getOriginalPortName(dut, t, tc.breakoutspeed)

		t.Logf("Configuring port %s with breakout of %d at speed %v", originalPortName.Name(), tc.numbreakouts, tc.breakoutspeed)

		matchedPort, matchedSlot, err := findMatchedPortAndSlot(compPortsList, originalPortName.Name())
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Sort: %s, Matched Slot: %s", matchedPort, matchedSlot)
		optic_name := ConvertIntfToCompName(originalPortName.Name(), matchedSlot)
		t.Log(optic_name)
		componentNameList = []string{optic_name}
		t.Logf("Matched port:%s Matched Slot is %s", matchedPort, matchedSlot)
		for _, element := range componentNameList {
			fmt.Print(element)
		}
		// loop the components
		for _, componentName := range componentNameList {

			var dutIntfIp string
			var ateIntfIp string
			if tc.breakoutspeed == oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB {
				dutIntfIp = dutPort2.IPv4
				ateIntfIp = atePort2.IPv4
			} else if tc.breakoutspeed == oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB {
				dutIntfIp = dutPort1.IPv4
				ateIntfIp = atePort1.IPv4
			}

			t.Logf("Starting Test for %s %v", componentName, tc)
			configContainer := &oc.Component_Port_BreakoutMode_Group{
				Index:         ygot.Uint8(schemaValue),
				NumBreakouts:  ygot.Uint8(tc.numbreakouts),
				BreakoutSpeed: oc.E_IfEthernet_ETHERNET_SPEED(tc.breakoutspeed),
			}
			groupContainer := &oc.Component_Port_BreakoutMode{Group: map[uint8]*oc.Component_Port_BreakoutMode_Group{schemaValue: configContainer}}
			breakoutContainer := &oc.Component_Port{BreakoutMode: groupContainer}
			portContainer := &oc.Component{Port: breakoutContainer, Name: ygot.String(componentName)}
			t.Logf("COMBO : %v*%v ", tc.numbreakouts, tc.breakoutspeed)

			gnmi.Delete(t, dut, gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue).Config())
			t.Run(fmt.Sprintf("Replace//component[%v]/config/port/breakout-mode/group[%v]/config: %v*%v", componentName, schemaValue, tc.numbreakouts, tc.breakoutspeed), func(t *testing.T) {
				t.Logf("The component name inside test: %v", componentName)
				path := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue)
				// defer observer.RecordYgot(t, "REPLACE", path)
				gnmi.Replace(t, dut, path.Config(), configContainer)

			})

			t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port/breakout-mode/group[%v]", componentName, schemaValue), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue)
				// defer observer.RecordYgot(t, "SUBSCRIBE", state)
				groupDetails := gnmi.Get(t, dut, state.Config())
				index := *groupDetails.Index
				numBreakouts := *groupDetails.NumBreakouts
				breakoutSpeed := groupDetails.BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})

			t.Run(fmt.Sprintf("Configure DUT Interfaces with IPv4 For %v %v", tc.numbreakouts, tc.breakoutspeed), func(t *testing.T) {
				t.Logf("Start DUT interface Config.")

				breakOutPorts, err := findNewPortNames(dut, t, originalPortName.Name(), tc.numbreakouts)
				t.Log(breakOutPorts)
				if err != nil {
					t.Fatal(err)
				}

				sortBreakoutPorts(breakOutPorts)
				t.Log(breakOutPorts)

				Dutipv4Subnets, err = IncrementIPNetwork(dutIntfIp, tc.numbreakouts, true, 1)
				Ateipv4Subnets, err = IncrementIPNetwork(ateIntfIp, tc.numbreakouts, true, 2)

				if err != nil {
					t.Fatalf("Failed to generate IPv4 subnet addresses for DUT: %v", err)
				}

				for idx, portName := range breakOutPorts {
					// Extract the IP address without the subnet mask.
					ipv4Address := strings.Split(Dutipv4Subnets[idx], "/")[0]
					t.Logf("Configuring port %s with IPv4 address %s", portName, ipv4Address)

					i := &oc.Interface{
						Name:        ygot.String(portName),
						Description: ygot.String("Configured by GNMI"),
						Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
					}
					i.Enabled = ygot.Bool(true)

					s := i.GetOrCreateSubinterface(0)
					s4 := s.GetOrCreateIpv4()

					a := s4.GetOrCreateAddress(ipv4Address)
					a.PrefixLength = ygot.Uint8(dutPort1.IPv4Len)

					dc := gnmi.OC()
					gnmi.Update(t, dut, dc.Interface(portName).Config(), i)

				}

				t.Log("Configuring the ATE")
				configureOTG(t, ate, &tc.breakoutspeed, Ateipv4Subnets, Dutipv4Subnets, int(tc.numbreakouts))

			})

			t.Run(fmt.Sprintf("Ping ATE from DUT via Breakout Interface %v %v", tc.numbreakouts, tc.breakoutspeed), func(t *testing.T) {

				for i, dutAddrs := range Dutipv4Subnets {
					t.Log(i)
					ip, _, err := net.ParseCIDR(dutAddrs)
					dutAddrs := ip.String() // This is the IP address without the CIDR mask.

					if err != nil {
						t.Fatalf("Invalid IP address: %v", err)
					}

					pingRequest := &spb.PingRequest{
						Destination: dutAddrs,
						L3Protocol:  tpb.L3Protocol_IPV4,
					}

					t.Logf("Starting Ping to Destination %v", dutAddrs)

					pingClient, err := gnoiClient.System().Ping(context.Background(), pingRequest)

					t.Log(pingClient.RecvMsg("any"))
					if err != nil {
						t.Fatalf("Failed to query gnoi endpoint: %v", err)
					}

					responses, err := fetchResponses(pingClient)
					if err != nil {
						t.Fatalf("Failed to handle gnoi ping client stream: %v", err)
					}
					t.Logf("Got ping responses: Items: %v\n, Content: %v\n\n", len(responses), responses)
					if len(responses) == 0 {
						t.Errorf("Number of responses to %v: got 0, want > 0", pingRequest.Destination)
					}

					if responses[3].Source != dutAddrs {
						t.Errorf("Did not get A ping responses from ATE source Interface %s", responses[3].Source)
					} else {
						t.Logf("Got a successful reply from ATE Source Interface: %s", responses[3].Source)
					}

				}

			})

			t.Run(fmt.Sprintf("Replace//component[%v]/config/port/ %v*%v", componentName, tc.numbreakouts, tc.breakoutspeed), func(t *testing.T) {
				path := gnmi.OC().Component(componentName)
				gnmi.Replace(t, dut, path.Config(), portContainer)

			})

			t.Run(fmt.Sprintf("Delete//component[%v]/config/port/breakout-mode/group[1]/config", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue)
				// defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Delete(t, dut, path.Config())
				verifyDelete(t, dut, componentName, schemaValue)
			})

		}
	}

}
