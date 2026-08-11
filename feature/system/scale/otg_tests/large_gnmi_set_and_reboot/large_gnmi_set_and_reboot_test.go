package large_gnmi_set_and_reboot_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/golang/ygot/ygot/ygot"
	"github.com/openconfig/featureprofiles/internal/components/components"
	"github.com/openconfig/featureprofiles/internal/fptest/fptest"
	spb "github.com/openconfig/gnoi/system/system_go_proto"
	"github.com/openconfig/ondatra/gnmi/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc/oc"
	"github.com/openconfig/ondatra/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func deviceBootStatus(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		// Poll system time as a sign of device being up
		val := gnmi.Lookup(t, dut, gnmi.OC().System().CurrentDatetime().State())
		if currentTime, ok := val.Val(); !ok || currentTime == "" {
			t.Logf("Device not yet reachable, keep polling ...")
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > 30 {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), 30)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
}

func verifyChassisIsAncestorLocal(t *testing.T, compMap map[string]*oc.Component, comp string) {
	visited := make(map[string]bool)
	for curr := comp; ; {
		if visited[curr] {
			t.Errorf("Component %s already visited; loop detected in the hierarchy.", curr)
			break
		}
		visited[curr] = true
		c, ok := compMap[curr]
		if !ok || c.GetParent() == "" {
			t.Errorf("Chassis component NOT found as an ancestor of component %s", comp)
			break
		}
		parentName := c.GetParent()
		parentComp, ok := compMap[parentName]
		if !ok {
			t.Errorf("Parent component %s not found in telemetry for component %s", parentName, curr)
			break
		}
		if parentComp.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS {
			t.Logf("Found chassis component as an ancestor of component %s", comp)
			break
		}
		// Not reached chassis yet; go one level up.
		curr = parentName
	}
}

func TestLargeGNMISetAndReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Pre-reboot checks
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)

	// Build the large configuration
	d := &oc.Root{}

	portIndex := 1
	lagIndex := 1

	createLAG := func(memberCount int) {
		lagName := fmt.Sprintf("port-channel%d", lagIndex)
		lag := d.GetOrCreateInterface(lagName)
		lag.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		lag.Enabled = ygot.Bool(true)
		lag.Name = ygot.String(lagName)
		lag.Description = ygot.String(fmt.Sprintf("LAG port %d", lagIndex))
		lag.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP

		subIntf := lag.GetOrCreateSubinterface(0)
		v4Addr := fmt.Sprintf("10.0.%d.1", lagIndex/255) // Just a dummy IP
		if lagIndex < 255 {
			v4Addr = fmt.Sprintf("10.0.0.%d", lagIndex)
		}
		subIntf.GetOrCreateIpv4().GetOrCreateAddress(v4Addr).PrefixLength = ygot.Uint8(24)
		subIntf.GetOrCreateIpv6().GetOrCreateAddress(fmt.Sprintf("2001:db8::%x", lagIndex)).PrefixLength = ygot.Uint8(64)

		for i := 0; i < memberCount; i++ {
			portName := fmt.Sprintf("port%d", portIndex)
			p := d.GetOrCreateInterface(portName)
			p.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			p.Enabled = ygot.Bool(true)
			p.Name = ygot.String(portName)
			p.Description = ygot.String(fmt.Sprintf("Physical port %d", portIndex))
			p.GetOrCreateEthernet().AggregateId = ygot.String(lagName)
			portIndex++
		}
		lagIndex++
	}

	// Configure 200 LAG interfaces with 2 member interfaces each on the DUT.
	for i := 0; i < 200; i++ {
		createLAG(2)
	}
	// Configure another 300 LAG interfaces with 1 member interface each on the DUT.
	for i := 0; i < 300; i++ {
		createLAG(1)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// We use WaitGroup to wait for both goroutines
	go func() {
		defer wg.Done()
		t.Logf("Starting gNMI Batch Set in Goroutine")

		b := &gnmi.SetBatch{}
		for _, intf := range d.Interface {
			gnmi.BatchReplace(b, gnmi.OC().Interface(*intf.Name).Config(), intf)
		}

		res := b.Set(t, dut)
		t.Logf("gNMI Batch Set completed successfully before reboot took effect. Result: %v", res)
	}()

	go func() {
		defer wg.Done()
		// Give a tiny moment to ensure gNMI Set starts before Reboot is triggered,
		// though the prompt says "trigger a complete chassis cold reboot ... in parallel".
		time.Sleep(50 * time.Millisecond)
		t.Logf("Starting gNOI Reboot in Goroutine")
		gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
		if err != nil {
			t.Errorf("Error dialing gNOI: %v", err)
			return
		}
		rebootRequest := &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Message: "SYS-5.1 Parallel Cold Reboot",
			Force:   true,
		}
		// Try triggering the reboot
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, err = gnoiClient.System().Reboot(ctxWithTimeout, rebootRequest)
		if err != nil {
			t.Logf("gNOI Reboot returned error, possibly transport closing: %v", err)
		} else {
			t.Logf("gNOI Reboot command sent successfully.")
		}
	}()

	// Wait for the operations to complete.
	wg.Wait()

	// Wait for device to boot up
	deviceBootStatus(t, dut)

	// Step 1: Verify boot time and software versions
	bootTimeAfterReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time after reboot: %v", bootTimeAfterReboot)
	if bootTimeAfterReboot == bootTimeBeforeReboot {
		t.Errorf("Boot time did not change after reboot. Before: %v, After: %v", bootTimeBeforeReboot, bootTimeAfterReboot)
	}

	// Validate components and software versions
	osList := components.FindSWComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_OPERATING_SYSTEM)
	if len(osList) == 0 {
		t.Log("No OS component found, skipping software version validation.")
	}
	for _, os := range osList {
		swVer := gnmi.Lookup(t, dut, gnmi.OC().Component(os).SoftwareVersion().State())
		if v, ok := swVer.Val(); ok && v != "" {
			t.Logf("Post-reboot system software version %q for component %v", v, os)
		} else {
			t.Errorf("Post-reboot system software version was not reported for component %v", os)
		}
	}

	// Step 2: Verify interface descriptions and IP addresses applied in SYS-5.1.1 are present
	t.Logf("Verifying configuration post-reboot")
	gotIntfs := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	if gotIntfs == nil {
		t.Fatalf("Failed to retrieve interfaces state after reboot")
	}

	// Check a few LAGs and physical interfaces to verify config persisted
	for i := 1; i <= 3; i++ {
		lagName := fmt.Sprintf("port-channel%d", i)
		portName := fmt.Sprintf("port%d", i)

		lagDesc := gnmi.Get(t, dut, gnmi.OC().Interface(lagName).Description().Config())
		if want := fmt.Sprintf("LAG port %d", i); lagDesc != want {
			t.Errorf("Interface %s description: got %v, want %v", lagName, lagDesc, want)
		}

		portDesc := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Description().Config())
		if want := fmt.Sprintf("Physical port %d", i); portDesc != want {
			t.Errorf("Interface %s description: got %v, want %v", portName, portDesc, want)
		}

		v4Addr := fmt.Sprintf("10.0.0.%d", i)
		ipv4Path := gnmi.OC().Interface(lagName).Subinterface(0).Ipv4().Address(v4Addr).Ip().Config()
		if got := gnmi.Get(t, dut, ipv4Path); got != v4Addr {
			t.Errorf("Interface %s IPv4 address: got %v, want %v", lagName, got, v4Addr)
		}
	}

	// Step 3: Issue a gNMI Set request to configure a test description on any one DUT interface.
	testInterface := "port-channel100"
	testDesc := "Test description post-reboot"
	t.Logf("Applying new description %q to %s to test configuration database", testDesc, testInterface)
	gnmi.Update(t, dut, gnmi.OC().Interface(testInterface).Description().Config(), testDesc)

	// Step 4: Verify the gnmi Set request succeeds (checked by gnmi.Update not failing)

	// Step 5: Verify the new description is applied correctly using a gNMI Get request.
	gotDesc := gnmi.Get(t, dut, gnmi.OC().Interface(testInterface).Description().Config())
	if gotDesc != testDesc {
		t.Errorf("Failed to update description post-reboot on %s: got %v, want %v", testInterface, gotDesc, testDesc)
	} else {
		t.Logf("Successfully verified new description %q on %s", gotDesc, testInterface)
	}
}

