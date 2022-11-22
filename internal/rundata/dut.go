// Copyright 2022 Google LLC
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

package rundata

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
	"github.com/openconfig/ygnmi/ygnmi"
)

// dutInfo retrieves the vendor, model, and OS version from the device from various
// OpenConfig paths.
type dutInfo struct {
	vendor string
	model  string
	osver  string
}

// setFromComponentChassis sets dutInfo from the first component of type CHASSIS.
//
//   - vendor from mfg-name (Arista, Cisco).
//   - model from either description (Cisco, Juniper) or part-no (Arista).
//   - osver from software-version (Juniper).
func (di *dutInfo) setFromComponentChassis(ctx context.Context, y components.Y) {
	if di.vendor != "" && di.model != "" && di.osver != "" {
		return // No-op if nothing needs to be set.
	}

	const chassisType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS

	chassisIDs, err := y.FindByType(ctx, chassisType)
	if err != nil {
		glog.Errorf("Could not find chassis: %v", err)
		return
	}

	chassisID := chassisIDs[0]

	componentPath := ocpath.Root().Component(chassisID)
	component, err := ygnmi.Get(ctx, y.Client, componentPath.State())
	if err != nil {
		glog.Errorf("Could not get chassis: %v", err)
		return
	}

	if glog.V(2) {
		if out, err := json.MarshalIndent(component, "", "  "); err == nil {
			glog.Info(string(out))
		}
	}

	if di.vendor == "" && component.MfgName != nil {
		di.vendor = *component.MfgName
		glog.V(2).Infof("Setting vendor from chassis mfg-name: %s", di.vendor)
	}

	if di.model == "" && component.PartNo != nil {
		di.model = *component.PartNo
		glog.V(2).Infof("Setting model from chassis part-no: %s", di.model)
	}

	if desc := component.Description; desc != nil {
		if di.model == "" || strings.HasPrefix(*desc, "Cisco ") {
			di.model = *component.Description
			glog.V(2).Infof("Setting model from chassis description: %s", di.model)
		}
	}

	if di.osver == "" && component.SoftwareVersion != nil {
		di.osver = *component.SoftwareVersion
		glog.V(2).Infof("Setting osver from chassis software-version: %s", di.osver)
	}
}

// setFromComponentOS sets dutInfo from the first component of type OPERATING_SYSTEM, osver from
// software-version (Arista, Cisco).
func (di *dutInfo) setFromComponentOS(ctx context.Context, y components.Y) {
	if di.osver != "" {
		return // No-op if osver is already set.
	}

	const osType = oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_OPERATING_SYSTEM

	osIDs, err := y.FindByType(ctx, osType)
	if err != nil {
		glog.Errorf("Could not find operating-system: %v", err)
		return
	}

	osID := osIDs[0]

	softVerPath := ocpath.Root().Component(osID).SoftwareVersion()
	softVer, err := ygnmi.Get(ctx, y.Client, softVerPath.State())
	if err != nil {
		glog.Errorf("Missing component software-version: %v", err)
	} else {
		di.osver = softVer
		glog.V(2).Infof("Setting osver from operating-system software-version: %s", di.osver)
	}
}

// setFromLLDP sets dutInfo vendor from LLDP system-description (Juniper).  This is less
// reliable because LLDP config can be changed.
func (di *dutInfo) setFromLLDP(ctx context.Context, y components.Y) {
	if di.vendor != "" {
		return // No-op is vendor is already set.
	}

	lldpPath := ocpath.Root().Lldp().SystemDescription()
	lldp, err := ygnmi.Get(ctx, y.Client, lldpPath.State())
	if err != nil {
		glog.Errorf("Could not get LLDP: %v", err)
	}
	const candidate = "Juniper Networks, Inc."
	if strings.Contains(lldp, candidate) {
		di.vendor = candidate
		glog.Errorf("Setting vendor from lldp system-description: %s", di.vendor)
	}
}

