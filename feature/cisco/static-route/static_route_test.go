package static_route_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

type testCase struct {
	name     string
	test     func(t *testing.T)
	validate func(t *testing.T)
}

type testCaseScale struct {
	name     string
	test1    func(t *testing.T)
	test2    func(t *testing.T)
	test3    func(t *testing.T)
	test4    func(t *testing.T)
	validate func(t *testing.T)
}

var cliHandle binding.CLIClient

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func TestIPv4StaticRouteRecurse(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ipAf := "ipv4"

	configureDUT(t, dut1)
	configureDUT(t, dut2)
	time.Sleep(120 * time.Second)
	cliHandle = dut2.RawAPIs().CLI(t)

	testCases := []testCase{
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHop(t, dut2, false, true,
					"100.100.100.1/32", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.1/32", true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHop(t, dut2, false, true,
					"100.100.100.2/32", "25.25.25.25")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.2/32", true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHop(t, dut2, false, true,
					"100.100.100.3/32", "35.35.35.35")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.3/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHop(t, dut2, false, true, dut2.Port(t, "port1").Name(),
					"100.100.100.4/32", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.4/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHop(t, dut2, false, true, dut2.Port(t, "port11").Name(),
					"100.100.100.5/32", "25.25.25.25")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.5/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-unreachable",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHop(t, dut2, false, true, "HundredGigE0/0/0/3",
					"100.100.100.6/32", "35.35.35.35")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.6/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHop(t, dut2, false, false,
					"100.100.100.7/32", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.7/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHop(t, dut2, false, false,
					"100.100.100.8/32", "25.25.25.25")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.8/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHop(t, dut2, false, false,
					"100.100.100.9/32", "35.35.35.35")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.9/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHop(t, dut2, false, false, dut2.Port(t, "port2").Name(),
					"100.100.100.10/32", "191.0.1.0")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.10/32", true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHop(t, dut2, false, false, dut2.Port(t, "port11").Name(),
					"100.100.100.11/32", "25.25.25.25")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.11/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHop(t, dut2, false, false, "HundredGigE0/0/0/3",
					"100.100.100.12/32", "35.35.35.35")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.12/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.13/32", "15.15.15.15", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.13/32", "15.15.15.15", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.13/32", "15.15.15.15", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.13/32", "15.15.15.15", 100, 100, 100, true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.13/32", "15.15.15.15", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.13/32", "15.15.15.15", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Static-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.14/32", "25.25.25.25", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.14/32", "25.25.25.25", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Static-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.14/32", "25.25.25.25", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.14/32", "25.25.25.25", 100, 100, 100, true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Static-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.14/32", "25.25.25.25", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.14/32", "25.25.25.25", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.15/32", "35.35.35.35", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.15/32", "35.35.35.35", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.15/32", "35.35.35.35", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.15/32", "35.35.35.35", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.15/32", "35.35.35.35", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.15/32", "35.35.35.35", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, true, dut2.Port(t, "port3").Name(),
					"100.100.100.16/32", "15.15.15.15", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.16/32", "15.15.15.15", 10, 10, 10, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, true, dut2.Port(t, "port11").Name(),
					"100.100.100.17/32", "25.25.25.25", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.17/32", "15.15.15.15", 10, 10, 10, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-unreachable-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, true, "HundredGigE0/0/0/3",
					"100.100.100.18/32", "35.35.35.35", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.18/32", "15.15.15.15", 10, 10, 10, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.19/32", "192.0.1.0", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.19/32", "192.0.1.0", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.19/32", "192.0.1.0", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.19/32", "192.0.1.0", 100, 100, 100, true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.19/32", "192.0.1.0", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.19/32", "192.0.1.0", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Static-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.20/32", "25.25.25.25", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.20/32", "25.25.25.25", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Static-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.20/32", "25.25.25.25", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.20/32", "25.25.25.25", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Static-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.20/32", "25.25.25.25", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.20/32", "25.25.25.25", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.21/32", "35.35.35.35", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.21/32", "35.35.35.35", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.21/32", "35.35.35.35", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.21/32", "35.35.35.35", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.21/32", "35.35.35.35", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.21/32", "35.35.35.35", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "Bundle-Ether100",
					"100.100.100.22/32", "195.0.1.0", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.22/32", "195.0.1.0", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "Bundle-Ether100",
					"100.100.100.22/32", "195.0.1.0", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.22/32", "195.0.1.0", 100, 100, 100, true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "Bundle-Ether100",
					"100.100.100.22/32", "195.0.1.0", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.22/32", "195.0.1.0", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port11").Name(),
					"100.100.100.23/32", "25.25.25.25", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.23/32", "25.25.25.25", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port11").Name(),
					"100.100.100.23/32", "25.25.25.25", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.23/32", "25.25.25.25", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port11").Name(),
					"100.100.100.23/32", "25.25.25.25", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.23/32", "25.25.25.25", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "HundredGigE0/0/0/3",
					"100.100.100.24/32", "35.35.35.35", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.24/32", "35.35.35.35", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "HundredGigE0/0/0/3",
					"100.100.100.24/32", "35.35.35.35", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.24/32", "35.35.35.35", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "HundredGigE0/0/0/2",
					"100.100.100.24/32", "35.35.35.35", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.24/32", "35.35.35.35", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-Invalid",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopInvalid(t, dut2, false, true,
					"100.100.100.25/32", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.25/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Invalid",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopInvalid(t, dut2, false, true, dut2.Port(t, "port1").Name(),
					"100.100.100.26/32", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.26/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-Invalid",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopInvalid(t, dut2, false, false,
					"100.100.100.27/32", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.27/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Invalid",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopInvalid(t, dut2, false, false, dut2.Port(t, "port1").Name(),
					"100.100.100.28/32", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.28/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-With-BFD",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopBFD(t, dut2, true, "100.100.100.29/32", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.29/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-With-BFD",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopBFD(t, dut2, true, dut2.Port(t, "port1").Name(),
					"100.100.100.30/32", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.30/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-With-BFD",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopBFD(t, dut2, false, "100.100.100.31/32", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.31/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-Interface-With-NextHop-With-BFD",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopBFD(t, dut2, false, dut2.Port(t, "port1").Name(),
					"100.100.100.32/32", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurse(t, dut2, ipAf, "100.100.100.32/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-No-Recurse-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv4StaticRouteNoRecurseNextHop(t, dut2, true,
					"100.100.100.33/32", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.33/32", "15.15.15.15", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-No-Recurse-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv4StaticRouteNoRecurseNextHop(t, dut2, true,
					"100.100.100.34/32", "25.25.25.25")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.34/32", "25.25.25.25", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-No-Recurse-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv4StaticRouteNoRecurseNextHop(t, dut2, true,
					"100.100.100.35/32", "35.35.35.35")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.35/32", "35.5.35.35", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-No-Recurse-With-Interface-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv4StaticRouteNoRecurseInterfaceNextHop(t, dut2, true, dut2.Port(t, "port1").Name(),
					"100.100.100.36/32", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.36/32", "15.15.15.15", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-No-Recurse-With-Interface-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv4StaticRouteNoRecurseInterfaceNextHop(t, dut2, true, dut2.Port(t, "port11").Name(),
					"100.100.100.37/32", "25.25.25.25")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.37/32", "25.25.25.25", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-No-Recurse-With-Interface-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv4StaticRouteNoRecurseInterfaceNextHop(t, dut2, true, "HundredGigE0/0/0/3",
					"100.100.100.38/32", "35.35.35.35")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.38/32", "35.35.35.35", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-No-Recurse-With-NextHop-Connected-With-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteNoRecurseInterfaceNextHopAttributes(t, dut2,
					"100.100.100.39/32", "15.15.15.15", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteNoRecurseAttributes(t, dut2, ipAf, "100.100.100.39/32", "15.15.15.15",
					10, 10, 10, true, false)
			},
		},
		{
			name: "IPv4-Static-Route-No-Recurse-With-NextHop-Connected-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteNoRecurseInterfaceNextHopAttributes(t, dut2,
					"100.100.100.39/32", "15.15.15.15", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteNoRecurseAttributes(t, dut2, ipAf, "100.100.100.39/32", "15.15.15.15",
					100, 100, 100, true, false)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			tc.test(t)
			tc.validate(t)
		})
	}
}

func TestIPv4StaticProcessRestart(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv4"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes configured: %v", bCount)

	util.ProcessRestart(t, dut, "ipv4_static")

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes present after ipv4_static process restart:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv4StaticRouteRecurse(t)
}

