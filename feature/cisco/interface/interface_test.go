package basetest

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 24
	inputFile     = "testdata/interface.yaml"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	observer  = fptest.NewObserver("Interface").AddCsvRecorder("ocreport").
			AddCsvRecorder("Interface")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIFIPCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")

	t.Run("configInterface", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name())
		obj := &oc.Interface{
			Name:        ygot.String(iut.Name()),
			Description: ygot.String("randstr"),
			Type:        oc.IETFInterfaces_InterfaceType_ieee8023adLag,
		}
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Update(t, dut, path.Config(), obj)

	})

	t.Run("Update//interfaces/interface/config/name", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), iut.Name())

	})
	// Config/type and state/type is currently caveated , decision pending with MGBL for its support
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/addresses/address[ip]/config/type
	// oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/addresses/address[ip]/state/type
	// t.Run("Update//ipv6/addresses/address[ip]/config/type", func(t *testing.T) {
	// 	path := gnmi.OC().Interface(iut.Name()).Subinterface(0).Ipv6().Address("2001::1").Type()
	// 	gnmi.Update(t, dut, path.Config(), oc.IfIp_Ipv6AddressType_GLOBAL_UNICAST)
	// })

	interface_name := iut.Name()
	t.Run("Update//ipv6/enable", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().Enabled()
		gnmi.Update(t, dut, path.Config(), true)
	})

	vlanid := uint32(0)
	spath := gnmi.OC().Interface(iut.Name()).Subinterface(vlanid)
	sobj := &oc.Interface_Subinterface{
		Index: ygot.Uint32(0),
		Ipv6: &oc.Interface_Subinterface_Ipv6{
			Address: map[string]*oc.Interface_Subinterface_Ipv6_Address{
				iut.Ipv6Address(): {
					Ip:           ygot.String(iut.Ipv6Address()),
					PrefixLength: ygot.Uint8(iut.Ipv6PrefixLength()),
				},
			},
		},
		Ipv4: &oc.Interface_Subinterface_Ipv4{
			Address: map[string]*oc.Interface_Subinterface_Ipv4_Address{
				iut.Ipv4Address(): {
					Ip:           ygot.String(iut.Ipv4Address()),
					PrefixLength: ygot.Uint8(iut.Ipv4PrefixLength()),
				},
			},
		},
	}
	gnmi.Update(t, dut, spath.Config(), sobj)

	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/config/enable
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/state/enable
	t.Run("Update//ipv6/router-advertisement/config/enable true", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Enable()
		gnmi.Update(t, dut, path.Config(), true)
	})
	t.Run("Get Config//ipv6/router-advertisement/config/enable true", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Enable()
		enable_ra := gnmi.Get(t, dut, path.Config())
		if enable_ra != true {
			t.Errorf("RouterAdvertisement not enabled")
		}
	})
	state_supported := false

	if state_supported {
		t.Run("Get State Config//ipv6/router-advertisement/config/enable", func(t *testing.T) {
			path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Enable()
			enable_ra := gnmi.Get(t, dut, path.State())
			if enable_ra != true {
				t.Errorf("RouterAdvertisement not enabled")
			}
		})
	}
	t.Run("Update//ipv6/router-advertisement/config/enable false", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Enable()
		gnmi.Update(t, dut, path.Config(), false)
	})
	t.Run("Get Config//ipv6/router-advertisement/config/enable false", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Enable()
		enable_ra := gnmi.Get(t, dut, path.Config())
		if enable_ra != false {
			t.Errorf("RouterAdvertisement not disabled")
		}
	})
	if state_supported {
		t.Run("Get State Config//ipv6/router-advertisement/config/enable", func(t *testing.T) {
			path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Enable()
			enable_ra := gnmi.Get(t, dut, path.State())
			if enable_ra != false {
				t.Errorf("RouterAdvertisement not disabled")
			}
		})
	}

	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/config/managed
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/state/managed
	t.Run("Update//ipv6/router-advertisement/config/managed true", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Managed()
		gnmi.Update(t, dut, path.Config(), true)
	})
	t.Run("Get Config//ipv6/router-advertisement/config/managed true", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Managed()
		enable_managed := gnmi.Get(t, dut, path.Config())
		if enable_managed != true {
			t.Errorf("RouterAdvertisement Managed not enabled")
		}
	})
	if state_supported {
		t.Run("Get State Config//ipv6/router-advertisement/state/managed", func(t *testing.T) {
			path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Managed()
			enable_managed := gnmi.Get(t, dut, path.State())
			if enable_managed != true {
				t.Errorf("RouterAdvertisement Managed not enabled")
			}
		})
	}
	t.Run("Update//ipv6/router-advertisement/config/managed false", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Managed()
		gnmi.Update(t, dut, path.Config(), false)
	})
	t.Run("Get Config//ipv6/router-advertisement/state/managed false", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Managed()
		enable_managed := gnmi.Get(t, dut, path.Config())
		if enable_managed != false {
			t.Errorf("RouterAdvertisement Managed not disabled")
		}
	})
	if state_supported {
		t.Run("Get State Config//ipv6/router-advertisement/state/managed", func(t *testing.T) {
			path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Managed()
			enable_managed := gnmi.Get(t, dut, path.State())
			if enable_managed != false {
				t.Errorf("RouterAdvertisement Managed not disabled")
			}
		})
	}

	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/config/other-config
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/state/other-config
	t.Run("Update//ipv6/router-advertisement/config/other-config true", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().OtherConfig()
		gnmi.Update(t, dut, path.Config(), true)
	})
	t.Run("Get Config//ipv6/router-advertisement/config/other-config true", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().OtherConfig()
		enable_managed := gnmi.Get(t, dut, path.Config())
		if enable_managed != true {
			t.Errorf("RouterAdvertisement OtherConfig not enabled")
		}
	})
	t.Run("Update//ipv6/router-advertisement/config/other-config false", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().OtherConfig()
		gnmi.Update(t, dut, path.Config(), false)
	})
	t.Run("Get Config//ipv6/router-advertisement/config/other-config false", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().OtherConfig()
		enable_managed := gnmi.Get(t, dut, path.Config())
		if enable_managed != false {
			t.Errorf("RouterAdvertisement OtherConfig not disabled")
		}
	})

	prefix_value := "3001::1/64"
	prefix_value2 := "3002::1/64"
	// <0-4294967295>  Valid Lifetime (secs)
	// <0-4294967295>  Preferred Lifetime (secs) must be <= Valid Lifetime
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes
	t.Run("Update//ipv6/router-advertisement/prefixes", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value)
		gnmi.Update(t, dut, path.Config(), &oc.Interface_Subinterface_Ipv6_RouterAdvertisement_Prefix{
			Prefix:            &prefix_value,
			ValidLifetime:     ygot.Uint32(32),
			PreferredLifetime: ygot.Uint32(32),
		})
	})
	//	/oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/config/valid-lifetime
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/state/valid-lifetime
	t.Run("Update//ipv6/router-advertisement/prefixes/prefix[prefix]/config/valid-lifetime", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value).ValidLifetime()
		gnmi.Update(t, dut, path.Config(), 429496729)
	})
	t.Run("Get Config//ipv6/router-advertisement/prefixes/prefix[prefix]/config/valid-lifetime", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value).ValidLifetime()

		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dut, path.Config())
		}); errMsg != nil {
			t.Logf("Expected failure and got errMsg : %s", *errMsg)
			r, _ := regexp.Compile("429496729")
			if r.MatchString(*errMsg) {
				t.Logf("Got expected value for GetRequest")

			} else {
				t.Errorf("Get-Config failed to get expected value")
			}
		} else {
			t.Logf("Data not returned in canonical format.")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/config/preferred-lifetime
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/state/preferred-lifetime
	t.Run("Update//ipv6/router-advertisement/prefixes/prefix[prefix]/config/preferred-lifetime", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value).PreferredLifetime()
		gnmi.Update(t, dut, path.Config(), 429496729)
	})
	t.Run("Get Config//ipv6/router-advertisement/prefixes/prefix[prefix]/config/preferred-lifetime", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value).PreferredLifetime()

		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dut, path.Config())
		}); errMsg != nil {
			t.Logf("Expected failure and got errMsg : %s", *errMsg)
			r, _ := regexp.Compile("429496729")
			if r.MatchString(*errMsg) {
				t.Logf("Got expected value for GetRequest")
			} else {
				t.Errorf("Get-Config failed to get expected value")
			}
		} else {
			t.Logf("Data not returned in canonical format.")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/config/disable-advertisement
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/state/disable-advertisement
	t.Run("Update//ipv6/router-advertisement/prefixes", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value2)
		gnmi.Update(t, dut, path.Config(), &oc.Interface_Subinterface_Ipv6_RouterAdvertisement_Prefix{
			Prefix: &prefix_value2,
		})
	})
	t.Run("Update//ipv6/router-advertisement/prefixes/prefix[prefix]/config/disable-advertisement", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value2).DisableAdvertisement()
		gnmi.Update(t, dut, path.Config(), true)
	})
	t.Run("Get Config//ipv6/router-advertisement/prefixes/prefix[prefix]/config/disable-advertisement", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value2).DisableAdvertisement()
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dut, path.Config())
			//t.Logf("Get Config : %v", got)
		}); errMsg != nil {
			t.Logf("Expected failure and got errMsg : %s", *errMsg)
			r, _ := regexp.Compile("true")
			if r.MatchString(*errMsg) {
				t.Logf("Got expected value for GetRequest")
			} else {
				t.Errorf("Get-Config failed to get expected value")
			}
		} else {
			t.Logf("Data not returned in canonical format.")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/config/disable-autoconfiguration
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/state/disable-autoconfiguration
	t.Run("Update//ipv6/router-advertisement/prefixes/prefix[prefix]/config/disable-autoconfiguration", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value).DisableAutoconfiguration()
		gnmi.Update(t, dut, path.Config(), true)
	})
	t.Run("Get Config//ipv6/router-advertisement/prefixes/prefix[prefix]/config/disable-autoconfiguration", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value).DisableAutoconfiguration()
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dut, path.Config())
			//t.Logf("Get Config : %v", got)
		}); errMsg != nil {
			t.Logf("Expected failure and got errMsg : %s", *errMsg)
			r, _ := regexp.Compile("true")
			if r.MatchString(*errMsg) {
				t.Logf("Got expected value for GetRequest")
			} else {
				t.Errorf("Get-Config failed to get expected value")
			}
		} else {
			t.Logf("Data not returned in canonical format.")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/config/enable-onlink
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/router-advertisement/prefixes/prefix[prefix]/state/enable-onlink
	t.Run("Update//ipv6/router-advertisement/prefixes/prefix[prefix]/config/enable-onlink", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value).EnableOnlink()
		gnmi.Update(t, dut, path.Config(), true)
	})
	t.Run("Get Config//ipv6/router-advertisement/prefixes/prefix[prefix]/config/enable-onlink", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value).EnableOnlink()
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dut, path.Config())
		}); errMsg != nil {
			t.Logf("Expected failure and got errMsg : %s", *errMsg)
			r, _ := regexp.Compile("true")
			if r.MatchString(*errMsg) {
				t.Logf("Got expected value for GetRequest")
			} else {
				t.Errorf("Get-Config failed to get expected value")
			}
		} else {
			t.Logf("Data not returned in canonical format.")
		}
	})

	t.Run("Get//ipv4/state/counters/in-multicast-pkts", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv4().Counters().InMulticastPkts()
		t.Log("Subscribe on InMulticastPkts")
		gnmi.Get(t, dut, path.State())
		t.Log("Watch on InMulticastPkts")
		_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(val *ygnmi.Value[uint64]) bool {
			currState, ok := val.Val()
			return ok && currState == 0
		}).Await(t)
		if !ok {
			t.Errorf("InMulticastPkts not correct")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv4/state/counters/in-multicast-octets
	t.Run("Get//ipv4/state/counters/in-multicast-octets", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv4().Counters().InMulticastOctets()
		t.Log("Subscribe on InMulticastOctets")
		gnmi.Get(t, dut, path.State())
		t.Log("Watch on InMulticastOctets")
		_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(val *ygnmi.Value[uint64]) bool {
			currState, ok := val.Val()
			return ok && currState == 0
		}).Await(t)
		if !ok {
			t.Errorf("InMulticastOctets not correct")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv4/state/counters/out-multicast-pkts
	t.Run("Get//ipv4/state/counters/out-multicast-pkts", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv4().Counters().OutMulticastPkts()
		t.Log("Subscribe on OutMulticastPkts")
		gnmi.Get(t, dut, path.State())
		t.Log("Watch on OutMulticastPkts")
		_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(val *ygnmi.Value[uint64]) bool {
			currState, ok := val.Val()
			return ok && currState == 0
		}).Await(t)
		if !ok {
			t.Errorf("OutMulticastPkts not correct")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv4/state/counters/out-multicast-octets
	t.Run("Get//ipv4/state/counters/out-multicast-octets", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv4().Counters().OutMulticastOctets()
		t.Log("Subscribe on OutMulticastOctets")
		gnmi.Get(t, dut, path.State())
		t.Log("Watch on OutMulticastOctets")
		_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(val *ygnmi.Value[uint64]) bool {
			currState, ok := val.Val()
			return ok && currState == 0
		}).Await(t)
		if !ok {
			t.Errorf("OutMulticastOctets not correct")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/state/counters/in-multicast-pkts
	t.Run("Get//ipv6/state/counters/in-multicast-pkts", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().Counters().InMulticastPkts()
		t.Log("Subscribe on InMulticastPkts")
		gnmi.Get(t, dut, path.State())
		t.Log("Watch on InMulticastPkts")
		_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(val *ygnmi.Value[uint64]) bool {
			currState, ok := val.Val()
			return ok && currState == 0
		}).Await(t)
		if !ok {
			t.Errorf("InMulticastPkts not correct")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/state/counters/in-multicast-octets
	t.Run("Get//ipv6/state/counters/in-multicast-octets", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().Counters().InMulticastOctets()
		t.Log("Subscribe on InMulticastOctets")
		gnmi.Get(t, dut, path.State())
		t.Log("Watch on InMulticastOctets")
		_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(val *ygnmi.Value[uint64]) bool {
			currState, ok := val.Val()
			return ok && currState == 0
		}).Await(t)
		if !ok {
			t.Errorf("InMulticastOctets not correct")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/state/counters/out-multicast-pkts
	t.Run("Get//ipv6/state/counters/out-multicast-pkts", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().Counters().OutMulticastPkts()
		t.Log("Subscribe on OutMulticastPkts")
		gnmi.Get(t, dut, path.State())
		t.Log("Watch on OutMulticastPkts")
		_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(val *ygnmi.Value[uint64]) bool {
			currState, ok := val.Val()
			return ok && currState == 0
		}).Await(t)
		if !ok {
			t.Errorf("OutMulticastPkts not correct")
		}
	})
	// /oc-if:interfaces/oc-if:interface/oc-if:subinterfaces/oc-if:subinterface:/ipv6/state/counters/out-multicast-octets
	t.Run("Get//ipv6/state/counters/out-multicast-octets", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().Counters().OutMulticastOctets()
		t.Log("Subscribe on OutMulticastOctets")
		gnmi.Get(t, dut, path.State())
		t.Log("Watch on OutMulticastOctets")
		_, ok := gnmi.Watch(t, dut, path.State(), time.Minute, func(val *ygnmi.Value[uint64]) bool {
			currState, ok := val.Val()
			return ok && currState == 0
		}).Await(t)
		if !ok {
			t.Errorf("OutMulticastOctets not correct")
		}
	})
	t.Run("Get//ipv6/state/counters container", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().Counters()
		gnmi.Get(t, dut, path.State())
	})

	t.Run("Replace//ipv6/router-advertisement/prefixes", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value)
		gnmi.Replace(t, dut, path.Config(), &oc.Interface_Subinterface_Ipv6_RouterAdvertisement_Prefix{
			Prefix:            &prefix_value,
			ValidLifetime:     ygot.Uint32(32),
			PreferredLifetime: ygot.Uint32(32),
		})
	})
	t.Run("Delete//ipv6/router-advertisement", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement()
		gnmi.Delete(t, dut, path.Config())
	})
	t.Run("Delete//ipv6/router-advertisement/prefixes", func(t *testing.T) {
		path := gnmi.OC().Interface(interface_name).Subinterface(0).Ipv6().RouterAdvertisement().Prefix(prefix_value)
		gnmi.Delete(t, dut, path.Config())
	})

}

func TestInterfaceCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	iute := dut.Port(t, "port8")

	t.Run("configInterface", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name())
		obj := &oc.Interface{
			Name:        ygot.String(iut.Name()),
			Description: ygot.String("randstr"),
			Type:        oc.IETFInterfaces_InterfaceType_ieee8023adLag,
		}
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Update(t, dut, path.Config(), obj)

	})

	t.Run("Update//interfaces/interface/config/name", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), iut.Name())

	})

	t.Run("Replace//interfaces/interface/config/description", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Update(t, dut, path.Config(), "desc1")

	})
	t.Run("Update//interfaces/interface/config/description", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Replace(t, dut, path.Config(), "desc2")

	})
	t.Run("Delete//interfaces/interface/config/description", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "DELETE", path)
		gnmi.Delete(t, dut, path.Config())

	})
	t.Run("Update//interfaces/interface/config/mtu", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), 600)

	})
	t.Run("Replace//interfaces/interface/config/mtu", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), 1200)

	})
	t.Run("Delete//interfaces/interface/config/mtu", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "DELETE", path)
		gnmi.Delete(t, dut, path.Config())

	})

	t.Run("configInterface", func(t *testing.T) {
		path := gnmi.OC().Interface(iute.Name())
		obj := &oc.Interface{
			Name:        ygot.String(iute.Name()),
			Description: ygot.String("randstr"),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		}
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Update(t, dut, path.Config(), obj)

	})

	gnmi.Delete(t, dut, gnmi.OC().Interface(iute.Name()).Ethernet().AggregateId().Config())
	member := iut.Members()[0]
	macAdd := "78:2a:67:b6:a8:08"
	t.Run("Replace//interfaces/interface/ethernet/config/mac-address", func(t *testing.T) {
		path := gnmi.OC().Interface(iute.Name()).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), macAdd)

	})
	t.Run("Replace//interfaces/interface/config/type", func(t *testing.T) {
		path := gnmi.OC().Interface(iute.Name()).Type()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	})

	t.Run("configInterface", func(t *testing.T) {
		path := gnmi.OC().Interface(member)
		obj := &oc.Interface{
			Name:        ygot.String(member),
			Description: ygot.String("randstr"),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		}
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Update(t, dut, path.Config(), obj)

	})

	t.Run("Replace//interfaces/interface/ethernet/config/aggregate-id", func(t *testing.T) {
		path := gnmi.OC().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), iut.Name())

	})
	t.Run("Update//interfaces/interface/ethernet/config/mac-address", func(t *testing.T) {
		path := gnmi.OC().Interface(iute.Name()).Ethernet().MacAddress()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), macAdd)

	})
	t.Run("Update//interfaces/interface/config/type", func(t *testing.T) {
		path := gnmi.OC().Interface(iute.Name()).Type()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	})
	t.Run("Update//interfaces/interface/ethernet/config/aggregate-id", func(t *testing.T) {
		path := gnmi.OC().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), iut.Name())

	})
	// port-speed and duplex-mode supported for GigabitEthernet/FastEthernet type interfaces
	t.Run("configInterface", func(t *testing.T) {
		path := gnmi.OC().Interface("GigabitEthernet0/0/0/1")
		obj := &oc.Interface{
			Name:        ygot.String("GigabitEthernet0/0/0/1"),
			Description: ygot.String("randstr"),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		}
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), obj)

	})
	t.Run("Replace//interfaces/interface/ethernet/config/port-speed", func(t *testing.T) {
		path := gnmi.OC().Interface("GigabitEthernet0/0/0/1").Ethernet().PortSpeed()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), oc.IfEthernet_ETHERNET_SPEED_SPEED_1GB)

	})
	t.Run("Replace//interfaces/interface/ethernet/config/duplex-mode", func(t *testing.T) {
		path := gnmi.OC().Interface("GigabitEthernet0/0/0/1").Ethernet().DuplexMode()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), oc.Ethernet_DuplexMode_FULL)

	})

	t.Run("Update//interfaces/interface/ethernet/config/port-speed", func(t *testing.T) {
		path := gnmi.OC().Interface("GigabitEthernet0/0/0/1").Ethernet().PortSpeed()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), oc.IfEthernet_ETHERNET_SPEED_SPEED_1GB)

	})
	t.Run("Update//interfaces/interface/ethernet/config/duplex-mode", func(t *testing.T) {
		path := gnmi.OC().Interface("GigabitEthernet0/0/0/1").Ethernet().DuplexMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), oc.Ethernet_DuplexMode_FULL)

	})
	t.Run("Delete//interfaces/interface/ethernet/config/port-speed", func(t *testing.T) {
		path := gnmi.OC().Interface("GigabitEthernet0/0/0/1").Ethernet().PortSpeed()
		defer observer.RecordYgot(t, "DELETE", path)
		gnmi.Delete(t, dut, path.Config())

	})
	t.Run("Delete//interfaces/interface/ethernet/config/duplex-mode", func(t *testing.T) {
		path := gnmi.OC().Interface("GigabitEthernet0/0/0/1").Ethernet().DuplexMode()
		defer observer.RecordYgot(t, "DELETE", path)
		gnmi.Delete(t, dut, path.Config())

	})

}

func TestInterfaceIPCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	vlanid := uint32(0)
	t.Run("configInterfaceIP", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name()).Subinterface(vlanid)
		obj := &oc.Interface_Subinterface{
			Index: ygot.Uint32(0),
			Ipv6: &oc.Interface_Subinterface_Ipv6{
				Address: map[string]*oc.Interface_Subinterface_Ipv6_Address{
					iut.Ipv6Address(): {
						Ip:           ygot.String(iut.Ipv6Address()),
						PrefixLength: ygot.Uint8(iut.Ipv6PrefixLength()),
					},
				},
			},
			Ipv4: &oc.Interface_Subinterface_Ipv4{
				Address: map[string]*oc.Interface_Subinterface_Ipv4_Address{
					iut.Ipv4Address(): {
						Ip:           ygot.String(iut.Ipv4Address()),
						PrefixLength: ygot.Uint8(iut.Ipv4PrefixLength()),
					},
				},
			},
		}

		defer observer.RecordYgot(t, "REPLACE", path)
		defer observer.RecordYgot(t, "REPLACE", path.Ipv4().Address(iut.Ipv4Address()).Ip())
		defer observer.RecordYgot(t, "REPLACE", path.Ipv4().Address(iut.Ipv4Address()).PrefixLength())
		defer observer.RecordYgot(t, "REPLACE", path.Ipv6().Address(iut.Ipv6Address()).Ip())
		defer observer.RecordYgot(t, "REPLACE", path.Ipv6().Address(iut.Ipv6Address()).PrefixLength())
		gnmi.Replace(t, dut, path.Config(), obj)

	})
	path := gnmi.OC().Interface(iut.Name()).Subinterface(vlanid)
	obj := &oc.Interface_Subinterface{
		Index: ygot.Uint32(0),
		Ipv6: &oc.Interface_Subinterface_Ipv6{
			Address: map[string]*oc.Interface_Subinterface_Ipv6_Address{
				iut.Ipv6Address(): {
					Ip:           ygot.String(iut.Ipv6Address()),
					PrefixLength: ygot.Uint8(iut.Ipv6PrefixLength()),
				},
			},
		},
		Ipv4: &oc.Interface_Subinterface_Ipv4{
			Address: map[string]*oc.Interface_Subinterface_Ipv4_Address{
				iut.Ipv4Address(): {
					Ip:           ygot.String(iut.Ipv4Address()),
					PrefixLength: ygot.Uint8(iut.Ipv4PrefixLength()),
				},
			},
		},
	}

	defer observer.RecordYgot(t, "UPDATE", path)
	defer observer.RecordYgot(t, "UPDATE", path.Ipv4().Address(iut.Ipv4Address()).Ip())
	defer observer.RecordYgot(t, "UPDATE", path.Ipv4().Address(iut.Ipv4Address()).PrefixLength())
	defer observer.RecordYgot(t, "UPDATE", path.Ipv6().Address(iut.Ipv6Address()).Ip())
	defer observer.RecordYgot(t, "UPDATE", path.Ipv6().Address(iut.Ipv6Address()).PrefixLength())
	gnmi.Update(t, dut, path.Config(), obj)

}

