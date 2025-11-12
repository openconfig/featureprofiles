package helper

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
)

type p4rtHelper struct{}

// P4Helper provides helper functions for P4RT operations
var P4Helper = &p4rtHelper{}

// GetP4InfoPath returns the absolute path to the P4Info file from the repository root.
// This function attempts to find the featureprofiles repository root and constructs
// the path to the P4Info file. If the repository root cannot be determined,
// it returns an empty string and users must provide their own path.
func GetP4InfoPath() string {
	// Try to find the repository root by looking for go.mod
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Walk up the directory tree to find the repository root
	dir := currentDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Found the repository root
			return filepath.Join(dir, "feature", "p4rt", "data", "wbb.p4info.pb.txt")
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod
			break
		}
		dir = parent
	}

	return ""
}

// P4RTNodesByPortForAllPorts returns a map of <portID>:<P4RTNodeName> for all the
// ports on the router.
func (p *p4rtHelper) P4RTNodesByPortForAllPorts(t *testing.T, dut *ondatra.DUTDevice) map[string]string {
	t.Helper()
	ports := make(map[string][]string) // <hardware-port>:[<portID>]
	for _, p := range getAllInterfacesFromDevice(t, dut) {
		hp := gnmi.Lookup(t, dut, gnmi.OC().Interface(p).HardwarePort().State())
		if v, ok := hp.Val(); ok {
			if _, ok = ports[v]; !ok {
				ports[v] = []string{p}
			} else {
				ports[v] = append(ports[v], p)
			}
		}
	}
	nodes := make(map[string]string) // <hardware-port>:<p4rtComponentName>
	for hp := range ports {
		p4Node := gnmi.Lookup(t, dut, gnmi.OC().Component(hp).Parent().State())
		if v, ok := p4Node.Val(); ok {
			nodes[hp] = v
		}
	}
	res := make(map[string]string) // <portID>:<P4RTNodeName>
	for k, v := range nodes {
		cType := gnmi.Lookup(t, dut, gnmi.OC().Component(v).Type().State())
		ct, ok := cType.Val()
		if !ok {
			continue
		}
		if ct != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT {
			continue
		}
		for _, p := range ports[k] {
			res[p] = v
		}
	}
	return res
}

// getAllInterfacesFromDevice retrieves all interfaces from the device filtering out
// non-physical interfaces.
func getAllInterfacesFromDevice(t *testing.T, dut *ondatra.DUTDevice) []string {
	var allIntfs []string

	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())

	for _, intf := range interfaces {
		// Filter based on interface type - only include physical Ethernet interfaces
		if intf.GetType() == oc.IETFInterfaces_InterfaceType_ethernetCsmacd {
			allIntfs = append(allIntfs, intf.GetName())
		}
	}

	return allIntfs
}

// ConfigureAndGetDeviceIDs configures p4rt device-id on all P4RT nodes on the DUT using batch configuration
// and returns a map of P4RT node names to their assigned device IDs.
func (p *p4rtHelper) ConfigureAndGetDeviceIDs(t *testing.T, dut *ondatra.DUTDevice, seedDeviceID uint64) map[string]uint64 {
	nodes := p.P4RTNodesByPortForAllPorts(t, dut)

	// Create a seeded random number generator
	// Ensure seed is positive for rand.NewSource
	seed := int64(seedDeviceID & ((^uint64(0)) >> 1))
	if seed == 0 {
		seed = 1
	}
	rng := rand.New(rand.NewSource(seed))

	seen := make(map[string]bool)
	usedIDs := make(map[uint64]bool)
	deviceIDs := make(map[string]uint64)

	batch := &gnmi.SetBatch{}
	for _, p4rtNode := range nodes {
		if seen[p4rtNode] {
			continue
		}
		seen[p4rtNode] = true

		// Generate random device ID in range [1, 18446744073709551615], ensuring uniqueness
		var deviceID uint64
		for {
			deviceID = rng.Uint64()
			if deviceID == 0 {
				continue
			}
			if !usedIDs[deviceID] {
				usedIDs[deviceID] = true
				break
			}
		}

		deviceIDs[p4rtNode] = deviceID

		t.Logf("Configuring P4RT Node: %s with device ID: %d", p4rtNode, deviceID)
		c := oc.Component{}
		c.Name = ygot.String(p4rtNode)
		c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
		c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
		gnmi.BatchUpdate(batch, gnmi.OC().Component(p4rtNode).Config(), &c)
	}
	batch.Set(t, dut)

	return deviceIDs
}

// configureDeviceIDs configures p4rt device-id on all P4RT nodes on the DUT using batch configuration.
func (p *p4rtHelper) ConfigureDeviceID(t *testing.T, dut *ondatra.DUTDevice, seedDeviceID uint64) {
	nodes := p.P4RTNodesByPortForAllPorts(t, dut)

	t.Logf("Printing content of nodes")
	for port, node := range nodes {
		t.Logf("Port: %s, P4RT Node: %s", port, node)
	}
	// Create a seeded random number generator
	// Ensure seed is positive for rand.NewSource
	seed := int64(seedDeviceID & ((^uint64(0)) >> 1))
	if seed == 0 {
		seed = 1
	}
	rng := rand.New(rand.NewSource(seed))

	seen := make(map[string]bool)
	usedIDs := make(map[uint64]bool)

	batch := &gnmi.SetBatch{}
	for _, p4rtNode := range nodes {
		if seen[p4rtNode] {
			continue
		}
		seen[p4rtNode] = true

		// Generate random device ID in range [1, 18446744073709551615], ensuring uniqueness
		var deviceID uint64
		for {
			deviceID = rng.Uint64()
			if deviceID == 0 {
				continue
			}
			if !usedIDs[deviceID] {
				usedIDs[deviceID] = true
				break
			}
		}

		t.Logf("Configuring P4RT Node: %s with device ID: %d", p4rtNode, deviceID)
		c := oc.Component{}
		c.Name = ygot.String(p4rtNode)
		c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
		c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
		gnmi.BatchUpdate(batch, gnmi.OC().Component(p4rtNode).Config(), &c)
	}
	batch.Set(t, dut)
}