func TestIPv4RIBMgrProcessRestart(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv4"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv4", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes configured: %v", bCount)

	util.ProcessRestart(t, dut, "rib_mgr")

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv4", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes present after rib_mgr process restart:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv4StaticRouteRecurse(t)
}
func TestIPv4EmsdProcessRestart(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv4"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv4", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes configured: %v", bCount)

	util.ProcessRestart(t, dut, "emsd")

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv4", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes present after emsd process restart:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv4StaticRouteRecurse(t)
}

func TestIPv4ReloadDUT(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv4"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes configured: %v", bCount)

	util.ReloadRouter(t, dut)
	time.Sleep(180 * time.Second)

	cliHandle = dut.RawAPIs().CLI(t)
	cliOutput, _ = showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes present after Router reload:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv4StaticRouteRecurse(t)
}

func TestIPv4RPFO(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv4"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes configured: %v", bCount)

	util.RPFO(t, dut)
	time.Sleep(120 * time.Second)

	cliHandle = dut.RawAPIs().CLI(t)
	cliOutput, _ = showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes present after RPFO:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv4StaticRouteRecurse(t)
}

func TestIPv4FlapInterfaces(t *testing.T) {

	interfaceList := []string{}
	ipAf := "ipv4"
	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	interfaceList = append(interfaceList, allInterfaceList[:6]...)
	interfaceList = append(interfaceList, "Bundle-Ether100")
	interfaceList = append(interfaceList, "Bundle-Ether101")

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes configured: %v", bCount)

	util.FlapBulkInterfaces(t, dut, interfaceList)
	time.Sleep(120 * time.Second)

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes present after Flap interfaces:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv4StaticRouteRecurse(t)
}

func TestIPv4DelMemberPort(t *testing.T) {

	bundleInterfaceList := []string{}
	downMemberInterfaceList := []string{}

	ipAf := "ipv4"
	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	bundleInterfaceList = append(bundleInterfaceList, allInterfaceList[6:10]...)
	downMemberInterfaceList = append(downMemberInterfaceList, bundleInterfaceList[0])
	downMemberInterfaceList = append(downMemberInterfaceList, bundleInterfaceList[2])

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes configured:%v", bCount)

	DelAddMemberPort(t, dut, downMemberInterfaceList)

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes present after Member port delete:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv4StaticRouteRecurse(t)
}

func TestIPv4AddMemberPort(t *testing.T) {

	bundleInterfaceList := []string{}
	upMemberInterfaceList := []string{}
	upInterfaceList := []string{}
	bundleNames := []string{"Bundle-Ether100", "Bundle-Ether101"}

	ipAf := "ipv4"
	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	bundleInterfaceList = append(bundleInterfaceList, allInterfaceList[6:10]...)
	upMemberInterfaceList = append(upMemberInterfaceList, bundleInterfaceList[0])
	upMemberInterfaceList = append(upMemberInterfaceList, bundleInterfaceList[2])

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes configured:%v", bCount)

	DelAddMemberPort(t, dut, upMemberInterfaceList, bundleNames)

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv4 Static routes present after Member port add:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv4StaticRouteRecurse(t)

	allInterfaceList = getInterfaceNameList(t, dut)
	upInterfaceList = append(upInterfaceList, allInterfaceList[:6]...)
	util.SetInterfaceStateScale(t, dut, upInterfaceList, true)

}

func TestIPv4NonDefaultVRF(t *testing.T) {

	dut2 := ondatra.DUT(t, "dut2")
	ipAf := "ipv4"

	configVRFInterface(t, dut2)
	configVRF(t, dut2)

	cliOutput, _ := showRouteCLI(t, dut2, cliHandle, ipAf, "", "static")
	prefixes := extractPrefixes(cliOutput.Output(), "ipv4")
	for i := 0; i < len(prefixes); i++ {
		if prefixes[i][:4] == "100." {
			gnmi.Delete(t, dut2, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
				Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance).Static(prefixes[i]).Config())
		}
	}
	vrfIntf := dut2.Port(t, "port12").Name()
	localPrefix := fmt.Sprintf("%s/%d", LOCAL_STATIC_ROUTE_VRF_BASE_IPv4, ipv4LBPrefixLen)
	unrslvPrefix := fmt.Sprintf("%s/%d", UNRSLV_STATIC_ROUTE_VRF_BASE_IPv4, ipv4LBPrefixLen)

	configBulkStaticRoute(t, dut2, localPrefix, vrfIntf, 5, true, nonDefaultVRF)
	configBulkStaticRoute(t, dut2, unrslvPrefix, "HundredGigE0/0/0/5", 5, true, nonDefaultVRF)

	testCases := []testCase{
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-NonDefaultVRF-Static",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopVRF(t, dut2, true,
					"110.110.110.1/32", "45.45.45.45")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseVRF(t, dut2, ipAf, "110.110.110.1/32", true, true)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-NextHop-NonDefaultVRF-Unreachable",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopVRF(t, dut2, true,
					"110.110.110.2/32", "55.55.55.55")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseVRF(t, dut2, ipAf, "110.110.110.2/32", true, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-NonDefaultVRF-Static",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopVRF(t, dut2, true, dut2.Port(t, "port11").Name(),
					"110.110.110.3/32", "45.45.45.45")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseVRF(t, dut2, ipAf, "110.110.110.3/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-True-With-Interface-With-NextHop-NonDefaultVRF-Unreachable",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseInterfaceNextHopVRF(t, dut2, true, "HundredGigE0/0/0/5",
					"110.110.110.4/32", "55.55.55.55")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseVRF(t, dut2, ipAf, "110.110.110.4/32", false, false)
			},
		},
		{
			name: "IPv4-Static-Route-With-Recurse-False-With-NextHop-NonDefaultVRF-Delete-Static",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopDeleteVRF(t, dut2, false,
					"110.110.110.1/32", "45.45.45.45")
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseVRF(t, dut2, ipAf, "110.110.110.1/32", false, false)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			tc.test(t)
			tc.validate(t)
		})
	}
}

