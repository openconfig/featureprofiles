package feature

import (
	"testing"

	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"
	oc "github.com/openconfig/ondatra/telemetry"

	"github.com/openconfig/ondatra"
)

// ConfigVrf Configures Vrf as per input file
func ConfigVrf(dev *ondatra.DUTDevice, t *testing.T, vrf *proto.InputVrf) error {

	dev.Config().NetworkInstance(vrf.Name).Update(t, configVrf(vrf))
	return nil

}

// ConfigVrf Replaces Vrfs as per input file
func ReplaceVrf(dev *ondatra.DUTDevice, t *testing.T, vrf *proto.InputVrf) error {

	dev.Config().NetworkInstance(vrf.Name).Replace(t, configVrf(vrf))
	return nil

}

// UnConfigVrf Deletes Vrfs as per input file
func UnConfigVrf(dev *ondatra.DUTDevice, t *testing.T, vrf *proto.InputVrf) error {

	dev.Config().NetworkInstance(vrf.Name).Delete(t)
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
