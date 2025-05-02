package static_route_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

type testCase struct {
	name     string
	test     func()
	validate func()
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func TestIPv4StaticRouteRecurse(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	// ate := ondatra.ATE(t, "ate")
	ipAf := "ipv4"

	configureDUT(t, dut1)
	configureDUT(t, dut2)
	// topo := configureATE(t, ate)
	// t.Log("ATE CONFIG: ", topo)
	// time.Sleep(30 * time.Minute)
	// configureTrafficFlow(t, ate, topo)

	testCases := []testCase{
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Connected",
			test: func() {
				testIPv4StaicRouteRecurseNextHop(t, dut2, false, true,
					"100.100.100.1/32", "15.15.15.15")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.1/32", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Static",
			test: func() {
				testIPv4StaicRouteRecurseNextHop(t, dut2, false, true,
					"100.100.100.2/32", "25.25.25.25")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.2/32", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Unreachable",
			test: func() {
				testIPv4StaicRouteRecurseNextHop(t, dut2, false, true,
					"100.100.100.3/32", "35.35.35.35")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.3/32", true, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Connected",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHop(t, dut2, false, true, "FourHundredGigE0/0/0/2",
					"100.100.100.4/32", "15.15.15.15")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.4/32", false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Static",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHop(t, dut2, false, true, "FourHundredGigE0/0/0/2",
					"100.100.100.5/32", "25.25.25.25")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.5/32", false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-unreachable",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHop(t, dut2, false, true, "FourHundredGigE0/0/0/2",
					"100.100.100.6/32", "35.35.35.35")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.6/32", false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Connected",
			test: func() {
				testIPv4StaicRouteRecurseNextHop(t, dut2, false, false,
					"100.100.100.7/32", "15.15.15.15")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.7/32", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Static",
			test: func() {
				testIPv4StaicRouteRecurseNextHop(t, dut2, false, false,
					"100.100.100.8/32", "25.25.25.25")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.8/32", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Unreachable",
			test: func() {
				testIPv4StaicRouteRecurseNextHop(t, dut2, false, false,
					"100.100.100.9/32", "35.35.35.35")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.9/32", true, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Connected",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHop(t, dut2, false, false, "FourHundredGigE0/0/0/2",
					"100.100.100.10/32", "15.15.15.15")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.10/32", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Static",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHop(t, dut2, false, false, "FourHundredGigE0/0/0/2",
					"100.100.100.11/32", "25.25.25.25")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.11/32", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHop(t, dut2, false, false, "FourHundredGigE0/0/0/2",
					"100.100.100.12/32", "35.35.35.35")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.12/32", true, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Connected-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.13/32", "15.15.15.15", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.13/32", "15.15.15.15", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Connected-With-Update-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.13/32", "15.15.15.15", 100, 100, 100)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.13/32", "15.15.15.15", 100, 100, 100, true, true)
			},
		},
		// //{
		// 	name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Connected-With-Delete-Attributes",
		// 	test: func() {
		// 		testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
		// 			"100.100.100.13/32", "15.15.15.15", 0, 0, 0)
		// 	},
		// //},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Static-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.14/32", "25.25.25.25", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.14/32", "25.25.25.25", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Static-With-Update-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.14/32", "25.25.25.25", 100, 100, 100)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.14/32", "25.25.25.25", 100, 100, 100, true, true)
			},
		},
		// //{
		// 	name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Static-With-Delete-Attributes",
		// 	test: func() {
		// 		testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
		// 			"100.100.100.14/32", "25.25.25.25", 0, 0, 0)
		// 	},
		// //},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Unreachable-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.15/32", "35.35.35.35", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.15/32", "25.25.25.25", 100, 100, 100, true, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Unreachable-With-Update-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
					"100.100.100.15/32", "35.35.35.35", 100, 100, 100)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.15/32", "25.25.25.25", 100, 100, 100, true, false)
			},
		},
		// // {
		// // 	name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Unreachable-With-Delete-Attributes",
		// // 	test: func() {
		// // 		testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
		// // 			"100.100.100.15/32", "35.35.35.35", 0, 0, 0)
		// // 	},
		// // },
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, true, "FourHundredGigE0/0/0/2",
					"100.100.100.16/32", "15.15.15.15", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.16/32", "15.15.15.15", 100, 100, 100, false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, true, "FourHundredGigE0/0/0/2",
					"100.100.100.17/32", "25.25.25.25", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.17/32", "15.15.15.15", 100, 100, 100, false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-unreachable-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, true, "FourHundredGigE0/0/0/2",
					"100.100.100.18/32", "35.35.35.35", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.18/32", "15.15.15.15", 100, 100, 100, false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Connected-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.19/32", "15.15.15.15", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.19/32", "15.15.15.15", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Connected-With-Update-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.19/32", "15.15.15.15", 100, 100, 100)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.19/32", "15.15.15.15", 100, 100, 100, true, true)
			},
		},
		// //{
		// 	name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Connected-With-Delete-Attributes",
		// 	test: func() {
		// 		testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, false,
		// 			"100.100.100.19/32", "15.15.15.15", 0, 0, 0)
		// 	},
		// //},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Static-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.20/32", "25.25.25.25", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.20/32", "25.25.25.25", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Static-With-Update-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.20/32", "25.25.25.25", 100, 100, 100)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.20/32", "25.25.25.25", 100, 100, 100, true, true)
			},
		},
		// //{
		// 	name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Static-With-Delete-Attributes",
		// 	test: func() {
		// 		testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, false,
		// 			"100.100.100.20/32", "25.25.25.25", 0, 0, 0)
		// 	},
		// // },
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Unreachable-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.21/32", "35.35.35.35", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.21/32", "35.35.35.35", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Unreachable-With-Update-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, false,
					"100.100.100.21/32", "35.35.35.35", 100, 100, 100)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.21/32", "35.35.35.35", 100, 100, 100, true, false)
			},
		},
		// //{
		// 	name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Unreachable-With-Delete-Attributes",
		// 	test: func() {
		// 		testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, false,
		// 			"100.100.100.9/32", "35.35.35.35", 0, 0, 0)
		// 	},
		// //},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
					"100.100.100.22/32", "15.15.15.15", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.22/32", "15.15.15.15", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Update-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
					"100.100.100.22/32", "15.15.15.15", 100, 100, 100)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.22/32", "15.15.15.15", 100, 100, 100, true, true)
			},
		},
		// //{
		// 	name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Delete-Attributes",
		// 	test: func() {
		// 		testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
		// 			"100.100.100.22/32", "15.15.15.15", 0, 0, 0)
		// 	},
		// //},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
					"100.100.100.23/32", "25.25.25.25", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.23/32", "25.25.25.25", 10, 10, 10, true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Update-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
					"100.100.100.23/32", "25.25.25.25", 100, 100, 100)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.23/32", "25.25.25.25", 10, 10, 10, true, true)
			},
		},
		// //{
		// 	name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Delete-Attributes",
		// 	test: func() {
		// 		testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
		// 			"100.100.100.23/32", "25.25.25.25", 0, 0, 0)
		// 	},
		// //},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
					"100.100.100.24/32", "35.35.35.35", 10, 10, 10)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.24/32", "35.35.35.35", 10, 10, 10, true, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Update-Attributes",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
					"100.100.100.24/32", "35.35.35.35", 100, 100, 100)
			},
			validate: func() {
				validateIPv4StaicRouteRecurseAttributes(t, dut2, ipAf, "100.100.100.24/32", "35.35.35.35", 100, 100, 100, true, false)
			},
		},
		// //{
		// 	name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Delete-Attributes",
		// 	test: func() {
		// 		testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
		// 			"100.100.100.24/32", "35.35.35.35", 0, 0, 0)
		// 	},
		// //},
		{

			name: "IPv4-Staic-Route-With-Recurse-True-With-NextHop-Invalid",
			test: func() {
				testIPv4StaicRouteRecurseNextHopInvalid(t, dut2, false, true,
					"100.100.100.25/32", "15:15:15::15")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.25/32", false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Invalid",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopInvalid(t, dut2, false, true, dut2.Port(t, "port1").Name(),
					"100.100.100.26/32", "15.15.15.15")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.4/32", false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-NextHop-Invalid",
			test: func() {
				testIPv4StaicRouteRecurseNextHopInvalid(t, dut2, false, false,
					"100.100.100.27/32", "15.15.15.15")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.25/32", false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Invalid",
			test: func() {
				testIPv4StaicRouteRecurseInterfaceNextHopInvalid(t, dut2, false, false, dut2.Port(t, "port1").Name(),
					"100.100.100.28/32", "15.15.15.15")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.4/32", false, false)
			},
		},
		{
			name: "IPv4-Staic-Route-With-Recurse-False-With-Interface-With-SR-Policy",
			test: func() {
				testIPv4StaicRouteRecursNextHopBFD(t, dut2, false, "100.100.100.28/32")
			},
			validate: func() {
				validateIPv4StaicRouteRecurse(t, dut2, ipAf, "100.100.100.4/32", false, false)
			},
		},

		{
			name: "IPv4-Staic-Route-No-Recurse-With-NextHop-Connected",
			test: func() {
				testIPv4StaicRouteNoRecurseNextHop(t, dut2, true,
					"100.100.100.29/32", "15.15.15.15")
			},
			validate: func() {
				validateIPv4StaicRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.29/32", "15.15.15.15", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-No-Recurse-With-NextHop-Static",
			test: func() {
				testIPv4StaicRouteNoRecurseNextHop(t, dut2, true,
					"100.100.100.30/32", "25.25.25.25")
			},
			validate: func() {
				validateIPv4StaicRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.30/32", "25.25.25.25", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-No-Recurse-With-NextHop-Unreachable",
			test: func() {
				testIPv4StaicRouteNoRecurseNextHop(t, dut2, true,
					"100.100.100.31/32", "35.35.35.35")
			},
			validate: func() {
				validateIPv4StaicRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.31/32", "35.5.35.35", true, false)
			},
		},
		{
			name: "IPv4-Staic-Route-No-Recurse-With-Interface-With-NextHop-Connected",
			test: func() {
				testIPv4StaicRouteNoRecurseNextHopInterface(t, dut2, true, dut2.Port(t, "port1").Name(),
					"100.100.100.32/32", "15.15.15.15")
			},
			validate: func() {
				validateIPv4StaicRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.32/32", "15.15.15.15", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-No-Recurse-With-Interface-With-NextHop-Static",
			test: func() {
				testIPv4StaicRouteNoRecurseNextHopInterface(t, dut2, true, dut2.Port(t, "port11").Name(),
					"100.100.100.33/32", "25.25.25.25")
			},
			validate: func() {
				validateIPv4StaicRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.33/32", "25.25.25.25", true, true)
			},
		},
		{
			name: "IPv4-Staic-Route-No-Recurse-With-Interface-With-NextHop-Unreachable",
			test: func() {
				testIPv4StaicRouteNoRecurseNextHopInterface(t, dut2, true, "FourHundredGigE0/0/0/2",
					"100.100.100.34/32", "35.35.35.35")
			},
			validate: func() {
				validateIPv4StaicRouteNoRecurse(t, dut2, true, ipAf, "100.100.100.34/32", "35.35.35.35", true, false)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			tc.test()
			tc.validate()
		})
	}
}

// func TestIPv6StaticRouteRecurse(t *testing.T) {

// dut1 := ondatra.DUT(t, "dut1")
// dut2 := ondatra.DUT(t, "dut2")
// ate := ondatra.ATE(t, "ate")

// configureDUT(t, dut1)
// configureDUT(t, dut2)
// topo := configureATE(t, ate)
// t.Log("ATE CONFIG: ", topo)
// time.Sleep(30 * time.Minute)
// configureTrafficFlow(t, ate, topo)

// testCases := []testCase{
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Connected",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHop(t, dut2, false, true,
// 			"100:100:100::1/128", "15:15:15::15")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::1/128", true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Static",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHop(t, dut2, false, true,
// 			"100:100:100::2/128", "25:25:25::25")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::2/128", true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Unreachable",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHop(t, dut2, false, true,
// 			"100:100:100::3/128", "35:35:35::35")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::3/128", true, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Connected",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHop(t, dut2, false, true, dut2.Port(t, "port1").Name(),
// 			"100:100:100::4/128", "15:15:15::15")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::4/128", false, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Static",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHop(t, dut2, false, true, dut2.Port(t, "port11").Name(),
// 			"100:100:100::5/128", "25:25:25::25")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::5/128", false, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-unreachable",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHop(t, dut2, false, true, "FourHundredGigE0/0/0/2",
// 			"100:100:100::6/128", "35:35:35::35")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::6/128", false, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Connected",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHop(t, dut2, false, false,
// 			"100:100:100::7/128", "15:15:15::15")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::7/128", true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Static",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHop(t, dut2, false, false,
// 			"100:100:100::8/128", "25:25:25::25")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::8/128", true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Unreachable",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHop(t, dut2, false, false,
// 			"100:100:100::9/128", "35:35:35::35")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::9/128", true, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Connected",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHop(t, dut2, false, false, dut2.Port(t, "port2").Name(),
// 			"100:100:100::10/128", "15:15:15::15")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::10/128", true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Static",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHop(t, dut2, false, false, dut2.Port(t, "port11").Name(),
// 			"100:100:100::11/128", "25:25:25::25")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::11/128", true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHop(t, dut2, false, false, "FourHundredGigE0/0/0/2",
// 			"100:100:100::12/128", "35:35:35::35")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::12/128", true, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Connected-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, true,
// 			"100:100:100::13/128", "15:15:15::15", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::13/128", "15:15:15::15", 10, 10, 10, true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Connected-With-Update-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, true,
// 			"100:100:100::13/128", "15:15:15::15", 100, 100, 100)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::13/128", "15:15:15::15", 100, 100, 100, true, true)
// 	},
// },
// //{
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Connected-With-Delete-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, true,
// 			"100:100:100::13/128", "15:15:15::15", 0, 0, 0)
// 	},
// //},
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Static-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, true,
// 			"100:100:100::14/128", "25:25:25::25", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::14/128", "25:25:25::25", 10, 10, 10, true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Static-With-Update-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, true,
// 			"100:100:100::14/128", "25:25:25::25", 100, 100, 100)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::14/128", "25:25:25::25", 100, 100, 100, true, true)
// 	},
// },
// //{
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Static-With-Delete-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, true,
// 			"100:100:100::14/128", "25:25:25::25", 0, 0, 0)
// 	},
// //},
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Unreachable-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, true,
// 			"100:100:100::15/128", "35:35:35::35", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::15/128", "25:25:25::25", 100, 100, 100, true, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Unreachable-With-Update-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, true,
// 			"100:100:100::15/128", "35:35:35::35", 100, 100, 100)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::15/128", "25:25:25::25", 100, 100, 100, true, false)
// 	},
// },
// // {
// // 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Unreachable-With-Delete-Attributes",
// // 	test: func() {
// // 		testIPv4StaicRouteRecurseNextHopAttributes(t, dut2, true,
// // 			"100:100:100::15/128", "35:35:35::35", 0, 0, 0)
// // 	},
// // },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Connected-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, true, dut2.Port(t, "port3").Name(),
// 			"100:100:100::16/128", "15:15:15::15", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::16/128", "15:15:15::15", 100, 100, 100, false, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Static-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, true, dut2.Port(t, "port11").Name(),
// 			"100:100:100::17/128", "25:25:25::25", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::17/128", "15:15:15::15", 100, 100, 100, false, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-unreachable-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, true, "FourHundredGigE0/0/0/2",
// 			"100:100:100::18/128", "35:35:35::35", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::18/128", "15:15:15::15", 100, 100, 100, false, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Connected-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, false,
// 			"100:100:100::19/128", "15:15:15::15", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::19/128", "15:15:15::15", 10, 10, 10, true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Connected-With-Update-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, false,
// 			"100:100:100::19/128", "15:15:15::15", 100, 100, 100)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::19/128", "15:15:15::15", 100, 100, 100, true, true)
// 	},
// },
// //{
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Connected-With-Delete-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, false,
// 			"100:100:100::19/128", "15:15:15::15", 0, 0, 0)
// 	},
// //},
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Static-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, false,
// 			"100:100:100::20/128", "25:25:25::25", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::20/128", "25:25:25::25", 10, 10, 10, true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Static-With-Update-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, false,
// 			"100:100:100::20/128", "25:25:25::25", 100, 100, 100)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::20/128", "25:25:25::25", 100, 100, 100, true, true)
// 	},
// },
// //{
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Static-With-Delete-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, false,
// 			"100:100:100::20/128", "25:25:25::25", 0, 0, 0)
// 	},
// // },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Unreachable-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, false,
// 			"100:100:100::21/128", "35:35:35::35", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::21/128", "35:35:35::35", 10, 10, 10, true, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Unreachable-With-Update-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, false,
// 			"100:100:100::21/128", "35:35:35::35", 100, 100, 100)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::21/128", "35:35:35::35", 100, 100, 100, true, false)
// 	},
// },
// //{
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Unreachable-With-Delete-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopAttributes(t, dut2, false,
// 			"100:100:100::9/128", "35:35:35::35", 0, 0, 0)
// 	},
// //},
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port4").Name(),
// 			"100:100:100::22/128", "15:15:15::15", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::22/128", "15:15:15::15", 10, 10, 10, true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Update-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port11").Name(),
// 			"100:100:100::22/128", "15:15:15::15", 100, 100, 100)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::22/128", "15:15:15::15", 100, 100, 100, true, true)
// 	},
// },
// //{
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Connected-With-Delete-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
// 			"100:100:100::22/128", "15:15:15::15", 0, 0, 0)
// 	},
// //},
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port11").Name(),
// 			"100:100:100::23/128", "25:25:25::25", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::23/128", "25:25:25::25", 10, 10, 10, true, true)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Update-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, dut2.Port(t, "port11").Name(),
// 			"100:100:100::23/128", "25:25:25::25", 100, 100, 100)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::23/128", "25:25:25::25", 10, 10, 10, true, true)
// 	},
// },
// //{
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Static-With-Delete-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
// 			"100:100:100::23/128", "25:25:25::25", 0, 0, 0)
// 	},
// //},
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/2",
// 			"100:100:100::24/128", "35:35:35::35", 10, 10, 10)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::24/128", "35:35:35::35", 10, 10, 10, true, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Update-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/3",
// 			"100:100:100::24/128", "35:35:35::35", 100, 100, 100)
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurseAttributes(t, dut2, "100:100:100::24/128", "35:35:35::35", 100, 100, 100, true, false)
// 	},
// },
// //{
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Unreachable-With-Delete-Attributes",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t, dut2, false, "FourHundredGigE0/0/0/1",
// 			"100:100:100::24/128", "35:35:35::35", 0, 0, 0)
// 	},
// //},
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-NextHop-Invalid",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopInvalid(t, dut2, false, true,
// 			"100:100:100::25/128", "15.15.15.15")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::25/128", false, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-True-With-Interface-With-NextHop-Invalid",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopInvalid(t, dut2, false, true, dut2.Port(t, "port1").Name(),
// 			"100:100:100::26/128", "15.15.15.15")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::4/128", false, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-NextHop-Invalid",
// 	test: func() {
// 		testIPv6StaicRouteRecurseNextHopInvalid(t, dut2, false, false,
// 			"100:100:100::27/128", "15.15.15.15")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::25/128", false, false)
// 	},
// },
// {
// 	name: "IPv6-Staic-Route-With-Recurse-False-With-Interface-With-NextHop-Invalid",
// 	test: func() {
// 		testIPv6StaicRouteRecurseInterfaceNextHopInvalid(t, dut2, false, false, dut2.Port(t, "port1").Name(),
// 			"100:100:100::28/128", "15.15.15.15")
// 	},
// 	validate: func() {
// 		validateIPv6StaicRouteRecurse(t, dut2, "100:100:100::4/128", false, false)
// 	},
// },
// 	{
// 		name: "IPv6-Staic-Route-No-Recurse-With-NextHop-Connected",
// 		test: func() {
// 			testIPv6StaicRouteNoRecurseNextHop(t, dut2, true,
// 				"100:100:100::29/128", "15:15:15::15")
// 		},
// 		validate: func() {
// 			validateIPv6StaicRouteNoRecurse(t, dut2, true, "100:100:100::29/128", "15:15:15::15", true, true)
// 		},
// 	},
// 	{
// 		name: "IPv6-Staic-Route-No-Recurse-With-NextHop-Static",
// 		test: func() {
// 			testIPv6StaicRouteNoRecurseNextHop(t, dut2, true,
// 				"100:100:100::30/128", "25:25:25::25")
// 		},
// 		validate: func() {
// 			validateIPv6StaicRouteNoRecurse(t, dut2, true, "100:100:100::30/128", "25:25:25::25", true, true)
// 		},
// 	},
// 	{
// 		name: "IPv6-Staic-Route-No-Recurse-With-NextHop-Unreachable",
// 		test: func() {
// 			testIPv6StaicRouteNoRecurseNextHop(t, dut2, true,
// 				"100:100:100::31/128", "35:35:35::35")
// 		},
// 		validate: func() {
// 			validateIPv6StaicRouteNoRecurse(t, dut2, true, "100:100:100::31/128", "35:35:35::35", true, false)
// 		},
// 	},
// 	{
// 		name: "IPv6-Staic-Route-No-Recurse-With-Interface-With-NextHop-Connected",
// 		test: func() {
// 			testIPv6StaicRouteNoRecurseNextHopInterface(t, dut2, true, dut2.Port(t, "port1").Name(),
// 				"100:100:100::32/128", "15:15:15::15")
// 		},
// 		validate: func() {
// 			validateIPv6StaicRouteNoRecurse(t, dut2, true,
// 				"100:100:100::32/128", "15:15:15::15", true, true)
// 		},
// 	},
// 	{
// 		name: "IPv6-Staic-Route-No-Recurse-With-Interface-With-NextHop-Static",
// 		test: func() {
// 			testIPv6StaicRouteNoRecurseNextHopInterface(t, dut2, true, dut2.Port(t, "port11").Name(),
// 				"100:100:100::33/128", "25:25:25::25")
// 		},
// 		validate: func() {
// 			validateIPv6StaicRouteNoRecurse(t, dut2, true, "100:100:100::33/128", "25:25:25::25", true, true)
// 		},
// 	},
// 	{
// 		name: "IPv6-Staic-Route-No-Recurse-With-Interface-With-NextHop-Unreachable",
// 		test: func() {
// 			testIPv6StaicRouteNoRecurseNextHopInterface(t, dut2, true, "FourHundredGigE0/0/0/2",
// 				"100:100:100::34/128", "35:35:35::35")
// 		},
// 		validate: func() {
// 			validateIPv6StaicRouteNoRecurse(t, dut2, true, "100:100:100::34/128", "35:35:35::35", true, false)
// 		},
// 	},
// }
// for _, tc := range testCases {
// 	t.Run(tc.name, func(t *testing.T) {
// 		t.Logf("Name: %s", tc.name)
// 		tc.test()
// 		tc.validate()
// 	})
// 	}
// }

func testIPv4StaicRouteRecurseNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	v4Prefix, nextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, recurse, "", v4Prefix, nextHop)
	gnmi.Update(t, dut, path.Config(), static)
}

func testIPv4StaicRouteRecurseNextHopAttributes(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	v4Prefix, v4nextHop string, metric, tag, distance uint32) {

	var ok bool

	if metric == 0 && tag == 0 && distance == 0 {
		delPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
			Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance).Static(v4Prefix).NextHop(v4nextHop)
		_, ok = gnmi.Watch(t, gnmiOptsForOnChange(t, dut), delPath.State(), 10*time.Second,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static_NextHop]) bool {
				gnmi.Delete(t, dut, delPath.Config())

				return v.IsPresent()
			}).Await(t)
	} else {
		static, path := configStaticRouteWithAttributes(t, dut, recurse, "", v4Prefix, v4nextHop, metric, tag, distance)
		_, ok = gnmi.Watch(t, gnmiOptsForOnChange(t, dut), path.State(), 10*time.Second,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol]) bool {
				gnmi.Update(t, dut, path.Config(), static)

				return v.IsPresent()
			}).Await(t)
	}
	if !ok {
		t.Errorf("SubscriptionMode_ON_CHANGE failed")
	}
}