func TestIPv6StaticRouteRecurse(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	configureDUT(t, dut1)
	configureDUT(t, dut2)
	time.Sleep(60 * time.Second)

	testCases := []testCase{
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHop(t, dut2, false, true,
					"100:100:100::1/128", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::1/128", true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHop(t, dut2, false, true,
					"100:100:100::2/128", "25:25:25::25")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::2/128", true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHop(t, dut2, false, true,
					"100:100:100::3/128", "35:35:35::35")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::3/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHop(t, dut2, false, true, dut2.Port(t, "port1").Name(),
					"100:100:100::4/128", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::4/128", false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHop(t, dut2, false, true, dut2.Port(t, "port11").Name(),
					"100:100:100::5/128", "25:25:25::25")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::5/128", false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-unreachable",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHop(t, dut2, false, true, "HundredGigE0/0/0/3",
					"100:100:100::6/128", "35:35:35::35")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::6/128", false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHop(t, dut2, false, false,
					"100:100:100::7/128", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::7/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHop(t, dut2, false, false,
					"100:100:100::8/128", "25:25:25::25")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::8/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHop(t, dut2, false, false,
					"100:100:100::9/128", "35:35:35::35")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::9/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHop(t, dut2, false, false, dut2.Port(t, "port2").Name(),
					"100:100:100::10/128", "191:0:1::0")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::10/128", true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHop(t, dut2, false, false, dut2.Port(t, "port11").Name(),
					"100:100:100::11/128", "25:25:25::25")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::11/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHop(t, dut2, false, false, "HundredGigE0/0/0/3",
					"100:100:100::12/128", "35:35:35::35")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::12/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100:100:100::13/128", "15:15:15::15", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::13/128", "15:15:15::15", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100:100:100::13/128", "15:15:15::15", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::13/128", "15:15:15::15", 100, 100, 100, true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Connected-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100:100:100::13/128", "15:15:15::15", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::13/128", "15:15:15::15", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Static-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100:100:100::14/128", "25:25:25::25", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::14/128", "25:25:25::25", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Static-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100:100:100::14/128", "25:25:25::25", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::14/128", "25:25:25::25", 100, 100, 100, true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Static-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100:100:100::14/128", "25:25:25::25", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::14/128", "25:25:25::25", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100:100:100::15/128", "35:35:35::35", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::15/128", "25:25:25::25", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100:100:100::15/128", "35:35:35::35", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::15/128", "25:25:25::25", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Unreachable-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributes(t, dut2, true,
					"100:100:100::15/128", "35:35:35::35", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::15/128", "35:35:35::35", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, true, dut2.Port(t, "port3").Name(),
					"100:100:100::16/128", "15:15:15::15", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::16/128", "15:15:15::15", 10, 10, 10, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, true, dut2.Port(t, "port11").Name(),
					"100:100:100::17/128", "25:25:25::25", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::17/128", "15:15:15::15", 10, 10, 10, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-unreachable-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, true, "HundredGigE0/0/0/3",
					"100:100:100::18/128", "35:35:35::35", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::18/128", "15:15:15::15", 10, 10, 10, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100:100:100::19/128", "192:0:1::0", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::19/128", "192:0:1::0", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100:100:100::19/128", "192:0:1::0", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::19/128", "192:0:1::0", 100, 100, 100, true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Connected-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100:100:100::19/128", "192:0:1::0", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::19/128", "192:0:1::0", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Static-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100:100:100::20/128", "25:25:25::25", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::20/128", "25:25:25::25", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Static-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100:100:100::20/128", "25:25:25::25", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::20/128", "25:25:25::25", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Static-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100:100:100::20/128", "25:25:25::25", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::20/128", "25:25:25::25", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100:100:100::21/128", "35:35:35::35", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::21/128", "35:35:35::35", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100:100:100::21/128", "35:35:35::35", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::21/128", "35:35:35::35", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Unreachable-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributes(t, dut2, false,
					"100:100:100::21/128", "35:35:35::35", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::21/32", "35:35:35::35", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "Bundle-Ether100",
					"100:100:100::22/128", "195:0:1::0", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::22/128", "195:0:1::0", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "Bundle-Ether100",
					"100:100:100::22/128", "195:0:1::0", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::22/128", "195:0:1::0", 100, 100, 100, true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "Bundle-Ether100",
					"100:100:100::22/128", "195:0:1::0", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::22/32", "195:0:1::0", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port11").Name(),
					"100:100:100::23/128", "25:25:25::25", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::23/128", "25:25:25::25", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port11").Name(),
					"100:100:100::23/128", "25:25:25::25", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::23/128", "25:25:25::25", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port11").Name(),
					"100:100:100::23/128", "25:25:25::25", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::23/128", "25:25:25::25", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "HundredGigE0/0/0/3",
					"100:100:100::24/128", "35:35:35::35", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::24/128", "35:35:35::35", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Update-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "HundredGigE0/0/0/3",
					"100:100:100::24/128", "35:35:35::35", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::24/128", "35:35:35::35", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Delete-Attributes",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "HundredGigE0/0/0/1",
					"100:100:100::24/128", "35:35:35::35", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributes(t, dut2, "100:100:100::24/128", "35:35:35::35", 0, 0, 0, false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-Invalid",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopInvalid(t, dut2, false, true,
					"100:100:100::25/128", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::25/128", false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-Invalid",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopInvalid(t, dut2, false, true, dut2.Port(t, "port1").Name(),
					"100:100:100::26/128", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopInvalid(t, dut2, false, true, dut2.Port(t, "port1").Name(),
					"100:100:100::26/128", "15.15.15.15")
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-Invalid",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopInvalid(t, dut2, false, false,
					"100:100:100::27/128", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::27/128", false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-Invalid",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopInvalid(t, dut2, false, false, dut2.Port(t, "port1").Name(),
					"100:100:100::28/128", "15.15.15.15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::28/128", false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-With-BFD",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopBFD(t, dut2, true, "100:100:100::29/128", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::29/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-With-BFD",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopBFD(t, dut2, true, dut2.Port(t, "port1").Name(),
					"100:100:100::30/128", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::30/128", false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-With-BFD",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopBFD(t, dut2, false, "100:100:100::31/128", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::31/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-Interface-With-NextHop-With-BFD",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopBFD(t, dut2, false, dut2.Port(t, "port1").Name(),
					"100:100:100::32/128", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurse(t, dut2, "100:100:100::32/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-No-Recurse-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv6StaticRouteNoRecurseNextHop(t, dut2, true,
					"100:100:100::33/128", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteNoRecurse(t, dut2, true, "100:100:100::33/128", "15:15:15::15", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-No-Recurse-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv6StaticRouteNoRecurseNextHop(t, dut2, true,
					"100:100:100::34/128", "25:25:25::25")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteNoRecurse(t, dut2, true, "100:100:100::34/128", "25:25:25::25", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-No-Recurse-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv6StaticRouteNoRecurseNextHop(t, dut2, true,
					"100:100:100::35/128", "35:35:35::35")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteNoRecurse(t, dut2, true, "100:100:100::35/128", "35:35:35::35", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-No-Recurse-With-Interface-With-NextHop-Connected",
			test: func(t *testing.T) {
				testIPv6StaticRouteNoRecurseInterfaceNextHop(t, dut2, true, dut2.Port(t, "port1").Name(),
					"100:100:100::36/128", "15:15:15::15")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteNoRecurseInterface(t, dut2, true,
					"100:100:100::36/128", "15:15:15::15", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-No-Recurse-With-Interface-With-NextHop-Static",
			test: func(t *testing.T) {
				testIPv6StaticRouteNoRecurseInterfaceNextHop(t, dut2, true, dut2.Port(t, "port11").Name(),
					"100:100:100::37/128", "25:25:25::25")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteNoRecurseInterface(t, dut2, true, "100:100:100::37/128", "25:25:25::25", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-No-Recurse-With-Interface-With-NextHop-Unreachable",
			test: func(t *testing.T) {
				testIPv6StaticRouteNoRecurseInterfaceNextHop(t, dut2, true, "HundredGigE0/0/0/3",
					"100:100:100::38/128", "35:35:35::35")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteNoRecurseInterface(t, dut2, true, "100:100:100::38/128", "35:35:35::35", true, false)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			tc.test(t)
			tc.validate(t)
		})
	}
}
func TestIPv6StaticProcessRestart(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv6"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes configured: %v", bCount)

	util.ProcessRestart(t, dut, "ipv6_static")

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes present after ipv6_static process restart:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv6StaticRouteRecurse(t)
}
func TestIPv6RIBMgrProcessRestart(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv6"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes configured: %v", bCount)

	util.ProcessRestart(t, dut, "rib_mgr")

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes present after rib_mgr process restart:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv6StaticRouteRecurse(t)
}
func TestIPv6EmsdProcessRestart(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv6"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes configured: %v", bCount)

	util.ProcessRestart(t, dut, "emsd")

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes present after emsd process restart:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv6StaticRouteRecurse(t)
}
func TestIPv6ReloadDUT(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv6"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes configured: %v", bCount)

	util.ReloadRouter(t, dut)
	time.Sleep(180 * time.Second)

	cliHandle = dut.RawAPIs().CLI(t)
	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes present after Router reload:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv6StaticRouteRecurse(t)
}
func TestIPv6RPFO(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")
	ipAf := "ipv6"

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes configured: %v", bCount)

	util.RPFO(t, dut)
	time.Sleep(120 * time.Second)

	cliHandle = dut.RawAPIs().CLI(t)
	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes present after RPFO:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv6StaticRouteRecurse(t)
}
func TestIPv6FlapInterfaces(t *testing.T) {

	interfaceList := []string{}
	ipAf := "ipv6"
	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	interfaceList = append(interfaceList, allInterfaceList[:6]...)
	interfaceList = append(interfaceList, "Bundle-Ether100")
	interfaceList = append(interfaceList, "Bundle-Ether101")

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes configured: %v", bCount)

	util.FlapBulkInterfaces(t, dut, interfaceList)
	time.Sleep(120 * time.Second)

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes present after Flap interfaces:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv6StaticRouteRecurse(t)
}
func TestIPv6DelMemberPort(t *testing.T) {

	downInterfaceList := []string{}
	bundleInterfaceList := []string{}
	downMemberInterfaceList := []string{}

	ipAf := "ipv6"
	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	downInterfaceList = append(downInterfaceList, allInterfaceList[:6]...)
	bundleInterfaceList = append(bundleInterfaceList, allInterfaceList[6:10]...)
	downMemberInterfaceList = append(downMemberInterfaceList, bundleInterfaceList[0])
	downMemberInterfaceList = append(downMemberInterfaceList, bundleInterfaceList[2])

	util.SetInterfaceStateScale(t, dut, downInterfaceList, false)

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes configured:%v", bCount)

	DelAddMemberPort(t, dut, downMemberInterfaceList)

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes present after Member port delete:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv6StaticRouteRecurse(t)
}
func TestIPv6AddMemberPort(t *testing.T) {

	bundleInterfaceList := []string{}
	upMemberInterfaceList := []string{}
	upInterfaceList := []string{}
	bundleNames := []string{"Bundle-Ether100", "Bundle-Ether101"}

	ipAf := "ipv6"
	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	bundleInterfaceList = append(bundleInterfaceList, allInterfaceList[6:10]...)
	upMemberInterfaceList = append(upMemberInterfaceList, bundleInterfaceList[0])
	upMemberInterfaceList = append(upMemberInterfaceList, bundleInterfaceList[2])

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	bCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes configured:%v", bCount)

	DelAddMemberPort(t, dut, upMemberInterfaceList, bundleNames)

	cliOutput, _ = showRouteCLI(t, dut, cliHandle, "ipv6", "", "static")
	aCount := strings.Count(cliOutput.Output(), "S ")
	t.Logf("IPv6 Static routes present after Member port add:%v", aCount)

	if bCount != aCount {
		t.Error("Number of static routes do not match")
	}
	deleteStaticRoute(t, dut, ipAf)
	TestIPv6StaticRouteRecurse(t)

	allInterfaceList = getInterfaceNameList(t, dut)
	upInterfaceList = append(upInterfaceList, allInterfaceList[:6]...)
	util.SetInterfaceStateScale(t, dut, upInterfaceList, true)

}
func TestIPv6NonDefaultVRF(t *testing.T) {

	dut2 := ondatra.DUT(t, "dut2")

	configVRFInterface(t, dut2)
	configVRF(t, dut2)

	cliOutput, _ := showRouteCLI(t, dut2, cliHandle, "ipv6", "", "static")
	prefixes := extractPrefixes(cliOutput.Output(), "ipv6")
	for i := 0; i < len(prefixes); i++ {
		if prefixes[i][:4] == "100:" {
			gnmi.Delete(t, dut2, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
				Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance).Static(prefixes[i]).Config())
		}
	}
	vrfIntf := dut2.Port(t, "port12").Name()
	localPrefix := fmt.Sprintf("%s/%d", LOCAL_STATIC_ROUTE_VRF_BASE_IPv6, ipv6LBPrefixLen)
	unrslvPrefix := fmt.Sprintf("%s/%d", UNRSLV_STATIC_ROUTE_VRF_BASE_IPv6, ipv6LBPrefixLen)

	configBulkStaticRoute(t, dut2, localPrefix, vrfIntf, 5, false, nonDefaultVRF)
	configBulkStaticRoute(t, dut2, unrslvPrefix, "HundredGigE0/0/0/5", 5, false, nonDefaultVRF)

	testCases := []testCase{
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-DefaultVRF-Static",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopVRF(t, dut2, true,
					"110:110:110::1/128", "45:45:45::45")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseVRF(t, dut2, "110:110:110::1/128", true, true)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-NextHop-DefaultVRF-Unreachable",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopVRF(t, dut2, true,
					"110:110:110::2/128", "55:55:55::55")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseVRF(t, dut2, "110:110:110::2/128", true, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-DefaultVRF-Static",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopVRF(t, dut2, true, dut2.Port(t, "port11").Name(),
					"110:110:110::3/128", "45:45:45::45")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseVRF(t, dut2, "110:110:110::3/128", false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-True-With-Interface-With-NextHop-DefaultVRF-Unreachable",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseInterfaceNextHopVRF(t, dut2, true, "HundredGigE0/0/0/5",
					"110:110:110::4/128", "55:55:55::55")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseVRF(t, dut2, "110:110:110::4/128", false, false)
			},
		},
		{
			name: "IPv6-Static-Route-With-Recurse-False-With-NextHop-DefaultVRF-Delete-Static",
			test: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopDeleteVRF(t, dut2, false,
					"110:110:110::1/128", "45:45:45::45")
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseVRF(t, dut2, "110:110:110::1/128", false, false)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			tc.test(t)
			tc.validate(t)
		})
	}
}

