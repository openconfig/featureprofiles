// 2022 Nokia.  This code is a Contribution to OpenConfig Feature Profiles
// project ("Work") made under the Google Software Grant and Corporate
// Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property
// are granted for any other purpose. This code is provided on an "as is" basis
// without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package fptest

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// Set Port Speed
func SetPortSpeed(t *testing.T, d *ondatra.DUTDevice, p string) {
	//
	var portSpeed = map[ondatra.Speed]oc.E_IfEthernet_ETHERNET_SPEED{
		ondatra.Speed1Gb:   oc.IfEthernet_ETHERNET_SPEED_SPEED_1GB,
		ondatra.Speed5Gb:   oc.IfEthernet_ETHERNET_SPEED_SPEED_5GB,
		ondatra.Speed10Gb:  oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
		ondatra.Speed100Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
		ondatra.Speed400Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
	}
	port := d.Port(t, p)
	ps := port.Speed()
	if ps == 0 {
		// Port speed is unspecified. Explicit config not performed
		return
	}
	speed := portSpeed[ps]
	t.Logf("Configuring %v port-speed to %v", port.Name(), speed)
	intf := &oc.Interface{Name: ygot.String(port.Name())}
	eth := intf.GetOrCreateEthernet()
	eth.SetPortSpeed(speed)
	gnmi.Update(t, d, gnmi.OC().Interface(intf.GetName()).Config(), intf)
}
