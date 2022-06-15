package otgutils

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	"google.golang.org/protobuf/encoding/protojson"
)

// using protojson to marshal will emit property names with lowerCamelCase
// instead of snake_case
var protoMarshaller = protojson.MarshalOptions{UseProtoNames: true}
var prettyProtoMarshaller = protojson.MarshalOptions{UseProtoNames: true, Multiline: true}

// This struct is used at tests level whenever WaitFor func is called
type WaitForOpts struct {
	Condition string
	Interval  time.Duration
	Timeout   time.Duration
}

// Struct used for fetching OTG stats
type MetricsTableOpts struct {
	ClearPrevious  bool
	FlowMetrics    gosnappi.MetricsResponseFlowMetricIter
	PortMetrics    gosnappi.MetricsResponsePortMetricIter
	AllPortMetrics gosnappi.MetricsResponsePortMetricIter
	Bgpv4Metrics   gosnappi.MetricsResponseBgpv4MetricIter
	Bgpv6Metrics   gosnappi.MetricsResponseBgpv6MetricIter
	IsisMetrics    gosnappi.MetricsResponseIsisMetricIter
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
			return (fmt.Errorf("error waiting for %s: %v", opts.Condition, err))
		}
		if done {
			log.Printf("Done waiting for %s\n", opts.Condition)
			return nil
		}

		if time.Since(start) > opts.Timeout {
			return (fmt.Errorf("timeout occurred while waiting for %s", opts.Condition))
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

	if opts.AllPortMetrics != nil {
		border := strings.Repeat("-", 15*8-10)
		out += "\nPort Metrics\n" + border + "\n"
		out += fmt.Sprintf(
			"%-15s%-15s%-15s%-15s%-15s%-15s%-15s%-15s\n",
			"Name", "Frames Tx", "Frames Rx", "Bytes Tx", "Bytes Rx", "FPS Tx", "FPS Rx", "Link",
		)
		for _, m := range opts.AllPortMetrics.Items() {
			if m != nil {
				name := m.Name()
				txFrames := m.FramesTx()
				rxFrames := m.FramesRx()
				txBytes := m.BytesTx()
				rxBytes := m.BytesRx()
				txRate := m.FramesTxRate()
				rxRate := m.FramesRxRate()
				link := m.Link()
				out += fmt.Sprintf(
					"%-15v%-15v%-15v%-15v%-15v%-15v%-15v%-15v\n",
					name, txFrames, rxFrames, txBytes, rxBytes, txRate, rxRate, link,
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

func IncrementedMac(mac string, i int) (string, error) {
	// Uses an mac string and increments it by the given i
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}
