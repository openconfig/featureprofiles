package helper

import (
	"fmt"
	"testing"

	textfsm "github.com/openconfig/featureprofiles/exec/utils/textfsm/textfsm"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	// "github.com/openconfig/ondatra/gnmi/oc"
)

type fibHelper struct{}

type AFTNHInfo struct {
	NextHopIndex     uint64
	NextHopWeight    uint64
	NextHopIP        string
	NextHopInterface string
}
type FIBAFTObject struct {
	Prefix       string
	NextHopGroup []uint64
	NextHop      []AFTNHInfo
}

type CEFNHInfo struct {
	NextHopAddress   string
	NextHopInterface string
	NextHopWeight    float64
}

type FIBCEFObject struct {
	Prefix string
	VRF    string
	CEFNH  []CEFNHInfo
}

// GetPrefixAFTNHG retrieves all outgoing NHG(next hop group) for a given prefix , with afiType string "ipv4" or "ipv6".
func (v *fibHelper) GetPrefixAFTNHG(t testing.TB, dut *ondatra.DUTDevice, prefix, vrf, afiType string) []uint64 {
	var NHG []uint64
	switch afiType {
	case "ipv4":
		aftIPv4Path := gnmi.OC().NetworkInstance(vrf).Afts().Ipv4Entry(prefix).State()
		aftGet := gnmi.Get(t, dut, aftIPv4Path)
		NHG = []uint64{aftGet.GetNextHopGroup()}
	case "ipv6":
		aftIPv6Path := gnmi.OC().NetworkInstance(vrf).Afts().Ipv6Entry(prefix).State()
		aftGet := gnmi.Get(t, dut, aftIPv6Path)
		NHG = []uint64{aftGet.GetNextHopGroup()}
	}
	return NHG
}

// GetPrefixAFTNH returns a map of NH index and corresponding weight for a given NHG.
func (v *fibHelper) GetPrefixAFTNHIndex(t testing.TB, dut *ondatra.DUTDevice, NHG uint64, vrf string) map[uint64]uint64 {
	nhMap := make(map[uint64]uint64)
	aftNHG := gnmi.OC().NetworkInstance(vrf).Afts().NextHopGroup(NHG).State()
	aftGet := gnmi.Get(t, dut, aftNHG)
	for i := range aftGet.NextHop {
		aftNH := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().NextHop(i).State())
		index := aftNH.GetIndex()
		weight := *aftGet.GetNextHop(i).Weight
		nhMap[index] = weight
	}
	return nhMap
}

// GetAFTNHIPAddr retrieves next-hop IP for a given NHIndex list.
func (v *fibHelper) GetAFTNHIPAddr(t testing.TB, dut *ondatra.DUTDevice, nhIndex []uint64, vrf string) []string {
	var nhIP []string
	for _, nhI := range nhIndex {
		aftNH := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().NextHop(nhI).State()
		aftGet := gnmi.Get(t, dut, aftNH)
		ipAddr := aftGet.GetIpAddress()
		nhIP = append(nhIP, ipAddr)
	}
	return nhIP
}

// GetAFTNHInterface retrieves next-hop Interface for a given NHIndex list.
func (v *fibHelper) GetAFTNHInterface(t testing.TB, dut *ondatra.DUTDevice, nhIndex []uint64, vrf string) []string {
	var nhInterface []string
	for _, nhI := range nhIndex {
		aftNH := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().NextHop(nhI).State()
		aftGet := gnmi.Get(t, dut, aftNH)
		intf := aftGet.GetInterfaceRef().GetInterface()
		nhInterface = append(nhInterface, intf)
	}
	return nhInterface
}

func (v *fibHelper) GetPrefixAFTObjects(t testing.TB, dut *ondatra.DUTDevice, prefix, vrf, afiType string) FIBAFTObject {
	aftObj := FIBAFTObject{}
	NHInfo := AFTNHInfo{}
	aftObj.Prefix = prefix
	aftObj.NextHopGroup = v.GetPrefixAFTNHG(t, dut, prefix, vrf, afiType)
	for nhI := range v.GetPrefixAFTNHIndex(t, dut, aftObj.NextHopGroup[0], vrf) {
		NHInfo.NextHopIndex = nhI
		NHInfo.NextHopIP = v.GetAFTNHIPAddr(t, dut, []uint64{NHInfo.NextHopIndex}, vrf)[0]
		NHInfo.NextHopWeight = v.GetPrefixAFTNHIndex(t, dut, aftObj.NextHopGroup[0], vrf)[NHInfo.NextHopIndex]
		aftObj.NextHop = append(aftObj.NextHop, NHInfo)
	}

	var nhPfxLength string
	if afiType == "ipv4" {
		nhPfxLength = "/32"
	} else {
		nhPfxLength = "/128"
	}
	for i, NH := range aftObj.NextHop {
		pathNHG := v.GetPrefixAFTNHG(t, dut, NH.NextHopIP+nhPfxLength, deviations.DefaultNetworkInstance(dut), afiType)
		pathNHI := v.GetPrefixAFTNHIndex(t, dut, pathNHG[0], deviations.DefaultNetworkInstance(dut))
		for nhI := range pathNHI {
			pathIntf := v.GetAFTNHInterface(t, dut, []uint64{nhI}, deviations.DefaultNetworkInstance(dut))
			aftObj.NextHop[i].NextHopInterface = pathIntf[0]
		}
	}
	return aftObj
}

func (v *fibHelper) GetPrefixCEFObjects(t testing.TB, dut *ondatra.DUTDevice, prefix, vrf, afiType string) FIBCEFObject {
	cefObj := FIBCEFObject{}
	NHInfo := CEFNHInfo{}
	cefObj.Prefix = prefix
	cefObj.VRF = vrf
	cliOutput := dut.CLI().Run(t, fmt.Sprintf("show cef vrf %s %s detail", vrf, prefix))
	//Build cef textfsm struct.
	var nhAddList []string
	ipCount := make(map[string]int)
	cefVrfTextfsm := textfsm.ShowCefVrfDetail{}
	if err := cefVrfTextfsm.Parse(cliOutput); err != nil {
		t.Fatalf("%v", err)
	}
	for _, cefCliVal := range cefVrfTextfsm.Rows {
		nhAddList = cefCliVal.GetAddressVal()
		for i, ip := range nhAddList {
			ipCount[ip]++
			if len(cefCliVal.GetInterfaceValue()) > 0 {
				NHInfo.NextHopInterface = cefCliVal.GetInterfaceValue()[i]
			}
		}

	}
	uniqueNH := make(map[string]bool)
	for _, nh := range nhAddList {
		if uniqueNH[nh] { // Skip if already exist
			continue

		}
		uniqueNH[nh] = true
		total := float64(len(nhAddList))
		wt := float64(ipCount[nh]) / total
		NHInfo.NextHopAddress = nh
		NHInfo.NextHopWeight = wt
		cefObj.CEFNH = append(cefObj.CEFNH, NHInfo)
	}
	return cefObj
}
