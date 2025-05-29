package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	inputFile = "testdata/rpl.yaml"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	observer  = fptest.NewObserver("RPL").AddCsvRecorder("ocreport").
			AddCsvRecorder("RPL")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
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
			// FIXME SetMed changed to either uint32 or enum value
			// statement.Actions.BgpActions.SetMed = oc.UnionString(policy.Bgpaction.Medtype)

		}
	}
	return statement
}
