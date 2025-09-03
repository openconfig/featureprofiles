package helper

import (
	"testing"

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

// GetPrefixAFTNHG retrieves all outgoing NHG(next hop group) for a given prefix.
func (v *fibHelper) GetPrefixAFTNHG(t testing.TB, dut *ondatra.DUTDevice, prefix, vrf string) []uint64 {
	t.Helper()
	var NHG []uint64
	aftIPv4Path := gnmi.OC().NetworkInstance(vrf).Afts().Ipv4Entry(prefix).State()
	aftGet := gnmi.Get(t, dut, aftIPv4Path)
	NHG = []uint64{aftGet.GetNextHopGroup()}
	return NHG
}

// GetPrefixAFTNH returns a map of NH index and corresponding weight for a given NHG.
func (v *fibHelper) GetPrefixAFTNHIndex(t testing.TB, dut *ondatra.DUTDevice, NHG uint64, vrf string) map[uint64]uint64 {
	t.Helper()
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
	t.Helper()
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
	t.Helper()
	var nhInterface []string
	for _, nhI := range nhIndex {
		aftNH := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().NextHop(nhI).State()
		aftGet := gnmi.Get(t, dut, aftNH)
		intf := aftGet.GetInterfaceRef().GetInterface()
		nhInterface = append(nhInterface, intf)
	}
	return nhInterface
}

func (v *fibHelper) GetPrefixAFTObjects(t testing.TB, dut *ondatra.DUTDevice, prefix, vrf string) FIBAFTObject {
	t.Helper()
	aftObj := FIBAFTObject{}
	NHInfo := AFTNHInfo{}
	aftObj.Prefix = prefix
	aftObj.NextHopGroup = v.GetPrefixAFTNHG(t, dut, prefix, vrf)
	for nhI := range v.GetPrefixAFTNHIndex(t, dut, aftObj.NextHopGroup[0], vrf) {
		NHInfo.NextHopIndex = nhI
		NHInfo.NextHopIP = v.GetAFTNHIPAddr(t, dut, []uint64{NHInfo.NextHopIndex}, vrf)[0]
		NHInfo.NextHopWeight = v.GetPrefixAFTNHIndex(t, dut, aftObj.NextHopGroup[0], vrf)[NHInfo.NextHopIndex]
		aftObj.NextHop = append(aftObj.NextHop, NHInfo)
	}

	for i, NH := range aftObj.NextHop {
		pathNHG := v.GetPrefixAFTNHG(t, dut, NH.NextHopIP+"/32", deviations.DefaultNetworkInstance(dut))
		pathNHI := v.GetPrefixAFTNHIndex(t, dut, pathNHG[0], deviations.DefaultNetworkInstance(dut))
		for nhI := range pathNHI {
			pathIntf := v.GetAFTNHInterface(t, dut, []uint64{nhI}, deviations.DefaultNetworkInstance(dut))
			aftObj.NextHop[i].NextHopInterface = pathIntf[0]
		}
	}
	return aftObj
}