// ConfigureInterfaceID configures interface IDs using batch configuration.
func (p *p4rtHelper) ConfigureInterfaceID(t *testing.T, dut *ondatra.DUTDevice, seedInterfaceID uint32) {
	rand := rand.New(rand.NewSource(int64(seedInterfaceID)))
	interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	seenIDs := make(map[uint32]bool)

	batch := &gnmi.SetBatch{}
	for _, intf := range interfaces {
		intfName := intf.GetName()
		if intfName == "" || strings.Contains(intfName, "MgmtEth") ||
			strings.Contains(intfName, "Loopback") ||
			strings.Contains(intfName, "Null") ||
			strings.Contains(intfName, "PTP") ||
			strings.Contains(intfName, "Bundle") {
			continue
		}
		// Generate random interface ID in range [1, 4294967039], ensuring uniqueness
		var id uint32
		for {
			id = uint32(rand.Intn(4294967039) + 1)
			if !seenIDs[id] {
				seenIDs[id] = true
				break
			}
		}

		// Create interface config structure with the ID field
		intfConfig := &oc.Interface{
			Type: intf.GetType(),
			Id:   ygot.Uint32(id),
			Name: ygot.String(intfName),
		}
		gnmi.BatchUpdate(batch, gnmi.OC().Interface(intfName).Config(), intfConfig)
	}
	batch.Set(t, dut)
}

// ============================================================================
// P4RT Client Management Functions
// ============================================================================

// P4RTClientRole defines the role of a P4RT client
type P4RTClientRole int

const (
	// P4RTClientRoleLeader indicates the client is the primary/leader
	P4RTClientRoleLeader P4RTClientRole = iota
	// P4RTClientRoleFollower indicates the client is a secondary/follower
	P4RTClientRoleFollower
)

// P4RTClientConfig holds configuration for a P4RT client
type P4RTClientConfig struct {
	Name        string         // Client name for identification
	Role        P4RTClientRole // Leader or Follower
	DeviceID    uint64         // P4RT device ID
	ElectionID  Uint128        // Election ID (High, Low)
	StreamName  string         // Stream channel name
	PacketQSize int            // Packet queue size for PacketIn
}

// Uint128 represents a 128-bit unsigned integer for election IDs
type Uint128 struct {
	High uint64
	Low  uint64
}

// ElectionIDGenerator generates sequential election IDs
type ElectionIDGenerator struct {
	current uint64
	high    uint64
}

// NewElectionIDGenerator creates a new election ID generator with a seed value
// The seed value will be used as the starting Low value for Uint128 election IDs
func NewElectionIDGenerator(seedLow uint64) *ElectionIDGenerator {
	return &ElectionIDGenerator{
		current: seedLow,
		high:    0,
	}
}

// Next returns the next election ID (increments Low by 1)
func (g *ElectionIDGenerator) Next() Uint128 {
	electionID := Uint128{
		High: g.high,
		Low:  g.current,
	}
	g.current++
	return electionID
}

// Previous returns the previous election ID (decrements Low by 1)
// This can be used to generate follower clients with lower election IDs
func (g *ElectionIDGenerator) Previous() Uint128 {
	g.current--
	electionID := Uint128{
		High: g.high,
		Low:  g.current,
	}
	return electionID
}

// Current returns the current election ID without incrementing
func (g *ElectionIDGenerator) Current() Uint128 {
	return Uint128{
		High: g.high,
		Low:  g.current,
	}
}

// Reset resets the generator to a new seed value
func (g *ElectionIDGenerator) Reset(seedLow uint64) {
	g.current = seedLow
}

// P4RTClient wraps the p4rt_client.P4RTClient with additional metadata
type P4RTClient struct {
	Client     *p4rt_client.P4RTClient
	Config     *P4RTClientConfig
	IsLeader   bool
	StreamName string
}

// P4RTClientManager manages multiple P4RT clients
type P4RTClientManager struct {
	Clients map[string]*P4RTClient
	Leader  *P4RTClient
	DUT     *ondatra.DUTDevice
}

// NewP4RTClientManager creates a new P4RT client manager
func NewP4RTClientManager(dut *ondatra.DUTDevice) *P4RTClientManager {
	return &P4RTClientManager{
		Clients: make(map[string]*P4RTClient),
		DUT:     dut,
	}
}