func TestIPv4StaticRouteRecurseScale(t *testing.T) {

	redisV4Prefix := fmt.Sprintf("%s/%d", "60.60.60.60", ipv4LBPrefixLen)
	localV4Prefix := fmt.Sprintf("%s/%d", "70.70.70.70", ipv4LBPrefixLen)
	unrslvV4Prefix := fmt.Sprintf("%s/%d", "80.80.80.80", ipv4LBPrefixLen)
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Port := dut1.Port(t, "port11").Name()
	dut2Port := dut2.Port(t, "port11").Name()
	ipv4 := true
	count := 9
	ipAf := "ipv4"

	configBulkStaticRoute(t, dut1, redisV4Prefix, dut1Port, count, ipv4, defaultVRF)
	configBulkStaticRoute(t, dut2, localV4Prefix, dut2Port, count, ipv4, defaultVRF)
	configBulkStaticRoute(t, dut2, unrslvV4Prefix, "FourHundredGigE0/0/0/3", count, ipv4, defaultVRF)

	testCases := []testCaseScale{
		{
			name: "IPv4-Static-Route-Scale-With-Recurse-True-With-NextHop-With-Attributes",
			test1: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.200.1/32", "69.69.69.69", 10, 10, 10)
			},
			test2: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.201.1/32", "79.79.79.79", 10, 10, 10)
			},
			test3: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.202.1/32", "89.89.89.89", 10, 10, 10)
			},
			test4: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.203.1/32", "6.0.1.1", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributesScale(t, dut2, ipAf, 10, 10, 10, false)
			},
		},
		{
			name: "IPv4-Static-Route-Scale-With-Recurse-True-With-NextHop-With-Update-Attributes",
			test1: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.200.1/32", "69.69.69.69", 100, 100, 100)
			},
			test2: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.201.1/32", "79.79.79.79", 100, 100, 100)
			},
			test3: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.202.1/32", "89.89.89.89", 100, 100, 100)
			},
			test4: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.203.1/32", "6.0.1.1", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributesScale(t, dut2, ipAf, 100, 100, 100, false)
			},
		},
		{
			name: "IPv4-Static-Route-Scale-With-Recurse-True-With-NextHop-With-Delete-Attributes",
			test1: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.200.1/32", "69.69.69.69", 0, 0, 0)
			},
			test2: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.201.1/32", "79.79.79.79", 0, 0, 0)
			},
			test3: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.202.1/32", "89.89.89.89", 0, 0, 0)
			},
			test4: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.203.1/32", "6.0.1.1", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributesScale(t, dut2, ipAf, 100, 100, 100, true)
			},
		},
		{
			name: "IPv4-Static-Route-Scale-With-Recurse-True-With-NextHop-With-Attributes",
			test1: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.200.1/32", "69.69.69.69", 10, 10, 10)
			},
			test2: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.201.1/32", "79.79.79.79", 10, 10, 10)
			},
			test3: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.202.1/32", "89.89.89.89", 10, 10, 10)
			},
			test4: func(t *testing.T) {
				testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut2, true, ipAf, "200.200.203.1/32", "6.0.1.1", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv4StaticRouteRecurseAttributesScale(t, dut2, ipAf, 10, 10, 10, false)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			tc.test1(t)
			tc.test2(t)
			tc.test3(t)
			tc.test4(t)
			tc.validate(t)
		})
	}
}

func TestIPv4StaticProcessRestartScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.ProcessRestart(t, dut, "ipv4_static")
	validateIPv4StaticRouteRecurseAttributesScale(t, dut, "ipv4", 10, 10, 10, false)
}

func TestIPv4RIBMgrProcessRestartScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.ProcessRestart(t, dut, "rib_mgr")
	validateIPv4StaticRouteRecurseAttributesScale(t, dut, "ipv4", 10, 10, 10, false)
}

func TestIPv4EmsdProcessRestartScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.ProcessRestart(t, dut, "emsd")
	validateIPv4StaticRouteRecurseAttributesScale(t, dut, "ipv4", 10, 10, 10, false)
}

