package feature

import (
	"testing"

	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"
	"github.com/pkg/errors"

	// "github.com/openconfig/ygot/ygot"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	"github.com/openconfig/ygot/ygot"
)

// ConfigRPL configures RPL from input file
func ConfigRPL(dev *ondatra.DUTDevice, t *testing.T, policy *proto.Input_RoutePolicy) error {
	if policy.Name == "" {
		return errors.Errorf("Cannot configure rouite-policy without name %v", policy)

	}
	rpl := &oc.RoutingPolicy{}
	rpd, err := rpl.NewPolicyDefinition(policy.Name)
	if err != nil {
		return errors.Errorf("cannot reuse routing policy definition %v", err)
	}

	statement, err := rpd.AppendNewStatement(policy.Name + "stmt")
	if err != nil {
		return errors.Errorf("cannot reuse routing policy definition %v", err)
	}

	updatePolicy(statement, policy.Policy)
	gnmi.Update(t, dev, gnmi.OC().RoutingPolicy().Config(), rpl)
	return nil
}

// ReplaceRPL Replaces RPL from input file
func ReplaceRPL(dev *ondatra.DUTDevice, t *testing.T, policy *proto.Input_RoutePolicy) error {
	if policy.Name == "" {
		return errors.Errorf("Cannot configure rouite-policy without name %v", policy)

	}
	rpl := &oc.RoutingPolicy{}
	rpd, err := rpl.NewPolicyDefinition(policy.Name)
	if err != nil {
		return errors.Errorf("cannot reuse routing policy definition %v", err)
	}

	statement, err := rpd.AppendNewStatement(policy.Name + "stmt")
	if err != nil {
		return errors.Errorf("cannot reuse routing policy definition %v", err)
	}

	updatePolicy(statement, policy.Policy)
	gnmi.Replace(t, dev, gnmi.OC().RoutingPolicy().Config(), rpl)
	return nil
}

// UnConfigRPL removes RPL configs from input file
func UnConfigRPL(dev *ondatra.DUTDevice, t *testing.T, policy *proto.Input_RoutePolicy) error {
	if policy.Name == "" {
		return errors.Errorf("Cannot configure rouite-policy without name %v", policy)
	}
	gnmi.Delete(t, dev, gnmi.OC().RoutingPolicy().PolicyDefinition(policy.Name).Config())
	return nil
}

func updatePolicy(statement *oc.RoutingPolicy_PolicyDefinition_Statement, policy *proto.Input_RoutePolicy_Policy) *oc.RoutingPolicy_PolicyDefinition_Statement {
	switch policy.Action {
	case proto.Input_RoutePolicy_accept:
		statement.Actions = &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{}
		statement.Actions.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	case proto.Input_RoutePolicy_reject:
		statement.Actions = &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{}
		statement.Actions.PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	}
	if policy.Bgpaction != nil {
		if statement.Actions == nil {
			statement.Actions = &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{}
		}
		if policy.Bgpaction != nil {
			statement.Actions.BgpActions = &oc.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions{}
		}
		if policy.Bgpaction.LocalPerf != 0 {
			statement.Actions.BgpActions.SetLocalPref = ygot.Uint32(uint32(policy.Bgpaction.LocalPerf))
		}
		if policy.Bgpaction.Aspathprepend != nil {
			statement.Actions.BgpActions.SetAsPathPrepend = &oc.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetAsPathPrepend{}
		}
		if policy.Bgpaction.Aspathprepend.Asn != 0 {
			statement.Actions.BgpActions.SetAsPathPrepend.Asn = ygot.Uint32(uint32(policy.Bgpaction.Aspathprepend.Asn))
		}
		if policy.Bgpaction.Aspathprepend.Repeatn != 0 {
			statement.Actions.BgpActions.SetAsPathPrepend.Asn = ygot.Uint32(uint32(policy.Bgpaction.Aspathprepend.Repeatn))
		}
		if policy.Bgpaction.Medtype != "" {
			statement.Actions.BgpActions.SetMed = oc.UnionString(policy.Bgpaction.Medtype)

		}

	}
	return statement
}