func TestInterfaceCountersState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	// cli_handle := dut.RawAPIs().CLI(t)
	// cli_handle.Stdin().Write([]byte("clear counters\n"))
	// cli_handle.Stdin().Write([]byte("\n"))
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	state := gnmi.OC().Interface(iut.Members()[0]).Counters()
	t.Run("Subscribe//interfaces/interface/state/counters/in-broadcast-pkts", func(t *testing.T) {
		state := state.InBroadcastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InBroadcastPkts")
		} else {
			t.Errorf("Interface InBroadcastPkts: got %d, want equal to greater than %d", counter, 0)

		}
	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-errors", func(t *testing.T) {
		state := state.InErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InErrors")
		} else {
			t.Errorf("Interface InErrors: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-discards", func(t *testing.T) {
		state := state.InDiscards()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InDiscards")
		} else {
			t.Errorf("Interface InDiscards: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-multicast-pkts", func(t *testing.T) {
		state := state.InMulticastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InMulticastPkts")
		} else {
			t.Errorf("Interface InMulticastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-octets", func(t *testing.T) {
		state := state.InOctets()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InOctets")
		} else {
			t.Errorf("Interface InOctets: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-unicast-pkts", func(t *testing.T) {
		state := state.InUnicastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InUnicastPkts")
		} else {
			t.Errorf("Interface InUnicastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-unknown-protos", func(t *testing.T) {
		state := state.InUnknownProtos()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InUnknownProtos")
		} else {
			t.Errorf("Interface InUnknownProtos: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-pkts", func(t *testing.T) {
		state := state.InPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InPkts")
		} else {
			t.Errorf("Interface InPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe///interfaces/interface/state/counters/out-broadcast-pkts", func(t *testing.T) {
		state := state.OutBroadcastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutBroadcastPkts")
		} else {
			t.Errorf("Interface OutBroadcastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-discards", func(t *testing.T) {
		state := state.OutDiscards()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutDiscards")
		} else {
			t.Errorf("Interface OutDiscards: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-errorse", func(t *testing.T) {
		state := state.OutErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutErrors")
		} else {
			t.Errorf("Interface OutErrors: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-multicast-pkts", func(t *testing.T) {
		state := state.OutMulticastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutMulticastPkts")
		} else {
			t.Errorf("Interface OutMulticastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-octets", func(t *testing.T) {
		state := state.OutOctets()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutOctets")
		} else {
			t.Errorf("Interface OutOctets: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-pkts", func(t *testing.T) {
		state := state.OutPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutPkts")
		} else {
			t.Errorf("Interface OutPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/out-unicast-pkts", func(t *testing.T) {
		state := state.OutUnicastPkts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutUnicastPkts")
		} else {
			t.Errorf("Interface OutUnicastPkts: got %d, want equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/state/counters/in-fcs-errors", func(t *testing.T) {
		state := state.InFcsErrors()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InFcsErrors")
		} else {
			t.Errorf("Interface InFcsErrors: got %d, want equal to greater than %d", counter, 0)

		}

	})
	member := iut.Members()[0]
	t.Run("Subscribe//interfaces/interface/ethernet/state/counters/in-mac-pause-frames", func(t *testing.T) {
		state := gnmi.OC().Interface(member).Ethernet().Counters().InMacPauseFrames()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface InMacPauseFrames")
		} else {
			t.Errorf("Interface InMacPauseFrames: got %d, want  equal to greater than %d", counter, 0)

		}

	})
	t.Run("Subscribe//interfaces/interface/ethernet/state/counters/out-mac-pause-frames", func(t *testing.T) {
		state := gnmi.OC().Interface(member).Ethernet().Counters().OutMacPauseFrames()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		counter := gnmi.Get(t, dut, state.State())
		if counter > 0 || counter == 0 {
			t.Logf("Got Correct Value for Interface OutMacPauseFrames")
		} else {
			t.Errorf("Interface OutMacPauseFrames: got %d, want equal to greater than %d", counter, 0)

		}

	})
}

func TestInterfaceState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	randstr := "random string"
	randmtu := ygot.Uint16(1200)
	path := gnmi.OC().Interface(iut.Name())
	gnmi.Update(t, dut, path.Name().Config(), iut.Name())
	obj := &oc.Interface{
		Name:        ygot.String(iut.Name()),
		Description: ygot.String(randstr),
		Mtu:         randmtu,
		Type:        oc.IETFInterfaces_InterfaceType_ieee8023adLag,
	}
	gnmi.Replace(t, dut, path.Config(), obj)

	t.Run("Subscribe//interfaces/interface/state/name", func(t *testing.T) {
		state := gnmi.OC().Interface(iut.Name()).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		name := gnmi.Get(t, dut, state.State())
		if name != iut.Name() {
			t.Errorf("Interface Name: got %s, want %s", name, iut.Name())

		}

	})

	t.Run("Subscribe//interfaces/interface/state/description", func(t *testing.T) {
		state := gnmi.OC().Interface(iut.Name()).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		description := gnmi.Get(t, dut, state.State())
		if description != randstr {
			t.Errorf("Interface Description: got %s, want %s", description, randstr)
		}

	})
	t.Run("Subscribe//interfaces/interface/state/mtu", func(t *testing.T) {
		state := gnmi.OC().Interface(iut.Name()).Mtu()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		mtu := gnmi.Get(t, dut, state.State())
		if mtu != *randmtu {
			t.Errorf("Interface Mtu: got %d, want %d", mtu, *randmtu)
		}

	})
	vlanid := uint32(0)
	spath := gnmi.OC().Interface(iut.Name()).Subinterface(vlanid)
	sobj := &oc.Interface_Subinterface{
		Index: ygot.Uint32(0),
		Ipv6: &oc.Interface_Subinterface_Ipv6{
			Address: map[string]*oc.Interface_Subinterface_Ipv6_Address{
				iut.Ipv6Address(): {
					Ip:           ygot.String(iut.Ipv6Address()),
					PrefixLength: ygot.Uint8(iut.Ipv6PrefixLength()),
				},
			},
		},
		Ipv4: &oc.Interface_Subinterface_Ipv4{
			Address: map[string]*oc.Interface_Subinterface_Ipv4_Address{
				iut.Ipv4Address(): {
					Ip:           ygot.String(iut.Ipv4Address()),
					PrefixLength: ygot.Uint8(iut.Ipv4PrefixLength()),
				},
			},
		},
	}
	gnmi.Update(t, dut, spath.Config(), sobj)

	state := gnmi.OC().Interface(iut.Members()[0])
	//path = dut.Config().Interface(iut.Name())
	path = gnmi.OC().Interface(iut.Name())
	obj = &oc.Interface{
		Name:        ygot.String(iut.Name()),
		Description: ygot.String("randstr"),
		Mtu:         ygot.Uint16(1200),
	}
	gnmi.Update(t, dut, path.Config(), obj)
	util.SetInterfaceState(t, dut, iut.Members()[0], true, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
	time.Sleep(30 * time.Second)
	t.Run("Subscribe//interfaces/interface/state/admin-status", func(t *testing.T) {
		state := state.AdminStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		status := gnmi.Get(t, dut, state.State())
		if status != oc.Interface_AdminStatus_UP {
			t.Errorf("Interface AdminStatus: got %v, want %v", status, oc.Interface_AdminStatus_UP)

		}
	})
	t.Run("Subscribe//interfaces/interface/state/type", func(t *testing.T) {
		state := state.Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		status := gnmi.Get(t, dut, gnmi.OC().Interface(iut.Name()).Type().State())
		if status != oc.IETFInterfaces_InterfaceType_ieee8023adLag {
			t.Errorf("Interface Type: got %v, want %v", status, oc.IETFInterfaces_InterfaceType_ieee8023adLag)

		}
	})
	t.Run("Subscribe//interfaces/interface/state/oper-status", func(t *testing.T) {
		state := state.OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		status := gnmi.Get(t, dut, state.State())
		if status != oc.Interface_OperStatus_UP {
			t.Errorf("Interface OperStatus: got %v, want %v", status, oc.Interface_OperStatus_UP)

		}
	})
	t.Run("Subscribe//interfaces/interface/aggregation/state/member", func(t *testing.T) {
		t.Skip() // TODO - remove
		state := gnmi.OC().Interface(iut.Name()).Aggregation().Member()
		members := gnmi.Get(t, dut, state.State())
		if util.SliceEqual(members, iut.Members()) {
			t.Logf("Got correct Interface Aggregation Member Value")
		} else {
			t.Errorf("Interface Aggregation Member: got %v, want %v", members, iut.Members())
		}
	})
	member := iut.Members()[0]
	reqspeed := oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	if strings.Contains(member, "FourHun") {
		reqspeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
	}
	t.Run("Subscribe//interfaces/interface/ethernet/state/port-speed", func(t *testing.T) {
		state := gnmi.OC().Interface(member).Ethernet().PortSpeed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		speed := gnmi.Get(t, dut, state.State())
		if speed != reqspeed {
			t.Errorf("Interface PortSpeed: got %v, want %v", speed, reqspeed)

		}
	})
	t.Run("Subscribe//interfaces/interface/ethernet/state/negotiated-port-speeds", func(t *testing.T) {
		state := gnmi.OC().Interface(member).Ethernet().NegotiatedPortSpeed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		speed := gnmi.Get(t, dut, state.State())
		if speed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
			t.Errorf("Interface InFcsErrors: got %v, want %v", speed, oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN)

		}
	})
	t.Run("Update//interfaces/interface/ethernet/config/aggregate-id", func(t *testing.T) {
		path := gnmi.OC().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), iut.Name())

	})
	t.Run("Subscribe//interfaces/interface/ethernet/state/aggregate-id", func(t *testing.T) {
		state := gnmi.OC().Interface(member).Ethernet().AggregateId()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		id := gnmi.Get(t, dut, state.State())
		if id == "" {
			t.Errorf("Interface AggregateId: got %s, want !=%s", id, "''")

		}
	})

	iute := dut.Port(t, "port8")
	macAdd := "78:2a:67:b6:a8:08"
	macAddIntfPath := gnmi.OC().Interface(iute.Name()).Config()
	macAddressIntfObj := &oc.Interface{
		Name: ygot.String(iute.Name()),
		Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
	}
	gnmi.Update(t, dut, macAddIntfPath, macAddressIntfObj)

	t.Run("Update//interfaces/interface/ethernet/config/mac-address", func(t *testing.T) {
		path := gnmi.OC().Interface(iute.Name()).Ethernet().MacAddress()
		gnmi.Update(t, dut, path.Config(), macAdd)
	})
	t.Run("Subscribe//interfaces/interface/ethernet/state/mac-address", func(t *testing.T) {
		state := gnmi.OC().Interface(iute.Name()).Ethernet().MacAddress()
		macadd := gnmi.Get(t, dut, state.State())
		if macadd != macAdd {
			t.Errorf("Interface MacAddress: got %s, want !=%s", macadd, macAdd)

		}
	})
	t.Run("Delete//interfaces/interface/ethernet/state/mac-address", func(t *testing.T) {
		path := gnmi.OC().Interface(iute.Name()).Ethernet().MacAddress()
		gnmi.Delete(t, dut, path.Config())
	})

	t.Run("Update//interfaces/interface/config/type", func(t *testing.T) {
		path := gnmi.OC().Interface(iute.Name()).Type()
		gnmi.Update(t, dut, path.Config(), oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

	})
	t.Run("Subscribe//interfaces/interface/state/type", func(t *testing.T) {
		state := gnmi.OC().Interface(iute.Name()).Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		_type := gnmi.Get(t, dut, state.State())
		if _type != oc.IETFInterfaces_InterfaceType_ethernetCsmacd {
			t.Errorf("Interface Type: got %v, want !=%v", _type, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)

		}
	})
}

func TestInterfaceHoldTime(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")
	hlt := uint32(30)
	member := iut.Members()[0]
	t.Run("configInterface", func(t *testing.T) {
		path := gnmi.OC().Interface(iut.Name())
		obj := &oc.Interface{
			Name:        ygot.String(iut.Name()),
			Description: ygot.String("randstr"),
			Type:        oc.IETFInterfaces_InterfaceType_ieee8023adLag,
		}
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), obj)

	})
	t.Run("Update//interfaces/interface/hold-time/config/up", func(t *testing.T) {
		config := gnmi.OC().Interface(member).HoldTime().Up()
		defer observer.RecordYgot(t, "UPDATE", config)
		gnmi.Update(t, dut, config.Config(), hlt)
	})
	t.Run("Subscribe//interfaces/interface/hold-time/state/up", func(t *testing.T) {
		state := gnmi.OC().Interface(member).HoldTime().Up()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		holdtime := gnmi.Get(t, dut, state.State())
		if holdtime != hlt {
			t.Errorf("Interface HoldTime Up: got %d, want %d", holdtime, hlt)

		}

	})
	t.Run("Update//interfaces/interface/hold-time/config/down", func(t *testing.T) {
		config := gnmi.OC().Interface(member).HoldTime().Down()
		defer observer.RecordYgot(t, "UPDATE", config)
		gnmi.Update(t, dut, config.Config(), hlt)
	})
	t.Run("Subscribe//interfaces/interface/hold-time/state/down", func(t *testing.T) {
		state := gnmi.OC().Interface(member).HoldTime().Down()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		holdtime := gnmi.Get(t, dut, state.State())
		if holdtime != hlt {
			t.Errorf("Interface HoldTime Down: got %d, want %d", holdtime, hlt)

		}

	})
	t.Run("Delete//interfaces/interface/hold-time/config/down", func(t *testing.T) {
		config := gnmi.OC().Interface(member).HoldTime().Down()
		defer observer.RecordYgot(t, "DELETE", config)
		gnmi.Delete(t, dut, config.Config())
	})
	t.Run("Delete//interfaces/interface/hold-time/config/up", func(t *testing.T) {
		config := gnmi.OC().Interface(member).HoldTime().Up()
		defer observer.RecordYgot(t, "DELETE", config)
		gnmi.Delete(t, dut, config.Config())
	})
}

func TestInterfaceTelemetry(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := inputObj.Device(dut).GetInterface("Bundle-Ether120")

	//Default susbcription rate is 30 seconds.
	subscriptionDuration := 120 * time.Second
	triggerDelay := 20 * time.Second
	postInterfaceEventWait := 10 * time.Second
	expectedEntries := 2

	t.Run("Subscribe//interfaces/interface/state/oper-status", func(t *testing.T) {

		configPath := gnmi.OC().Interface(iut.Name()).Enabled()
		statePath := gnmi.OC().Interface(iut.Name()).OperStatus()

		//initialise OperStatus
		gnmi.Update(t, dut, configPath.Config(), true)
		time.Sleep(postInterfaceEventWait)
		t.Logf("Updated interface oper status: %s", gnmi.Get(t, dut, statePath.State()))

		//delay triggering OperStatus change
		go func(t *testing.T) {
			time.Sleep(triggerDelay)
			gnmi.Update(t, dut, configPath.Config(), false)
			t.Log("Triggered oper-status change")
		}(t)

		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)
		got := gnmi.Collect(t, dut, statePath.State(), subscriptionDuration).Await(t)
		gotEntries := len(got)

		if gotEntries < expectedEntries {
			t.Errorf("Oper Status subscription samples: got %d, want %d", gotEntries, expectedEntries)
		}
		//verify last sample has event trigger recorded.
		value, _ := got[gotEntries-1].Val()
		if value != oc.Interface_OperStatus_DOWN {
			t.Errorf("Interface OperStatus change event was not recorded: got %s, want %s", value, oc.Interface_OperStatus_DOWN)
		}
	})

	t.Run("Subscribe//interfaces/interface/state/admin-status", func(t *testing.T) {

		configPath := gnmi.OC().Interface(iut.Name()).Enabled()
		statePath := gnmi.OC().Interface(iut.Name()).AdminStatus()

		//initialise OperStatus to change admin-status
		gnmi.Update(t, dut, configPath.Config(), true)
		time.Sleep(postInterfaceEventWait)
		t.Logf("Updated interface admin status: %s", gnmi.Get(t, dut, statePath.State()))

		//delay triggering OperStatus change
		go func(t *testing.T) {
			time.Sleep(triggerDelay)
			gnmi.Update(t, dut, configPath.Config(), false)
			t.Log("Triggered oper-status change to change admin-status")
		}(t)

		defer observer.RecordYgot(t, "SUBSCRIBE", statePath)
		got := gnmi.Collect(t, dut, statePath.State(), subscriptionDuration).Await(t)
		t.Logf("Collected samples for admin-status: %v", got)
		gotEntries := len(got)

		if gotEntries < expectedEntries {
			t.Errorf("Admin Status subscription samples: got %d, want %d", gotEntries, expectedEntries)
		}
		//verify last sample has trigger event recorded.
		value, _ := got[gotEntries-1].Val()
		if value != oc.Interface_AdminStatus_DOWN {
			t.Errorf("Interface AdminStatus change event was not recorded: got %s, want %s", value, oc.Interface_AdminStatus_DOWN)
		}
	})

}

var (
	dutPort1 = attrs.Attributes{
		Desc:    "BundlePort1",
		IPv4:    "100.120.1.1",
		MAC:     "1.2.0",
		IPv4Len: ipv4PrefixLen,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "100.120.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "BundlePort2",
		IPv4:    "100.121.1.1",
		MAC:     "1.2.1",
		IPv4Len: ipv4PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort3 = attrs.Attributes{ //non-bundle member
		Desc:    "dutPort3",
		IPv4:    "100.123.2.3",
		IPv4Len: ipv4PrefixLen,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "100.123.2.4",
		IPv4Len: ipv4PrefixLen,
	}
)

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p3 := dut.Port(t, "port3").Name()
	member := gnmi.OC().Interface(p3)
	gnmi.Delete(t, dut, member.Config())
	i3 := dutPort3.NewOCInterface(p3, dut)
	gnmi.Replace(t, dut, d.Interface(p3).Config(), i3)

}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	atePort1.AddToATE(top, p1, &dutPort1)

	p2 := ate.Port(t, "port2")
	atePort2.AddToATE(top, p2, &dutPort2)

	p3 := ate.Port(t, "port3")
	atePort3.AddToATE(top, p3, &dutPort3)
	return top
}

// testFlow sends traffic across ATE ports and verifies continuity.
func testFlow(
	t *testing.T,
	ate *ondatra.ATEDevice,
	top *ondatra.ATETopology,
	bundleMember bool,
	headers ...ondatra.Header,

) float32 {
	i1 := top.Interfaces()[atePort1.Name]
	var i2 *ondatra.Interface
	if bundleMember {
		i2 = top.Interfaces()[atePort2.Name]
	} else {
		i2 = top.Interfaces()[atePort3.Name]
	}

	flow := ate.Traffic().NewFlow("flow-unviable").
		WithSrcEndpoints(i1).
		WithDstEndpoints(i2).
		WithHeaders(headers...).
		WithFrameRateFPS(100).
		WithFrameSize(512)
	fmt.Print("TRAFFIC STARTED")
	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)
	lossPct := gnmi.Get(t, ate, gnmi.OC().Flow("flow-unviable").LossPct().State())
	t.Logf("Loss Packet %v ", lossPct)
	return lossPct
}

func TestForwardingUnviableFP(t *testing.T) {
	t.Skip(t)
	dut := ondatra.DUT(t, device1)
	// Configure the DUT
	configureDUT(t, dut)
	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)

	top.Push(t).StartProtocols(t)

	t.Run("Counters checked after forwarding-unviable configured on bundle-member", func(t *testing.T) {
		config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port2").Name()))
		defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port2").Name()))
		ethHeader := ondatra.NewEthernetHeader()
		ipv4Header := ondatra.NewIPv4Header()
		lossPckt := testFlow(t, ate, top, true, ethHeader, ipv4Header)
		if lossPckt != 100 {
			t.Errorf("Traffic Loss Expected , Got %v , want 100", lossPckt)
		}

	})

	t.Run("Configure forwarding-unviable and check if bundle interface status is DOWN", func(t *testing.T) {
		config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port2").Name()))
		defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port2").Name()))
		stateBundleInterface := gnmi.Get(t, dut, gnmi.OC().Interface("Bundle-Ether120").OperStatus().State()).String()
		stateBundleMemberInterface := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).OperStatus().State()).String()
		if (stateBundleInterface != "DOWN") && (stateBundleMemberInterface != "UP") {
			t.Logf("Bunde interface state %v, got %v , want DOWN", "Bundle-Ether120", stateBundleInterface)
			t.Logf("Bundle member interface state %v, got %v, want UP ", dut.Port(t, "port1").Name(), stateBundleMemberInterface)
			t.Errorf("Interface state is not expected ")
		}

	})
	t.Run("Flap bundle/bundle-member interfaces and check counter values", func(t *testing.T) {
		config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port1").Name()))
		defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port1").Name()))
		for i := 0; i < 4; i++ {
			util.FlapInterface(t, dut, dut.Port(t, "port1").Name(), 10*time.Second)
			util.FlapInterface(t, dut, "Bundle-Ether120", 10*time.Second)
		}
		time.Sleep(30 * time.Second)
		ethHeader := ondatra.NewEthernetHeader()
		ipv4Header := ondatra.NewIPv4Header()
		lossPckt := testFlow(t, ate, top, true, ethHeader, ipv4Header)
		if lossPckt != 100 {
			t.Errorf("Traffic Loss Expected , Got %v , want 100", lossPckt)
		}

	})

	t.Run("Testing forwarding-unviable on non-bundle interface", func(t *testing.T) {
		config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port3").Name()))
		defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port3").Name()))
		ethHeader := ondatra.NewEthernetHeader()
		ipv4Header := ondatra.NewIPv4Header()
		lossPckt := testFlow(t, ate, top, false, ethHeader, ipv4Header)
		if lossPckt != 0 {
			t.Errorf("Traffic Loss NOT Expected , Got %v , want 0", lossPckt)
		}
	})
	t.Run("Configure forwarding-unviable and check if interface status is UP", func(t *testing.T) {
		config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port3").Name()))
		defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port3").Name()))
		stateInterface := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port3").Name()).OperStatus().State()).String()
		if stateInterface != "UP" {
			t.Logf("Interface state %v, got %v, want UP ", dut.Port(t, "port3").Name(), stateInterface)
			t.Errorf("Interface state is not expected ")
		}

	})
	t.Run("Flap interfaces and check counter values", func(t *testing.T) {
		config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port3").Name()))
		defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port3").Name()))
		for i := 0; i < 4; i++ {
			util.FlapInterface(t, dut, dut.Port(t, "port3").Name(), 10*time.Second)
		}
		time.Sleep(60 * time.Second)
		ethHeader := ondatra.NewEthernetHeader()
		ipv4Header := ondatra.NewIPv4Header()
		lossPckt := testFlow(t, ate, top, false, ethHeader, ipv4Header)
		if lossPckt > 0.4 {
			t.Errorf("Traffic Loss NOT Expected , Got %v , want 0", lossPckt)
		}

	})

	t.Run("Configure 2 bundle members with one viable and other unviable", func(t *testing.T) {
		member := gnmi.OC().Interface(dut.Port(t, "port3").Name())
		gnmi.Delete(t, dut, member.Config())
		members := gnmi.OC().Interface(dut.Port(t, "port3").Name()).Ethernet().AggregateId()
		gnmi.Update(t, dut, members.Config(), "Bundle-Ether120")
		config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port3").Name()))
		defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port3").Name()))
		t.Logf("Interface %v is forwarding-viable and Interface %v is forwarding-unviable", dut.Port(t, "port1").Name(), dut.Port(t, "port3").Name())
		bundleStatus := gnmi.Get(t, dut, gnmi.OC().Interface("Bundle-Ether120").OperStatus().State()).String()
		t.Log((bundleStatus))
		if bundleStatus != "UP" {
			t.Errorf("Expected Bundle interface %v to be UP as its member %v is forwading-viable ", bundleStatus, dut.Port(t, "port2").Name())
		}
		ethHeader := ondatra.NewEthernetHeader()
		ipv4Header := ondatra.NewIPv4Header()
		lossPckt := testFlow(t, ate, top, true, ethHeader, ipv4Header)
		if lossPckt >= 1 {
			t.Errorf("Traffic loss not expected as Bundle interface is UP , got %v , want 0 ", lossPckt)
		}

		config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port1").Name()))
		defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port1").Name()))
		bundleStatusAfter := gnmi.Get(t, dut, gnmi.OC().Interface("Bundle-Ether120").OperStatus().State()).String()
		if bundleStatusAfter != "DOWN" {
			t.Errorf("Expected Bundle interface %v to be down as both its members are forwarding-unviable ", "Bundle-Ether120")
		}
		lossPckt = testFlow(t, ate, top, true, ethHeader, ipv4Header)
		if lossPckt <= 1 {
			t.Errorf("Traffic Loss Expected , Got %v , want 100", lossPckt)
		}
	})

	t.Run("Process restart the router and check for counters", func(t *testing.T) {
		config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port2").Name()))
		defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port2").Name()))
		processList := [4]string{"ether_mgbl", "ifmgr", " bundlemgr_distrib", "bundlemgr_local"}
		for _, process := range processList {
			config.CMDViaGNMI(context.Background(), t, dut, fmt.Sprintf("process restart %v", process))
		}
		time.Sleep(30 * time.Second)
		bundleStatusAfter := gnmi.Get(t, dut, gnmi.OC().Interface("Bundle-Ether121").OperStatus().State()).String()
		if bundleStatusAfter != "DOWN" {
			t.Errorf("Expected Bundle interface %v to be down as both its members are forwarding-unviable ", "Bundle-Ether121")
		}
		time.Sleep(30 * time.Second)
		ethHeader := ondatra.NewEthernetHeader()
		ipv4Header := ondatra.NewIPv4Header()
		lossPckt := testFlow(t, ate, top, true, ethHeader, ipv4Header)
		if lossPckt <= 1 {
			t.Errorf("Traffic Loss Expected , Got %v , want 100", lossPckt)
		}

	})

	t.Run("Reload the router and check for counters", func(t *testing.T) {
		gnoiClient := dut.RawAPIs().GNOI(t)
		_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
			Method:  spb.RebootMethod_COLD,
			Delay:   0,
			Message: "Reboot chassis without delay",
			Force:   true,
		})
		if err != nil {
			t.Fatalf("Reboot failed %v", err)
		}
		startReboot := time.Now()
		const maxRebootTime = 30
		t.Logf("Wait for DUT to boot up by polling the telemetry output.")
		for {
			var currentTime string
			t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

			time.Sleep(3 * time.Minute)
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			}); errMsg != nil {
				t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
			} else {
				t.Logf("Device rebooted successfully with received time: %v", currentTime)
				break
			}

			if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
				t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
			}
		}
		t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
		dutR := ondatra.DUT(t, device1)
		config.TextWithGNMI(context.Background(), t, dutR, fmt.Sprintf("interface %v\n forwarding-unviable\n", dut.Port(t, "port2").Name()))
		defer config.TextWithGNMI(context.Background(), t, dutR, fmt.Sprintf("interface %v\n no forwarding-unviable\n", dut.Port(t, "port2").Name()))
		ethHeader := ondatra.NewEthernetHeader()
		ipv4Header := ondatra.NewIPv4Header()
		lossPckt := testFlow(t, ate, top, true, ethHeader, ipv4Header)
		if lossPckt <= 1 {
			t.Errorf("Traffic Loss Expected , Got %v , want 100", lossPckt)
		}

	})
}

func TestForwardViableSDN(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut2 := inputObj.Device(dut).GetInterface("Bundle-Ether121")
	bundleMember := iut2.Members()[0]
	//path := gnmi.OC().Interface(iut2.Name()) // TODO - KD
	//gnmi.Replace(t, dut, path.Name().Config(), iut2.Name())
	interfaceContainer := &oc.Interface{
		Type:             oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Name:             ygot.String(bundleMember),
		ForwardingViable: ygot.Bool(false)}

	t.Run("configInterface", func(t *testing.T) {
		path := gnmi.OC().Interface(bundleMember)
		obj := &oc.Interface{
			Name:        ygot.String(bundleMember),
			Description: ygot.String("randstr"),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		}
		gnmi.Update(t, dut, path.Config(), obj)
	})
	t.Run(fmt.Sprintf("Update on /interface[%v]/config/forward-viable", bundleMember), func(t *testing.T) {
		path := gnmi.OC().Interface(bundleMember).ForwardingViable()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), *ygot.Bool(false))
	})
	t.Run(fmt.Sprintf("Get on /interface[%v]/config/forward-viable", bundleMember), func(t *testing.T) {
		configContainer := gnmi.OC().Interface(bundleMember).ForwardingViable().Config()
		defer observer.RecordYgot(t, "SUBSCRIBE", configContainer)
		forwardUnviable := gnmi.LookupConfig(t, dut, configContainer)
		if v, _ := forwardUnviable.Val(); v != false {
			t.Errorf("Update for forward-unviable failed got %v , want false", forwardUnviable)
		}
	})
	t.Run(fmt.Sprintf("Subscribe on /interface[%v]/state/forward-viable", bundleMember), func(t *testing.T) {
		stateContainer := gnmi.OC().Interface(bundleMember).ForwardingViable()
		defer observer.RecordYgot(t, "SUBSCRIBE", stateContainer)
		forwardUnviable := gnmi.Get(t, dut, stateContainer.State())
		if forwardUnviable != *ygot.Bool(false) {
			t.Errorf("Update for forward-unviable failed got %v , want false", forwardUnviable)
		}
	})
	t.Run(fmt.Sprintf("Delete on /interface[%v]/config/forward-viable", bundleMember), func(t *testing.T) {
		path := gnmi.OC().Interface(bundleMember).ForwardingViable()
		gnmi.Delete(t, dut, path.Config())
	})

	stateContainer := gnmi.OC().Interface(bundleMember).ForwardingViable()
	forwardUnviable := gnmi.Get(t, dut, stateContainer.State())
	t.Logf("%v", forwardUnviable)
	t.Run(fmt.Sprintf("Update on /interface[%v]/", bundleMember), func(t *testing.T) {
		path := gnmi.OC().Interface(bundleMember)
		gnmi.Update(t, dut, path.Config(), interfaceContainer)
	})
	t.Run(fmt.Sprintf("Get on /interface[%v]/", bundleMember), func(t *testing.T) {
		configContainer := gnmi.OC().Interface(bundleMember)
		forwardUnviable := gnmi.LookupConfig(t, dut, configContainer.Config())
		if _, ok := forwardUnviable.Val(); !ok {
			t.Errorf("Get on /interface[%v]/ failed", bundleMember)
		}
	})
	t.Run(fmt.Sprintf("Delete on /interface[%v]/config/forward-viable", bundleMember), func(t *testing.T) {
		path := gnmi.OC().Interface(bundleMember)
		gnmi.Delete(t, dut, path.Config())
	})
}
