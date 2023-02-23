package feature

import (
	"fmt"
	"math"
	"math/bits"
	"net"
	"strings"
	"testing"

	"github.com/pkg/errors"

	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"
	"github.com/openconfig/featureprofiles/tools/inputcisco/solver"
	"github.com/openconfig/ondatra"

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// ConfigInterfaces configures interfaces as given in input file
func ConfigInterfaces(dev *ondatra.DUTDevice, t *testing.T, intf *proto.Input_Interface) error {
	if strings.HasPrefix(strings.ToLower(intf.Name), "bundle") {
		for _, intfname := range intf.Members {
			createInterface(t, dev, intf, oc.IETFInterfaces_InterfaceType_ieee8023adLag)
			createBundleInterface(t, dev, solver.Solver(t, dev, intfname), intf.Name, intf)
		}
	} else if strings.HasPrefix(strings.ToLower(intf.Name), "loopback") {
		createInterface(t, dev, intf, oc.IETFInterfaces_InterfaceType_softwareLoopback)
	} else {
		createInterface(t, dev, intf, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
	}
	vlanid := uint32(1)
	encapid := uint16(1)
	if len(intf.Vlan) > 0 {
		for _, vlan := range intf.Vlan {
			createVlan(t, dev, intf, vlan, &vlanid, &encapid)
		}
	}
	return nil
}

// UnConfigInterfaces removes interfaces as given in input file
func UnConfigInterfaces(dev *ondatra.DUTDevice, t *testing.T, intf *proto.Input_Interface) error {
	if strings.HasPrefix(strings.ToLower(intf.Name), "bundle") {
		for _, intfname := range intf.Members {
			deleteInterface(t, dev, intf, oc.IETFInterfaces_InterfaceType_ieee8023adLag)
			deleteBundleInterface(t, dev, solver.Solver(t, dev, intfname), intf.Name, intf)
		}
	} else if strings.HasPrefix(strings.ToLower(intf.Name), "loopback") {
		deleteInterface(t, dev, intf, oc.IETFInterfaces_InterfaceType_softwareLoopback)
	} else {
		deleteInterface(t, dev, intf, oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
	}
	return nil
}

func createInterface(t *testing.T, dut *ondatra.DUTDevice, intf *proto.Input_Interface, intftype oc.E_IETFInterfaces_InterfaceType) {
	var intfname string
	if intf.Name != "" {
		intfname = intf.Name
	} else {
		intfname = solver.Solver(t, dut, intf.Id)
		intf.Name = intfname
	}
	config := buildInterface(intf, intftype)
	gnmi.Update(t, dut, gnmi.OC().Interface(intfname).Config(), config)
}
func deleteInterface(t *testing.T, dut *ondatra.DUTDevice, intf *proto.Input_Interface, intftype oc.E_IETFInterfaces_InterfaceType) {
	var intfname string
	if intf.Name != "" {
		intfname = intf.Name
	} else {
		intfname = solver.Solver(t, dut, intf.Id)
		intf.Name = intfname
	}
	gnmi.Delete(t, dut, gnmi.OC().Interface(intfname).Config())
}

func buildInterface(intf *proto.Input_Interface, intftype oc.E_IETFInterfaces_InterfaceType) *oc.Interface {
	model := oc.Interface{
		//
		Name:    &intf.Name,
		Enabled: ygot.Bool(!intf.Disabled),
		Type:    intftype,
	}
	if intf.Disabled {
		model.Enabled = ygot.Bool(false)
	} else {
		model.Enabled = ygot.Bool(true)
	}
	if intf.Mtu != 0 {
		model.Mtu = ygot.Uint16(uint16(intf.Mtu))
	}
	model.Subinterface = map[uint32]*oc.Interface_Subinterface{}
	model.Subinterface[0] = &oc.Interface_Subinterface{
		Index: ygot.Uint32(0),
	}
	if intf.Ipv4Address != "" && intf.Ipv4PrefixLength != 0 {
		model.Subinterface[0].Ipv4 = getSubintfIpv4(&intf.Ipv4Address, intf.Ipv4PrefixLength)
	}
	if intf.Ipv6Address != "" && intf.Ipv4PrefixLength != 0 {
		model.Subinterface[0].Ipv6 = getSubintfIpv6(&intf.Ipv6Address, intf.Ipv6PrefixLength)
	}
	return &model
}

func createBundleInterface(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, bundleName string, intf *proto.Input_Interface) {

	member := &oc.Interface{
		Enabled: ygot.Bool(true),
		Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Name:    ygot.String(interfaceName),
		Ethernet: &oc.Interface_Ethernet{
			AggregateId: ygot.String(bundleName),
		},
	}
	updateResponse := gnmi.Update(t, dut, gnmi.OC().Interface(interfaceName).Config(), member)
	t.Logf("Update response : %v", updateResponse)
}

func deleteBundleInterface(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, bundleName string, intf *proto.Input_Interface) {
	updateResponse := gnmi.Delete(t, dut, gnmi.OC().Interface(interfaceName).Config())
	t.Logf("Update response : %v", updateResponse)
}

func createVlan(t *testing.T, dut *ondatra.DUTDevice, intf *proto.Input_Interface, vlan *proto.Input_Vlan, vlanid *uint32, encapid *uint16) {
	var intfname string
	if intf.Name != "" {
		intfname = intf.Name
	} else {
		intfname = solver.Solver(t, dut, intf.Id)
	}
	config := getSubintfConfig(t, dut, intf, vlan, vlanid, encapid)
	gnmi.Update(t, dut, gnmi.OC().Interface(intfname).Config(), config)

}

func getSubintfConfig(t *testing.T, dut *ondatra.DUTDevice, intf *proto.Input_Interface, vlan *proto.Input_Vlan, vlanid *uint32, encapid *uint16) *oc.Interface {
	if vlan.Scale == 0 {
		vlan.Scale = 1
	}
	v4dec, v6dec := &uint128{}, &uint128{}
	v4ip, v6ip := &vlan.Ipv4Address, &vlan.Ipv6Address
	var err error
	config := &oc.Interface{
		// Name: ygot.String(intfname),
	}
	config.Subinterface = map[uint32]*oc.Interface_Subinterface{}
	_v4ip, v4net, v4err := net.ParseCIDR(strings.Join([]string{vlan.Ipv4Address, fmt.Sprintf("%v", vlan.Ipv4PrefixLength)}, "/"))
	_v6ip, v6net, v6err := net.ParseCIDR(strings.Join([]string{vlan.Ipv6Address, fmt.Sprintf("%v", vlan.Ipv6PrefixLength)}, "/"))
	if v4err == nil {
		v4dec = convertIPDec(_v4ip, "v4")
	}
	if v6err == nil {
		v6dec = convertIPDec(_v6ip, "v6")
	}
	for iter := int32(0); iter < vlan.Scale; iter++ {
		ocvlan := &oc.Interface_Subinterface{
			Index: ygot.Uint32(*vlanid)}
		if v4err == nil {
			ocvlan.Ipv4 = getSubintfIpv4(v4ip, vlan.Ipv4PrefixLength)
			v4dec, *v4ip, err = nextIP(*v4dec, vlan.V4Step, v4net, "v4")
			if err != nil {
				vlan.Scale = iter + 1
			}
		}
		if v6err == nil {
			ocvlan.Ipv6 = getSubintfIpv6(v6ip, vlan.Ipv6PrefixLength)
			v6dec, *v6ip, err = nextIP(*v6dec, vlan.V6Step, v6net, "v6")
			if err != nil {
				vlan.Scale = iter + 1
			}
		}
		addEncapsulation(ocvlan, vlan, encapid)
		config.Subinterface[*vlanid] = ocvlan
		*vlanid++
	}
	return config
}

func getSubintfIpv4(v4ip *string, prefixlength int32) *oc.Interface_Subinterface_Ipv4 {
	model := &oc.Interface_Subinterface_Ipv4{
		Address: map[string]*oc.Interface_Subinterface_Ipv4_Address{
			*v4ip: {
				Ip:           ygot.String(*v4ip),
				PrefixLength: ygot.Uint8(uint8(prefixlength)),
			},
		},
	}
	return model
}

func getSubintfIpv6(v6ip *string, prefixlength int32) *oc.Interface_Subinterface_Ipv6 {
	model := &oc.Interface_Subinterface_Ipv6{
		Address: map[string]*oc.Interface_Subinterface_Ipv6_Address{
			*v6ip: {
				Ip:           ygot.String(*v6ip),
				PrefixLength: ygot.Uint8(uint8(prefixlength)),
			},
		},
	}
	return model
}

func addEncapsulation(model *oc.Interface_Subinterface, vlan *proto.Input_Vlan, encapid *uint16) {
	if vlan.Encapsulation != 0 {
		var encap oc.Interface_Subinterface_Vlan_VlanId_Union
		encap, *encapid = getvlanID(vlan.Encapsulation, encapid)
		model.Vlan = &oc.Interface_Subinterface_Vlan{VlanId: encap}
	}
}

func getvlanID(encap proto.Input_Encap, encapid *uint16) (oc.Interface_Subinterface_Vlan_VlanId_Union, uint16) {
	var ret oc.Interface_Subinterface_Vlan_VlanId_Union
	ret = nil
	switch encap {
	case proto.Input_dot1ad_dot1q:
		dot1ad := *encapid
		*encapid++
		dot1q := *encapid
		*encapid++
		ret = oc.UnionString(fmt.Sprintf("%d.%d", dot1ad, dot1q))
	case proto.Input_dot1q:
		ret = oc.UnionUint16(*encapid)
		*encapid++
	default:
		return nil, *encapid
	}
	return ret, *encapid
}

func convertIPDec(addr net.IP, ipv string) *uint128 {
	var ipbit net.IP
	groupLen := uint64(0)
	iprep := uint128{0, 0}
	//default is ipv6
	switch ipv {
	case "v4":
		groupLen = uint64(4)
		ipbit = addr.To4()
	default:
		groupLen = uint64(16)
		ipbit = addr.To16()
	}
	for iter := uint64(0); iter < groupLen; iter++ {
		multiplier := 8 * (groupLen - 1 - iter)
		temp := uint128{Low: uint64(ipbit[iter]), High: 0}
		temp = temp.leftShift(multiplier)
		iprep = iprep.add(temp.Low, temp.High)
	}
	return &iprep
}
func nextIP(iprep uint128, step int32, v4net *net.IPNet, ipv string) (*uint128, string, error) {
	if step == 0 {
		step = 1
	}
	groupLen := uint64(0)
	var _max uint128
	//default is ipv6
	switch ipv {
	case "v4":
		groupLen = uint64(4)
		_max = uint128{Low: uint64(math.MaxUint32), High: 0}
	default:
		groupLen = uint64(16)
		_max = uint128{Low: uint64(math.MaxUint64), High: uint64(math.MaxUint64)}
	}
	diff := _max.sub(iprep.Low, iprep.High)
	if !diff.compare(uint64(step), 0) {
		return nil, "", errors.Errorf("Adress Exhaustion")
	}
	iprep = iprep.add(uint64(step), 0)

	addr := []byte{}
	for iter := uint64(0); iter < groupLen; iter++ {
		multiplier := 8 * (groupLen - 1 - iter)
		addr = append(addr, byte((iprep.rightShift(multiplier).Low)&0xFF))
	}

	address := net.IP(addr)

	return &iprep, address.String(), nil
}

type uint128 struct {
	Low, High uint64
}

func (x uint128) add(low uint64, high uint64) (y uint128) {
	_low, carry := bits.Add64(x.Low, low, 0)
	_high, _ := bits.Add64(x.High, high, carry)
	y.Low = _low
	y.High = _high
	return
}

func (x uint128) sub(low uint64, high uint64) (y uint128) {
	//return 0 incase of negative
	_low, borrow := bits.Sub64(x.Low, low, 0)
	_high, borrow := bits.Sub64(x.High, high, borrow)
	y.Low = _low
	y.High = _high
	if borrow != 0 {
		y.Low = 0
		y.High = 0
	}
	return
}
func (x *uint128) rightShift(step uint64) (y uint128) {
	if step > 64 {
		y.Low = x.High >> (step - 64)
		y.High = 0
	} else {
		y.Low = x.Low>>step | x.High<<(64-step)
		y.High = x.High >> step
	}
	return
}
func (x *uint128) leftShift(step uint64) (y uint128) {
	if step > 64 {
		y.High = x.Low << (step - 64)
		y.Low = 0
	} else {
		y.High = x.High<<step | x.Low>>(64-step)
		y.Low = x.Low << step
	}
	return y
}

func (x *uint128) compare(low uint64, high uint64) bool {
	if x.High > high {
		return true
	} else if x.High < high {
		return false
	} else if x.Low > low {
		return true
	} else if x.Low < low {
		return false
	}
	return true
}

// GetIFs returns the interface objects after solving them for scale
func GetIFs(dev *ondatra.DUTDevice, t *testing.T, intf *proto.Input_Interface) []*IfObject {
	ifs := []*IfObject{}
	name := intf.Name
	vrf := "default"
	if intf.Vrf != "" {
		vrf = intf.Vrf
	}
	if name == "" {
		name = solver.Solver(t, dev, intf.Id)
	}
	vlans := []*IfObject{}
	vlanid := uint32(1)
	encapid := uint16(1)
	if len(intf.Vlan) > 0 {
		for _, vlan := range intf.Vlan {
			vlans = append(vlans, getVlan(t, dev, intf, vlan, &vlanid, &encapid)...)
		}
	}
	members := []string{}
	for _, member := range intf.Members {
		members = append(members, solver.Solver(t, dev, member))

	}
	ifs = append(ifs, &IfObject{
		members:        members,
		id:             intf.Id,
		name:           name,
		v4address:      intf.Ipv4Address,
		v4prefixlength: intf.Ipv4PrefixLength,
		v4addressmask:  fmt.Sprintf("%s.%d", intf.Ipv4Address, intf.Ipv4PrefixLength),
		v6address:      intf.Ipv6Address,
		v6prefixlength: intf.Ipv6PrefixLength,
		vrf:            vrf,
		v6addressmask:  fmt.Sprintf("%s.%d", intf.Ipv6Address, intf.Ipv6PrefixLength),
		vlans:          vlans,
		group:          intf.Group,
	})

	ifs = append(ifs, vlans...)

	return ifs

}

// IfObject holds interface information
type IfObject struct {
	id             string
	name           string
	v4address      string
	v4prefixlength int32
	v4addressmask  string
	v6address      string
	v6prefixlength int32
	v6addressmask  string
	vrf            string
	group          string
	vlans          []*IfObject
	members        []string
}

// Name return interface Name
func (x *IfObject) Name() string { return x.name }

// Members return interface aggregation Members
func (x *IfObject) Members() []string { return x.members }

// ID return interface ID
func (x *IfObject) ID() string { return x.id }

// Ipv4Address return interface Ipv4Address
func (x *IfObject) Ipv4Address() string { return x.v4address }

// Ipv4AddressMask return interface Ipv4AddressMask
func (x *IfObject) Ipv4AddressMask() string { return x.v4addressmask }

// Ipv4PrefixLength return interface Ipv4PrefixLength
func (x *IfObject) Ipv4PrefixLength() uint8 { return uint8(x.v4prefixlength) }

// Ipv6Address return interface Ipv6Address
func (x *IfObject) Ipv6Address() string { return x.v6address }

// Ipv6AddressMask return interface Ipv6AddressMask
func (x *IfObject) Ipv6AddressMask() string { return x.v6addressmask }

// Ipv6PrefixLength return interface Ipv6PrefixLength
func (x *IfObject) Ipv6PrefixLength() uint8 { return uint8(x.v6prefixlength) }

// Vrf return interface Vrf
func (x *IfObject) Vrf() string { return x.vrf }

// Group return interface Group
func (x *IfObject) Group() string { return x.group }

func getVlan(t *testing.T, dut *ondatra.DUTDevice, intf *proto.Input_Interface, vlan *proto.Input_Vlan, vlanid *uint32, encapid *uint16) []*IfObject {
	vlans := []*IfObject{}
	if vlan.Scale == 0 {
		vlan.Scale = 1
	}
	if vlan.V4Step == 0 {
		vlan.V4Step = 1
	}
	if vlan.V6Step == 0 {
		vlan.V6Step = 1
	}
	var intfname string
	if intf.Name != "" {
		intfname = intf.Name
	} else {
		intfname = solver.Solver(t, dut, intf.Id)
	}
	v4dec := &uint128{}
	v6dec := &uint128{}
	_v4ip, v4net, v4err := net.ParseCIDR(strings.Join([]string{vlan.Ipv4Address, fmt.Sprintf("%v", vlan.Ipv4PrefixLength)}, "/"))
	_v6ip, v6net, v6err := net.ParseCIDR(strings.Join([]string{vlan.Ipv6Address, fmt.Sprintf("%v", vlan.Ipv6PrefixLength)}, "/"))

	if v4err == nil {
		v4dec = convertIPDec(_v4ip, "v4")
	}
	if v6err == nil {
		v6dec = convertIPDec(_v6ip, "v6")
	}
	v4ip := vlan.Ipv4Address
	v6ip := vlan.Ipv6Address
	var err error
	for iter := int32(0); iter < vlan.Scale; iter++ {
		if v4err == nil {
			if iter != int32(0) {
				v4dec, v4ip, err = nextIP(*v4dec, vlan.V4Step, v4net, "v4")
				if err != nil {
					return vlans
				}
				v6dec, v6ip, err = nextIP(*v6dec, vlan.V6Step, v6net, "v6")
				if err != nil {
					return vlans
				}
			}
		} else {
			v4ip = ""
			v6ip = ""
		}
		intfData := &IfObject{
			name:  fmt.Sprintf("%s.%d", intfname, *vlanid),
			group: vlan.Group,
		}
		if v4ip != "" {
			intfData.v4address = v4ip
			intfData.v4prefixlength = vlan.Ipv4PrefixLength
			intfData.v4addressmask = fmt.Sprintf("%s/%d", v4ip, vlan.Ipv4PrefixLength)
		}
		if v6ip != "" {
			intfData.v6address = v4ip
			intfData.v6prefixlength = vlan.Ipv6PrefixLength
			intfData.v6addressmask = fmt.Sprintf("%s/%d", v6ip, vlan.Ipv6PrefixLength)
		}
		if vlan.Vrf == "" {
			intfData.vrf = "default"
		} else {
			intfData.vrf = vlan.Vrf
		}
		vlans = append(vlans, intfData)
		*vlanid++
	}
	return vlans
}
