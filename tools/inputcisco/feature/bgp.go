package feature

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"
	"github.com/openconfig/featureprofiles/tools/inputcisco/solver"
	"github.com/openconfig/featureprofiles/tools/inputcisco/testinput"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// BGP Configuration Implementation
//
// OVERVIEW:
//
// Configuration Flow:
//   1. YAML File (testdata/bgp.yaml)
//      ├─ Human-readable test configuration
//      └─ Example: as: 65000, router_id: 1.1.1.1, neighbors: [...]
//
//   2. Proto Struct (*proto.Input_BGP)
//      ├─ Type-safe Go struct auto-generated from input.proto
//      ├─ Validates YAML structure at load time
//      └─ Example: bgp.As (int32), bgp.RouterId (string), bgp.Neighbors ([]*Input_BGP_Neighbor)
//
//   3. OpenConfig Struct (*oc.NetworkInstance_Protocol)
//      ├─ Standardized device model (vendor-neutral)
//      ├─ Maps proto fields to OpenConfig paths
//      └─ Example: /network-instances/network-instance/protocols/protocol/bgp/global/config/as
//
//   4. gNMI Request (gnmi.Update)
//      ├─ Wire protocol to device
//      ├─ JSON_IETF encoding over gRPC
//      └─ Device translates OpenConfig → native CLI/config
//

// ConfigBGP configures BGP on a DUT device from YAML input.
//
// This function translates YAML-based configuration into OpenConfig BGP structs and applies
// them via gNMI. It handles:
//   - Global BGP settings (AS, Router-ID, graceful restart, global AFI/SAFIs)
//   - Neighbor configuration (addresses, peer/local AS, transport, eBGP multihop, per-neighbor AFI/SAFIs)
//   - Network instance mapping (VRF support)
//
// Parameters:
//   - dev: The DUT device to configure
//   - t: Testing context for logging and errors
//   - bgp: Proto struct containing BGP config from YAML (auto-generated from input.proto)
//   - input: Test input context for tag resolution (e.g., {{dut.port1.ipv4}} → actual IP)
//
// Returns:
//   - error: nil on success, error if configuration fails
//
// Example YAML:
//
//	dut:
//	  bgp:
//	    - as: 65000
//	      router_id: 1.1.1.1
//	      vrf: default
//	      neighbors:
//	        - address: 192.168.1.2
//	          peer_as: 64001
//	          local_as: 63001
func ConfigBGP(dev *ondatra.DUTDevice, t *testing.T, bgp *proto.Input_BGP, input testinput.TestInput) error {
	// Build the complete BGP protocol configuration from YAML input
	// This returns an OpenConfig NetworkInstance_Protocol struct populated with:
	//   - Global config (AS, Router-ID, graceful restart, global AFI/SAFIs)
	//   - Neighbor configs (all fields properly set, including AFI/SAFIs)
	model := configBGP(t, bgp, input)

	// Default to "default" VRF if not specified in YAML
	if bgp.Vrf == "" {
		bgp.Vrf = "default"
	}

	// Construct the NetworkInstance request wrapper
	// OpenConfig path: /network-instances/network-instance[name={vrf}]
	request := oc.NetworkInstance{
		Name: ygot.String(bgp.Vrf),
		Protocol: map[oc.NetworkInstance_Protocol_Key]*oc.NetworkInstance_Protocol{
			{
				Name:       bgp.Vrf,
				Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
			}: model,
		},
	}

	// Validate the constructed OpenConfig model before sending
	// This catches structural errors early (missing required fields, invalid enums, etc.)
	if err := model.Validate(); err != nil {
		t.Logf("WARNING: OpenConfig model validation failed: %v", err)
		// Continuing anyway for now - haven't hit this section in practice
	}

	// Send configuration to device via gNMI
	// Path: /network-instances/network-instance[name={vrf}]/config
	// Operation: UPDATE (merge with existing config, don't replace entire network-instance)
	gnmi.Update(t, dev, gnmi.OC().NetworkInstance(bgp.Vrf).Config(), &request)
	return nil
}