// CreateClients creates multiple P4RT clients with specified configurations
// The first client with Role=Leader will be set as the leader
func (m *P4RTClientManager) CreateClients(t *testing.T, configs []*P4RTClientConfig) error {
	t.Helper()

	for _, config := range configs {
		client := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
		if err := client.P4rtClientSet(m.DUT.RawAPIs().P4RT(t)); err != nil {
			return fmt.Errorf("failed to initialize p4rt client %s: %v", config.Name, err)
		}

		p4rtClient := &P4RTClient{
			Client:     client,
			Config:     config,
			IsLeader:   config.Role == P4RTClientRoleLeader,
			StreamName: config.StreamName,
		}

		m.Clients[config.Name] = p4rtClient

		if p4rtClient.IsLeader && m.Leader == nil {
			m.Leader = p4rtClient
		}

		t.Logf("Created P4RT client: %s (Role: %v, DeviceID: %d, ElectionID: %d/%d)",
			config.Name, config.Role, config.DeviceID, config.ElectionID.High, config.ElectionID.Low)
	}

	if m.Leader == nil {
		return fmt.Errorf("no leader client configured")
	}

	return nil
}

// SetupClientArbitration sends client arbitration messages for all clients
func (m *P4RTClientManager) SetupClientArbitration(ctx context.Context, t *testing.T) error {
	t.Helper()

	for name, p4rtClient := range m.Clients {
		config := p4rtClient.Config

		// Create stream parameters
		streamParameter := p4rt_client.P4RTStreamParameters{
			Name:        config.StreamName,
			DeviceId:    config.DeviceID,
			ElectionIdH: config.ElectionID.High,
			ElectionIdL: config.ElectionID.Low,
		}

		// Create stream channel
		p4rtClient.Client.StreamChannelCreate(&streamParameter)

		// Send arbitration message
		if err := p4rtClient.Client.StreamChannelSendMsg(&config.StreamName, &p4v1pb.StreamMessageRequest{
			Update: &p4v1pb.StreamMessageRequest_Arbitration{
				Arbitration: &p4v1pb.MasterArbitrationUpdate{
					DeviceId: config.DeviceID,
					ElectionId: &p4v1pb.Uint128{
						High: config.ElectionID.High,
						Low:  config.ElectionID.Low,
					},
				},
			},
		}); err != nil {
			return fmt.Errorf("error sending ClientArbitration for %s: %v", name, err)
		}

		// Wait for arbitration response
		if _, _, arbErr := p4rtClient.Client.StreamChannelGetArbitrationResp(&config.StreamName, 1); arbErr != nil {
			if err := p4rtutils.StreamTermErr(p4rtClient.Client.StreamTermErr); err != nil {
				return fmt.Errorf("stream termination error for %s: %v", name, err)
			}
			return fmt.Errorf("arbitration response error for %s: %v", name, arbErr)
		}

		// Set packet queue size if specified
		if config.PacketQSize > 0 {
			p4rtClient.Client.StreamChannelGet(&config.StreamName).SetPacketQSize(config.PacketQSize)
		}

		t.Logf("Client %s arbitration successful (Leader: %v)", name, p4rtClient.IsLeader)
	}

	return nil
}

// SetupForwardingPipeline sends SetForwardingPipelineConfig to the leader client
func (m *P4RTClientManager) SetupForwardingPipeline(ctx context.Context, t *testing.T, p4InfoFile string) error {
	t.Helper()

	if m.Leader == nil {
		return fmt.Errorf("no leader client available")
	}

	// Load p4info file
	p4Info, err := utils.P4InfoLoad(&p4InfoFile)
	if err != nil {
		return fmt.Errorf("failed to load p4info file: %v", err)
	}

	config := m.Leader.Config

	// Send SetForwardingPipelineConfig
	if err := m.Leader.Client.SetForwardingPipelineConfig(&p4v1pb.SetForwardingPipelineConfigRequest{
		DeviceId: config.DeviceID,
		ElectionId: &p4v1pb.Uint128{
			High: config.ElectionID.High,
			Low:  config.ElectionID.Low,
		},
		Action: p4v1pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4v1pb.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4v1pb.ForwardingPipelineConfig_Cookie{
				Cookie: 159,
			},
		},
	}); err != nil {
		return fmt.Errorf("error sending SetForwardingPipelineConfig: %v", err)
	}

	t.Logf("SetForwardingPipelineConfig successful for leader client: %s", config.Name)
	return nil
}

// GetClient returns a specific P4RT client by name
func (m *P4RTClientManager) GetClient(name string) (*P4RTClient, error) {
	client, ok := m.Clients[name]
	if !ok {
		return nil, fmt.Errorf("client %s not found", name)
	}
	return client, nil
}

// GetLeader returns the leader P4RT client
func (m *P4RTClientManager) GetLeader() *P4RTClient {
	return m.Leader
}

// WriteTableEntry writes a table entry using the specified client
func (c *P4RTClient) WriteTableEntry(t *testing.T, entry *p4v1pb.TableEntry, updateType p4v1pb.Update_Type) error {
	t.Helper()

	writeReq := &p4v1pb.WriteRequest{
		DeviceId: c.Config.DeviceID,
		ElectionId: &p4v1pb.Uint128{
			High: c.Config.ElectionID.High,
			Low:  c.Config.ElectionID.Low,
		},
		Updates: []*p4v1pb.Update{
			{
				Type: updateType,
				Entity: &p4v1pb.Entity{
					Entity: &p4v1pb.Entity_TableEntry{
						TableEntry: entry,
					},
				},
			},
		},
		Atomicity: p4v1pb.WriteRequest_CONTINUE_ON_ERROR,
	}

	if err := c.Client.Write(writeReq); err != nil {
		return fmt.Errorf("failed to write table entry: %v", err)
	}

	t.Logf("Table entry written successfully (Type: %v)", updateType)
	return nil
}

