package helpers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// using protojson to marshal will emit property names with lowerCamelCase
// instead of snake_case
var protoMarshaller = protojson.MarshalOptions{UseProtoNames: true}
var prettyProtoMarshaller = protojson.MarshalOptions{UseProtoNames: true, Multiline: true}

type WaitForOpts struct {
	Condition string
	Interval  time.Duration
	Timeout   time.Duration
}

type MetricsTableOpts struct {
	ClearPrevious bool
	FlowMetrics   gosnappi.MetricsResponseFlowMetricIter
	PortMetrics   gosnappi.MetricsResponsePortMetricIter
	Bgpv4Metrics  gosnappi.MetricsResponseBgpv4MetricIter
	Bgpv6Metrics  gosnappi.MetricsResponseBgpv6MetricIter
	IsisMetrics   gosnappi.MetricsResponseIsisMetricIter
}

type StatesTableOpts struct {
	ClearPrevious       bool
	Ipv4NeighborsStates gosnappi.StatesResponseNeighborsv4StateIter
	Ipv6NeighborsStates gosnappi.StatesResponseNeighborsv6StateIter
}

func Timer(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %d ms", name, elapsed.Milliseconds())
}

func LogWarnings(warnings []string) {
	for _, w := range warnings {
		log.Printf("WARNING: %v", w)
	}
}

func LogErrors(errors *[]string) error {
	if errors == nil {
		return fmt.Errorf("")
	}
	for _, e := range *errors {
		log.Printf("ERROR: %v", e)
	}

	return fmt.Errorf("%v", errors)
}

func PrettyStructString(v interface{}) string {
	var bytes []byte
	var err error

	switch v := v.(type) {
	case protoreflect.ProtoMessage:
		bytes, err = prettyProtoMarshaller.Marshal(v)
		if err != nil {
			log.Println(err)
			return ""
		}
	default:
		bytes, err = json.MarshalIndent(v, "", "  ")
		if err != nil {
			log.Println(err)
			return ""
		}
	}

	return string(bytes)
}

func ProtoToJsonStruct(in protoreflect.ProtoMessage, out interface{}) error {
	log.Println("Marshalling from proto to json struct ...")

	bytes, err := protoMarshaller.Marshal(in)
	if err != nil {
		return fmt.Errorf("could not marshal from proto to json: %v", err)
	}
	if err := json.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("could not unmarshal from json to struct: %v", err)
	}
	return nil
}

func JsonStructToProto(in interface{}, out protoreflect.ProtoMessage) error {
	log.Println("Marshalling from struct to json ... ")

	bytes, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("could not marshal from struct to json: %v", err)
	}
	if err := protojson.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("could not unmarshal from json to proto: %v", err)
	}
	return nil
}

func WaitFor(t *testing.T, fn func() (bool, error), opts *WaitForOpts) error {
	if opts == nil {
		opts = &WaitForOpts{
			Condition: "condition to be true",
		}
	}
	defer Timer(time.Now(), fmt.Sprintf("Waiting for %s", opts.Condition))

	if opts.Interval == 0 {
		opts.Interval = 500 * time.Millisecond
	}
	if opts.Timeout == 0 {
		opts.Timeout = 120 * time.Second
	}

	start := time.Now()
	log.Printf("Waiting for %s ...\n", opts.Condition)

	for {
		done, err := fn()
		if err != nil {
			t.Fatal(fmt.Errorf("error waiting for %s: %v", opts.Condition, err))
		}
		if done {
			log.Printf("Done waiting for %s\n", opts.Condition)
			return nil
		}

		if time.Since(start) > opts.Timeout {
			t.Errorf("Timeout occurred while waiting for %s", opts.Condition)
			return nil
		}
		time.Sleep(opts.Interval)
	}
}

func ClearScreen() {
	switch runtime.GOOS {
	case "darwin":
		fallthrough
	case "linux":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		return
	}
}

