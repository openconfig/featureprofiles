package helper

import (
	"testing"

	gosnappi "github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/ondatra"
)

type TgenHelper struct{}

// TGENConfig is the interface to configure TGEN interfaces
type TGENConfig interface {
	ConfigureTgenInterface(t *testing.T) *TGENTopology
	ConfigureTGENFlows(t *testing.T) *TGENFlow
}

// TGENTopology holds either ATE or OTG topology object
type TGENTopology struct {
	ATE *ondatra.ATETopology
	OTG gosnappi.Config
}

type TGENFlow struct {
	ATE *ondatra.ATEFlow
	OTG gosnappi.Flow
}

// TgenConfigParam holds the configuration input for both ATE and OTG
type TgenConfigParam struct {
	DutIntfAttr  []attrs.Attributes
	TgenIntfAttr []attrs.Attributes
	TgenPortList []*ondatra.Port
}

type ATEParam struct {
	Params *TgenConfigParam
}

func (a *ATEParam) ConfigureTgenInterface(t *testing.T) *TGENTopology {
	t.Helper()
	ate := ondatra.ATE(t, "ate")
	topo := ate.Topology().New()

	for i, intf := range a.Params.TgenIntfAttr {
		intf.AddToATE(topo, a.Params.TgenPortList[i], &a.Params.DutIntfAttr[i])
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t).StartProtocols(t)

	return &TGENTopology{
		ATE: topo,
	}
}

type OTGParam struct {
	Params *TgenConfigParam
}

func (o *OTGParam) ConfigureTgenInterface(t *testing.T) *TGENTopology {
	otg := ondatra.ATE(t, "ate").OTG()
	topo := gosnappi.NewConfig()

	for i, intf := range o.Params.TgenIntfAttr {
		intf.AddToOTG(topo, o.Params.TgenPortList[i], &o.Params.DutIntfAttr[i])
	}

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, topo)
	otg.StartProtocols(t)

	return &TGENTopology{
		OTG: topo,
	}
}

func (a *ATEParam) ConfigureTGENFlows(t *testing.T) *TGENFlow {
    t.Helper()
    otg := ondatra.ATE(t, "ate").OTG()
    flow := gosnappi.NewFlow()

    // Example logic for configuring flows
    for _, intf := range o.Params.TgenIntfAttr {
        flow.SetName("Flow_" + intf.Name)
        // Add more flow configuration logic here
    }

    t.Logf("Pushing flow config to OTG...")
    ate.PushFlowConfig(t, flow)

    return &TGENFlow{
        OTG: flow,
    }
}

func (o *OTGParam) ConfigureTGENFlows(t *testing.T) *TGENFlow {
    t.Helper()
    otg := ondatra.ATE(t, "ate").OTG()
    flow := gosnappi.NewFlow()

    // Example logic for configuring flows
    for _, intf := range o.Params.TgenIntfAttr {
        flow.SetName("Flow_" + intf.Name)
        // Add more flow configuration logic here
    }

    t.Logf("Pushing flow config to OTG...")
    otg.PushFlowConfig(t, flow)

    return &TGENFlow{
        OTG: flow,
    }
}

// ConfigureTGEN selects the Tgen API ATE vs OTG ased on useOTG flag
func (h *TgenHelper) ConfigureTGEN(useOTG bool, param *TgenConfigParam) TGENConfig {
	if useOTG {
		return &OTGParam{Params: param}
	}
	return &ATEParam{Params: param}
}
