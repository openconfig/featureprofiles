package otgconfighelpers

import (
	"fmt"
	"net"
	"strings"

	"github.com/open-traffic-generator/snappi/gosnappi"
)

// V4IsisStRouteInfo is a struct that contains the data needed to generate a V4 route in the ISIS topology.
type V4IsisStRouteInfo struct {
	addressFirstOctet string
	prefix            int
	count             int
}

// SetAddressFirstOctet sets the first octet of the address of the V4 route.
func (v *V4IsisStRouteInfo) SetAddressFirstOctet(addressFirstOctet string) *V4IsisStRouteInfo {
	v.addressFirstOctet = addressFirstOctet
	return v
}

// SetPrefix sets the prefix of the V4 route.
func (v *V4IsisStRouteInfo) SetPrefix(prefix int) *V4IsisStRouteInfo {
	v.prefix = prefix
	return v
}

// SetCount sets the count of the V4 route.
func (v *V4IsisStRouteInfo) SetCount(count int) *V4IsisStRouteInfo {
	v.count = count
	return v
}

// V6IsisStRouteInfo is a struct that contains the data needed to generate a V6 route in the ISIS topology.
type V6IsisStRouteInfo struct {
	addressFirstOctet string
	prefix            int
	count             int
}

// SetAddressFirstOctet sets the first octet of the address of the V6 route.
func (v *V6IsisStRouteInfo) SetAddressFirstOctet(addressFirstOctet string) *V6IsisStRouteInfo {
	v.addressFirstOctet = addressFirstOctet
	return v
}

// SetPrefix sets the prefix of the V6 route.
func (v *V6IsisStRouteInfo) SetPrefix(prefix int) *V6IsisStRouteInfo {
	v.prefix = prefix
	return v
}

// SetCount sets the count of the V6 route.
func (v *V6IsisStRouteInfo) SetCount(count int) *V6IsisStRouteInfo {
	v.count = count
	return v
}

// GridIsisData is a struct that contains the data needed to generate a grid of ISIS devices.
type GridIsisData struct {
	blockName          string
	config             gosnappi.Config
	row                int
	col                int
	systemIDFirstOctet string
	linkIP4FirstOctet  int
	nextLinkIP4ToUse   net.IP
	linkIP6FirstOctet  string
	v4StRoute          *V4IsisStRouteInfo
	v6StRoute          *V6IsisStRouteInfo
	linkMultiplier     int
}

// SetRow sets the row of the grid.
func (v *GridIsisData) SetRow(row int) *GridIsisData {
	v.row = row
	return v
}

// SetCol sets the column of the grid.
func (v *GridIsisData) SetCol(col int) *GridIsisData {
	v.col = col
	return v
}

// SetSystemIDFirstOctet sets the ISISsystem ID first octet of the grid.
// all the ISIS system IDs will be in the format of XX.XXXX.XXXX.XXXX.XX where XX is the first octet.
func (v *GridIsisData) SetSystemIDFirstOctet(firstOctSysID string) *GridIsisData {
	v.systemIDFirstOctet = firstOctSysID
	return v
}

// SetLinkIP4FirstOctet sets the first octet of the link IP4 address.
// The link IP4 address will be in the format of XX.XX.XX.XX/31 where XX is the first octet.
func (v *GridIsisData) SetLinkIP4FirstOctet(oct int) *GridIsisData {
	v.linkIP4FirstOctet = oct
	v.nextLinkIP4ToUse = net.ParseIP(fmt.Sprintf("%d.0.0.0", oct))
	return v
}

// NextLinkIP4ToUse returns a pointer to the next link IP4 address to use.
func (v *GridIsisData) NextLinkIP4ToUse() *net.IP {
	return &v.nextLinkIP4ToUse
}

// SetLinkIP6FirstOctet sets the first octet of the link IP6 address.
// The link IP6 address will be in the format of XX:XX:XX:XX:XX:XX:XX:XX/64 where XX is the first octet.
func (v *GridIsisData) SetLinkIP6FirstOctet(oct string) *GridIsisData {
	v.linkIP6FirstOctet = oct
	return v
}