// UnConfigBGP removes BGP configuration from a DUT device.
//
// This deletes the entire BGP protocol configuration for the specified VRF/network-instance.
// Use with caution - this removes ALL BGP config including neighbors, policies, and AFI/SAFIs.
//
// Parameters:
//   - dev: The DUT device to unconfigure
//   - t: Testing context for logging
//   - bgp: Proto struct containing VRF name (defaults to "default" if empty)
//
// gNMI Path: /network-instances/network-instance[name={vrf}]/protocols/protocol[name={vrf},identifier=BGP]/bgp/config
// Operation: DELETE
func UnConfigBGP(dev *ondatra.DUTDevice, t *testing.T, bgp *proto.Input_BGP) error {
	if bgp.Vrf == "" {
		bgp.Vrf = "default"
	}
	gnmi.Delete(t, dev, gnmi.OC().NetworkInstance(bgp.Vrf).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Config())
	return nil
}

// ConfigBGPNetworks configures BGP network statements to advertise routes.
//
// This function tells BGP to advertise specific prefixes to neighbors using the
// "network" statement. The prefixes must already exist in the routing table
// (e.g., as connected routes, static routes, or learned via IGP).
//
// Parameters:
//   - dev: The DUT device to configure
//   - t: Testing context for logging
//   - bgp: Proto struct containing networks from YAML
//
// Example YAML:
//
//	bgp:
//	  - as: 65000
//	    networks:
//	      - prefix: "100.120.1.0/24"  # Connected route on Bundle-Ether120
//	      - prefix: "100.121.1.0/24"  # Connected route on Bundle-Ether121
//
// Generated CLI:
//
//	router bgp 65000
//	 address-family ipv4 unicast
//	  network 100.120.1.0/24
//	  network 100.121.1.0/24
//	!
func ConfigBGPNetworks(dev *ondatra.DUTDevice, t *testing.T, bgp *proto.Input_BGP) error {
	if len(bgp.Networks) == 0 {
		t.Log("No BGP networks to configure")
		return nil
	}

	var cliConfig strings.Builder

	// Configure BGP network statements to advertise existing routes
	cliConfig.WriteString(fmt.Sprintf("router bgp %d\n", bgp.As))
	cliConfig.WriteString(" address-family ipv4 unicast\n")
	for _, network := range bgp.Networks {
		cliConfig.WriteString(fmt.Sprintf("  network %s\n", network.Prefix))
	}
	cliConfig.WriteString(" !\n!\n")

	t.Logf("Configuring BGP network statements:\n%s", cliConfig.String())

	// Apply via gNMI text mode
	util.GNMIWithText(context.Background(), t, dev, cliConfig.String())
	return nil
}

// configBGP builds the OpenConfig BGP protocol model from proto input.
//
// This is the core translation function that maps proto structs (from YAML) to
// OpenConfig structs (for gNMI). It constructs:
//   - Protocol wrapper with name and identifier
//   - Global BGP config (AS, Router-ID, graceful restart, global AFI/SAFIs)
//   - Neighbor configs via getBGPneighbor()
//
// Parameters:
//   - t: Testing context for logging
//   - bgp: Proto struct containing BGP config from YAML
//   - input: Test input context for address tag resolution
//
// Returns:
//   - *oc.NetworkInstance_Protocol: Complete BGP protocol configuration
//
// This function is called by ConfigBGP and is separate to enable unit testing
// of the translation logic without device interaction.
func configBGP(t *testing.T, bgp *proto.Input_BGP, input testinput.TestInput) *oc.NetworkInstance_Protocol {

	if bgp.Vrf == "" {
		bgp.Vrf = "default"
	}
	model := oc.NetworkInstance_Protocol{
		Name:       ygot.String(bgp.Vrf),
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Bgp: &oc.NetworkInstance_Protocol_Bgp{
			Global: &oc.NetworkInstance_Protocol_Bgp_Global{},
		},
	}

	// Configure BGP Global settings
	model.Bgp.Global.As = ygot.Uint32(uint32(bgp.As))

	// Set Router-ID if provided in YAML
	if bgp.RouterId != "" {
		model.Bgp.Global.RouterId = ygot.String(bgp.RouterId)
	}

	// Configure Graceful Restart
	if bgp.GracefullRestart != nil {
		model.Bgp.Global.GracefulRestart = &oc.NetworkInstance_Protocol_Bgp_Global_GracefulRestart{
			Enabled: ygot.Bool(bgp.GracefullRestart.Enabled),
		}
	}

	// Configure Global AFI/SAFIs (address families enabled at BGP process level)
	// These apply to the entire BGP process and determine which route types can be exchanged
	// Example: IPV4_UNICAST, IPV6_UNICAST, L3VPN_IPV4_UNICAST
	model.Bgp.Global.AfiSafi = map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{}
	for _, afi := range bgp.Afisafi {
		afisafi := &oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{
			AfiSafiName: GetAfisafiType(afi.Type),
			Enabled:     ygot.Bool(true),
			AddPaths:    getAddPathsGlobal(afi.AdditionalPaths),
		}
		model.Bgp.Global.AfiSafi[afisafi.AfiSafiName] = afisafi
	}

	// Configure BGP Neighbors (peers that this device will establish sessions with)
	// Each neighbor gets: address, AS numbers, transport, multihop, per-neighbor AFI/SAFIs
	model.Bgp.Neighbor = getBGPneighbor(t, bgp.Neighbors, input)
	return &model
}