// setFromSystem sets dutInfo from /system/state/software-version for osver.
//
// This is the new OpenConfig mechanism.
func (di *dutInfo) setFromSystem(ctx context.Context, y components.Y) {
	//lint:ignore U1000 Uncomment this once Ondatra updates the ygnmi generated package.
	const notSupported = `
	if di.model != "" {
		modelPath := ocpath.Root().System().Model()
		model, err := ygnmi.Get(ctx, y.Client, modelPath.State())
		if err != nil {
			glog.Errorf("Missing system model: %v", err)
		} else {
			di.model = model
			glog.V(2).Infof("Setting model from system model: %s", di.model)
		}
	}

	if di.osver != "" {
		softVerPath := ocpath.Root().System().SoftwareVersion()
		softVer, err := ygnmi.Get(ctx, y.Client, softVerPath.State())
		if err != nil {
			glog.Errorf("Missing system software-version: %v", err)
		} else {
			di.osver = softVer
			glog.V(2).Infof("Setting osver from system software-version: %s", di.osver)
		}
	}
`
}

// shortVendor canonicalizes full vendor string in short uppercase form, if possible.
func (di *dutInfo) shortVendor() string {
	if di.vendor == "" {
		return ""
	}
	vendors := []ondatra.Vendor{
		ondatra.ARISTA, ondatra.CISCO, ondatra.DELL, ondatra.JUNIPER, ondatra.IXIA, ondatra.CIENA,
		ondatra.PALOALTO, ondatra.ZPE, ondatra.NOKIA,
	}
	fullUpper := strings.ToUpper(di.vendor)
	for _, vendor := range vendors {
		shortUpper := strings.ToUpper(vendor.String())
		if strings.Contains(fullUpper, shortUpper) {
			return shortUpper
		}
	}
	if strings.Contains(fullUpper, "PALO ALTO") {
		return "PALOALTO"
	}
	return strings.SplitN(fullUpper, " ", 2)[0]
}

// ciscoRE reduces model string from e.g. "Cisco xxxx n-slot Chassis" to just "xxxx".
var ciscoRE = regexp.MustCompile(`Cisco (.*?) .*`)

// jnpRE reduces model string from e.g. "JNP10008 [PTX10008]" to just "PTX10008".
var jnpRE = regexp.MustCompile(`JNP.* \[(.*)\]`)

// shortModel canonicalizes full model to short form.
func (di *dutInfo) shortModel() string {
	if matches := ciscoRE.FindStringSubmatch(di.model); len(matches) >= 2 {
		return matches[1]
	}
	if matches := jnpRE.FindStringSubmatch(di.model); len(matches) >= 2 {
		return matches[1]
	}
	return di.model
}

// put exports the dutInfo to a map with the given dut ID.
func (di *dutInfo) put(m map[string]string, id string) {
	if di.vendor != "" {
		m[id+".vendor.full"] = di.vendor
		m[id+".vendor"] = di.shortVendor()
	}
	if di.model != "" {
		m[id+".model.full"] = di.model
		m[id+".model"] = di.shortModel()
	}
	if di.osver != "" {
		m[id+".os_version"] = di.osver
	}
}

// newDUTInfo creates a newly populated dutInfo.
func newDUTInfo(ctx context.Context, y components.Y) *dutInfo {
	di := &dutInfo{}
	di.setFromSystem(ctx, y)
	di.setFromComponentChassis(ctx, y)
	di.setFromComponentOS(ctx, y)
	di.setFromLLDP(ctx, y)
	return di
}

// dutsInfo populates the DUT properties for all DUTs in the reservation.
func dutsInfo(ctx context.Context, m map[string]string, resv *binding.Reservation) {
	for id, dut := range resv.DUTs {
		gnmic, err := dut.DialGNMI(ctx)
		if err != nil {
			glog.Errorf("Could not dial GNMI to dut %s: %v", dut.Name(), err)
			continue
		}
		yc, err := ygnmi.NewClient(gnmic)
		if err != nil {
			glog.Errorf("Could not create ygnmi.Client for dut %s: %v", dut.Name(), err)
			continue
		}
		newDUTInfo(ctx, components.Y{Client: yc}).put(m, id)
	}
}
