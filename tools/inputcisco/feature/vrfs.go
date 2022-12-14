package feature

import (
	"testing"

	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// ConfigVrf Configures Vrf as per input file
func ConfigVrf(dev *ondatra.DUTDevice, t *testing.T, vrf *proto.InputVrf) error {

	gnmi.Update(t, dev, gnmi.OC().NetworkInstance(vrf.Name).Config(), configVrf(vrf))
	return nil

}

// ReplaceVrf Replaces Vrfs as per input file
func ReplaceVrf(dev *ondatra.DUTDevice, t *testing.T, vrf *proto.InputVrf) error {

	gnmi.Replace(t, dev, gnmi.OC().NetworkInstance(vrf.Name).Config(), configVrf(vrf))
	return nil

}

// UnConfigVrf Deletes Vrfs as per input file
func UnConfigVrf(dev *ondatra.DUTDevice, t *testing.T, vrf *proto.InputVrf) error {

	gnmi.Delete(t, dev, gnmi.OC().NetworkInstance(vrf.Name).Config())
	return nil

}
func configVrf(vrf *proto.InputVrf) *oc.NetworkInstance {
	request := oc.NetworkInstance{}
	request.Name = &vrf.Name
	if vrf.Description != "" {
		request.Description = &vrf.Description
	}

	if vrf.Rd != "" {
		request.RouteDistinguisher = &vrf.Rd

	}
	return &request
}
