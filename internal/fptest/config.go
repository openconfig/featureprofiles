package fptest

import (
	"flag"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

var (
	// Some devices require the config to be pruned for these to work.  We are still undecided
	// whether they should be deviations; pending OpenConfig clarifications.
	pruneComponents      = flag.Bool("prune_components", true, "Prune components that are not ports.  Use this to preserve the breakout-mode settings.")
	pruneLLDP            = flag.Bool("prune_lldp", true, "Prune LLDP config.")
	setEthernetFromState = flag.Bool("set_ethernet_from_state", true, "Set interface/ethernet config from state, mostly to get the port-speed settings correct.")

	// This has no known effect except to reduce logspam while debugging.
	pruneQoS = flag.Bool("prune_qos", true, "Prune QoS config.")

	// Experimental flags that will likely become a deviation.
	cannotConfigurePortSpeed = flag.Bool("cannot_config_port_speed", false, "Some devices depending on the type of line card may not allow changing port speed, while still supporting the port speed leaf.")

	// Flags to ensure test passes without any dependency to the device config
	baseOCConfigIsPresent = flag.Bool("base_oc_config_is_present", false, "No OC config is loaded on router, so Get config on the root returns no data.")
)

// GetDeviceConfig gets a full config from a device but refurbishes it enough so it can be
// pushed out again.  Ideally, we should be able to push the config we get from the same
// device without modification, but this is not explicitly defined in OpenConfig.
func GetDeviceConfig(t testing.TB, dev gnmi.DeviceOrOpts) *oc.Root {
	t.Helper()

	// Gets all the config (read-write) paths from root, not the state (read-only) paths.
	config := gnmi.Get[*oc.Root](t, dev, gnmi.OC().Config())
	WriteQuery(t, "Untouched", gnmi.OC().Config(), config)

	// load the base oc config from the device state when no oc config is loaded
	if !*baseOCConfigIsPresent {
		if ondatra.DUT(t, "dut").Vendor() == ondatra.CISCO {
			intfsState := gnmi.GetAll(t, dev, gnmi.OC().InterfaceAny().State())
			for _, intf := range intfsState {
				ygot.PruneConfigFalse(oc.SchemaTree["Interface"], intf)
				config.DeleteInterface(intf.GetName())
				if intf.GetName() == "Loopback0" || intf.GetName() == "PTP0/RP1/CPU0/0" || intf.GetName() == "Null0" || intf.GetName() == "PTP0/RP0/CPU0/0" {
					continue
				}
				intf.ForwardingViable = nil
				intf.Mtu = nil
				intf.HoldTime = nil
				for _, sub := range intf.Subinterface {
					if sub.Ipv6 != nil {
						sub.Ipv6.Autoconf = nil
						if adv := sub.Ipv6.GetRouterAdvertisement(); adv != nil {
							adv.Suppress = nil
							for _, p := range sub.Ipv6.GetRouterAdvertisement().Prefix {
								p.DisableAdvertisement = nil
							}
						}
					}
				}
				config.AppendInterface(intf)
			}
			vrfsStates := gnmi.GetAll(t, dev, gnmi.OC().NetworkInstanceAny().State())
			for _, vrf := range vrfsStates {
				// only needed for containerOp
				if vrf.GetName() == "**iid" {
					continue
				}
				if vrf.GetName() == "DEFAULT" {
					config.NetworkInstance = nil
					vrf.Interface = nil
					for _, ni := range config.NetworkInstance {
						ni.Mpls = nil
					}
				}
				ygot.PruneConfigFalse(oc.SchemaTree["NetworkInstance"], vrf)
				vrf.Table = nil
				vrf.RouteLimit = nil
				vrf.Mpls = nil
				for _, intf := range vrf.Interface {
					intf.AssociatedAddressFamilies = nil
				}
				for _, protocol := range vrf.Protocol {
					for _, routes := range protocol.Static {
						routes.Description = nil
					}
				}
				config.AppendNetworkInstance(vrf)
			}
		}
	}

	if *pruneComponents {
		for cname, component := range config.Component {
			// Keep the port components in order to preserve the breakout-mode config.
			if component.GetPort() == nil {
				delete(config.Component, cname)
				continue
			}
			// Need to prune subcomponents that may have a leafref to a component that was
			// pruned.
			component.Subcomponent = nil
		}
	}

	if *setEthernetFromState {
		for iname, iface := range config.Interface {
			if iface.GetEthernet() == nil {
				continue
			}
			// Ethernet config may not contain meaningful values if it wasn't explicitly
			// configured, so use its current state for the config, but prune non-config leaves.
			e := iface.GetEthernet()
			if len(iface.GetHardwarePort()) != 0 {
				breakout := config.GetComponent(iface.GetHardwarePort()).GetPort().GetBreakoutMode()
				// Set port speed to unknown for non breakout interfaces
				if breakout.GetGroup(1) == nil && e != nil {
					e.SetPortSpeed(oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN)
				}
			}
			ygot.PruneConfigFalse(oc.SchemaTree["Interface_Ethernet"], e)
			if e.PortSpeed != 0 && e.PortSpeed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
				iface.Ethernet = e
			}
			// need to set mac address for mgmt interface to nil
			if iname == "MgmtEth0/RP0/CPU0/0" || iname == "MgmtEth0/RP1/CPU0/0" && deviations.SkipMacaddressCheck(ondatra.DUT(t, "dut")) {
				e.MacAddress = nil
			}
			// need to set mac address for bundle interface to nil
			if iface.Ethernet.AggregateId != nil && deviations.SkipMacaddressCheck(ondatra.DUT(t, "dut")) {
				iface.Ethernet.MacAddress = nil
				continue
			}
		}
	}

	if !*cannotConfigurePortSpeed {
		for _, iface := range config.Interface {
			if iface.GetEthernet() == nil {
				continue
			}
			iface.GetEthernet().PortSpeed = oc.IfEthernet_ETHERNET_SPEED_UNSET
			iface.GetEthernet().DuplexMode = oc.Ethernet_DuplexMode_UNSET
			iface.GetEthernet().EnableFlowControl = nil
		}
	}

	if *pruneLLDP && config.Lldp != nil {
		config.Lldp.ChassisId = nil
		config.Lldp.ChassisIdType = oc.Lldp_ChassisIdType_UNSET
	}

	if *pruneQoS {
		config.Qos = nil
	}

	pruneUnsupportedPaths(config)

	WriteQuery(t, "Touched", gnmi.OC().Config(), config)
	return config
}