func testIPv4StaicRouteRecurseNextHopInvalid(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	v4Prefix, nextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, recurse, "", v4Prefix, nextHop)
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Update(t, dut, path.Config(), static)
	}); errMsg != nil {
		fmt.Printf("Debug: errmsg:%v\n", errMsg)
		if strings.Contains(*errMsg, "'ip-static' detected the 'warning' condition 'Invalid Address Family'") {
			t.Log("Test Case failed as expected")
		} else {
			t.Error("Test case failed with unexpected failure")
		}
	} else {
		t.Error("Test case did not receive expected failure")
	}
}

func testIPv4StaicRouteRecursNextHopBFD(t *testing.T, dut *ondatra.DUTDevice, recurse bool, v4Prefix string) {

	static, path := configStaticRouteBFD(t, dut, recurse, v4Prefix)

	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Update(t, dut, path.Config(), static)
	}); errMsg != nil {
		fmt.Printf("Debug: errmsg:%v\n", errMsg)
		if strings.Contains(*errMsg, "'ip-static' detected the 'warning' condition 'Invalid Address Family'") {
			t.Log("Test Case failed as expected")
		} else {
			t.Error("Test case failed with unexpected failure")
		}
	} else {
		t.Error("Test case did not receive expected failure")
	}
}

func testIPv4StaicRouteRecurseInterfaceNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	interfaceName, v4Prefix, v4nextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, recurse, interfaceName, v4Prefix, v4nextHop)

	if interfaceName != "" && recurse == true {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, path.Config(), static)
		}); errMsg != nil {
			fmt.Printf("Debug: errmsg:%v\n", errMsg)
			if strings.Contains(*errMsg, "Recurse cannot be set to true with nexthop as interface") {
				t.Log("Test Case failed as expected")
			} else {
				t.Error("Test case failed with unexpected failure")
			}
		} else {
			t.Error("Test case did not receive expected failure")
		}
	} else {
		static, path := configStaticRoute(t, dut, noRecurse, recurse, interfaceName, v4Prefix, v4nextHop)
		gnmi.Update(t, dut, path.Config(), static)
	}
}