// WriteTableEntries writes multiple table entries in a single request
func (c *P4RTClient) WriteTableEntries(t *testing.T, entries []*p4v1pb.TableEntry, updateType p4v1pb.Update_Type) error {
	t.Helper()

	updates := make([]*p4v1pb.Update, len(entries))
	for i, entry := range entries {
		updates[i] = &p4v1pb.Update{
			Type: updateType,
			Entity: &p4v1pb.Entity{
				Entity: &p4v1pb.Entity_TableEntry{
					TableEntry: entry,
				},
			},
		}
	}

	writeReq := &p4v1pb.WriteRequest{
		DeviceId: c.Config.DeviceID,
		ElectionId: &p4v1pb.Uint128{
			High: c.Config.ElectionID.High,
			Low:  c.Config.ElectionID.Low,
		},
		Updates:   updates,
		Atomicity: p4v1pb.WriteRequest_CONTINUE_ON_ERROR,
	}

	if err := c.Client.Write(writeReq); err != nil {
		return fmt.Errorf("failed to write table entries: %v", err)
	}

	t.Logf("Successfully wrote %d table entries (Type: %v)", len(entries), updateType)
	return nil
}

// DeleteTableEntry deletes a table entry
func (c *P4RTClient) DeleteTableEntry(t *testing.T, entry *p4v1pb.TableEntry) error {
	return c.WriteTableEntry(t, entry, p4v1pb.Update_DELETE)
}

// InsertTableEntry inserts a new table entry
func (c *P4RTClient) InsertTableEntry(t *testing.T, entry *p4v1pb.TableEntry) error {
	return c.WriteTableEntry(t, entry, p4v1pb.Update_INSERT)
}

// ModifyTableEntry modifies an existing table entry
func (c *P4RTClient) ModifyTableEntry(t *testing.T, entry *p4v1pb.TableEntry) error {
	return c.WriteTableEntry(t, entry, p4v1pb.Update_MODIFY)
}

// PacketInMessage represents a decoded PacketIn message
type PacketInMessage struct {
	Metadata    []*p4v1pb.PacketMetadata
	Payload     []byte
	SrcMAC      string
	DstMAC      string
	VlanID      uint16
	EtherType   layers.EthernetType
	IngressPort string
	EgressPort  string
}

// ReceivePackets receives PacketIn messages from the stream channel
func (c *P4RTClient) ReceivePackets(t *testing.T, count int, timeout time.Duration) ([]*PacketInMessage, error) {
	t.Helper()

	_, packets, err := c.Client.StreamChannelGetPackets(&c.StreamName, uint64(count), timeout)
	if err != nil {
		return nil, fmt.Errorf("error receiving packets: %v", err)
	}

	messages := make([]*PacketInMessage, 0, len(packets))
	for _, packet := range packets {
		if packet.Pkt == nil {
			continue
		}

		msg := &PacketInMessage{
			Metadata: packet.Pkt.GetMetadata(),
			Payload:  packet.Pkt.GetPayload(),
		}

		// Decode packet
		srcMAC, dstMAC, vlanID, etherType := DecodePacket(t, msg.Payload)
		msg.SrcMAC = srcMAC
		msg.DstMAC = dstMAC
		msg.VlanID = vlanID
		msg.EtherType = etherType

		// Extract ingress/egress port from metadata if available
		for _, meta := range msg.Metadata {
			switch meta.GetMetadataId() {
			case 1: // ingress_port (typical ID, may vary by P4 program)
				msg.IngressPort = fmt.Sprintf("%d", meta.GetValue())
			case 2: // egress_port (typical ID, may vary by P4 program)
				msg.EgressPort = fmt.Sprintf("%d", meta.GetValue())
			}
		}

		messages = append(messages, msg)
	}

	t.Logf("Received %d PacketIn messages", len(messages))
	return messages, nil
}

// SendPacketOut sends a PacketOut message
func (c *P4RTClient) SendPacketOut(t *testing.T, payload []byte, metadata []*p4v1pb.PacketMetadata) error {
	t.Helper()

	packetOut := &p4v1pb.PacketOut{
		Payload:  payload,
		Metadata: metadata,
	}

	if err := c.Client.StreamChannelSendMsg(&c.StreamName, &p4v1pb.StreamMessageRequest{
		Update: &p4v1pb.StreamMessageRequest_Packet{
			Packet: packetOut,
		},
	}); err != nil {
		return fmt.Errorf("failed to send PacketOut: %v", err)
	}

	t.Logf("PacketOut sent successfully (payload size: %d bytes)", len(payload))
	return nil
}

