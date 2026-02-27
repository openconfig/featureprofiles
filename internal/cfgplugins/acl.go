package cfgplugins

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	IPFragmentFirst   = oc.Transport_BuiltinDetail_TCP_INITIAL
	IPFragmentLater   = oc.Transport_BuiltinDetail_FRAGMENT
	ICMPv4ProtocolNum = 1
	ICMPv6ProtocolNum = 58
	TCPProtocolNum    = 6
	UDPProtocolNum    = 17
	DefaultEntryID    = 990
	matchAllV4        = "0.0.0.0/0"
	matchAllV6        = "::/0"
)

type AclParams struct {
	Name          string
	DefaultPermit bool
	ACLType       oc.E_Acl_ACL_TYPE
	Intf          string
	Ingress       bool
	Terms         []AclTerm
	Update        bool
}

type AclTerm struct {
	SeqID             uint32
	Description       string
	Permit            bool
	IPSrc             string
	IPDst             string
	L4SrcPort         uint32
	L4SrcPortRange    string
	L4DstPort         uint32
	L4DstPortRange    string
	ICMPCode          int64
	ICMPType          int64
	IPInitialFragment bool
	Protocol          uint8
	Log               bool
}

var (
	defaultRuleV4 = AclTerm{
		SeqID: DefaultEntryID,
		IPSrc: matchAllV4,
		IPDst: matchAllV4,
	}

	defaultRuleV6 = AclTerm{
		SeqID: DefaultEntryID,
		IPSrc: matchAllV6,
		IPDst: matchAllV6,
	}

	ndpPermitTerm = AclTerm{
		SeqID:    DefaultEntryID - 10,
		Permit:   true,
		Protocol: ICMPv6ProtocolNum,
		IPSrc:    matchAllV6,
		IPDst:    matchAllV6,
	}
)

func createACLEntry(aclSet *oc.Acl_AclSet, term AclTerm, aclType oc.E_Acl_ACL_TYPE) {
	entry := aclSet.GetOrCreateAclEntry(term.SeqID)
	if term.Permit {
		entry.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	} else {
		entry.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_DROP
	}
	if term.Log {
		entry.GetOrCreateActions().LogAction = oc.Acl_LOG_ACTION_LOG_SYSLOG
	}

	switch aclType {
	case oc.Acl_ACL_TYPE_ACL_IPV4:
		ipv4 := entry.GetOrCreateIpv4()
		if term.IPSrc != "" {
			ipv4.SourceAddress = ygot.String(term.IPSrc)
		}
		if term.IPDst != "" {
			ipv4.DestinationAddress = ygot.String(term.IPDst)
		}
		if term.Protocol != 0 {
			ipv4.SetProtocol(oc.UnionUint8(uint8(term.Protocol)))
			if term.Protocol == ICMPv4ProtocolNum {
				icmp := ipv4.GetOrCreateIcmpv4()
				icmp.Code = oc.E_Icmpv4Types_CODE(term.ICMPCode)
				icmp.Type = oc.E_Icmpv4Types_TYPE(term.ICMPType)
			}
		}
	case oc.Acl_ACL_TYPE_ACL_IPV6:
		ipv6 := entry.GetOrCreateIpv6()
		if term.IPSrc != "" {
			ipv6.SourceAddress = ygot.String(term.IPSrc)
		}
		if term.IPDst != "" {
			ipv6.DestinationAddress = ygot.String(term.IPDst)
		}
		if term.Protocol != 0 {
			ipv6.SetProtocol(oc.UnionUint8(uint8(term.Protocol)))
			if term.Protocol == ICMPv6ProtocolNum {
				icmp := ipv6.GetOrCreateIcmpv6()
				icmp.Code = oc.E_Icmpv6Types_CODE(term.ICMPCode)
				icmp.Type = oc.E_Icmpv6Types_TYPE(term.ICMPType)
			}
		}
	}

	if term.Protocol == TCPProtocolNum || term.Protocol == UDPProtocolNum {
		transport := entry.GetOrCreateTransport()
		if term.L4SrcPort != 0 {
			transport.SourcePort = oc.UnionUint16(term.L4SrcPort)
		}
		if term.L4SrcPortRange != "" {
			transport.SourcePortSet = ygot.String(term.L4SrcPortRange)
		}
		if term.L4DstPort != 0 {
			transport.DestinationPort = oc.UnionUint16(term.L4DstPort)
		}
		if term.L4DstPortRange != "" {
			transport.DestinationPortSet = ygot.String(term.L4DstPortRange)
		}
	}
}