// SetLinkMultiplier sets the number of links between each pair of nodes in the grid.
func (v *GridIsisData) SetLinkMultiplier(multiplier int) *GridIsisData {
	v.linkMultiplier = multiplier
	return v
}

// SetBlockName sets the name of the grid , this will be used to create the name of the ISIS devices
// in the grid.
func (v *GridIsisData) SetBlockName(blockName string) *GridIsisData {
	v.blockName = blockName
	return v
}

// V4RouteInfo returns the default V4 route info for the grid.
func (v *GridIsisData) V4RouteInfo() *V4IsisStRouteInfo {
	v.v4StRoute = &V4IsisStRouteInfo{
		addressFirstOctet: "10",
		prefix:            32,
		count:             1,
	}
	return v.v4StRoute
}

// V6RouteInfo returns the default V6 route info for the grid.
func (v *GridIsisData) V6RouteInfo() *V6IsisStRouteInfo {
	v.v6StRoute = &V6IsisStRouteInfo{
		addressFirstOctet: "10",
		prefix:            64,
		count:             1,
	}
	return v.v6StRoute
}

// GridIsisTopo is a struct that contains the data needed to generate the topology of the grid.
type GridIsisTopo struct {
	blockName         string
	gridNodes         [][]int
	devices           []gosnappi.Device
	linkIP4FirstOctet int
	linkIP6FirstOctet string
	linkMultiplier    int
}

// Connect connects one of the devices in the grid to the given emulated router.
func (v *GridIsisTopo) Connect(emuDev gosnappi.Device, rowIdx int, colIdx int, nextLinkIP4ToUse *net.IP) error {
	devIdx := v.gridNodes[rowIdx][colIdx]
	simDev := v.devices[devIdx]
	emuIdx := len(v.devices)
	if err := createLink(emuDev, simDev, emuIdx, devIdx,
		v.linkIP4FirstOctet, v.linkIP6FirstOctet, 1, nextLinkIP4ToUse); err != nil {
		return err
	}

	return nil
}

// Device returns the device at the given row and column index.
func (v *GridIsisTopo) Device(rowIdx int, colIdx int) gosnappi.Device {
	devIdx := v.gridNodes[rowIdx][colIdx]
	simDev := v.devices[devIdx]
	return simDev
}

// GenerateTopology generates the topology of the grid.
func (v *GridIsisData) GenerateTopology() (GridIsisTopo, error) {
	gridTopo := GridIsisTopo{
		linkIP4FirstOctet: v.linkIP4FirstOctet,
		linkIP6FirstOctet: v.linkIP6FirstOctet,
		linkMultiplier:    v.linkMultiplier,
		blockName:         v.blockName,
	}
	if v.row <= 1 || v.col <= 1 {
		return gridTopo, fmt.Errorf("grid must have more than one row or col")
	}
	if v.row > 255 || v.col > 255 {
		return gridTopo, fmt.Errorf("grid col and row must be less than or equal to 255")
	}

	if len(v.systemIDFirstOctet) == 0 {
		return gridTopo, fmt.Errorf("system ID first octet for ISIS must be configured")
	}

	gridTopo.gridNodes = make([][]int, v.row)
	for i := range gridTopo.gridNodes {
		gridTopo.gridNodes[i] = make([]int, v.col)
	}

	nodeIDx := 0
	for rowIdx := 0; rowIdx < v.row; rowIdx++ {
		for colIdx := 0; colIdx < v.col; colIdx++ {
			gridTopo.gridNodes[rowIdx][colIdx] = nodeIDx
			if dev, err := createSimDev(v.config, nodeIDx, v.systemIDFirstOctet, rowIdx, colIdx, v.v4StRoute, v.v6StRoute, v.blockName); err != nil {
				return gridTopo, err
			} else {
				gridTopo.devices = append(gridTopo.devices, dev)
				nodeIDx++
			}
		}
	}

	for rowIdx, row1 := range gridTopo.gridNodes {
		for colIdx, val1 := range row1 {
			if colIdx+1 != v.col {
				val2 := row1[colIdx+1]
				if err := createLink(gridTopo.devices[val1], gridTopo.devices[val2],
					val1, val2, v.linkIP4FirstOctet, v.linkIP6FirstOctet, v.linkMultiplier, &v.nextLinkIP4ToUse); err != nil {
					return gridTopo, err
				}
			}
			if rowIdx+1 != v.row {
				row2 := gridTopo.gridNodes[rowIdx+1]
				val2 := row2[colIdx]
				if err := createLink(gridTopo.devices[val1], gridTopo.devices[val2],
					val1, val2, v.linkIP4FirstOctet, v.linkIP6FirstOctet, v.linkMultiplier, &v.nextLinkIP4ToUse); err != nil {
					return gridTopo, err
				}
			}
		}
	}

	return gridTopo, nil
}

