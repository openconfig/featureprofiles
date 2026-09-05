package large_gnmi_set_and_reboot_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestLargeGNMISetAndReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Pre-reboot checks - Capture the initial boot time.
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)

	// Define the root config to collect interfaces in a batch map.
	d := &oc.Root{}

	portIndex := 1
	lagIndex := 1

	// Helper function to generate aggregate LAGs and member physical interfaces.
	createLAG := func(memberCount int) {
		lagName := fmt.Sprintf("port-channel%d", lagIndex)
		lag := d.GetOrCreateInterface(lagName)
		lag.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		lag.Enabled = ygot.Bool(true)
		lag.Name = ygot.String(lagName)
		lag.Description = ygot.String(fmt.Sprintf("LAG port %d", lagIndex))
		lag.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP

		subIntf := lag.GetOrCreateSubinterface(0)
		v4Addr := fmt.Sprintf("100.64.%d.%d", lagIndex/252, (lagIndex%252)+1)
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

	// SYS-5.1.1 - gNMI Batch Set and reboot immediately.
	// SYS-5.1.1 - Step 1: Create a gNMI batch configuration to:
	// Configure description and IP addresses on all 700 Physical and 500 LAG interfaces.
	for i := 0; i < 200; i++ {
		createLAG(2)
	}
	for i := 0; i < 300; i++ {
		createLAG(1)
	}

	t.Cleanup(func() {
		t.Logf("Cleaning up configured interfaces...")
		b := &gnmi.SetBatch{}
		for _, intf := range d.Interface {
			gnmi.BatchDelete(b, gnmi.OC().Interface(*intf.Name).Config())
		}
		b.Set(t, dut)
	})

	var wg sync.WaitGroup
	wg.Add(2)

	setInitiated := make(chan struct{})

	// Trigger goroutines for parallel operations.
	// Thread 1: Perform the large gNMI Batch Set.
	go func() {
		defer wg.Done()
		t.Logf("SYS-5.1.1 - Step 2: Starting gNMI Batch Set in Goroutine")

		b := &gnmi.SetBatch{}
		for _, intf := range d.Interface {
			gnmi.BatchReplace(b, gnmi.OC().Interface(*intf.Name).Config(), intf)
		}

		close(setInitiated)
		res := b.Set(t, dut)
		t.Logf("gNMI Batch Set completed successfully before reboot took effect. Result: %v", res)
	}()

	// Thread 2: Trigger a cold reboot of the chassis.
	go func() {
		defer wg.Done()
		<-setInitiated
		t.Logf("SYS-5.1.1 - Step 3: Starting gNOI Reboot in Goroutine")
		gnoiClient := dut.RawAPIs().GNOI(t)
		rebootRequest := &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Message: "SYS-5.1 Parallel Cold Reboot",
			Force:   true,
		}

		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, err := gnoiClient.System().Reboot(ctxWithTimeout, rebootRequest)
		if err != nil {
			t.Logf("SYS-5.1.1 - Step 3 NOTE: gNOI Reboot returned error, possibly transport closing: %v", err)
		} else {
			t.Logf("gNOI Reboot command sent successfully.")
		}
	}()

	// Wait for both concurrent operations to conclude.
	wg.Wait()

	// Wait for the device to boot up and return to service. (SYS-5.1.1 - Step 4)
	bootTimeAfterReboot := helpers.AwaitDUTReboot(t, dut, bootTimeBeforeReboot)

	// SYS-5.1.2 - Post-reboot configuration validation

	// SYS-5.1.2 - Step 1: Verify boot time and software versions.
	t.Logf("SYS-5.1.2 - Step 1: DUT boot time after reboot: %v", bootTimeAfterReboot)
	if bootTimeAfterReboot <= bootTimeBeforeReboot && bootTimeBeforeReboot != 0 {
		t.Errorf("Boot time did not increase after reboot. Before: %v, After: %v", bootTimeBeforeReboot, bootTimeAfterReboot)
	}

	// Validate OS, BIOS, and BootLoader software versions dynamically.
	compTypes := []oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT{
		oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_OPERATING_SYSTEM,
		oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_BIOS,
		oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_BOOT_LOADER,
	}

	for _, cType := range compTypes {
		cList := components.FindSWComponentsByType(t, dut, cType)
		if len(cList) == 0 {
			t.Logf("No %v components found, skipping software version validation for this type.", cType)
			continue
		}
		for _, comp := range cList {
			swVer := gnmi.Lookup(t, dut, gnmi.OC().Component(comp).SoftwareVersion().State())
			if v, ok := swVer.Val(); ok && v != "" {
				t.Logf("Post-reboot system software version %q for component %v (%v)", v, comp, cType)
			} else {
				t.Errorf("Post-reboot system software version was not reported for component %v (%v)", comp, cType)
			}
		}
	}

	// SYS-5.1.2 - Step 2: Verify interface descriptions and IP addresses applied in SYS-5.1.1 are present.
	t.Logf("SYS-5.1.2 - Step 2: Verifying configuration post-reboot scaling...")

	helpers.ValidateInterfaceConfigState(t, dut, d, 2*time.Minute)

	// Iterate over our generated structure to assert it actually committed on the Config tree.
	gotInterfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().Config())
	gotIntfMap := make(map[string]*oc.Interface)
	for _, intf := range gotInterfaces {
		gotIntfMap[intf.GetName()] = intf
	}

	for _, expectedIntf := range d.Interface {
		name := expectedIntf.GetName()
		gotIntf, ok := gotIntfMap[name]
		if !ok {
			t.Errorf("Interface %s not found in Config", name)
			continue
		}

		if want := expectedIntf.GetDescription(); gotIntf.GetDescription() != want {
			t.Errorf("Interface %s Config description: got %v, want %v", name, gotIntf.GetDescription(), want)
		}

		if expectedIntf.GetType() == oc.IETFInterfaces_InterfaceType_ieee8023adLag {
			subIntf := expectedIntf.GetSubinterface(0)
			gotSub := gotIntf.GetSubinterface(0)
			if subIntf != nil {
				if gotSub == nil {
					t.Errorf("Interface %s subinterface 0 not found in Config", name)
					continue
				}
				if subIntf.GetIpv4() != nil {
					if gotSub.GetIpv4() == nil {
						t.Errorf("Interface %s IPv4 Config not found", name)
						continue
					}
					for expectedIP := range subIntf.GetIpv4().Address {
						if gotAddr := gotSub.GetIpv4().GetAddress(expectedIP); gotAddr == nil || gotAddr.GetIp() != expectedIP {
							t.Errorf("Interface %s IPv4 Config address: got %v, want %v", name, gotAddr.GetIp(), expectedIP)
						}
					}
				}
				if subIntf.GetIpv6() != nil {
					if gotSub.GetIpv6() == nil {
						t.Errorf("Interface %s IPv6 Config not found", name)
						continue
					}
					for expectedIP := range subIntf.GetIpv6().Address {
						if gotAddr := gotSub.GetIpv6().GetAddress(expectedIP); gotAddr == nil || gotAddr.GetIp() != expectedIP {
							t.Errorf("Interface %s IPv6 Config address: got %v, want %v", name, gotAddr.GetIp(), expectedIP)
						}
					}
				}
			}
		}
	}

	// SYS-5.1.2 - Step 3: Issue a gNMI Set request to configure a test description on any one DUT interface.
	testInterface := "port-channel100"
	testDesc := "Test description post-reboot"
	t.Logf("SYS-5.1.2 - Step 3: Applying new description %q to %s to test configuration database unlock", testDesc, testInterface)
	gnmi.Update(t, dut, gnmi.OC().Interface(testInterface).Description().Config(), testDesc)

	// SYS-5.1.2 - Step 4: Verify the gnmi Set request succeeds (implied by gnmi.Update not failing)

	// SYS-5.1.2 - Step 5: Verify the new description is applied correctly using a gNMI Get request via State.
	t.Logf("SYS-5.1.2 - Step 5: Awaiting state propagation of description %q to %s", testDesc, testInterface)
	val, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(testInterface).Description().State(), time.Minute, func(v *ygnmi.Value[string]) bool {
		val, present := v.Val()
		return present && val == testDesc
	}).Await(t)

	if !ok {
		t.Errorf("Failed to update description post-reboot on %s: state did not update to %v", testInterface, testDesc)
	} else {
		gotDesc, _ := val.Val()
		t.Logf("Successfully verified new description %q on %s via State", gotDesc, testInterface)
	}
}
