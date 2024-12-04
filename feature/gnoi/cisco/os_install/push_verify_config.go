package os_install_test

import (
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

type bgpAttrs struct {
	rplName, peerGrpNamev4    string
	prefixLimit, dutAS, ateAS uint32
	grRestartTime             uint16
}
type bgpNeighbor struct {
	localAs, peerAs, pfxLimit uint32
	neighborip                string
	isV4                      bool
}

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutAttrs = attrs.Attributes{
		Desc:    "To ATE",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	bgpGlobalAttrs = bgpAttrs{
		rplName:       "ALLOW",
		grRestartTime: 60,
		prefixLimit:   200,
		dutAS:         64500,
		ateAS:         64501,
		peerGrpNamev4: "BGP-PEER-GROUP-V4",
	}
	bgpNbr1 = bgpNeighbor{localAs: bgpGlobalAttrs.dutAS, peerAs: bgpGlobalAttrs.ateAS, pfxLimit: bgpGlobalAttrs.prefixLimit, neighborip: ateSrc.IPv4, isV4: true}
)

func testPushAndVerifyInterfaceConfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Logf("Create and push interface config to the DUT")
	dutPort := dut.Port(t, "port1")
	dutPortName := dutPort.Name()
	intf1 := dutAttrs.NewOCInterface(dutPortName, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intf1.GetName()).Config(), intf1)

	dc := gnmi.OC().Interface(dutPortName).Config()
	in := configInterface(dutPortName, dutAttrs.Desc, dutAttrs.IPv4, dutAttrs.IPv4Len, dut)
	fptest.LogQuery(t, fmt.Sprintf("%s to Replace()", dutPort), dc, in)
	gnmi.Replace(t, dut, dc, in)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		ocPortName := dut.Port(t, "port1").Name()
		fptest.AssignToNetworkInstance(t, dut, ocPortName, deviations.DefaultNetworkInstance(dut), 0)
	}

	t.Logf("Fetch interface config from the DUT using Get RPC and verify it matches with the config that was pushed earlier")
	if val, present := gnmi.LookupConfig(t, dut, dc).Val(); present {
		compareStructs(t, in, val)
	} else {
		t.Errorf("Config %v Get() failed", dc)
	}
}

// configInterface generates an interface's configuration based on the the attributes given.
func configInterface(name, desc, ipv4 string, prefixlen uint8, dut *ondatra.DUTDevice) *oc.Interface {
	i := &oc.Interface{}
	i.Name = ygot.String(name)
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()

	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}

	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return i
}

func testPushAndVerifyBGPConfig(t *testing.T, dut *ondatra.DUTDevice) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	t.Logf("Create and push BGP config to the DUT")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := bgpCreateNbr(dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

	t.Logf("Fetch BGP config from the DUT using Get RPC and verify it matches with the config that was pushed earlier")
	if val, present := gnmi.LookupConfig(t, dut, dutConfPath.Config()).Val(); present {
		compareStructs(t, dutConf, val)
	} else {
		t.Errorf("Config %v Get() failed", dutConfPath.Config())
	}
}

func compareStructs(t *testing.T, dutConf, val interface{}) {
	vdutConf := reflect.ValueOf(dutConf).Elem()
	vval := reflect.ValueOf(val).Elem()
	tdutConf := vdutConf.Type()

	for i := 0; i < vdutConf.NumField(); i++ {
		fieldName := tdutConf.Field(i).Name
		fdutConf := vdutConf.Field(i)
		fval := vval.FieldByName(fieldName)
		if fdutConf.Kind() == reflect.Ptr && fdutConf.IsNil() {
			continue
		} else {
			if fdutConf.Kind() == reflect.Ptr && fdutConf.Elem().Kind() == reflect.Struct {
				compareStructs(t, fdutConf.Interface(), fval.Interface())
			} else if fdutConf.Kind() == reflect.Map && fdutConf.IsValid() {
				for _, key := range fdutConf.MapKeys() {
					strct := fdutConf.MapIndex(key)
					strctVal := fval.MapIndex(key)
					log.Printf("strct %v \t, strctVal %v\n", strct, strctVal)
					if strct.Kind() == reflect.Map {
						compareStructs(t, strct.Interface(), strctVal.Interface())
					} else if fdutConf.Kind() == reflect.Ptr && fdutConf.IsNil() {
						continue
					} else if strct.Kind() == reflect.Ptr && strct.Elem().Kind() == reflect.Struct {
						compareStructs(t, strct.Interface(), strctVal.Interface())
					}
				}
			} else if reflect.DeepEqual(fdutConf.Interface(), fval.Interface()) {
				t.Logf("The field %s is equal in both structs\n got: %#v \t want: %#v \n", fieldName, fdutConf.Interface(), fval.Interface())
			} else {
				if !(fieldName == "SendCommunity") {
					t.Errorf("The field %s is not equal in both structs\n got: %#v \t want: %#v \n", fieldName, fdutConf.Interface(), fval.Interface())
				} else {
					continue
				}
			}
		}

	}
}

// bgpCreateNbr creates a BGP object with neighbor pointing to ateSrc
func bgpCreateNbr(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(uint32(bgpGlobalAttrs.dutAS))
	global.RouterId = ygot.String(dutSrc.IPv4)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	pgv4 := bgp.GetOrCreatePeerGroup(bgpGlobalAttrs.peerGrpNamev4)
	pgv4.PeerAs = ygot.Uint32(uint32(bgpGlobalAttrs.ateAS))
	pgv4.PeerGroupName = ygot.String(bgpGlobalAttrs.peerGrpNamev4)
	nbr := bgpNbr1
	nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
	nv4.PeerAs = ygot.Uint32(nbr.peerAs)
	nv4.Enabled = ygot.Bool(true)
	nv4.PeerGroup = ygot.String(bgpGlobalAttrs.peerGrpNamev4)
	return niProto
}