// NewGridIsisData creates a new GridIsisData struct.
func NewGridIsisData(c gosnappi.Config) GridIsisData {
	return GridIsisData{
		config:    c,
		v4StRoute: nil,
		v6StRoute: nil,
	}
}

// createSimDev creates a simulated device in the ISIS topology.
func createSimDev(config gosnappi.Config, nodeIDx int, systemIDFirstOctet string, srcIdx int, dstIDx int, v4RouteInfo *V4IsisStRouteInfo, v6RouteInfo *V6IsisStRouteInfo, blockName string) (gosnappi.Device, error) {
	otgIdx := nodeIDx + 1
	var deviceName string
	var teRtrID string

	if dstIDx == -1 {
		deviceName = fmt.Sprintf("%s_%sd%d.sim.%d", blockName, systemIDFirstOctet, otgIdx, srcIdx)
		teRtrID = fmt.Sprintf("%s.10.0.%d", systemIDFirstOctet, srcIdx)

	} else {
		deviceName = fmt.Sprintf("%s_%sd%d.sim.%d.%d", blockName, systemIDFirstOctet, otgIdx, srcIdx, dstIDx)
		teRtrID = fmt.Sprintf("%s.%d.%d.1", systemIDFirstOctet, srcIdx, dstIDx)
	}
	dev := config.Devices().Add().SetName(deviceName)

	// System ID is 6 octets long, the first octet is taken as input for the block , the rest 5 octets
	// are used to identify the device in the grid , so if the node index in the HEX format is more
	// than 10 octets then we this means we ran out of space.
	systemIDLastOctets := fmt.Sprintf("%02x", otgIdx)
	if len(systemIDLastOctets) > 10 {
		return nil, fmt.Errorf("ran out of system ID space")
	}
	systemID := systemIDFirstOctet + strings.Repeat("0", 10-len(systemIDLastOctets)) + systemIDLastOctets
	simRtrIsis := dev.Isis().
		SetName(deviceName + ".isis").
		SetSystemId(systemID)

	simRtrIsis.Basic().
		SetIpv4TeRouterId(teRtrID).
		SetHostname(deviceName).
		SetEnableWideMetric(false)

	if v4RouteInfo != nil {
		v4Route := simRtrIsis.V4Routes().Add().
			SetName(simRtrIsis.Name() + ".isis.v4routes").
			SetLinkMetric(10).
			SetOriginType(gosnappi.IsisV4RouteRangeOriginType.INTERNAL)

		ipv4Prefix := fmt.Sprintf("%s.0.%d.0", v4RouteInfo.addressFirstOctet, srcIdx+1)
		if dstIDx != -1 {
			ipv4Prefix = fmt.Sprintf("%s.%d.%d.0", v4RouteInfo.addressFirstOctet, srcIdx+1, dstIDx+1)
		}
		v4Route.Addresses().Add().
			SetAddress(ipv4Prefix).
			SetPrefix(uint32(v4RouteInfo.prefix)).
			SetCount(uint32(v4RouteInfo.count))
	}

	if v6RouteInfo != nil {
		v6Route := simRtrIsis.V6Routes().Add().
			SetName(simRtrIsis.Name() + ".isis.v6routes").
			SetLinkMetric(10).
			SetOriginType(gosnappi.IsisV6RouteRangeOriginType.INTERNAL)

		ipv6Prefix := fmt.Sprintf("%s:%d::0", v6RouteInfo.addressFirstOctet, srcIdx+1)
		if dstIDx != -1 {
			ipv6Prefix = fmt.Sprintf("%s:%d:%d::0", v6RouteInfo.addressFirstOctet, srcIdx+1, dstIDx+1)
		}

		v6Route.Addresses().Add().
			SetAddress(ipv6Prefix).
			SetPrefix(uint32(v6RouteInfo.prefix)).
			SetCount(uint32(v6RouteInfo.count))
	}

	return dev, nil
}

