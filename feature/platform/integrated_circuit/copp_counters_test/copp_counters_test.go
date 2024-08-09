// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package copp_counters_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	gnmic "github.com/openconfig/gnmic/pkg/api/path"
	// "github.com/openconfig/lemming/gnmi/oc"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra/gnmi"
)

type utilization struct {
	used                uint64
	free                uint64
	upperThreshold      uint8
	upperThresholdClear uint8
}

func (u *utilization) percent() uint8 {
	if u.used == 0 && u.free == 0 {
		return 0
	}
	return uint8(u.used * 100 / (u.used + u.free))
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func getSubsriptionSlice() []*gnmipb.Subscription {
	filesList := []string{"./OC_Paths_Trap.txt", "./OC_Paths_LPTS.txt"}
	// filesList := []string{"./OC_Paths_Trap_d.txt", "./OC_Paths_LPTS_d.txt"}
	// filesList := []string{"./OC_Paths_LPTS_alt.txt"}

	var lines []string
	for _, entry := range filesList {
		file, err := os.Open(entry)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
	}

	var subPaths []*gnmipb.Path

	for _, line := range lines {
		split := strings.SplitN(line, ":", 2)

		path, _ := gnmic.ParsePath(split[1])
		path.Origin = split[0]

		subPaths = append(subPaths, path)
	}

	var subs []*gnmipb.Subscription
	for _, path := range subPaths {
		subs = append(subs, &gnmipb.Subscription{
			Path: &gnmipb.Path{
				Elem:   path.GetElem(),
				Origin: path.GetOrigin(),
			},
			Mode: gnmipb.SubscriptionMode_TARGET_DEFINED,
			// SampleInterval: 1000000000,
		})
	}

	return subs
}

func ParsePath(pathStr string) *gnmipb.Path {
	split := strings.SplitN(pathStr, ":", 2)

	path, _ := gnmic.ParsePath(split[1])
	path.Origin = split[0]

	return path
}

func TestCoppCounterPaths(t *testing.T) {

	dut := ondatra.DUT(t, "dut")

	gnmiClient := dut.RawAPIs().GNMI(t)

	// gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().ControlPlaneTraffic()

	subList := getSubsriptionSlice()

	for _, subEntry := range subList {
		subClient, err := gnmiClient.Subscribe(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		subReq := &gnmipb.SubscribeRequest{
			Request: &gnmipb.SubscribeRequest_Subscribe{
				Subscribe: &gnmipb.SubscriptionList{
					// Prefix:       &gnmipb.Path{},
					Subscription: []*gnmipb.Subscription{subEntry},
					Mode:         gnmipb.SubscriptionList_ONCE,
					Encoding:     gnmipb.Encoding_PROTO,
				},
			},
		}

		subClient.Send(subReq)

		for {
			resp, err := subClient.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("error while reading response: %v", err)
			} else {
				origin := resp.GetUpdate().GetPrefix().GetOrigin()
				prefixElems := resp.GetUpdate().GetPrefix().GetElem()
				fmt.Printf("%s:", origin)
				for i, elem := range prefixElems {
					if i > 0 {
						fmt.Print("/")
					}
					fmt.Printf("%s", elem.GetName())
				}
				for i, upd := range resp.GetUpdate().GetUpdate() {
					if i == 0 {
						pathElems := upd.GetPath().GetElem()
						for i, elem := range pathElems {
							if i == len(pathElems)-1 {
								break
							}
							if i > 0 {
								fmt.Print("/")
							}
							fmt.Printf("%s", elem.GetName())
						}
					}
					fmt.Printf("/%s: %d\n", upd.GetPath().GetElem()[len(upd.GetPath().GetElem())-1].GetName(), upd.GetVal().GetUintVal())
				}
			}

		}
		err = subClient.CloseSend()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println()
	}
}

// func configureDUTPorts(t *testing.T, dut *ondatra.DUTDevice) {
// 	p1 := dut.Port(t, "port1")
// 	p2 := dut.Port(t, "port2")
// 	b := &gnmi.SetBatch{}
// 	gnmi.BatchReplace(b, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
// 	gnmi.BatchReplace(b, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
// 	b.Set(t, dut)
// }

// func configureOTGPorts(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) []gosnappi.Device {
// 	t.Helper()
//
// 	p1 := ate.Port(t, "port1")
// 	p2 := ate.Port(t, "port2")
//
// 	d1 := atePort1.AddToOTG(top, p1, &dutPort1)
// 	d2 := atePort2.AddToOTG(top, p2, &dutPort2)
// 	
// 	ate.Ports()
//
// 	ateP
// 	
// 	return []gosnappi.Device{d1, d2}
// }

func runTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	ate.OTG().StartTraffic(t)
	time.Sleep(5 * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogLAGMetrics(t, ate.OTG(), top)
}

func configureOTGFlowSNMP(t *testing.T, config gosnappi.Config, dutPort ondatra.Port, atePort ondatra.Port) {
	t.Helper()
	
	p1 := config.Ports().Add().SetName(atePort.ID()).SetLocation(atePort.Name())
	
	// Define a traffic flow
	flow := config.Flows().Add().SetName("SNMP Traffic")
	flow.TxRx().Port().SetTxName(p1.Name())
		// .SetRxName(p1.Name())
	// Configure Ethernet layer
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue("00:0c:29:73:8b:9e")
	eth.Dst().SetValue("00:0c:29:73:8b:9f")
	// Configure IPv4 layer
	ipv4 := flow.Packet().Add().Ipv4()
	ipv4.Src().SetValue("192.168.1.10")
	ipv4.Dst().SetValue("192.168.1.1")
	// Configure UDP layer
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValue(12345)
	udp.DstPort().SetValue(162) // SNMP trap port
	
	
	// Configure SNMP payload
	snmp := flow.Packet().Add().Custom()
	snmpPayload := 
`0x30 0x81 0xc9 0x02 0x01 0x03 0x30 0x11 0x02 0x04 0x30 0xf6 0xf3 0xd9 0x02 0x03 0x00 0xff 0xe3 0x04 0x01 0x07 0x02 0x01 0x03 0x04 0x37 0x30 0x35 0x04 0x0d 0x80 0x00 0x1f 0x88 0x80 0x59 0xdc 0x48 0x61 0x45 0xa2 0x63 0x22 0x02 0x01 0x08 0x02 0x02 0x0a 0xb9 0x04 0x05 0x70 0x69 0x70 0x70 0x6f 0x04 0x0c 0x0d 0xe5 0x24 0x29 0xf9 0x86 0x68 0x6b 0xb0 0x72 0x5e 0xa8 0x04 0x08 0x00 0x00 0x00 0x01 0x03 0xd5 0x32 0x1e 0x04 0x78 0x74 0xa9 0xa8 0xf4 0x56 0x14 0x4a 0xef 0xc7 0x86 0x01 0x21 0xe3 0xfb 0xcf 0x8e 0xcc 0x9c 0x83 0xe6 0x8a 0x47 0x0e 0x99 0xfc 0x59 0x7b 0x07 0x15 0xcd 0x14 0xe3 0x10 0x1a 0xde 0xfd 0xe8 0x0c 0x8a 0x0b 0x3a 0x66 0xb4 0xe9 0xa0 0x03 0x4e 0x0f 0x35 0x7f 0xf2 0xc0 0xdf 0x15 0xde 0x5b 0x2e 0xc4 0x7c 0xa9 0xbc 0xb7 0x3f 0x11 0x70 0x02 0x0c 0x1e 0x8b 0x8c 0x08 0x07 0xf1 0x1c 0xaf 0xfd 0xe7 0x13 0xd5 0xab 0x68 0x1c 0x09 0xf8 0x88 0x99 0x01 0xe5 0xf9 0xe6 0xe1 0x1f 0xbf 0x66 0x65 0xd9 0x69 0x90 0x3e 0x7f 0x72 0x3a 0xcf 0x39 0x00 0x0a 0x2c 0x9f 0x59 0x1e 0x0f 0x7f 0x05 0xe3 0xa1 0x5f 0xf6 0x64 0xa7 0xa7`
	
	fmt.Println(snmpPayload)
	snmp.SetBytes(snmpPayload)

	// snmp := flow.Packet().Add().Snmpv2C()
	// snmp.SetData(gosnappi.NewFlowSnmpv2C().Data())
	
	// Set transmission parameters
	flow.Rate().SetPercentage(100)  // 100% line rate
	flow.Duration().FixedPackets().SetPackets(10)  // Send 10 packets
}


func TestCoppCounterPathsOTG(t *testing.T) {
	t.Skip()
	
	dut := ondatra.DUT(t, "dut")

	gnmiClient := dut.RawAPIs().GNMI(t)

	// OTG
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	// devs := configureOTGPorts(t, ate, top)
	// ports
	
	p1Dut := dut.Port(t, "port1")
	// p2Dut := dut.Port(t, "port2")
	
	p1Ate := ate.Port(t, "port1")
	// p2Ate := ate.Port(t, "port1")
	
	configureOTGFlowSNMP(t, top, *p1Dut, *p1Ate)

	ate.OTG().PushConfig(t, top)

	// t.Log(devs, p1Dut, p2Dut)
	
	ate.OTG().StartProtocols(t)
	runTraffic(t, ate, top)
	ate.OTG().StopProtocols(t)

	// wait for gnmi to update
	time.Sleep(30 * time.Second)
	
	// flow := top.Flows().Add()
	// arp := flow.Packet().Add().Arp()
	// OTG END

	// gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().ControlPlaneTraffic()

	subList := getSubsriptionSlice()

	for _, subEntry := range subList {
		subClient, err := gnmiClient.Subscribe(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		subReq := &gnmipb.SubscribeRequest{
			Request: &gnmipb.SubscribeRequest_Subscribe{
				Subscribe: &gnmipb.SubscriptionList{
					// Prefix:       &gnmipb.Path{},
					Subscription: []*gnmipb.Subscription{subEntry},
					Mode:         gnmipb.SubscriptionList_ONCE,
					Encoding:     gnmipb.Encoding_PROTO,
				},
			},
		}

		subClient.Send(subReq)

		// resp, _ := subClient.Recv()

		// jsonOb, _ := json.MarshalIndent(resp, "", "\t")
		// fmt.Println(string(jsonOb))

		for {
			resp, err := subClient.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("error while reading response: %v", err)
			} else {
				origin := resp.GetUpdate().GetPrefix().GetOrigin()
				prefixElems := resp.GetUpdate().GetPrefix().GetElem()
				fmt.Printf("%s:", origin)
				for i, elem := range prefixElems {
					if i > 0 {
						fmt.Print("/")
					}
					fmt.Printf("%s", elem.GetName())
				}
				for i, upd := range resp.GetUpdate().GetUpdate() {
					if i == 0 {
						pathElems := upd.GetPath().GetElem()
						for i, elem := range pathElems {
							if i == len(pathElems)-1 {
								break
							}
							if i > 0 {
								fmt.Print("/")
							}
							fmt.Printf("%s", elem.GetName())
						}
					}
					fmt.Printf("/%s: %d\n", upd.GetPath().GetElem()[len(upd.GetPath().GetElem())-1].GetName(), upd.GetVal().GetUintVal())
				}
			}

		}
		err = subClient.CloseSend()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println()
	}
}

func TestAggregateCounterPaths(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	gnmiClient := dut.RawAPIs().GNMI(t)

	subList := getSubsriptionSlice()
	
	fmt.Println("Getting real counters")
	realLeaf := gnmi.Get(t, dut, gnmi.OC().Component("0/RP0/CPU0-NPU0").IntegratedCircuit().PipelineCounters().ControlPlaneTraffic().State())
	fmt.Println("Finished getting real counters")
	
	realLeafMap := map[string]uint64{
		"queued": realLeaf.GetQueuedAggregate(), 
		"queued-bytes": realLeaf.GetQueuedBytesAggregate(),
		"dropped": realLeaf.GetDroppedAggregate(), 
		"dropped-bytes": realLeaf.GetDroppedBytesAggregate(),
	}

	manualAggregation := map[string]uint64{
		"queued": 0,
		"queued-bytes": 0,
		"dropped": 0,
		"dropped-bytes": 0,
	}
	
	fmt.Printf("%s\n%s\t\t%s\t\t%s\t%s\t\t\t\n",
		"leaf-name",
		"queued",
		"queued-bytes",
		"dropped",
		"dropped-bytes",
	)
	
	for _, subEntry := range subList {
		subClient, err := gnmiClient.Subscribe(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		subReq := &gnmipb.SubscribeRequest{
			Request: &gnmipb.SubscribeRequest_Subscribe{
				Subscribe: &gnmipb.SubscriptionList{
					// Prefix:       &gnmipb.Path{},
					Subscription: []*gnmipb.Subscription{subEntry},
					Mode:         gnmipb.SubscriptionList_ONCE,
					Encoding:     gnmipb.Encoding_PROTO,
				},
			},
		}

		subClient.Send(subReq)

		tempmap := map[string]uint64{}
		var pathname string
		
		for {
			resp, err := subClient.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("error while reading response: %v", err)
			} else {
				for i, upd := range resp.GetUpdate().GetUpdate() {
					if i == 0 {
						pathname = upd.GetPath().GetElem()[len(upd.GetPath().GetElem())-2].GetName()
					}
					key := upd.GetPath().GetElem()[len(upd.GetPath().GetElem())-1].GetName()
					val := upd.GetVal().GetUintVal()
					tempmap[key] = val
					manualAggregation[key] = manualAggregation[key] + val
				}
			}
		}
		fmt.Printf("%s\n%d\t\t%dB\t\t%d\t\t%dB\t\t\n",
			pathname,
			tempmap["queued"],
			tempmap["queued-bytes"],
			tempmap["dropped"],
			tempmap["dropped-bytes"],
		)
		err = subClient.CloseSend()
		if err != nil {
			t.Fatal(err)
		}
	}
	
	fmt.Printf("%s\n%d\t\t%dB\t\t%d\t\t%dB\t\t\n",
		"TOTAL",
		manualAggregation["queued"],
		manualAggregation["queued-bytes"],
		manualAggregation["dropped"],
		manualAggregation["dropped-bytes"],
	)

	fmt.Printf("%s\n%d\t\t%dB\t\t%d\t\t%dB\t\t\n",
		"WANT",
		realLeafMap["queued"],
		realLeafMap["queued-bytes"],
		realLeafMap["dropped"],
		realLeafMap["dropped-bytes"],
	)
	
	if eq := reflect.DeepEqual(manualAggregation, realLeafMap); !eq {
		t.Fatalf("manual calculation of aggregate counter leaves do not match real values.\ncalculated aggregation: %+v\nreal values: %+v", manualAggregation, realLeafMap)
	}

}

func GetAllNativeModel(t testing.TB, dut *ondatra.DUTDevice, str string) (any, error) {

	split := strings.Split(str, ":")
	origin := split[0]
	paths := strings.Split(split[1], "/")
	pathelems := []*gnmipb.PathElem{}
	for _, path := range paths {
		pathelems = append(pathelems, &gnmipb.PathElem{Name: path})
	}

	req := &gnmipb.GetRequest{
		Path: []*gnmipb.Path{
			{
				Origin: origin,
				Elem:   pathelems,
			},
		},
		Type:     gnmipb.GetRequest_ALL,
		Encoding: gnmipb.Encoding_JSON_IETF,
	}
	var responseRawObj any
	restartResp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed GNMI GET request on native model: \n%v", req)
	} else {
		jsonIetfData := restartResp.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
		err = json.Unmarshal(jsonIetfData, &responseRawObj)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal native model GET json")
		}
	}
	return responseRawObj, nil
}