func testIPv4StaicRouteRecurseInterfaceNextHopInvalid(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	interfaceName, v4Prefix, nextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, recurse, interfaceName, v4Prefix, nextHop)
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Update(t, dut, path.Config(), static)
	}); errMsg != nil {
		fmt.Printf("Debug: errmsg:%v\n", errMsg)
		if strings.Contains(*errMsg, "'ip-static' detected the 'warning' condition 'Invalid Address Family'") {
			t.Log("Test Case failed as expected")
		} else {
			t.Error("Test case failed with unexpected failure")
		}
	} else {
		t.Error("Test case did not receive expected failure")
	}
}

func testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, v4Prefix, v4nextHop string, metric, tag, distance uint32) {

	var ok bool

	if interfaceName != "" && recurse == true {
		static, path := configStaticRouteWithAttributes(t, dut, recurse, interfaceName, v4Prefix, v4nextHop, metric, tag, distance)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, path.Config(), static)
		}); errMsg != nil {
			fmt.Printf("Debug: errmsg:%v\n", errMsg)
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
			Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance).Static(v4Prefix).NextHop(v4nextHop)
		_, ok = gnmi.Watch(t, gnmiOptsForOnChange(t, dut), delPath.State(), 10*time.Second,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Static_NextHop]) bool {
				gnmi.Delete(t, dut, delPath.Config())

				return v.IsPresent()
			}).Await(t)
	} else {
		static, path := configStaticRouteWithAttributes(t, dut, recurse, interfaceName, v4Prefix, v4nextHop, metric, tag, distance)
		_, ok = gnmi.Watch(t, gnmiOptsForOnChange(t, dut), path.State(), 10*time.Second,
			func(v *ygnmi.Value[*oc.NetworkInstance_Protocol]) bool {
				gnmi.Update(t, dut, path.Config(), static)

				return v.IsPresent()
			}).Await(t)
	}
	if !ok {
		t.Errorf("SubscriptionMode_ON_CHANGE failed")
	}
}

func testIPv4StaicRouteNoRecurseNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool,
	v4Prefix, v4NextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, false, "", v4Prefix, v4NextHop)
	gnmi.Update(t, dut, path.Config(), static)

}

func testIPv4StaicRouteNoRecurseNextHopInterface(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool,
	interfaceName, v4Prefix, v4NextHop string) {

	static, path := configStaticRoute(t, dut, noRecurse, false, interfaceName, v4Prefix, v4NextHop)
	gnmi.Update(t, dut, path.Config(), static)
}

func testIPv6StaicRouteRecurseNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	v6Prefix, nextHop string) {

	testIPv4StaicRouteRecurseNextHop(t, dut, noRecurse, recurse, v6Prefix, nextHop)
}

func testIPv6StaicRouteRecurseNextHopAttributes(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	v6Prefix, v6nextHop string, metric, tag, distance uint32) {

	testIPv4StaicRouteRecurseNextHopAttributes(t, dut, recurse, v6Prefix, v6nextHop, metric, tag, distance)
}

func testIPv6StaicRouteRecurseNextHopInvalid(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	v6Prefix, nextHop string) {

	testIPv4StaicRouteRecurseNextHopInvalid(t, dut, noRecurse, recurse, v6Prefix, nextHop)
}

func testIPv6StaicRouteRecurseInterfaceNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	interfaceName, v6Prefix, v6nextHop string) {

	testIPv4StaicRouteRecurseInterfaceNextHop(t, dut, noRecurse, recurse, interfaceName, v6Prefix, v6nextHop)
}
func testIPv6StaicRouteRecurseInterfaceNextHopInvalid(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	interfaceName, v6Prefix, v6nextHop string) {

	testIPv4StaicRouteRecurseInterfaceNextHopInvalid(t, dut, noRecurse, recurse, interfaceName, v6Prefix, v6nextHop)
}

