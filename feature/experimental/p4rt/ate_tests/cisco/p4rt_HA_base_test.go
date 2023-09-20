package cisco_p4rt_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	p4rt_client "github.com/cisco-open/go-p4/p4rt_client"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

var (
	// exclude_LC = []string{"0/0/CPU0", "0/7/CPU0"}
	exclude_LC = []string{}
)

func getComponentList(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) []string {
	result := []string{}
	resp := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	component := oc.Component{}
	component.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	pattern, _ := regexp.Compile(`.*-NPU\d+`)

	for _, c := range resp {
		name := c.GetName()
		match := false
		for _, lc := range exclude_LC {
			if strings.Contains(name, lc) {
				match = true
				break
			}
		}
		if match {
			continue
		}
		if match := pattern.MatchString(name); match && !strings.Contains(name, "FC") {
			result = append(result, name)
		}
	}
	return result
}

func TestP4RTTMP(t *testing.T) {
	t.Skip()
	if !*ciscoFlags.HATests {
		t.Skip()
	}
	dut := ondatra.DUT(t, "dut")

	component := oc.Component{}
	component.IntegratedCircuit = &oc.Component_IntegratedCircuit{}

	resp := []string{"0/1/CPU0-NPU0", "0/1/CPU0-NPU1"}
	for i, name := range resp {
		component.Name = ygot.String(name)
		component.IntegratedCircuit.NodeId = ygot.Uint64(deviceID + uint64(i))
		gnmi.Replace(t, dut, gnmi.OC().Component(name).Config(), &component)
	}

}

func TestP4RTHA(t *testing.T) {
	if !*ciscoFlags.HATests {
		t.Skip()
	}
	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	// top.Push(t).StartProtocols(t)

	t.Logf("Start to configure device-id")
	configureDeviceID(ctx, t, dut)
	t.Logf("Start to configure port-id")
	configurePortID(ctx, t, dut)

	npus := getComponentList(ctx, t, dut)
	// npus = npus[:3]
	// fmt.Println(len(npus), npus)

	p4rtClientA := p4rt_client.P4RTClient{}
	if err := p4rtClientA.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	p4rtClientB := p4rt_client.P4RTClient{}
	if err := p4rtClientB.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	p4rtClientC := p4rt_client.P4RTClient{}
	if err := p4rtClientC.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	p4rtClientD := p4rt_client.P4RTClient{}
	if err := p4rtClientD.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	interfaceList := []string{}
	for i := 121; i < 128; i++ {
		interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
	}

	interfaces := interfaces{
		in:  []string{"Bundle-Ether120"},
		out: interfaceList,
	}

	args := &testArgs{
		ctx:         ctx,
		p4rtClientA: &p4rtClientA,
		p4rtClientB: &p4rtClientB,
		p4rtClientC: &p4rtClientC,
		p4rtClientD: &p4rtClientD,
		npus:        npus,
		dut:         dut,
		ate:         ate,
		top:         top,
		usecase:     0,
		interfaces:  &interfaces,
		prefix: &gribiPrefix{
			scale:           1,
			host:            "11.11.11.0",
			vrfName:         "TE",
			vipPrefixLength: "32",

			vip1Ip: "192.0.2.40",
			vip2Ip: "192.0.2.42",

			vip1NhIndex:  uint64(100),
			vip1NhgIndex: uint64(100),

			vip2NhIndex:  uint64(200),
			vip2NhgIndex: uint64(200),

			vrfNhIndex:  uint64(1000),
			vrfNhgIndex: uint64(1000),
		},
	}

	t.Logf("Start to get p4rt client")
	if err := setupScaleP4RTClient(ctx, t, npus, args); err != nil {
		t.Fatalf("Could not setup p4rt client: %v", err)
	}

	// time.Sleep(3600 * time.Second)

	for _, tt := range P4RTHATestcase {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			tt.fn(ctx, t, args)

			time.Sleep(5 * time.Second)
		})
	}

}

func setupScaleP4RTClient(ctx context.Context, t *testing.T, npus []string, args *testArgs) error {

	// Backup clients
	backupClients := []*p4rt_client.P4RTClient{
		args.p4rtClientB, args.p4rtClientC, args.p4rtClientD,
	}

	for i := 0; i < len(npus); i++ {
		if err := setupPrimaryP4RTClient(ctx, t, args.p4rtClientA, deviceID+uint64(i), electionID, fmt.Sprint(deviceID+uint64(i))); err != nil {
			return err
		}
		for j, client := range backupClients {
			if err := setupBackupP4RTClient(ctx, t, client, deviceID+uint64(i), electionID-1-uint64(j), fmt.Sprint(deviceID+uint64(i))); err != nil {
				return err
			}
		}
	}
	return nil
}