// func configureOTGFlows(t *testing.T,
// 	top gosnappi.Config,
// 	devs []gosnappi.Device) {
// 	t.Helper()
//
// 	otgP1 := devs[0]
// 	otgP2 := devs[1]
//
// 	srcV4 := otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
// 	srcV6 := otgP1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
//
// 	dst1V4 := otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
// 	dst1V6 := otgP2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
//
// 	v4F := top.Flows().Add()
// 	v4F.SetName(v4Flow).Metrics().SetEnable(true)
// 	v4F.TxRx().Device().SetTxNames([]string{srcV4.Name()}).SetRxNames([]string{dst1V4.Name()})
//
// 	v4FEth := v4F.Packet().Add().Ethernet()
// 	v4FEth.Src().SetValue(atePort1.MAC)
//
// 	v4FIp := v4F.Packet().Add().Ipv4()
// 	v4FIp.Src().SetValue(srcV4.Address())
// 	v4FIp.Dst().Increment().SetStart(v4TrafficStart).SetCount(254)
//
// 	eth := v4F.EgressPacket().Add().Ethernet()
// 	ethTag := eth.Dst().MetricTags().Add()
// 	ethTag.SetName("MACTrackingv4").SetOffset(36).SetLength(12)
//
// 	v6F := top.Flows().Add()
// 	v6F.SetName(v6Flow).Metrics().SetEnable(true)
// 	v6F.TxRx().Device().SetTxNames([]string{srcV6.Name()}).SetRxNames([]string{dst1V6.Name()})
//
// 	v6FEth := v6F.Packet().Add().Ethernet()
// 	v6FEth.Src().SetValue(atePort1.MAC)
//
// 	v6FIP := v6F.Packet().Add().Ipv6()
// 	v6FIP.Src().SetValue(srcV6.Address())
// 	v6FIP.Dst().Increment().SetStart(v6TrafficStart).SetCount(1)
//
// 	eth = v6F.EgressPacket().Add().Ethernet()
// 	ethTag = eth.Dst().MetricTags().Add()
// 	ethTag.SetName("MACTrackingv6").SetOffset(36).SetLength(12)
//
// }