func TestIPv4ReloadDUTScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.ReloadRouter(t, dut)
	time.Sleep(180 * time.Second)
	cliHandle = dut.RawAPIs().CLI(t)
	validateIPv4StaticRouteRecurseAttributesScale(t, dut, "ipv4", 10, 10, 10, false)
}
func TestIPv4RPFOScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.RPFO(t, dut)
	time.Sleep(180 * time.Second)
	cliHandle = dut.RawAPIs().CLI(t)
	validateIPv4StaticRouteRecurseAttributesScale(t, dut, "ipv4", 10, 10, 10, false)
}
func TestIPv4FlapInterfacesScale(t *testing.T) {

	interfaceList := []string{}
	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	interfaceList = append(interfaceList, allInterfaceList[:6]...)
	interfaceList = append(interfaceList, "Bundle-Ether100")
	interfaceList = append(interfaceList, "Bundle-Ether101")

	util.FlapBulkInterfaces(t, dut, interfaceList)
	time.Sleep(180 * time.Second)
	validateIPv4StaticRouteRecurseAttributesScale(t, dut, "ipv4", 10, 10, 10, false)
}
func TestIPv4DelMemberPortScale(t *testing.T) {

	downInterfaceList := []string{}
	bundleInterfaceList := []string{}
	downMemberInterfaceList := []string{}

	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	downInterfaceList = append(downInterfaceList, allInterfaceList[:6]...)
	bundleInterfaceList = append(bundleInterfaceList, allInterfaceList[6:10]...)
	downMemberInterfaceList = append(downMemberInterfaceList, bundleInterfaceList[0])
	downMemberInterfaceList = append(downMemberInterfaceList, bundleInterfaceList[2])

	util.SetInterfaceStateScale(t, dut, downInterfaceList, false)
	DelAddMemberPort(t, dut, downMemberInterfaceList)
	validateIPv4StaticRouteRecurseAttributesScale(t, dut, "ipv4", 10, 10, 10, false)
}
func TestIPv4AddMemberPortScale(t *testing.T) {

	bundleInterfaceList := []string{}
	upMemberInterfaceList := []string{}
	upInterfaceList := []string{}
	bundleNames := []string{"Bundle-Ether100", "Bundle-Ether101"}

	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	bundleInterfaceList = append(bundleInterfaceList, allInterfaceList[6:10]...)
	upMemberInterfaceList = append(upMemberInterfaceList, bundleInterfaceList[0])
	upMemberInterfaceList = append(upMemberInterfaceList, bundleInterfaceList[2])

	DelAddMemberPort(t, dut, upMemberInterfaceList, bundleNames)
	validateIPv4StaticRouteRecurseAttributesScale(t, dut, "ipv4", 10, 10, 10, false)

	allInterfaceList = getInterfaceNameList(t, dut)
	upInterfaceList = append(upInterfaceList, allInterfaceList[:6]...)
	util.SetInterfaceStateScale(t, dut, upInterfaceList, true)
}

func TestIPv6StaticRouteRecurseScale(t *testing.T) {

	redisV4Prefix := fmt.Sprintf("%s/%d", "60:60:60::60", ipv6LBPrefixLen)
	localV4Prefix := fmt.Sprintf("%s/%d", "70:70:70::70", ipv6LBPrefixLen)
	unrslvV4Prefix := fmt.Sprintf("%s/%d", "80:80:80::80", ipv6LBPrefixLen)
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dut1Port := dut1.Port(t, "port11").Name()
	dut2Port := dut2.Port(t, "port11").Name()
	ipv4 := false
	count := 9

	configBulkStaticRoute(t, dut1, redisV4Prefix, dut1Port, count, ipv4, defaultVRF)
	configBulkStaticRoute(t, dut2, localV4Prefix, dut2Port, count, ipv4, defaultVRF)
	configBulkStaticRoute(t, dut2, unrslvV4Prefix, "HundredGigE0/0/0/3", count, ipv4, defaultVRF)

	testCases := []testCaseScale{
		{
			name: "IPv6-Static-Route-Scale-With-Recurse-True-With-NextHop-With-Attributes",
			test1: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:200::1/128", "69:69:69::69", 10, 10, 10)
			},
			test2: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:201::1/128", "79:79:79::79", 10, 10, 10)
			},
			test3: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:202::1/128", "89:89:89::89", 10, 10, 10)
			},
			test4: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:203::1/128", "6:0:1::1", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributesScale(t, dut2, 10, 10, 10, false)
			},
		},
		{
			name: "IPv6-Static-Route-Scale-With-Recurse-True-With-NextHop-With-Update-Attributes",
			test1: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:200::1/128", "69:69:69::69", 100, 100, 100)
			},
			test2: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:201::1/128", "79:79:79::79", 100, 100, 100)
			},
			test3: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:202::1/128", "89:89:89::89", 100, 100, 100)
			},
			test4: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:203::1/128", "6:0:1::1", 100, 100, 100)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributesScale(t, dut2, 100, 100, 100, false)
			},
		},
		{
			name: "IPv6-Static-Route-Scale-With-Recurse-True-With-NextHop-With-Delete-Attributes",
			test1: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:200::1/128", "69:69:69::69", 0, 0, 0)
			},
			test2: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:201::1/128", "79:79:79::79", 0, 0, 0)
			},
			test3: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:202::1/128", "89:89:89::89", 0, 0, 0)
			},
			test4: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:203::1/128", "6:0:1::1", 0, 0, 0)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributesScale(t, dut2, 100, 100, 100, true)
			},
		},
		{
			name: "IPv6-Static-Route-Scale-With-Recurse-True-With-NextHop-With-Attributes",
			test1: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:200::1/128", "69:69:69::69", 10, 10, 10)
			},
			test2: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:201::1/128", "79:79:79::79", 10, 10, 10)
			},
			test3: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:202::1/128", "89:89:89::89", 10, 10, 10)
			},
			test4: func(t *testing.T) {
				testIPv6StaticRouteRecurseNextHopAttributesScale(t, dut2, true, "200:200:203::1/128", "6:0:1::0", 10, 10, 10)
			},
			validate: func(t *testing.T) {
				validateIPv6StaticRouteRecurseAttributesScale(t, dut2, 10, 10, 10, false)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			tc.test1(t)
			tc.test2(t)
			tc.test3(t)
			tc.test4(t)
			tc.validate(t)
		})
	}
}

func TestIPv6StaticProcessRestartScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.ProcessRestart(t, dut, "ipv6_static")
	validateIPv6StaticRouteRecurseAttributesScale(t, dut, 10, 10, 10, false)
}

func TestIPv6RIBMgrProcessRestartScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.ProcessRestart(t, dut, "rib_mgr")
	validateIPv6StaticRouteRecurseAttributesScale(t, dut, 10, 10, 10, false)
}

func TestIPv6EmsdProcessRestartScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.ProcessRestart(t, dut, "emsd")
	validateIPv6StaticRouteRecurseAttributesScale(t, dut, 10, 10, 10, false)
}

func TestIPv6ReloadDUTScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.ReloadRouter(t, dut)
	time.Sleep(180 * time.Second)
	cliHandle = dut.RawAPIs().CLI(t)
	validateIPv6StaticRouteRecurseAttributesScale(t, dut, 10, 10, 10, false)
}
func TestIPv6RPFOScale(t *testing.T) {

	dut := ondatra.DUT(t, "dut2")

	util.RPFO(t, dut)
	time.Sleep(180 * time.Second)
	cliHandle = dut.RawAPIs().CLI(t)
	validateIPv6StaticRouteRecurseAttributesScale(t, dut, 10, 10, 10, false)
}
func TestIPv6FlapInterfacesScale(t *testing.T) {

	interfaceList := []string{}
	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	interfaceList = append(interfaceList, allInterfaceList[:6]...)
	interfaceList = append(interfaceList, "Bundle-Ether100")
	interfaceList = append(interfaceList, "Bundle-Ether101")

	util.FlapBulkInterfaces(t, dut, interfaceList)
	time.Sleep(180 * time.Second)
	validateIPv6StaticRouteRecurseAttributesScale(t, dut, 10, 10, 10, false)
}
func TestIPv6DelMemberPortScale(t *testing.T) {

	bundleInterfaceList := []string{}
	downMemberInterfaceList := []string{}

	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	bundleInterfaceList = append(bundleInterfaceList, allInterfaceList[6:10]...)
	downMemberInterfaceList = append(downMemberInterfaceList, bundleInterfaceList[0])
	downMemberInterfaceList = append(downMemberInterfaceList, bundleInterfaceList[2])

	DelAddMemberPort(t, dut, downMemberInterfaceList)
	validateIPv6StaticRouteRecurseAttributesScale(t, dut, 10, 10, 10, false)
}
func TestIPv6AddMemberPortScale(t *testing.T) {

	bundleInterfaceList := []string{}
	upMemberInterfaceList := []string{}
	upInterfaceList := []string{}
	bundleNames := []string{"Bundle-Ether100", "Bundle-Ether101"}

	dut := ondatra.DUT(t, "dut2")
	allInterfaceList := getInterfaceNameList(t, dut)

	bundleInterfaceList = append(bundleInterfaceList, allInterfaceList[6:10]...)
	upMemberInterfaceList = append(upMemberInterfaceList, bundleInterfaceList[0])
	upMemberInterfaceList = append(upMemberInterfaceList, bundleInterfaceList[2])

	DelAddMemberPort(t, dut, upMemberInterfaceList, bundleNames)
	validateIPv6StaticRouteRecurseAttributesScale(t, dut, 10, 10, 10, false)

	allInterfaceList = getInterfaceNameList(t, dut)
	upInterfaceList = append(upInterfaceList, allInterfaceList[:6]...)
	util.SetInterfaceStateScale(t, dut, upInterfaceList, true)
}