// CopyDeviceConfig returns a deep copy of a device config but refurbishes it enough so it can be
// pushed out again
func CopyDeviceConfig(t testing.TB, dut *ondatra.DUTDevice, config *oc.Root) *oc.Root {
	if deviations.SkipMacaddressCheck(dut) {
		*setEthernetFromState = false
	}

	o, err := ygot.DeepCopy(config)
	if err != nil {
		t.Fatalf("Cannot copy baseConfig: %v", err)
	}

	copyConfig := o.(*oc.Root)

	if *setEthernetFromState {
		setEthernetFromBase(t, config, copyConfig)
	}

	return copyConfig
}

func pruneUnsupportedPaths(config *oc.Root) {
	for _, ni := range config.NetworkInstance {
		ni.Fdb = nil
	}
}

// setEthernetFromBase merges the ethernet config from the interfaces in base config into
// the destination config.
func setEthernetFromBase(t testing.TB, base *oc.Root, config *oc.Root) {
	t.Helper()

	for iname, iface := range config.Interface {
		eb := base.GetInterface(iname).GetEthernet()
		ec := iface.GetOrCreateEthernet()
		if eb == nil || ec == nil {
			continue
		}
		if err := ygot.MergeStructInto(ec, eb); err != nil {
			t.Errorf("Cannot merge %s ethernet: %v", iname, err)
		}
	}
}