func PrintMetricsTable(opts *MetricsTableOpts) {
	if opts == nil {
		return
	}
	opts.ClearPrevious = false
	out := "\n"

	if opts.Bgpv4Metrics != nil {
		border := strings.Repeat("-", 20*9+5)
		out += "\nBGPv4 Metrics\n" + border + "\n"
		rowNames := []string{
			"Name",
			"Session State",
			"Session Flaps",
			"Routes Advertised",
			"Routes Received",
			"Route Withdraws Tx",
			"Route Withdraws Rx",
			"Keepalives Tx",
			"Keepalives Rx",
		}

		for _, rowName := range rowNames {
			out += fmt.Sprintf("%-28s", rowName)
			for _, d := range opts.Bgpv4Metrics.Items() {
				if d != nil {
					switch rowName {
					case "Name":
						out += fmt.Sprintf("%-25v", d.Name())
					case "Session State":
						out += fmt.Sprintf("%-25v", d.SessionState())
					case "Session Flaps":
						out += fmt.Sprintf("%-25v", d.SessionFlapCount())
					case "Routes Advertised":
						out += fmt.Sprintf("%-25v", d.RoutesAdvertised())
					case "Routes Received":
						out += fmt.Sprintf("%-25v", d.RoutesReceived())
					case "Route Withdraws Tx":
						out += fmt.Sprintf("%-25v", d.RouteWithdrawsSent())
					case "Route Withdraws Rx":
						out += fmt.Sprintf("%-25v", d.RouteWithdrawsReceived())
					case "Keepalives Tx":
						out += fmt.Sprintf("%-25v", d.KeepalivesSent())
					case "Keepalives Rx":
						out += fmt.Sprintf("%-25v", d.KeepalivesReceived())
					}
				}
			}

			out += "\n"

		}
		out += border + "\n\n"
	}

	if opts.Bgpv6Metrics != nil {
		border := strings.Repeat("-", 20*9+5)
		out += "\nBGPv6 Metrics\n" + border + "\n"
		rowNames := []string{
			"Name",
			"Session State",
			"Session Flaps",
			"Routes Advertised",
			"Routes Received",
			"Route Withdraws Tx",
			"Route Withdraws Rx",
			"Keepalives Tx",
			"Keepalives Rx",
		}

		for _, rowName := range rowNames {
			out += fmt.Sprintf("%-28s", rowName)
			for _, d := range opts.Bgpv6Metrics.Items() {
				if d != nil {
					switch rowName {
					case "Name":
						out += fmt.Sprintf("%-25v", d.Name())
					case "Session State":
						out += fmt.Sprintf("%-25v", d.SessionState())
					case "Session Flaps":
						out += fmt.Sprintf("%-25v", d.SessionFlapCount())
					case "Routes Advertised":
						out += fmt.Sprintf("%-25v", d.RoutesAdvertised())
					case "Routes Received":
						out += fmt.Sprintf("%-25v", d.RoutesReceived())
					case "Route Withdraws Tx":
						out += fmt.Sprintf("%-25v", d.RouteWithdrawsSent())
					case "Route Withdraws Rx":
						out += fmt.Sprintf("%-25v", d.RouteWithdrawsReceived())
					case "Keepalives Tx":
						out += fmt.Sprintf("%-25v", d.KeepalivesSent())
					case "Keepalives Rx":
						out += fmt.Sprintf("%-25v", d.KeepalivesReceived())
					}
				}
			}

			out += "\n"

		}
		out += border + "\n\n"
	}

	if opts.IsisMetrics != nil {
		border := strings.Repeat("-", 20*9+5)
		out += "\nIS-IS Metrics\n" + border + "\n"
		rowNames := []string{
			"Name",
			"L1 Sessions UP",
			"L1 Sessions Flap",
			"L1 Broadcast Hellos Sent",
			"L1 Broadcast Hellos Recv",
			"L1 P2P Hellos Sent",
			"L1 P2P Hellos Recv",
			"L1 Lsp Sent",
			"L1 Lsp Recv",
			"L1 Database Size",
			"L2 Sessions UP",
			"L2 Sessions Flap",
			"L2 Broadcast Hellos Sent",
			"L2 Broadcast Hellos Recv",
			"L2 P2P Hellos Sent",
			"L2 P2P Hellos Recv",
			"L2 Lsp Sent",
			"L2 Lsp Recv",
			"L2 Database Size",
		}
		for _, rowName := range rowNames {
			out += fmt.Sprintf("%-28s", rowName)
			for _, d := range opts.IsisMetrics.Items() {
				if d != nil {
					switch rowName {
					case "Name":
						out += fmt.Sprintf("%-25v", d.Name())
					case "L1 Sessions UP":
						out += fmt.Sprintf("%-25v", d.L1SessionsUp())
					case "L1 Sessions Flap":
						out += fmt.Sprintf("%-25v", d.L1SessionFlap())
					case "L1 Broadcast Hellos Sent":
						out += fmt.Sprintf("%-25v", d.L1BroadcastHellosSent())
					case "L1 Broadcast Hellos Recv":
						out += fmt.Sprintf("%-25v", d.L1BroadcastHellosReceived())
					case "L1 P2P Hellos Sent":
						out += fmt.Sprintf("%-25v", d.L1PointToPointHellosSent())
					case "L1 P2P Hellos Recv":
						out += fmt.Sprintf("%-25v", d.L1PointToPointHellosReceived())
					case "L1 Lsp Sent":
						out += fmt.Sprintf("%-25v", d.L1LspSent())
					case "L1 Lsp Recv":
						out += fmt.Sprintf("%-25v", d.L1LspReceived())
					case "L1 Database Size":
						out += fmt.Sprintf("%-25v", d.L1DatabaseSize())
					case "L2 Sessions UP":
						out += fmt.Sprintf("%-25v", d.L2SessionsUp())
					case "L2 Sessions Flap":
						out += fmt.Sprintf("%-25v", d.L2SessionFlap())
					case "L2 Broadcast Hellos Sent":
						out += fmt.Sprintf("%-25v", d.L2BroadcastHellosSent())
					case "L2 Broadcast Hellos Recv":
						out += fmt.Sprintf("%-25v", d.L2BroadcastHellosReceived())
					case "L2 P2P Hellos Sent":
						out += fmt.Sprintf("%-25v", d.L2PointToPointHellosSent())
					case "L2 P2P Hellos Recv":
						out += fmt.Sprintf("%-25v", d.L2PointToPointHellosReceived())
					case "L2 Lsp Sent":
						out += fmt.Sprintf("%-25v", d.L2LspSent())
					case "L2 Lsp Recv":
						out += fmt.Sprintf("%-25v", d.L2LspReceived())
					case "L2 Database Size":
						out += fmt.Sprintf("%-25v", d.L2DatabaseSize())
					}
				}
			}
			out += "\n"
		}
		out += border + "\n\n"
	}

	if opts.PortMetrics != nil {
		border := strings.Repeat("-", 15*4+5)
		out += "\nPort Metrics\n" + border + "\n"
		out += fmt.Sprintf(
			"%-15s%-15s%-15s%-15s\n",
			"Name", "Frames Tx", "Frames Rx", "FPS Tx",
		)
		for _, m := range opts.PortMetrics.Items() {
			if m != nil {
				name := m.Name()
				tx := m.FramesTx()
				rx := m.FramesRx()
				txRate := m.FramesTxRate()

				out += fmt.Sprintf(
					"%-15v%-15v%-15v%-15v\n",
					name, tx, rx, txRate,
				)
			}
		}
		out += border + "\n\n"
	}

	if opts.FlowMetrics != nil {
		border := strings.Repeat("-", 32*3+10)
		out += "\nFlow Metrics\n" + border + "\n"
		out += fmt.Sprintf("%-25s%-25s%-25s%-25s%-25s\n", "Name", "Frames Tx", "Frames Rx", "FPS Tx", "FPS Rx")
		for _, m := range opts.FlowMetrics.Items() {
			if m != nil {
				name := m.Name()
				tx := m.FramesTx()
				rx := m.FramesRx()
				txRate := m.FramesTxRate()
				rxRate := m.FramesRxRate()
				out += fmt.Sprintf("%-25v%-25v%-25v%-25v%-25v\n", name, tx, rx, txRate, rxRate)
			}
		}
		out += border + "\n\n"
	}

	if opts.ClearPrevious {
		ClearScreen()
	}
	log.Println(out)
}