// DecodePacket decodes L2 header in the packet and returns source/destination MAC, VLAN ID, and Ethernet type
func DecodePacket(t *testing.T, packetData []byte) (string, string, uint16, layers.EthernetType) {
	t.Helper()

	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	d1qHeader := packet.Layer(layers.LayerTypeDot1Q)

	srcMAC, dstMAC := "", ""
	vlanID := uint16(0)
	etherType := layers.EthernetType(0)

	if etherHeader != nil {
		header, decoded := etherHeader.(*layers.Ethernet)
		if decoded {
			srcMAC, dstMAC = header.SrcMAC.String(), header.DstMAC.String()
			if header.EthernetType != layers.EthernetTypeDot1Q {
				return srcMAC, dstMAC, vlanID, header.EthernetType
			}
		}
	}

	if d1qHeader != nil {
		header, decoded := d1qHeader.(*layers.Dot1Q)
		if decoded {
			vlanID = header.VLANIdentifier
			etherType = header.Type
		}
	}

	return srcMAC, dstMAC, vlanID, etherType
}

// DecodeIPPacket decodes IP layer and returns source/destination IPs and protocol
func DecodeIPPacket(t *testing.T, packetData []byte) (srcIP, dstIP string, protocol uint8, ttl uint8) {
	t.Helper()

	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)

	// Try IPv4
	if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
		ipv4, _ := ipv4Layer.(*layers.IPv4)
		return ipv4.SrcIP.String(), ipv4.DstIP.String(), uint8(ipv4.Protocol), ipv4.TTL
	}

	// Try IPv6
	if ipv6Layer := packet.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
		ipv6, _ := ipv6Layer.(*layers.IPv6)
		return ipv6.SrcIP.String(), ipv6.DstIP.String(), uint8(ipv6.NextHeader), ipv6.HopLimit
	}

	return "", "", 0, 0
}

// ReadTableEntries reads table entries from the device
func (c *P4RTClient) ReadTableEntries(t *testing.T, tableID uint32) ([]*p4v1pb.TableEntry, error) {
	t.Helper()

	readReq := &p4v1pb.ReadRequest{
		DeviceId: c.Config.DeviceID,
		Entities: []*p4v1pb.Entity{
			{
				Entity: &p4v1pb.Entity_TableEntry{
					TableEntry: &p4v1pb.TableEntry{
						TableId: tableID,
					},
				},
			},
		},
	}

	entries := []*p4v1pb.TableEntry{}
	readRespClient, err := c.Client.Read(readReq)
	if err != nil {
		return nil, fmt.Errorf("failed to read table entries: %v", err)
	}

	// Read responses from the stream
	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout reading table entries")
		default:
			readResp, err := readRespClient.Recv()
			if err != nil {
				// Check if stream is done (io.EOF)
				if err.Error() == "EOF" {
					t.Logf("Read %d table entries from table ID %d", len(entries), tableID)
					return entries, nil
				}
				return nil, fmt.Errorf("error receiving read response: %v", err)
			}

			for _, entity := range readResp.GetEntities() {
				if entry := entity.GetTableEntry(); entry != nil {
					entries = append(entries, entry)
				}
			}
		}
	}
}

// Cleanup closes all client connections
func (m *P4RTClientManager) Cleanup(t *testing.T) {
	t.Helper()

	for name := range m.Clients {
		t.Logf("Cleaning up P4RT client: %s", name)
		m.Clients[name].Client.ServerDisconnect()
	}
}

// CreateDefaultLeaderFollowerClients creates a standard setup with one leader and one follower
func CreateDefaultLeaderFollowerClients(t *testing.T, dut *ondatra.DUTDevice, deviceID uint64, streamName string) (*P4RTClientManager, error) {
	t.Helper()

	manager := NewP4RTClientManager(dut)

	// Initialize election ID generator with seed value 100
	electionIDGen := NewElectionIDGenerator(100)

	// Get leader election ID (100) without incrementing
	leaderElectionID := electionIDGen.Current()

	// Get follower election ID (99) by decrementing once
	followerElectionID := electionIDGen.Previous()

	configs := []*P4RTClientConfig{
		{
			Name:        "leader",
			Role:        P4RTClientRoleLeader,
			DeviceID:    deviceID,
			ElectionID:  leaderElectionID, // 100
			StreamName:  streamName,
			PacketQSize: 10000,
		},
		{
			Name:        "follower",
			Role:        P4RTClientRoleFollower,
			DeviceID:    deviceID,
			ElectionID:  followerElectionID, // 99
			StreamName:  streamName,
			PacketQSize: 10000,
		},
	}

	if err := manager.CreateClients(t, configs); err != nil {
		return nil, err
	}

	return manager, nil
}

// CreateDefaultLeaderFollowerClientsWithElectionID creates a standard setup with one leader and one follower
// using the provided ElectionIDGenerator. This allows reusing the same generator across multiple calls
// to avoid election ID conflicts when creating clients for different deviceIDs or streamNames.
//
// The generator will be used to create two election IDs:
//   - Leader gets the first Previous() call result
//   - Follower gets the second Previous() call result
//
// Example usage for multiple NPUs:
//
//	electionIDGen := helper.NewElectionIDGenerator(100)
//	for streamName, deviceID := range deviceIDs {
//	    manager, _ := helper.CreateDefaultLeaderFollowerClientsWithElectionID(t, dut, deviceID, streamName, electionIDGen)
//	    // First iteration: leader=100, follower=99
//	    // Second iteration: leader=98, follower=97
//	    // Third iteration: leader=96, follower=95
//	}
func CreateDefaultLeaderFollowerClientsWithElectionID(t *testing.T, dut *ondatra.DUTDevice, deviceID uint64, streamName string, electionIDGen *ElectionIDGenerator) (*P4RTClientManager, error) {
	t.Helper()

	manager := NewP4RTClientManager(dut)

	// Get leader election ID by decrementing
	leaderElectionID := electionIDGen.Previous()

	// Get follower election ID by decrementing again
	followerElectionID := electionIDGen.Previous()

	configs := []*P4RTClientConfig{
		{
			Name:        "leader",
			Role:        P4RTClientRoleLeader,
			DeviceID:    deviceID,
			ElectionID:  leaderElectionID,
			StreamName:  streamName,
			PacketQSize: 10000,
		},
		{
			Name:        "follower",
			Role:        P4RTClientRoleFollower,
			DeviceID:    deviceID,
			ElectionID:  followerElectionID,
			StreamName:  streamName,
			PacketQSize: 10000,
		},
	}

	if err := manager.CreateClients(t, configs); err != nil {
		return nil, err
	}

	t.Logf("Created leader (ElectionID: %d) and follower (ElectionID: %d) for device %d, stream %s",
		leaderElectionID.Low, followerElectionID.Low, deviceID, streamName)

	return manager, nil
}

