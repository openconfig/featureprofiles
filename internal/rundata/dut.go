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
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
	"github.com/openconfig/ygnmi/ygnmi"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

// DUTInfo retrieves the vendor, model, and OS version from the device from various
// OpenConfig paths.
type DUTInfo struct {
	Vendor string
	Model  string
	OSVer  string
}

// setFromComponentChassis sets DUTInfo from the first component of type CHASSIS.
//
//   - vendor from mfg-name (Arista, Cisco, newer Nokia).
//   - model from either description (Cisco, Juniper, newer Nokia) or part-no (Arista).
//   - osver from software-version (Juniper).
func (di *DUTInfo) setFromComponentChassis(ctx context.Context, y components.Y) {
	if di.Vendor != "" && di.Model != "" && di.OSVer != "" {
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
			glog.Infof("Chassis component: %s", string(out))
		}
	}

	if di.Vendor == "" && component.MfgName != nil {
		di.Vendor = *component.MfgName
		glog.V(2).Infof("Setting vendor from chassis mfg-name: %s", di.Vendor)
	}

	if di.Model == "" && component.PartNo != nil {
		di.Model = *component.PartNo
		glog.V(2).Infof("Setting model from chassis part-no: %s", di.Model)
	}

	if desc := component.Description; desc != nil {
		switch {
		case strings.HasPrefix(di.Vendor, "Cisco"):
			fallthrough
		case strings.HasPrefix(di.Vendor, "Nokia"):
			fallthrough
		case di.Model == "":
			di.Model = *desc
			glog.V(2).Infof("Setting model from chassis description: %s", di.Model)
		}
	}

	if di.OSVer == "" && component.SoftwareVersion != nil {
		di.OSVer = *component.SoftwareVersion
		glog.V(2).Infof("Setting osver from chassis software-version: %s", di.OSVer)
	}
}

// setFromComponentOS sets DUTInfo from the first component of type OPERATING_SYSTEM, osver from
// software-version (Arista, Cisco).
func (di *DUTInfo) setFromComponentOS(ctx context.Context, y components.Y) {
	if di.OSVer != "" {
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
		di.OSVer = softVer
		glog.V(2).Infof("Setting osver from operating-system software-version: %s", di.OSVer)
	}
}

// setFromLLDP sets DUTInfo vendor (Juniper, older Nokia) and osver (older Nokia) from
// LLDP system-description.  This is less reliable because LLDP config can be changed.
func (di *DUTInfo) setFromLLDP(ctx context.Context, y components.Y) {
	if di.Vendor != "" && di.OSVer != "" {
		return // No-op if vendor and osver are already set.
	}

	lldpPath := ocpath.Root().Lldp().SystemDescription()
	lldp, err := ygnmi.Get(ctx, y.Client, lldpPath.State())
	if err != nil {
		glog.Errorf("Could not get LLDP: %v", err)
	}
	glog.V(2).Infof("LLDP system-description: %s", lldp)

	if juniper := "Juniper Networks, Inc."; strings.Contains(lldp, juniper) {
		di.Vendor = juniper
		glog.Infof("Setting vendor from lldp system-description: %s", di.Vendor)
	}

	if srlinux := "SRLinux-v"; strings.HasPrefix(lldp, srlinux) {
		if di.Vendor == "" {
			di.Vendor = "Nokia"
			glog.Infof("Setting vendor from lldp system-description: %s", di.Vendor)
		}

		if di.OSVer == "" {
			parts := strings.Split(lldp, " ")
			di.OSVer = parts[0][len(srlinux):]
			glog.Infof("Setting osver from lldp system-description: %s", di.OSVer)
		}
	}
}

// setFromSystem sets DUTInfo from /system/state/software-version for osver.
//
// This is the new OpenConfig mechanism.
func (di *DUTInfo) setFromSystem(ctx context.Context, y components.Y) {
	// Do not fetch ocpath.Root().System().State() because it contains large
	// subtrees such as /system/aaa.

	if di.OSVer == "" {
		softVerPath := ocpath.Root().System().SoftwareVersion()
		softVer, err := ygnmi.Get(ctx, y.Client, softVerPath.State())
		if err != nil {
			glog.Errorf("Missing system software-version: %v", err)
		} else {
			di.OSVer = softVer
			glog.V(2).Infof("Setting osver from system software-version: %s", di.OSVer)
		}
	}

	//lint:ignore U1000 Uncomment this once "model" is defined.
	const notSupported = `
	if di.model == "" {
		modelPath := ocpath.Root().System().Model()
		model, err := ygnmi.Get(ctx, y.Client, modelPath.State())
		if err != nil {
			glog.Errorf("Missing system model: %v", err)
		} else {
			di.model = model
			glog.V(2).Infof("Setting model from system model: %s", di.model)
		}
	}
`
}

// shortVendor canonicalizes full vendor string in short uppercase form, if possible.
func (di *DUTInfo) shortVendor() string {
	if di.Vendor == "" {
		return ""
	}
	vendors := []ondatra.Vendor{
		ondatra.ARISTA, ondatra.CISCO, ondatra.DELL, ondatra.JUNIPER, ondatra.IXIA, ondatra.CIENA,
		ondatra.PALOALTO, ondatra.ZPE, ondatra.NOKIA,
	}
	fullUpper := strings.ToUpper(di.Vendor)
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
func (di *DUTInfo) shortModel() string {
	if matches := ciscoRE.FindStringSubmatch(di.Model); len(matches) >= 2 {
		return matches[1]
	}
	if matches := jnpRE.FindStringSubmatch(di.Model); len(matches) >= 2 {
		return matches[1]
	}
	return di.Model
}

// put exports the DUTInfo to a map with the given dut ID.
func (di *DUTInfo) put(m map[string]string, id string) {
	if di.Vendor != "" {
		m[id+".vendor.full"] = di.Vendor
		m[id+".vendor"] = di.shortVendor()
	}
	if di.Model != "" {
		m[id+".model.full"] = di.Model
		m[id+".model"] = di.shortModel()
	}
	if di.OSVer != "" {
		m[id+".os_version"] = di.OSVer
	}
}

// NewDUTInfo creates a newly populated DUTInfo.
func NewDUTInfo(ctx context.Context, gnmic gpb.GNMIClient) (*DUTInfo, error) {
	yc, err := ygnmi.NewClient(gnmic)
	if err != nil {
		return nil, fmt.Errorf("could not create ygnmiClient for dut: %v", err)
	}
	y := components.Y{Client: yc}
	di := &DUTInfo{}
	di.setFromSystem(ctx, y)
	di.setFromComponentChassis(ctx, y)
	di.setFromComponentOS(ctx, y)
	di.setFromLLDP(ctx, y)
	return di, err
}

// dutsInfo populates the DUT properties for all DUTs in the reservation.
func dutsInfo(ctx context.Context, m map[string]string, resv *binding.Reservation) {
	for id, dut := range resv.DUTs {
		gnmic, err := dut.DialGNMI(ctx)
		if err != nil {
			glog.Errorf("Could not dial GNMI to dut %s: %v", dut.Name(), err)
			continue
		}
		dInfo, err := NewDUTInfo(ctx, gnmic)
		if err != nil {
			glog.Errorf("Could not get DUTInfo for dut %v: %v", dut.Name(), err)
			continue
		}
		dInfo.put(m, id)
	}
}