func GetCapturePorts(c gosnappi.Config) []string {
	capturePorts := []string{}
	if c == nil {
		return capturePorts
	}

	for _, capture := range c.Captures().Items() {
		capturePorts = append(capturePorts, capture.PortNames()...)
	}
	return capturePorts
}

func CleanupTest(t *testing.T, ate *ondatra.ATEDevice, otg *ondatra.OTG, stopProtocols bool, stopTraffic bool) {
	if stopTraffic {
		otg.StopTraffic(t)
	}
	if stopProtocols {
		otg.StopProtocols(t)
	}
	otg.PushConfig(t, otg.NewConfig(t))
}

func WatchFlowMetrics(t *testing.T, otg *ondatra.OTG, c gosnappi.Config, opts *WaitForOpts) error {
	start := time.Now()
	for {
		fMetrics, err := GetFlowMetrics(t, otg, c)
		if err != nil {
			return err
		}
		PrintMetricsTable(&MetricsTableOpts{
			ClearPrevious: false,
			FlowMetrics:   fMetrics,
		})

		if time.Since(start) > opts.Timeout {
			return nil
		}
		time.Sleep(opts.Interval)
	}
}

func PrintStatesTable(opts *StatesTableOpts) {
	if opts == nil {
		return
	}
	out := "\n"

	if opts.Ipv4NeighborsStates != nil {
		border := strings.Repeat("-", 25*3+5)
		out += "\nIPv4 Neighbors States\n" + border + "\n"
		out += fmt.Sprintf(
			"%-25s%-25s%-25s\n",
			"Ethernet Name", "IPv4 Address", "Link Layer Address",
		)
		for _, state := range opts.Ipv4NeighborsStates.Items() {
			if state != nil {
				ethernetName := state.EthernetName()
				ipv4Address := state.Ipv4Address()
				linkLayerAddress := ""
				if state.HasLinkLayerAddress() {
					linkLayerAddress = state.LinkLayerAddress()
				}

				out += fmt.Sprintf(
					"%-25v%-25v%-25v\n",
					ethernetName, ipv4Address, linkLayerAddress,
				)
			}
		}
		out += border + "\n\n"
	}

	if opts.Ipv6NeighborsStates != nil {
		border := strings.Repeat("-", 35*3+5)
		out += "\nIPv6 Neighbors States\n" + border + "\n"
		out += fmt.Sprintf(
			"%-25s%-55s%-55s\n",
			"Ethernet Name", "IPv6 Address", "Link Layer Address",
		)
		for _, state := range opts.Ipv6NeighborsStates.Items() {
			if state != nil {
				ethernetName := state.EthernetName()
				ipv6Address := state.Ipv6Address()
				linkLayerAddress := ""
				if state.HasLinkLayerAddress() {
					linkLayerAddress = state.LinkLayerAddress()
				}

				out += fmt.Sprintf(
					"%-25v%-55v%-55v\n",
					ethernetName, ipv6Address, linkLayerAddress,
				)
			}
		}
		out += border + "\n\n"
	}

	if opts.ClearPrevious {
		ClearScreen()
	}
	log.Println(out)
}

func expectedElementsPresent(expected, actual []string) bool {
	exists := make(map[string]bool)
	for _, value := range actual {
		exists[value] = true
	}
	for _, value := range expected {
		if !exists[value] {
			return false
		}
	}
	return true
}