// ============================================================================
// WBB ACL Table Entry Programming Functions
// ============================================================================

// ProgramTableEntry programs ACL WBB ingress table entries
func (c *P4RTClient) ProgramTableEntry(t *testing.T, entries []*p4rtutils.ACLWbbIngressTableEntryInfo) error {
	t.Helper()

	updates := p4rtutils.ACLWbbIngressTableEntryGet(entries)

	writeReq := &p4v1pb.WriteRequest{
		DeviceId: c.Config.DeviceID,
		ElectionId: &p4v1pb.Uint128{
			High: c.Config.ElectionID.High,
			Low:  c.Config.ElectionID.Low,
		},
		Updates:   updates,
		Atomicity: p4v1pb.WriteRequest_CONTINUE_ON_ERROR,
	}

	if err := c.Client.Write(writeReq); err != nil {
		return fmt.Errorf("failed to program table entries: %v", err)
	}

	t.Logf("Successfully programmed %d ACL table entries", len(entries))
	return nil
}

// DeleteTableEntry deletes ACL WBB ingress table entries
func (c *P4RTClient) DeleteTableEntries(t *testing.T, entries []*p4rtutils.ACLWbbIngressTableEntryInfo) error {
	t.Helper()

	// Change all entries to DELETE type
	for _, entry := range entries {
		entry.Type = p4v1pb.Update_DELETE
	}

	return c.ProgramTableEntry(t, entries)
}

// CreateACLWbbTableEntry creates a single ACL WBB ingress table entry info
func CreateACLWbbTableEntry(etherType uint16, etherTypeMask uint16, priority uint32) *p4rtutils.ACLWbbIngressTableEntryInfo {
	return &p4rtutils.ACLWbbIngressTableEntryInfo{
		Type:          p4v1pb.Update_INSERT,
		EtherType:     etherType,
		EtherTypeMask: etherTypeMask,
		Priority:      priority,
	}
}

// CreateACLWbbTableEntryWithTTL creates an ACL WBB ingress table entry with TTL matching
func CreateACLWbbTableEntryWithTTL(isIPv4 bool, ttl uint8, ttlMask uint8, priority uint32) *p4rtutils.ACLWbbIngressTableEntryInfo {
	entry := &p4rtutils.ACLWbbIngressTableEntryInfo{
		Type:     p4v1pb.Update_INSERT,
		TTL:      ttl,
		TTLMask:  ttlMask,
		Priority: priority,
	}
	if isIPv4 {
		entry.IsIpv4 = 0x1
	} else {
		entry.IsIpv6 = 0x1
	}
	return entry
}

// CreateDefaultACLEntries creates default ACL entries for GDP, LLDP, and Traceroute
func CreateDefaultACLEntries() []*p4rtutils.ACLWbbIngressTableEntryInfo {
	gdpEthType := uint16(0x6007)
	lldpEthType := uint16(layers.EthernetTypeLinkLayerDiscovery)

	return []*p4rtutils.ACLWbbIngressTableEntryInfo{
		// GDP entry
		{
			Type:          p4v1pb.Update_INSERT,
			EtherType:     gdpEthType,
			EtherTypeMask: 0xFFFF,
			Priority:      1,
		},
		// LLDP entry
		{
			Type:          p4v1pb.Update_INSERT,
			EtherType:     lldpEthType,
			EtherTypeMask: 0xFFFF,
			Priority:      1,
		},
		// IPv4 Traceroute entry (TTL = 1)
		{
			Type:     p4v1pb.Update_INSERT,
			IsIpv4:   0x1,
			TTL:      0x1,
			TTLMask:  0xFF,
			Priority: 1,
		},
		// IPv6 Traceroute entry (Hop Limit = 1)
		{
			Type:     p4v1pb.Update_INSERT,
			IsIpv6:   0x1,
			TTL:      0x1,
			TTLMask:  0xFF,
			Priority: 1,
		},
	}
}

// ============================================================================
// Packet Generation Functions
// ============================================================================