func testIPv4StaticRouteRecurseNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	v4Prefix, nextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, recurse, "", v4Prefix, nextHop, defaultVRF)
	gnmi.Update(t, dut, path.Config(), static)
}

func testIPv4StaticRouteRecurseInterfaceNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	interfaceName, v4Prefix, v4nextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, recurse, interfaceName, v4Prefix, v4nextHop, defaultVRF)

	if interfaceName != "" && recurse == true {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, path.Config(), static)
		}); errMsg != nil {
			if strings.Contains(*errMsg, "Recurse cannot be set to true with nexthop as interface") {
				t.Log("Test Case failed as expected")
			} else {
				t.Error("Test case failed with unexpected failure")
			}
		} else {
			t.Error("Test case did not receive expected failure")
		}
	} else {
		static, path := configStaticRoute(t, dut, noRecurse, recurse, interfaceName, v4Prefix, v4nextHop, defaultVRF)
		gnmi.Update(t, dut, path.Config(), static)
	}
}

func testIPv4StaticRouteRecurseNextHopAttributes(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	v4Prefix, v4nextHop string, metric, tag, distance uint32) {

	ok := true

	if metric == 0 && tag == 0 && distance == 0 {
		delPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
			Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance).Static(v4Prefix).NextHop("0")
		_, ok = gnmi.Watch(t, gnmiOptsForOnChange(t, dut), delPath.State(), ON_CHANGE_TIMEOUT,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static_NextHop]) bool {
				gnmi.Delete(t, dut, delPath.Config())

				return v.IsPresent()
			}).Await(t)
	} else if metric == 100 && tag == 100 && distance == 100 {
		static, path := configStaticRouteWithAttributes(t, dut, recurse, "", v4Prefix, v4nextHop, metric, tag, distance)
		_, ok = gnmi.Watch(t, gnmiOptsForOnChange(t, dut), path.State(), ON_CHANGE_TIMEOUT,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol]) bool {
				gnmi.Update(t, dut, path.Config(), static)

				return v.IsPresent()
			}).Await(t)
	}
	if !ok {
		t.Errorf("SubscriptionMode_ON_CHANGE failed")
	}
	if metric == 10 && tag == 10 && distance == 10 {
		static, path := configStaticRouteWithAttributes(t, dut, recurse, "", v4Prefix, v4nextHop, metric, tag, distance)
		gnmi.Update(t, dut, path.Config(), static)
	}
}

func testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, v4Prefix, v4nextHop string, metric, tag, distance uint32) {

	ok := true

	if interfaceName != "" && recurse == true {
		static, path := configStaticRouteWithAttributes(t, dut, recurse, interfaceName, v4Prefix, v4nextHop, metric, tag, distance)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, path.Config(), static)
		}); errMsg != nil {
			if strings.Contains(*errMsg, "Recurse cannot be set to true with nexthop as interface") {
				t.Log("Test Case failed as expected")
				ok = true
			} else {
				t.Error("Test case failed with unexpected failure")
			}
		} else {
			t.Error("Test case did not receive expected failure")
		}
	} else if metric == 0 && tag == 0 && distance == 0 && recurse == false {
		delPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
			Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance).Static(v4Prefix).NextHop("0")
		_, ok = gnmi.Watch(t, gnmiOptsForOnChange(t, dut), delPath.State(), ON_CHANGE_TIMEOUT,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static_NextHop]) bool {
				gnmi.Delete(t, dut, delPath.Config())

				return v.IsPresent()
			}).Await(t)
	} else if metric == 100 && tag == 100 && distance == 100 && recurse == false {
		static, path := configStaticRouteWithAttributes(t, dut, recurse, interfaceName, v4Prefix, v4nextHop, metric, tag, distance)
		_, ok = gnmi.Watch(t, gnmiOptsForOnChange(t, dut), path.State(), ON_CHANGE_TIMEOUT,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol]) bool {
				gnmi.Update(t, dut, path.Config(), static)

				return v.IsPresent()
			}).Await(t)
	}
	if !ok {
		t.Errorf("SubscriptionMode_ON_CHANGE failed")
	}
	if metric == 10 && tag == 10 && distance == 10 && recurse == false {
		static, path := configStaticRouteWithAttributes(t, dut, recurse, interfaceName, v4Prefix, v4nextHop, metric, tag, distance)
		gnmi.Update(t, dut, path.Config(), static)
	}
}

func testIPv4StaticRouteRecurseNextHopInvalid(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	v4Prefix, nextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, recurse, "", v4Prefix, nextHop, defaultVRF)
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Update(t, dut, path.Config(), static)
	}); errMsg != nil {
		if strings.Contains(*errMsg, "'ip-static' detected the 'warning' condition 'Invalid Address Family'") ||
			strings.Contains(*errMsg, "Recurse cannot be set to true with nexthop as interface") {
			t.Log("Test Case failed as expected")
		} else {
			t.Error("Test case failed with unexpected failure")
		}
	} else {
		t.Error("Test case did not receive expected failure")
	}
}

func testIPv4StaticRouteRecurseNextHopBFD(t *testing.T, dut *ondatra.DUTDevice, recurse bool, v4Prefix, v4NextHop string) {

	static, path := configStaticRouteBFD(t, dut, recurse, "", v4Prefix, v4NextHop)
	gnmi.Update(t, dut, path.Config(), static)
}

func testIPv4StaticRouteRecurseInterfaceNextHopBFD(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, v4Prefix, v4NextHop string) {

	static, path := configStaticRouteBFD(t, dut, recurse, interfaceName, v4Prefix, v4NextHop)

	if recurse == true {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, path.Config(), static)
		}); errMsg != nil {
			if strings.Contains(*errMsg, "Recurse cannot be set to true with nexthop as interface") {
				t.Log("Test Case failed as expected")
			} else {
				t.Error("Test case failed with unexpected failure")
			}
		} else {
			t.Error("Test case did not receive expected failure")
		}
	} else {
		gnmi.Update(t, dut, path.Config(), static)
	}
}

func testIPv4StaticRouteRecurseInterfaceNextHopInvalid(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	interfaceName, v4Prefix, nextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, recurse, interfaceName, v4Prefix, nextHop, defaultVRF)

	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Update(t, dut, path.Config(), static)
	}); errMsg != nil {
		if strings.Contains(*errMsg, "'ip-static' detected the 'warning' condition 'Invalid Address Family'") ||
			strings.Contains(*errMsg, "Recurse cannot be set to true with nexthop as interface") {
			t.Log("Test Case failed as expected")
		} else {
			t.Error("Test case failed with unexpected failure")
		}
	} else {
		t.Error("Test case did not receive expected failure")
	}
}

func testIPv4StaticRouteNoRecurseNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool,
	v4Prefix, v4NextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, false, "", v4Prefix, v4NextHop, defaultVRF)
	gnmi.Update(t, dut, path.Config(), static)
}

func testIPv4StaticRouteNoRecurseInterfaceNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool,
	interfaceName, v4Prefix, v4NextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, false, interfaceName, v4Prefix, v4NextHop, defaultVRF)
	gnmi.Update(t, dut, path.Config(), static)
}