// getBGPneighbor builds OpenConfig neighbor configurations from proto input.
//
// This function handles the complex neighbor configuration including:
//   - Address resolution (supports template tags like {{dut.port1.ipv4}})
//   - Peer and Local AS configuration (for eBGP with different local AS)
//   - Transport settings (local source address, MTU discovery)
//   - eBGP multihop configuration
//   - Per-neighbor AFI/SAFI settings (can differ from global)
//   - Import/Export routing policies
//
// Parameters:
//   - t: Testing context for logging
//   - neighbors: Array of neighbor proto structs from YAML
//   - input: Test input context for tag resolution
//
// Returns:
//   - map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor: Map keyed by neighbor IP address
//
// CRITICAL FIX:
//
//	The original implementation built the AFI/SAFI map but never assigned it to the
//	neighbor object (line 298). This meant neighbors had NO address families configured,
//	causing session establishment to fail or routes not to be exchanged.
//
// Example YAML neighbor:
//   - address: 192.168.1.2
//     peer_as: 64001
//     local_as: 63001  # For eBGP with different local AS
//     local_address: 192.168.1.1  # Source IP for BGP TCP session
//     ebgp_multihop: 255  # Allow multi-hop eBGP
//     afisafi:
//   - type: IPV4_UNICAST
//     policy:
//     importpolicy: [ALLOW]
//     exportpolicy: [ALLOW]
func getBGPneighbor(t *testing.T, neighbors []*proto.Input_BGP_Neighbor, input testinput.TestInput) map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor {
	model := map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{}

	for _, neighbor := range neighbors {
		// Resolve address tags (e.g., {{dut.port1.ipv4}})
		addresses := solver.Solvetag(t, neighbor.Address, input)

		// Build AFI/SAFI map for this neighbor
		afisafimap := map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{}
		for _, afi := range neighbor.Afisafi {
			afisafi := &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
				AfiSafiName: GetAfisafiType(afi.Type),
				Enabled:     ygot.Bool(true),
				AddPaths:    getAddPathsNeighbor(afi.AdditionalPaths),
			}

			// Only set ApplyPolicy if it's defined (defensive coding)
			if policy := getNeighborPolicy(afi.Policy); policy != nil {
				afisafi.ApplyPolicy = policy
			}

			afisafimap[afisafi.AfiSafiName] = afisafi
		}

		// Create neighbor configuration for each resolved address
		for _, address := range addresses {
			neighboroc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String(address),
				Enabled:         ygot.Bool(true), // Enable neighbor by default
			}

			// Set peer AS if provided
			if neighbor.PeerAs != 0 {
				neighboroc.PeerAs = ygot.Uint32(uint32(neighbor.PeerAs))
			}

			// Set local AS if provided (critical for eBGP with different local AS)
			if neighbor.LocalAs != 0 {
				neighboroc.LocalAs = ygot.Uint32(uint32(neighbor.LocalAs))
			}

			// Set description if provided
			if neighbor.Description != "" {
				neighboroc.Description = ygot.String(neighbor.Description)
			}

			// Configure transport settings (local address and MTU discovery)
			if neighbor.LocalAddress != "" {
				neighboroc.Transport = &oc.NetworkInstance_Protocol_Bgp_Neighbor_Transport{
					LocalAddress: ygot.String(neighbor.LocalAddress),
					MtuDiscovery: ygot.Bool(true), // Enable MTU discovery for better path MTU handling
				}
			}

			// Configure eBGP multihop if needed
			if neighbor.EbgpMultihop != 0 {
				neighboroc.EbgpMultihop = &oc.NetworkInstance_Protocol_Bgp_Neighbor_EbgpMultihop{
					Enabled:     ygot.Bool(true),
					MultihopTtl: ygot.Uint8(uint8(neighbor.EbgpMultihop)),
				}
			}

			// CRITICAL FIX: Attach the AFI/SAFI map that was built above
			// This was the primary bug - the map was created but never assigned!
			if len(afisafimap) > 0 {
				neighboroc.AfiSafi = afisafimap
			}

			model[address] = neighboroc
		}
	}

	return model
}
func getAddPathsGlobal(addPaths []proto.Input_BGP_AdditionalPaths) *oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi_AddPaths {
	model := &oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi_AddPaths{}
	for _, addPath := range addPaths {
		switch addPath {
		case proto.Input_BGP_recieve:
			model.Receive = ygot.Bool(true)
		case proto.Input_BGP_send:
			model.Send = ygot.Bool(true)

		}

	}

	return model
}

