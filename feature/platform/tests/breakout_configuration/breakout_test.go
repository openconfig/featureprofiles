package breakoutConfiguration

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygot/ygot"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	maxPingRetries = 3 // Set the number of retry attempts
	schemaValue    = 0
)

var (
	dutPortName                string
	foundExpectedInterfaceFlag bool = false
	breakOutCompName           string
	fullInterfaceName          string
	foundComp                  bool
	dutPort1                   = attrs.Attributes{
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

	// Order the ports from 1 to 8
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})

	if *breakoutspeed == oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB {
		t.Logf("Speed is %v", *breakoutspeed)
	} else if *breakoutspeed == oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB {
		t.Logf("Speed is needed to start port assignment on port5 as that is "+
			"where 10G ports are in setup %v", *breakoutspeed)
		ports = ports[4:] // Assuming ports 5+ are 10G
	}

	for i, port := range ports {

		// Remove the subnet mask from the IPv4 address
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
		t.Logf("Port Name: %s", port.Name())
		t.Logf("Port ID: %s", port.ID())

		t.Logf("ATE IPv4 Add is : %v and port is %v", ateIpAddress, port.ID())
		t.Logf("ATE IPv4 GW is DUT IP of : %v and port is %v", dutIpAddress, port.ID())

		// Add the port to the topology
		topPort := top.Ports().Add().SetName(ate.Port(t, port.ID()).ID())

		// Add a device for each port and set the Ethernet details
		i1 := top.Devices().Add().SetName(ate.Port(t, port.ID()).ID())
		macAddress := fmt.Sprintf("02:00:01:0%v:01:01", i+1)
		eth1 := i1.Ethernets().Add().SetName(port.ID() + ".Eth").SetMac(macAddress)

		// Set the Ethernet connection to the appropriate port
		eth1.Connection().SetPortName(topPort.Name())

		// Configure the IPv4 address for this interface
		eth1.Ipv4Addresses().Add().SetName(port.ID() + ateIpAddress).
			SetAddress(ateIpAddress).SetGateway(dutIpAddress).
			SetPrefix(uint32(24)) // Assuming /24 subnet

		// Exit loop when we've reached the number of breakout ports
		if i == numbreakouts-1 {
			break
		}
	}

	// Show the OTG Config
	t.Log("Complete configuration:", top.String())
	ate.OTG().PushConfig(t, top)
	time.Sleep(time.Second * 30)
	ate.OTG().StartProtocols(t)

	return top
}

func TestPlatformBreakoutConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var Dutipv4Subnets []string
	var Ateipv4Subnets []string

	cases := []struct {
		numbreakouts  uint8
		breakoutspeed oc.E_IfEthernet_ETHERNET_SPEED
		portPrefix    string
		dutIntfIP     string
		ateIntfIp     string
	}{
		{
			numbreakouts:  4,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
			portPrefix:    "HundredGigE",
			dutIntfIP:     dutPort1.IPv4,
			ateIntfIp:     atePort1.IPv4,
		},
		{
			portPrefix:    "HundredGigE",
			numbreakouts:  2,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
			dutIntfIP:     dutPort1.IPv4,
			ateIntfIp:     atePort1.IPv4,
		},
		{
			numbreakouts:  4,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
			portPrefix:    "TenGigE",
			dutIntfIP:     dutPort2.IPv4,
			ateIntfIp:     atePort2.IPv4,
		},
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	ate := ondatra.ATE(t, "ate")

	for _, tc := range cases {
		tc := tc // Capture range variable
		t.Run(fmt.Sprintf("Starting case for %d X %v", tc.numbreakouts, tc.breakoutspeed), func(t *testing.T) {

			if dut.Vendor() == ondatra.CISCO {
				breakOutCompName, fullInterfaceName, foundComp = getCompName(dut, dutPort1.IPv4, tc.portPrefix, t)
				t.Logf("breakOutCompName is: %s fullInterfaceName is %s: "+
					"fullInterfaceName and foundComp is %v", breakOutCompName, fullInterfaceName, foundComp)
				componentNameList = []string{breakOutCompName}
			} else {
				t.Fatalf("Unsupported vendor %s.  Need to add breakout component names.", dut.Vendor())
			}

			for _, componentName := range componentNameList {
				t.Logf("Starting Test for %s %v", componentName, tc)
				configContainer := &oc.Component_Port_BreakoutMode_Group{
					Index:         ygot.Uint8(0),
					NumBreakouts:  ygot.Uint8(tc.numbreakouts),
					BreakoutSpeed: oc.E_IfEthernet_ETHERNET_SPEED(tc.breakoutspeed),
				}
				groupContainer := &oc.Component_Port_BreakoutMode{Group: map[uint8]*oc.Component_Port_BreakoutMode_Group{1: configContainer}}
				breakoutContainer := &oc.Component_Port{BreakoutMode: groupContainer}
				portContainer := &oc.Component{Port: breakoutContainer, Name: ygot.String(componentName)}

				if deviations.VerifyExpectedBreakoutSupportedConfig(dut) {
					// deviation is the output "show controllers phy breakout interface" this command returns the following output
					// this will tell us if a given optic supports the attempted breakout config before applying it
					// leading to false positive failures
					//
					// DUT#show controllers optics 0/0/0/30 breakout-details
					// Optics Port                     : Optics0_0_0_30
					// No:of Breakouts                 : 2
					// Physical Channels per intf      : 2
					// Interface Speed                 : 100G
					t.Logf("sending fullInterfaceName to func %s", fullInterfaceName)
					if !isBreakoutSupported(t, dut, fullInterfaceName, tc.numbreakouts, tc.breakoutspeed) {
						t.Skipf("Skipping test case %dx%s: Configuration not supported",
							tc.numbreakouts, getSpeedValue(tc.breakoutspeed))
						return
					}
				}

				// Apply configuration
				gnmi.Update(t, dut, gnmi.OC().Component(componentName).Name().Config(), componentName)
				gnmi.Delete(t, dut, gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue).Config())
				path := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue)
				gnmi.Replace(t, dut, path.Config(), configContainer)

				t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port/breakout-mode/group[%v]",
					componentName, schemaValue), func(t *testing.T) {
					state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue)
					groupDetails := gnmi.Get(t, dut, state.Config())
					index := *groupDetails.Index
					numBreakouts := *groupDetails.NumBreakouts
					breakoutSpeed := groupDetails.BreakoutSpeed
					verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(),
						breakoutSpeed.String(), t)
				})

				t.Run(fmt.Sprintf("Configure DUT Interfaces with IPv4 For %v %v",
					tc.numbreakouts, tc.breakoutspeed), func(t *testing.T) {
					t.Logf("Start DUT interface Config.")

					breakOutPorts, err := findNewPortNames(dut, t, dutPortName, tc.numbreakouts)
					if err != nil {
						t.Fatal(err)
					}

					if dut.Vendor() == ondatra.CISCO {
						sortBreakoutPorts(breakOutPorts)
					}

					Dutipv4Subnets, err = IncrementIPNetwork(tc.dutIntfIP, tc.numbreakouts, true, 1)
					if err != nil {
						t.Fatalf("Failed to generate IPv4 subnet addresses for DUT: %v", err)
					}

					Ateipv4Subnets, err = IncrementIPNetwork(tc.ateIntfIp, tc.numbreakouts, true, 2)
					if err != nil {
						t.Fatalf("Failed to generate IPv4 subnet addresses for ATE: %v", err)
					}

					for idx, portName := range breakOutPorts {
						ipv4Address := strings.Split(Dutipv4Subnets[idx], "/")[0]
						t.Logf("Configuring port %s with IPv4 address %s", portName, ipv4Address)

						i := &oc.Interface{
							Name:        ygot.String(portName),
							Description: ygot.String("Configured by GNMI"),
							Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
							Enabled:     ygot.Bool(true),
						}

						s := i.GetOrCreateSubinterface(0)
						s4 := s.GetOrCreateIpv4()
						a := s4.GetOrCreateAddress(ipv4Address)
						a.PrefixLength = ygot.Uint8(dutPort1.IPv4Len)

						gnmi.Update(t, dut, gnmi.OC().Interface(portName).Config(), i)
					}

					t.Log("Configuring the ATE")
					configureOTG(t, ate, &tc.breakoutspeed, Ateipv4Subnets, Dutipv4Subnets,
						int(tc.numbreakouts))
				})

				t.Run(fmt.Sprintf("Ping ATE from DUT via Breakout Interface %v %v",
					tc.numbreakouts, tc.breakoutspeed), func(t *testing.T) {
					for _, dutAddrs := range Ateipv4Subnets {
						ip, _, err := net.ParseCIDR(dutAddrs)
						if err != nil {
							t.Fatalf("Invalid IP address: %v", err)
						}
						dutAddrs = ip.String()

						pingRequest := &spb.PingRequest{
							Destination: dutAddrs,
							L3Protocol:  tpb.L3Protocol_IPV4,
						}

						t.Logf("Starting Ping to Destination %v", dutAddrs)

						var responses []*spb.PingResponse
						var pingClient spb.System_PingClient

						for attempt := 1; attempt <= maxPingRetries; attempt++ {
							t.Logf("Ping attempt %d to %s", attempt, dutAddrs)
							pingClient, err = gnoiClient.System().Ping(context.Background(), pingRequest)
							if err != nil {
								t.Errorf("Failed to query gnoi endpoint on attempt %d: %v", attempt, err)
								time.Sleep(2 * time.Second)
								continue
							}

							responses, err = fetchResponses(pingClient)
							if err != nil {
								t.Errorf("Failed to handle gnoi ping client stream on attempt %d: %v",
									attempt, err)
								time.Sleep(2 * time.Second)
								continue
							}

							if len(responses) > 0 {
								t.Logf("Got ping responses on attempt %d: Items: %v, Content: %v",
									attempt, len(responses), responses)
								break
							}

							t.Logf("No responses received on attempt %d, retrying...", attempt)
							time.Sleep(2 * time.Second)
						}

						if len(responses) == 0 {
							t.Fatalf("Failed to get any ping responses to %v after %d attempts",
								pingRequest.Destination, maxPingRetries)
						}

						if responses[3].Source != dutAddrs {
							t.Errorf("Did not get a ping response from ATE source Interface %s",
								responses[3].Source)
						} else {
							t.Logf("Got a successful reply from ATE Source Interface: %s",
								responses[3].Source)
						}
					}
				})

				t.Run(fmt.Sprintf("Replace//component[%v]/config/port/ %v*%v",
					componentName, tc.numbreakouts, tc.breakoutspeed), func(t *testing.T) {
					path := gnmi.OC().Component(componentName)
					gnmi.Replace(t, dut, path.Config(), portContainer)
				})

				t.Run(fmt.Sprintf("Delete//component[%v]/config/port/breakout-mode/group[1]/config",
					componentName), func(t *testing.T) {
					path := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(schemaValue)
					gnmi.Delete(t, dut, path.Config())
					verifyDelete(t, dut, componentName, schemaValue)
				})
			}
		})
	}
}