func testIPv6StaicRouteRecurseInterfaceNextHopAttributes(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, v6Prefix, v6nextHop string, metric, tag, distance uint32) {

	testIPv4StaicRouteRecurseInterfaceNextHopAttributes(t, dut, recurse, interfaceName, v6Prefix, v6nextHop, metric, tag, distance)
}

func testIPv6StaicRouteNoRecurseNextHop(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool,
	v6Prefix, nextHop string) {

	testIPv4StaicRouteNoRecurseNextHop(t, dut, noRecurse, v6Prefix, nextHop)
}

func testIPv6StaicRouteNoRecurseNextHopInterface(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool,
	interfaceName, v6Prefix, nextHop string) {

	testIPv4StaicRouteNoRecurseNextHopInterface(t, dut, noRecurse, interfaceName, v6Prefix, nextHop)
}

func validateIPv4StaicRouteRecurse(t *testing.T, dut *ondatra.DUTDevice, ipAf, v4Prefix string,
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

	cli := fmt.Sprintf("show route %s unicast %s\n", ipAf, v4Prefix)
	cliHandle := dut.RawAPIs().CLI(t)
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)

	cliOutput, _ := cliHandle.RunCommand(ctx, cli)
	if installRIB == true {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Logf("Route installed in RIB for refix %s as expected", v4Prefix)
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

func validateIPv4StaicRouteRecurseAttributes(t *testing.T, dut *ondatra.DUTDevice, ipAf, v4Prefix, v4nextHop string,
	metric, tag, distance uint32, installConfig, installRIB bool) {

	path := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *ciscoFlags.DefaultNetworkInstance).
		Static(v4Prefix)
	op := gnmi.Lookup(t, dut, path.State())

	if installConfig == true && op.IsPresent() {
		t.Logf("Route available in running-config for prefix %v as expected", v4Prefix)
		val, _ := op.Val()
		if metric != val.GetNextHop(v4nextHop).GetMetric() ||
			val.GetNextHop(v4nextHop).GetPreference() != distance ||
			oc.UnionString(strconv.Itoa(int(tag))) != val.GetSetTag() {

			t.Errorf("Error for Static Route attributes for prefix %v with nextHop %v", v4Prefix, v4nextHop)
			t.Errorf("want metric:%v, got:%v", metric, val.GetNextHop(v4nextHop).GetMetric())
			t.Errorf("want tag:%v, got:%v", tag, val.GetSetTag())
			t.Errorf("want distance:%v, got:%v", distance, val.GetNextHop(v4nextHop).GetPreference())
		}
	} else if installConfig == false && !op.IsPresent() {
		t.Logf("Route not available in running-config for prefix %v as expected", v4Prefix)
	} else {
		t.Errorf("Error in running-config for route with prefix :%v", v4Prefix)
	}

	cli := fmt.Sprintf("show route %s unicast %s\n", ipAf, v4Prefix)
	cliHandle := dut.RawAPIs().CLI(t)
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)

	cliOutput, _ := cliHandle.RunCommand(ctx, cli)
	if installRIB == true {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Logf("Route installed in RIB for refix %s as expected", v4Prefix)
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

func validateIPv4StaicRouteNoRecurse(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool, ipAf, v4Prefix, v4NextHop string,
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
		fmt.Printf("Debug:recurse:%v\n", recurse)
	}

	if installConfig == true && op.IsPresent() {
		t.Logf("Route available in running-config for prefix %v as expected", v4Prefix)
	} else if installConfig == false && !op.IsPresent() {
		t.Logf("Route not available in running-config for prefix %v as expected", v4Prefix)
	} else {
		t.Errorf("Error in running-config for route with prefix :%v", v4Prefix)
	}

	cli := fmt.Sprintf("show route %s unicast %s\n", ipAf, v4Prefix)
	cliHandle := dut.RawAPIs().CLI(t)
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)

	cliOutput, _ := cliHandle.RunCommand(ctx, cli)
	if installRIB == true {
		if strings.Contains(cliOutput.Output(), v4Prefix) && strings.Contains(cliOutput.Output(), "static") {
			t.Logf("Route installed in RIB for refix %s as expected", v4Prefix)
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

func validateIPv6StaicRouteRecurse(t *testing.T, dut *ondatra.DUTDevice, v6Prefix string,
	installConfig, installRIB bool) {

	ipAf := "ipv6"
	validateIPv4StaicRouteRecurse(t, dut, ipAf, v6Prefix, installConfig, installRIB)
}

func validateIPv6StaicRouteRecurseAttributes(t *testing.T, dut *ondatra.DUTDevice, v6Prefix, v6NextHop string,
	metric, tag, distance uint32, installConfig, installRIB bool) {

	ipAf := "ipv6"
	validateIPv4StaicRouteRecurseAttributes(t, dut, ipAf, v6Prefix, v6NextHop, metric, tag, distance, installConfig, installRIB)
}

func validateIPv6StaicRouteNoRecurse(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool, v6Prefix, v6NextHop string,
	installConfig, installRIB bool) {

	ipAf := "ipv6"
	validateIPv4StaicRouteNoRecurse(t, dut, noRecurse, ipAf, v6Prefix, v6NextHop, installConfig, installRIB)
}

func validateIPv6StaicRouteNoRecurseInterface(t *testing.T, dut *ondatra.DUTDevice, noRecurse bool, v6Prefix, v6NextHop string,
	installConfig, installRIB bool) {

	ipAf := "ipv6"
	validateIPv4StaicRouteNoRecurse(t, dut, noRecurse, ipAf, v6Prefix, v6NextHop, installConfig, installRIB)
}