// CreateGDPPacket creates a GDP (Cisco Group Domain of Interpretation) packet payload
func CreateGDPPacket(srcMAC, dstMAC string) ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	src, err := net.ParseMAC(srcMAC)
	if err != nil {
		return nil, fmt.Errorf("invalid source MAC: %v", err)
	}
	dst, err := net.ParseMAC(dstMAC)
	if err != nil {
		return nil, fmt.Errorf("invalid destination MAC: %v", err)
	}

	pktEth := &layers.Ethernet{
		SrcMAC:       src,
		DstMAC:       dst,
		EthernetType: layers.EthernetType(0x6007), // GDP
	}

	// Create 64-byte payload
	payload := make([]byte, 64)
	for i := 0; i < 64; i++ {
		payload[i] = byte(i)
	}

	if err := gopacket.SerializeLayers(buf, opts, pktEth, gopacket.Payload(payload)); err != nil {
		return nil, fmt.Errorf("failed to serialize GDP packet: %v", err)
	}

	return buf.Bytes(), nil
}

// CreateLLDPPacket creates an LLDP (Link Layer Discovery Protocol) packet payload
func CreateLLDPPacket(srcMAC, dstMAC string, chassisID []byte, portID string, ttl uint16) ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	src, err := net.ParseMAC(srcMAC)
	if err != nil {
		return nil, fmt.Errorf("invalid source MAC: %v", err)
	}
	dst, err := net.ParseMAC(dstMAC)
	if err != nil {
		return nil, fmt.Errorf("invalid destination MAC: %v", err)
	}

	pktEth := &layers.Ethernet{
		SrcMAC:       src,
		DstMAC:       dst,
		EthernetType: layers.EthernetTypeLinkLayerDiscovery,
	}

	pktLLDP := &layers.LinkLayerDiscovery{
		ChassisID: layers.LLDPChassisID{
			Subtype: layers.LLDPChassisIDSubTypeMACAddr,
			ID:      chassisID,
		},
		PortID: layers.LLDPPortID{
			Subtype: layers.LLDPPortIDSubtypeIfaceName,
			ID:      []byte(portID),
		},
		TTL: ttl,
	}

	if err := gopacket.SerializeLayers(buf, opts, pktEth, pktLLDP); err != nil {
		return nil, fmt.Errorf("failed to serialize LLDP packet: %v", err)
	}

	return buf.Bytes(), nil
}

// CreateTraceroutePacket creates an IPv4 ICMP traceroute packet payload with specified TTL
func CreateTraceroutePacket(srcMAC, dstMAC string, srcIP, dstIP string, ttl uint8, seq uint16) ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	src, err := net.ParseMAC(srcMAC)
	if err != nil {
		return nil, fmt.Errorf("invalid source MAC: %v", err)
	}
	dst, err := net.ParseMAC(dstMAC)
	if err != nil {
		return nil, fmt.Errorf("invalid destination MAC: %v", err)
	}

	pktEth := &layers.Ethernet{
		SrcMAC:       src,
		DstMAC:       dst,
		EthernetType: layers.EthernetTypeIPv4,
	}

	pktIPv4 := &layers.IPv4{
		Version:  4,
		TTL:      ttl,
		SrcIP:    net.ParseIP(srcIP).To4(),
		DstIP:    net.ParseIP(dstIP).To4(),
		Protocol: layers.IPProtocolICMPv4,
		Flags:    layers.IPv4DontFragment,
	}

	pktICMP := &layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
		Seq:      seq,
	}

	// Create 32-byte payload
	payload := make([]byte, 32)
	for i := 0; i < 32; i++ {
		payload[i] = byte(i)
	}

	if err := gopacket.SerializeLayers(buf, opts, pktEth, pktIPv4, pktICMP, gopacket.Payload(payload)); err != nil {
		return nil, fmt.Errorf("failed to serialize traceroute packet: %v", err)
	}

	return buf.Bytes(), nil
}

// CreateTracerouteIPv6Packet creates an IPv6 ICMPv6 traceroute packet payload with specified hop limit
func CreateTracerouteIPv6Packet(srcMAC, dstMAC string, srcIP, dstIP string, hopLimit uint8, seq uint16) ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	src, err := net.ParseMAC(srcMAC)
	if err != nil {
		return nil, fmt.Errorf("invalid source MAC: %v", err)
	}
	dst, err := net.ParseMAC(dstMAC)
	if err != nil {
		return nil, fmt.Errorf("invalid destination MAC: %v", err)
	}

	pktEth := &layers.Ethernet{
		SrcMAC:       src,
		DstMAC:       dst,
		EthernetType: layers.EthernetTypeIPv6,
	}

	pktIPv6 := &layers.IPv6{
		Version:    6,
		HopLimit:   hopLimit,
		SrcIP:      net.ParseIP(srcIP),
		DstIP:      net.ParseIP(dstIP),
		NextHeader: layers.IPProtocolICMPv6,
	}

	pktICMP := &layers.ICMPv6{
		TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0),
	}
	pktICMP.SetNetworkLayerForChecksum(pktIPv6)

	// Create 32-byte payload
	payload := make([]byte, 32)
	for i := 0; i < 32; i++ {
		payload[i] = byte(i)
	}

	if err := gopacket.SerializeLayers(buf, opts, pktEth, pktIPv6, pktICMP, gopacket.Payload(payload)); err != nil {
		return nil, fmt.Errorf("failed to serialize IPv6 traceroute packet: %v", err)
	}

	return buf.Bytes(), nil
}

// ============================================================================
// PacketOut Helper Functions
// ============================================================================