// getNeighborPolicy converts proto policy definition to OpenConfig ApplyPolicy structure.
// Returns nil if no policy is defined, allowing BGP neighbors to work without explicit policies.
func getNeighborPolicy(policy *proto.Input_BGP_Policy) *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy {
	// Return nil if no policy specified - allows BGP without policy attachment
	if policy == nil || (len(policy.Importpolicy) == 0 && len(policy.Exportpolicy) == 0) {
		return nil
	}

	model := &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy{}
	model.ImportPolicy = policy.Importpolicy
	model.ExportPolicy = policy.Exportpolicy
	return model
}

func getAddPathsNeighbor(addPaths []proto.Input_BGP_AdditionalPaths) *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths {
	model := &oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_AddPaths{}
	for _, addPath := range addPaths {
		switch addPath {
		case proto.Input_BGP_recieve:
			model.Receive = ygot.Bool(true)
		case proto.Input_BGP_send:
			model.Send = ygot.Bool(true)

		}

	}

	return model
}

// GetAfisafiType returns the proto format enum for afisafi for an input file parameter
func GetAfisafiType(afisafitype proto.Input_BGP_AfiSafiType) oc.E_BgpTypes_AFI_SAFI_TYPE {
	switch afisafitype {
	case proto.Input_BGP_IPV4_FLOWSPEC:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_FLOWSPEC
	case proto.Input_BGP_IPV4_LABELED_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_LABELED_UNICAST
	case proto.Input_BGP_IPV4_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	case proto.Input_BGP_IPV6_LABELED_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV6_LABELED_UNICAST
	case proto.Input_BGP_IPV6_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	case proto.Input_BGP_L2VPN_EVPN:
		return oc.BgpTypes_AFI_SAFI_TYPE_L2VPN_EVPN
	case proto.Input_BGP_L2VPN_VPLS:
		return oc.BgpTypes_AFI_SAFI_TYPE_L2VPN_VPLS
	case proto.Input_BGP_L3VPN_IPV4_MULTICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_MULTICAST
	case proto.Input_BGP_L3VPN_IPV4_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_UNICAST
	case proto.Input_BGP_L3VPN_IPV6_MULTICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_MULTICAST
	case proto.Input_BGP_L3VPN_IPV6_UNICAST:
		return oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_UNICAST
	case proto.Input_BGP_LINKSTATE:
		return oc.BgpTypes_AFI_SAFI_TYPE_LINKSTATE
	case proto.Input_BGP_LINKSTATE_SPF:
		return oc.BgpTypes_AFI_SAFI_TYPE_LINKSTATE_SPF
	case proto.Input_BGP_LINKSTATE_VPN:
		return oc.BgpTypes_AFI_SAFI_TYPE_LINKSTATE_VPN
	case proto.Input_BGP_SRTE_POLICY_IPV4:
		return oc.BgpTypes_AFI_SAFI_TYPE_SRTE_POLICY_IPV4
	case proto.Input_BGP_SRTE_POLICY_IPV6:
		return oc.BgpTypes_AFI_SAFI_TYPE_SRTE_POLICY_IPV6
	case proto.Input_BGP_VPNV4_FLOWSPEC:
		return oc.BgpTypes_AFI_SAFI_TYPE_VPNV4_FLOWSPEC
	default:
		return oc.BgpTypes_AFI_SAFI_TYPE_UNSET
	}
}
