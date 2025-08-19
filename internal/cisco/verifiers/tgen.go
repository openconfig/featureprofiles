// Package verifiers provides verifiers APIs to verify oper data for different component verifications.

package verifiers

import (
	"testing"

	gosnappi "github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

type tgenVerifier struct{}

type TGENFlow struct {
	ATE []*ondatra.Flow
	OTG []gosnappi.Flow
}

// TGENConfig is the interface to configure TGEN interfaces
type TGENValidate interface {
	ValidateTrafficLoss(t testing.TB) bool
}

// TgenConfigParam holds the configuration input for both ATE and OTG
type TgenValidationParam struct {
	Tolerance float64 // tolerance for traffic loss
	WantLoss  bool    // true if want to validate loss, false if want to validate no loss
	Flows     *helper.TGENFlow
}

type ATEParam struct {
	Params *TgenValidationParam
}

type OTGParam struct {
	Params *TgenValidationParam
}

// ValidateTrafficLoss validates the traffic loss for the given flows, returns true if there is traffic loss, else false.
func (atep *ATEParam) ValidateTrafficLoss(t testing.TB) bool {
	ate := ondatra.ATE(t, "ate")
	var trafficLoss bool = false
	for _, flow := range atep.Params.Flows.ATE {
		flowPath := gnmi.OC().Flow(flow.Name())
		t.Log("Verify no traffic loss")
		got := gnmi.Get(t, ate, flowPath.LossPct().State())
		if atep.Params.WantLoss {
			if got < 100 {
				t.Fatalf("LossPct for flow %s: got %g, want 100", flow.Name(), got)
				trafficLoss = true
			}
		} else {
			if got > 0 {
				t.Logf("LossPct for flow %s: got %g, want 0", flow.Name(), got)

			}
		}
	}
	return trafficLoss
}

// ValidateTrafficLoss validates the traffic loss for the given flows, returns true if there is traffic loss, else false.
func (otgp *OTGParam) ValidateTrafficLoss(t testing.TB) bool {
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	var trafficLoss bool = false
	for _, flow := range otgp.Params.Flows.OTG {
		outPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))

		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow)
		}
		if otgp.Params.WantLoss {
			if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
				t.Fatalf("LossPct for flow %s: got %v, want 0", flow.Name(), got)
			}
		} else {
			if got := ((outPkts - inPkts) * 100) / outPkts; got != 100 {
				t.Fatalf("LossPct for flow %s: got %v, want 100", flow.Name(), got)
			}
		}
	}
	return trafficLoss
}

// ConfigureTGEN selects the Tgen API ATE vs OTG ased on useOTG flag
func (h *tgenVerifier) ValidateTGEN(useOTG bool, param *TgenValidationParam) TGENValidate {
	if useOTG {
		return &OTGParam{Params: param}
	}
	return &ATEParam{Params: param}
}
