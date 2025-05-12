package os_install_test

import (
	"fmt"
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

// compareStructs iterates over want comparing values with got
func compareStructs(t *testing.T, want, got interface{}) {
	vwant := reflect.ValueOf(want).Elem()
	vgot := reflect.ValueOf(got).Elem()
	twant := vwant.Type()

	// Iterate over the fields of the struct
	for i := 0; i < vwant.NumField(); i++ {
		fieldName := twant.Field(i).Name
		fwant := vwant.Field(i)
		fgot := vgot.FieldByName(fieldName)
		// check if the field is a pointer and is nil, this means its the end of the leaf node
		if fwant.Kind() == reflect.Ptr && fwant.IsNil() {
			continue
		} else {
			// check if the field is a struct or a pointer to a struct, if so call compareStructs recursively
			if fwant.Kind() == reflect.Ptr && fwant.Elem().Kind() == reflect.Struct {
				compareStructs(t, fwant.Interface(), fgot.Interface())
				// check if the field is a slice and non empty, if so iterate over the slice and call compareStructs recursively
			} else if fwant.Kind() == reflect.Map && fwant.IsValid() {
				for _, key := range fwant.MapKeys() {
					strct := fwant.MapIndex(key)
					strctVal := fgot.MapIndex(key)
					// if there is a map inside a struct, call compareStructs recursively
					if strct.Kind() == reflect.Map {
						compareStructs(t, strct.Interface(), strctVal.Interface())
					} else if fwant.Kind() == reflect.Ptr && fwant.IsNil() {
						continue
					} else if strct.Kind() == reflect.Ptr && strct.Elem().Kind() == reflect.Struct {
						compareStructs(t, strct.Interface(), strctVal.Interface())
					}
				}
				// check if the enum type is 0, this means the field is not set and should be skipped
			} else if fwant.Kind() == reflect.Int64 && fwant.Int() == 0 {
				t.Logf("Skipping default value with the field %s\n", fieldName)
				continue
				// compare the field values if they are the same or not
			} else if reflect.DeepEqual(fwant.Interface(), fgot.Interface()) {
				t.Logf("The field %s is equal in both structs\n got: %#v \t want: %#v \n", fieldName, fwant.Interface(), fgot.Interface())
			} else {
				t.Errorf("The field %s is not equal in both structs\n got: %#v \t want: %#v \n", fieldName, fwant.Interface(), fgot.Interface())
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