func ConfigureACL(t *testing.T, batch *gnmi.SetBatch, params AclParams) {
	t.Helper()
	aclRoot := &oc.Root{}
	acl := aclRoot.GetOrCreateAcl()
	acl.CounterCapability = oc.Acl_ACL_COUNTER_CAPABILITY_AGGREGATE_ONLY
	aclSet := acl.GetOrCreateAclSet(params.Name, params.ACLType)
	aclSet.Type = params.ACLType

	defaultTerm := defaultRuleV4
	if params.ACLType == oc.Acl_ACL_TYPE_ACL_IPV6 {
		defaultTerm = defaultRuleV6

		if !params.DefaultPermit {
			createACLEntry(aclSet, ndpPermitTerm, params.ACLType)
		}
	}
	defaultTerm.Permit = params.DefaultPermit

	for _, term := range params.Terms {
		createACLEntry(aclSet, term, params.ACLType)
	}
	createACLEntry(aclSet, defaultTerm, params.ACLType)

	if params.Update {
		t.Logf("Updating ACL %s", params.Name)
		gnmi.BatchUpdate(batch, gnmi.OC().Acl().AclSet(params.Name, params.ACLType).Config(), aclSet)
		return
	}

	t.Logf("Creating ACL %s", params.Name)
	gnmi.BatchReplace(batch, gnmi.OC().Acl().AclSet(params.Name, params.ACLType).Config(), aclSet)

	aclIface := acl.GetOrCreateInterface(params.Intf)
	if params.Ingress {
		aclIface.GetOrCreateIngressAclSet(params.Name, params.ACLType)
	} else {
		aclIface.GetOrCreateEgressAclSet(params.Name, params.ACLType)
	}
	aclIface.GetOrCreateInterfaceRef().Interface = ygot.String(params.Intf)
	aclIface.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)

	t.Logf("Applying ACL %s to Interface %s", params.Name, params.Intf)
	gnmi.BatchReplace(batch, gnmi.OC().Acl().Interface(params.Intf).Config(), aclIface)
}

func DeleteACL(t *testing.T, batch *gnmi.SetBatch, params AclParams) {
	t.Helper()

	if params.Name == "" || params.ACLType == oc.Acl_ACL_TYPE_UNSET || params.Intf == "" {
		t.Fatal("unable to delete ACL, missing required parameters")
		return
	}

	if params.Ingress {
		t.Logf("Removing Ingress ACL from Interface %s", params.Intf)
		gnmi.BatchDelete(batch, gnmi.OC().Acl().Interface(params.Intf).IngressAclSet(params.Name, params.ACLType).Config())
	} else {
		t.Logf("Removing Egress ACL from Interface %s", params.Intf)
		gnmi.BatchDelete(batch, gnmi.OC().Acl().Interface(params.Intf).EgressAclSet(params.Name, params.ACLType).Config())
	}

	t.Log("Deleting ACL")
	gnmi.BatchDelete(batch, gnmi.OC().Acl().AclSet(params.Name, params.ACLType).Config())
}
func EnableACLCountersFromCLI(t *testing.T, dut *ondatra.DUTDevice, params AclParams) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var ipStr string
		switch params.ACLType {
		case oc.Acl_ACL_TYPE_ACL_IPV4:
			ipStr = "ip"
		case oc.Acl_ACL_TYPE_ACL_IPV6:
			ipStr = "ipv6"
		}

		countersCommand := fmt.Sprintf(`%s access-list %s
	counters per-entry
	!`, ipStr, params.Name)
		helpers.GnmiCLIConfig(t, dut, countersCommand)
		return
	default:
		t.Logf("ACL counter enabling not implemented for vendor %s, skipping", dut.Vendor())
	}
}