func testIPv4StaticRouteNoRecurseInterfaceNextHopAttributes(t *testing.T, dut *ondatra.DUTDevice,
	v4Prefix, v4nextHop string, metric, tag, distance uint32) {

	ok := true

	if metric == 100 && tag == 100 && distance == 100 {
		static, path := configStaticRouteNoRecurseWithAttributes(t, dut, v4Prefix, v4nextHop, metric, tag, distance)
		_, ok = gnmi.Watch(t, gnmiOptsForOnChange(t, dut), path.State(), ON_CHANGE_TIMEOUT,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol]) bool {
				gnmi.Update(t, dut, path.Config(), static)

				return v.IsPresent()
			}).Await(t)
	}
	if !ok {
		t.Errorf("SubscriptionMode_ON_CHANGE failed")
	}
	if metric == 10 && tag == 10 && distance == 10 {
		static, path := configStaticRouteNoRecurseWithAttributes(t, dut, v4Prefix, v4nextHop, metric, tag, distance)
		gnmi.Update(t, dut, path.Config(), static)
	}
}

func testIPv4StaticRouteRecurseNextHopVRF(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	v6Prefix, v6NextHop string) {

	static, path := configStaticRoute(t, dut, false, true, "", v6Prefix, v6NextHop, nonDefaultVRF)
	gnmi.Update(t, dut, path.Config(), static)
}

func testIPv4StaticRouteRecurseInterfaceNextHopVRF(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, v4Prefix, v4NextHop string) {

	static, path := configStaticRoute(t, dut, false, true, interfaceName, v4Prefix, v4NextHop, nonDefaultVRF)
	if recurse == true {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, path.Config(), static)
		}); errMsg != nil {
			if strings.Contains(*errMsg, "Recurse cannot be set to true with nexthop as interface") {
				t.Log("Test Case failed as expected")
			} else {
				t.Error("Test case failed with unexpected failure")
			}
		} else {
			t.Error("Test case did not receive expected failure")
		}
	} else {
		gnmi.Update(t, dut, path.Config(), static)
	}
}

func testIPv4StaticRouteRecurseNextHopDeleteVRF(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	v4Prefix, v4NextHop string) {

	path := gnmi.OC().NetworkInstance(nonDefaultVRF).Protocol(ProtocolSTATIC, defaultVRF).Static(v4Prefix)
	gnmi.Delete(t, dut, path.Config())
}

func testIPv4StaticRouteRecurseNextHopAttributesScale(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	ipAf, v4Prefix, v4nextHop string, metric, tag, distance uint32) {

	if metric == 0 && tag == 0 && distance == 0 {
		batchConfig := &gnmi.SetBatch{}
		for i := 0; i < 250; i++ {
			delPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
				Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance).Static(v4Prefix).NextHop("0")
			if ipAf == "ipv4" {
				v4Prefix = getScaleNewIPv4(strings.Split(v4Prefix, "/")[0]) + "/32"
			} else {
				v4Prefix = getScaleNewIPv6(strings.Split(v4Prefix, "/")[0]) + "/128"
			}
			gnmi.BatchDelete(batchConfig, delPath.Config())
		}
		batchConfig.Set(t, dut)
	} else {
		batchConfig := &gnmi.SetBatch{}
		for i := 0; i < 250; i++ {
			static, path := configStaticRouteWithAttributes(t, dut, recurse, "", v4Prefix, v4nextHop, metric, tag, distance)
			if ipAf == "ipv4" {
				v4Prefix = getScaleNewIPv4(strings.Split(v4Prefix, "/")[0]) + "/32"
			} else {
				v4Prefix = getScaleNewIPv6(strings.Split(v4Prefix, "/")[0]) + "/128"
			}
			gnmi.BatchUpdate(batchConfig, path.Config(), static)
		}
		batchConfig.Set(t, dut)
	}
}

func testIPv6StaticRouteRecurseNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	v6Prefix, nextHop string) {

	testIPv4StaticRouteRecurseNextHop(t, dut, noRecurse, recurse, v6Prefix, nextHop)
}

func testIPv6StaticRouteRecurseNextHopAttributes(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	v6Prefix, v6nextHop string, metric, tag, distance uint32) {

	testIPv4StaticRouteRecurseNextHopAttributes(t, dut, recurse, v6Prefix, v6nextHop, metric, tag, distance)
}

func testIPv6StaticRouteRecurseNextHopInvalid(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	v6Prefix, nextHop string) {

	testIPv4StaticRouteRecurseNextHopInvalid(t, dut, noRecurse, recurse, v6Prefix, nextHop)
}

func testIPv6StaticRouteRecurseNextHopBFD(t *testing.T, dut *ondatra.DUTDevice, recurse bool, v6Prefix, v6NextHop string) {

	testIPv4StaticRouteRecurseNextHopBFD(t, dut, recurse, v6Prefix, v6NextHop)
}

func testIPv6StaticRouteRecurseInterfaceNextHopBFD(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, v6Prefix, v6NextHop string) {

	testIPv4StaticRouteRecurseInterfaceNextHopBFD(t, dut, recurse, interfaceName, v6Prefix, v6NextHop)
}

func testIPv6StaticRouteRecurseInterfaceNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	interfaceName, v6Prefix, v6nextHop string) {

	testIPv4StaticRouteRecurseInterfaceNextHop(t, dut, noRecurse, recurse, interfaceName, v6Prefix, v6nextHop)
}
func testIPv6StaticRouteRecurseInterfaceNextHopInvalid(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	interfaceName, v6Prefix, v6nextHop string) {

	testIPv4StaticRouteRecurseInterfaceNextHopInvalid(t, dut, noRecurse, recurse, interfaceName, v6Prefix, v6nextHop)
}

func testIPv6StaticRouteRecurseInterfaceNextHopAttributes(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, v6Prefix, v6nextHop string, metric, tag, distance uint32) {

	testIPv4StaticRouteRecurseInterfaceNextHopAttributes(t, dut, recurse, interfaceName, v6Prefix, v6nextHop, metric, tag, distance)
}

func testIPv6StaticRouteNoRecurseNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool,
	v6Prefix, nextHop string) {

	testIPv4StaticRouteNoRecurseNextHop(t, dut, noRecurse, v6Prefix, nextHop)
}

func testIPv6StaticRouteNoRecurseInterfaceNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool,
	interfaceName, v6Prefix, nextHop string) {

	testIPv4StaticRouteNoRecurseInterfaceNextHop(t, dut, noRecurse, interfaceName, v6Prefix, nextHop)
}

func testIPv6StaticRouteRecurseNextHopVRF(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	v4Prefix, v4NextHop string) {

	testIPv4StaticRouteRecurseNextHopVRF(t, dut, recurse, v4Prefix, v4NextHop)
}

func testIPv6StaticRouteRecurseInterfaceNextHopVRF(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, v6Prefix, v6NextHop string) {

	testIPv4StaticRouteRecurseInterfaceNextHopVRF(t, dut, recurse, interfaceName, v6Prefix, v6NextHop)
}

func testIPv6StaticRouteRecurseNextHopDeleteVRF(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	v4Prefix, v4NextHop string) {

	testIPv4StaticRouteRecurseNextHopDeleteVRF(t, dut, recurse, v4Prefix, v4NextHop)
}

func testIPv6StaticRouteRecurseNextHopAttributesScale(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	v6Prefix, v6nextHop string, metric, tag, distance uint32) {

	testIPv4StaticRouteRecurseNextHopAttributesScale(t, dut, recurse, "ipv6", v6Prefix, v6nextHop, metric, tag, distance)
}

func validateIPv4StaticRouteRecurse(t *testing.T, dut *ondatra.DUTDevice, ipAf, v4Prefix string,
	installConfig, installRIB bool) {

	path := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *ciscoFlags.DefaultNetworkInstance).
		Static(v4Prefix)
	op := gnmi.Lookup(t, dut, path.State())

	if installConfig == true && op.IsPresent() {
		t.Logf("Route available in running-config for prefix %v as expected", v4Prefix)
	} else if installConfig == false && !op.IsPresent() {
		t.Logf("Route not available in running-config for prefix %v as expected", v4Prefix)
	} else {
		t.Errorf("Error in running-config for route with prefix :%v", v4Prefix)
	}

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, v4Prefix)
	if installRIB == true {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Logf("Route installed in RIB for prefix %s as expected", v4Prefix)
		} else {
			t.Errorf("Error for prefix %s, Route not installed in RIB ", v4Prefix)
		}
	} else {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Errorf("Error for prefix %s, Route should not be installed", v4Prefix)
		} else {
			t.Logf("Route for prefix %s not installed in RIB as expected", v4Prefix)
		}
	}
}