// SendGDPPacketOut sends a GDP packet via PacketOut
func (c *P4RTClient) SendGDPPacketOut(t *testing.T, srcMAC, dstMAC string, egressPort uint32) error {
	t.Helper()

	payload, err := CreateGDPPacket(srcMAC, dstMAC)
	if err != nil {
		return err
	}

	metadata := []*p4v1pb.PacketMetadata{
		{
			MetadataId: 1, // egress_port
			Value:      []byte(fmt.Sprint(egressPort)),
		},
	}

	return c.SendPacketOut(t, payload, metadata)
}

// SendLLDPPacketOut sends an LLDP packet via PacketOut
func (c *P4RTClient) SendLLDPPacketOut(t *testing.T, srcMAC, dstMAC string, chassisID []byte, portID string, ttl uint16, egressPort uint32) error {
	t.Helper()

	payload, err := CreateLLDPPacket(srcMAC, dstMAC, chassisID, portID, ttl)
	if err != nil {
		return err
	}

	metadata := []*p4v1pb.PacketMetadata{
		{
			MetadataId: 1, // egress_port
			Value:      []byte(fmt.Sprint(egressPort)),
		},
	}

	return c.SendPacketOut(t, payload, metadata)
}

// SendTraceroutePacketOut sends a traceroute packet via PacketOut with submit_to_ingress flag
func (c *P4RTClient) SendTraceroutePacketOut(t *testing.T, srcMAC, dstMAC string, srcIP, dstIP string, ttl uint8, seq uint16) error {
	t.Helper()

	payload, err := CreateTraceroutePacket(srcMAC, dstMAC, srcIP, dstIP, ttl, seq)
	if err != nil {
		return err
	}

	metadata := []*p4v1pb.PacketMetadata{
		{
			MetadataId: 1, // egress_port
			Value:      []byte("0"),
		},
		{
			MetadataId: 2, // submit_to_ingress
			Value:      []byte{1},
		},
		{
			MetadataId: 3, // unused_pad
			Value:      []byte{0},
		},
	}

	return c.SendPacketOut(t, payload, metadata)
}

// SendTracerouteIPv6PacketOut sends an IPv6 traceroute packet via PacketOut
func (c *P4RTClient) SendTracerouteIPv6PacketOut(t *testing.T, srcMAC, dstMAC string, srcIP, dstIP string, hopLimit uint8, seq uint16) error {
	t.Helper()

	payload, err := CreateTracerouteIPv6Packet(srcMAC, dstMAC, srcIP, dstIP, hopLimit, seq)
	if err != nil {
		return err
	}

	metadata := []*p4v1pb.PacketMetadata{
		{
			MetadataId: 1, // egress_port
			Value:      []byte("0"),
		},
		{
			MetadataId: 2, // submit_to_ingress
			Value:      []byte{1},
		},
		{
			MetadataId: 3, // unused_pad
			Value:      []byte{0},
		},
	}

	return c.SendPacketOut(t, payload, metadata)
}

// SendPacketsInLoop sends multiple packets with a specified delay between each packet
func (c *P4RTClient) SendPacketsInLoop(t *testing.T, payload []byte, metadata []*p4v1pb.PacketMetadata, count int, delay time.Duration) error {
	t.Helper()

	for i := 0; i < count; i++ {
		if err := c.SendPacketOut(t, payload, metadata); err != nil {
			return fmt.Errorf("failed to send packet %d: %v", i, err)
		}
		if delay > 0 && i < count-1 {
			time.Sleep(delay)
		}
	}

	t.Logf("Successfully sent %d packets with %v delay", count, delay)
	return nil
}

// Simple usage
// manager, _ := helper.CreateDefaultLeaderFollowerClients(t, dut, deviceID, "stream1")
// manager.SetupClientArbitration(ctx, t)
// manager.SetupForwardingPipeline(ctx, t, "p4info.pb.txt")

// leader := manager.GetLeader()
// leader.InsertTableEntry(t, myTableEntry)

// packets, _ := leader.ReceivePackets(t, 10, 30*time.Second)
// for _, pkt := range packets {
//     t.Logf("Received: %s -> %s", pkt.SrcMAC, pkt.DstMAC)
// }

// Program default ACL entries for GDP, LLDP, Traceroute
// entries := helper.CreateDefaultACLEntries()
// leader.ProgramTableEntry(t, entries)
// defer leader.DeleteTableEntries(t, entries)

// // Send GDP packet
// leader.SendGDPPacketOut(t, "00:01:02:03:04:05", "00:0a:da:f0:f0:f0", 10)

// // Send LLDP packet
// chassisID := []byte{0x01, 0x01, 0x01, 0x01, 0x01, 0x01}
// leader.SendLLDPPacketOut(t, "00:01:02:03:04:05", "01:80:c2:00:00:0e", chassisID, "port1", 100, 10)

// // Send Traceroute packet with TTL=1
// leader.SendTraceroutePacketOut(t, "00:01:02:03:04:05", "02:F6:65:64:00:08", "192.0.2.1", "192.0.2.2", 1, 1)

// // Send multiple packets with delay
// payload, _ := helper.CreateGDPPacket("00:01:02:03:04:05", "00:0a:da:f0:f0:f0")
// metadata := []*p4v1pb.PacketMetadata{{MetadataId: 1, Value: []byte("10")}}
// leader.SendPacketsInLoop(t, payload, metadata, 2000, 2600*time.Microsecond)