// createLink creates a simulated link between the two devices in the ISIS topology.
func createLink(d1 gosnappi.Device, d2 gosnappi.Device, IDx1 int, IDx2 int, linkIP4FirstOctet int, linkIP6FirstOctet string, linkMultiplier int, ipv4Add *net.IP) error {
	d1name := d1.Name()
	d2name := d2.Name()

	idx1 := IDx1 + 1
	idx2 := IDx2 + 1

	for i := 0; i < linkMultiplier; i++ {

		eth1Name := fmt.Sprintf("%veth%d", d1name, len(d1.Ethernets().Items())+1)
		eth2Name := fmt.Sprintf("%veth%d", d2name, len(d2.Ethernets().Items())+1)
		isisInf1Name := fmt.Sprintf("%vIsisinf%d", d1name, len(d1.Isis().Interfaces().Items())+1)
		isisInf2Name := fmt.Sprintf("%vIsisinf%d", d2name, len(d2.Isis().Interfaces().Items())+1)

		d1eth := d1.Ethernets().Add().SetName(eth1Name)
		d1eth.Connection().SimulatedLink().SetRemoteSimulatedLink(eth2Name)

		d1.Isis().Interfaces().Add().SetName(isisInf1Name).SetEthName(eth1Name).
			SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

		d2eth := d2.Ethernets().Add().SetName(eth2Name)
		d2eth.Connection().SimulatedLink().SetRemoteSimulatedLink(eth1Name)

		d2.Isis().Interfaces().Add().SetName(isisInf2Name).SetEthName(eth2Name).
			SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

		if linkIP4FirstOctet == 0 && len(linkIP6FirstOctet) == 0 {
			return fmt.Errorf("first octet of IP4 or IP6 for link must be configured")
		}

		if linkIP4FirstOctet != 0 {
			ip1Name := fmt.Sprintf("%vip4", eth1Name)
			ip2Name := fmt.Sprintf("%vip4", eth2Name)
			ip1 := ipv4Add.String()

			*ipv4Add = nextIP(*ipv4Add)
			if ip4 := (*ipv4Add).To4(); ip4 == nil || ip4[0] != byte(linkIP4FirstOctet) {
				return fmt.Errorf("no free ipv4 address in the major subnet %d/8", linkIP4FirstOctet)
			}
			ip2 := ipv4Add.String()

			*ipv4Add = nextIP(*ipv4Add)

			d1eth.Ipv4Addresses().Add().
				SetName(ip1Name).
				SetAddress(ip1).
				SetGateway(ip2).
				SetPrefix(31)

			d2eth.Ipv4Addresses().Add().
				SetName(ip2Name).
				SetAddress(ip2).
				SetGateway(ip1).
				SetPrefix(31)
		}

		if len(linkIP6FirstOctet) != 0 {
			ip1Name := fmt.Sprintf("%vip6", eth1Name)
			ip2Name := fmt.Sprintf("%vip6", eth2Name)
			ip1 := fmt.Sprintf("%s::%d:%d:%d", linkIP6FirstOctet, idx1, idx2, i*2+1)
			ip2 := fmt.Sprintf("%s::%d:%d:%d", linkIP6FirstOctet, idx1, idx2, i*2+2)

			d1eth.Ipv6Addresses().Add().
				SetName(ip1Name).
				SetAddress(ip1).
				SetGateway(ip2)

			d2eth.Ipv6Addresses().Add().
				SetName(ip2Name).
				SetAddress(ip2).
				SetGateway(ip1)
		}
	}
	return nil
}

// nextIP returns the next IPv4 or v6 address in the same subnet.
func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)

	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] > 0 {
			break
		}
	}
	return next
}