func validateIPv4StaticRouteRecurseAttributes(t *testing.T, dut *ondatra.DUTDevice, ipAf, v4Prefix, v4nextHop string,
	metric, tag, distance uint32, installConfig, installRIB bool) {

	path := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *ciscoFlags.DefaultNetworkInstance).
		Static(v4Prefix)
	op := gnmi.Lookup(t, dut, path.State())

	if installConfig == true && op.IsPresent() {
		t.Logf("Route available in running-config for prefix %v as expected", v4Prefix)
		val, _ := op.Val()
		if metric != val.GetNextHop("0").GetMetric() {
			fmt.Printf("want metric:%v got:%v\n", metric, val.GetNextHop("0").GetMetric())
		}
		if distance != val.GetNextHop("0").GetPreference() {
			fmt.Printf("want preference:%v got:%v\n", distance, val.GetNextHop("0").GetPreference())
		}
		var gotTag uint32
		switch v := val.GetSetTag().(type) {
		case oc.UnionUint32:
			gotTag = uint32(v)
		case oc.UnionString:
			tag, _ := strconv.ParseUint(string(v), 10, 32)
			gotTag = uint32(tag)
		default:
			t.Logf("Type not found :%T\n", v)
		}
		if tag != gotTag {
			fmt.Printf("want tag:%v got:%v\n", tag, gotTag)
		}
	} else if installConfig == false && !op.IsPresent() {
		t.Logf("Route not available in running-config for prefix %v as expected", v4Prefix)
	} else {
		t.Errorf("Error in running-config for route with prefix :%v", v4Prefix)
	}
	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, v4Prefix)
	if installRIB == true {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Logf("Route installed in RIB for prefix %s as expected", v4Prefix)
		} else {
			t.Errorf("Error for prefix %s, Route not installed in RIB ", v4Prefix)
		}
	} else {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Errorf("Error for prefix %s, Route should not be installed", v4Prefix)
		} else {
			t.Logf("Route for prefix %s not installed in RIB as expected", v4Prefix)
		}
	}
}

func validateIPv4StaticRouteNoRecurse(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool, ipAf, v4Prefix, v4NextHop string,
	installConfig, installRIB bool) {

	path := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *ciscoFlags.DefaultNetworkInstance).
		Static(v4Prefix)
	op := gnmi.Lookup(t, dut, path.State())
	val, _ := op.Val()

	if noRecurse == true {
		recurse := val.GetNextHop(v4NextHop).GetRecurse()
		if recurse != false {
			t.Errorf("Recurse leaf set wrongly. Want false, Got %v", recurse)
		}
	}

	if installConfig == true && op.IsPresent() {
		t.Logf("Route available in running-config for prefix %v as expected", v4Prefix)
	} else if installConfig == false && !op.IsPresent() {
		t.Logf("Route not available in running-config for prefix %v as expected", v4Prefix)
	} else {
		t.Errorf("Error in running-config for route with prefix :%v", v4Prefix)
	}

	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, v4Prefix)
	if installRIB == true {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Logf("Route installed in RIB for prefix %s as expected", v4Prefix)
		} else {
			t.Errorf("Error for prefix %s, Route not installed in RIB ", v4Prefix)
		}
	} else {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Errorf("Error for prefix %s, Route should not be installed", v4Prefix)
		} else {
			t.Logf("Route for prefix %s not installed in RIB as expected", v4Prefix)
		}
	}
}

func validateIPv4StaticRouteNoRecurseAttributes(t *testing.T, dut *ondatra.DUTDevice,
	ipAf, v4Prefix, v4NextHop string, metric, tag, distance uint32,
	installConfig, installRIB bool) {

}

func validateIPv4StaticRouteRecurseVRF(t *testing.T, dut *ondatra.DUTDevice, ipAf, v4Prefix string,
	installConfig, installRIB bool) {

	path := gnmi.OC().NetworkInstance(nonDefaultVRF).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *ciscoFlags.DefaultNetworkInstance).
		Static(v4Prefix)
	op := gnmi.Lookup(t, dut, path.State())

	if installConfig == true && op.IsPresent() {
		t.Logf("Route available in running-config for prefix %v as expected", v4Prefix)
	} else if installConfig == false && !op.IsPresent() {
		t.Logf("Route not available in running-config for prefix %v as expected", v4Prefix)
	} else {
		t.Errorf("Error in running-config for route with prefix :%v", v4Prefix)
	}

	cliOutput, _ := showRouteVRFCLI(t, dut, cliHandle, nonDefaultVRF, ipAf, v4Prefix)
	if installRIB == true {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Logf("Route installed in RIB for prefix %s as expected", v4Prefix)
		} else {
			t.Errorf("Error for prefix %s, Route not installed in RIB ", v4Prefix)
		}
	} else {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Errorf("Error for prefix %s, Route should not be installed", v4Prefix)
		} else {
			t.Logf("Route for prefix %s not installed in RIB as expected", v4Prefix)
		}
	}
}

func validateIPv4StaticRouteRecurseAttributesScale(t *testing.T, dut *ondatra.DUTDevice,
	ipAf string, metric, tag, distance uint32, delete bool) {

	path := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *ciscoFlags.DefaultNetworkInstance).
		StaticAny()
	got := gnmi.CollectAll(t, gnmiOptsForSample(t, dut, 10*time.Second), path.State(), 5*time.Second).Await(t)
	validCount := 0

	for _, g := range got {
		val, _ := g.Val()
		if val.GetPrefix()[:4] == "200." && ipAf == "ipv4" ||
			val.GetPrefix()[:4] == "200:" && ipAf == "ipv6" {

			if val.GetNextHop("0").GetPreference() == distance &&
				val.GetNextHop("0").GetMetric() == metric {
				validCount++
			}
		}
	}
	if delete != true {
		if validCount != 1000 {
			t.Errorf("Total number of valid static routes in running-config is %v, want 1000", validCount)
		}
	} else {
		if validCount != 0 {
			t.Errorf("Total number of static routes in running-config is %v, want 0", validCount)
		}
	}

	validCount = 0
	cliOutput, _ := showRouteCLI(t, dut, cliHandle, ipAf, "", "static")
	prefixes := extractPrefixes(cliOutput.Output(), ipAf)

	for i := 0; i < len(prefixes); i++ {
		if prefixes[i][:4] == "200:" && ipAf == "ipv6" ||
			prefixes[i][:4] == "200." && ipAf == "ipv4" {
			validCount++
		}
	}
	if delete != true {
		if validCount != 750 {
			t.Errorf("Total number of valid static routes in RIB is %v, want 750", validCount)
		}
	} else {
		if validCount != 0 {
			t.Errorf("Total number of static routes in RIB is %v, want 0", validCount)
		}
	}
}
func validateIPv6StaticRouteRecurse(t *testing.T, dut *ondatra.DUTDevice, v6Prefix string,
	installConfig, installRIB bool) {

	ipAf := "ipv6"
	validateIPv4StaticRouteRecurse(t, dut, ipAf, v6Prefix, installConfig, installRIB)
}

func validateIPv6StaticRouteRecurseAttributes(t *testing.T, dut *ondatra.DUTDevice, v6Prefix, v6NextHop string,
	metric, tag, distance uint32, installConfig, installRIB bool) {

	ipAf := "ipv6"
	validateIPv4StaticRouteRecurseAttributes(t, dut, ipAf, v6Prefix, v6NextHop, metric, tag, distance, installConfig, installRIB)
}

func validateIPv6StaticRouteNoRecurse(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool, v6Prefix, v6NextHop string,
	installConfig, installRIB bool) {

	ipAf := "ipv6"
	validateIPv4StaticRouteNoRecurse(t, dut, noRecurse, ipAf, v6Prefix, v6NextHop, installConfig, installRIB)
}

func validateIPv6StaticRouteNoRecurseInterface(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool, v6Prefix, v6NextHop string,
	installConfig, installRIB bool) {

	ipAf := "ipv6"
	validateIPv4StaticRouteNoRecurse(t, dut, noRecurse, ipAf, v6Prefix, v6NextHop, installConfig, installRIB)
}

func validateIPv6StaticRouteRecurseVRF(t *testing.T, dut *ondatra.DUTDevice, v4Prefix string,
	installConfig, installRIB bool) {

	ipAf := "ipv6"
	validateIPv4StaticRouteRecurseVRF(t, dut, ipAf, v4Prefix, installConfig, installRIB)
}

func validateIPv6StaticRouteRecurseAttributesScale(t *testing.T, dut *ondatra.DUTDevice,
	metric, tag, distance uint32, delete bool) {

	ipAf := "ipv6"
	validateIPv4StaticRouteRecurseAttributesScale(t, dut, ipAf, metric, tag, distance, delete)
}
