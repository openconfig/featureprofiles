// Package intf defines mechanisms to interface with an underlying interface
// of a container for use in a traffic generator.
package intf

import (
	"context"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

const (
	// RTM_NEWNEIGH is the event sent from netlink when an ARP entry is added to
	// the ARP table.
	RTM_NEWNEIGH uint16 = 28
)

// getLink is an internal implementation to retrieve an interface based on its name.
func getLink(name string) (netlink.Link, error) {
	h, err := netlink.NewHandle()
	if err != nil {
		return nil, fmt.Errorf("cannot open handle, err: %v")
	}

	link, err := h.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("cannot find link %s, err: %v", name, err)
	}

	return link, nil
}

// ValidInterface determines whether the interface 'intf' is a valid interface
// on the local system.
func ValidInterface(name string) bool {
	if _, err := getLink(name); err != nil {
		return false
	}
	return true
}

// AddIP configures IP address addr on interface intf, returning an error if this
// is not possible.
func AddIP(intf string, addr *net.IPNet) error {
	l, err := getLink(intf)
	if err != nil {
		return fmt.Errorf("error finding link, %v", err)
	}

	// Configure address on link.
	if err := netlink.AddrAdd(l, &netlink.Addr{IPNet: addr}); err != nil {
		return fmt.Errorf("cannot add address, %v", err)
	}

	return nil
}

// AwaitARP retrives the MAC address for the specified IP. If the MAC is not
// currently known, it will block until it has been resolved. If the context that is
// passed into the function includes a timeout, then the function will return after
// the timeout expires.
func AwaitARP(ctx context.Context, addr net.IP) (net.HardwareAddr, error) {
	// Check that the ARP entry isn't already there before starting.
	neighs, err := netlink.NeighList(0, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot get ARP table, err: %v", err)
	}

	for _, n := range neighs {
		if n.IP.To4().Equal(addr) {
			return n.HardwareAddr, nil
		}
	}

	ch := make(chan net.HardwareAddr, 1)
	arpCh := make(chan netlink.NeighUpdate)
	doneCh := make(chan struct{})
	go func(arpCh chan netlink.NeighUpdate, doneCh chan struct{}) {
		for {
			select {
			case upd := <-arpCh:
				if upd.Type == RTM_NEWNEIGH {
					if upd.Neigh.IP.Equal(addr) {
						ch <- upd.Neigh.HardwareAddr
					}
				}
			case <-doneCh:
				return
			}
		}
	}(arpCh, doneCh)

	if err := netlink.NeighSubscribe(arpCh, doneCh); err != nil {
		return nil, fmt.Errorf("cannot open neighbour subscription, err: %v", err)
	}

	select {
	case mac := <-ch:
		if mac == nil {
			return nil, fmt.Errorf("cannot resolve MAC address")
		}
		return mac, nil
	case <-ctx.Done():
		doneCh <- struct{}{}
		return nil, ctx.Err()
	}
}

func GetInterfaces() ([]netlink.Link, error) {
	h, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("can't open netlink handle, %v", err)
	}

	links, err := h.LinkList()
	if err != nil {
		return nil, fmt.Errorf("can't list links, %v", err)
	}

	return links, nil
}

type Neighbour struct {
	IP        net.IP
	Interface string
	MAC       net.HardwareAddr
}

func ARPUpdates(ctx context.Context, inform chan Neighbour) error {
	intfIndex, err := intfCache()
	if err != nil {
		return err
	}

	arpCh := make(chan netlink.NeighUpdate)
	doneCh := make(chan struct{})
	go func(arpCh chan netlink.NeighUpdate) {
		for {
			select {
			case upd := <-arpCh:
				klog.Infof("got arp update %v", upd)
				//select {
				//case
				inform <- toNeigh(upd.Neigh, intfIndex)
				klog.Infof("wrote ARP update.")
				//default:
				//}
			}
		}
	}(arpCh)

	if err := netlink.NeighSubscribe(arpCh, doneCh); err != nil {
		return err
	}

	return nil
}

func intfCache() (map[int]string, error) {
	interfaces, err := GetInterfaces()
	if err != nil {
		return nil, fmt.Errorf("cannot get interface list, err: %v", err)
	}

	intfIndex := map[int]string{}
	for _, i := range interfaces {
		attrs := i.Attrs()
		intfIndex[attrs.Index] = attrs.Name
	}
	return intfIndex, nil
}

func ARPEntries() ([]Neighbour, error) {
	intfIndex, err := intfCache()
	if err != nil {
		return nil, err
	}
	neighs, err := netlink.NeighList(0, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot get ARP table, err: %v", err)
	}

	rn := []Neighbour{}
	for _, n := range neighs {
		rn = append(rn, toNeigh(n, intfIndex))
	}
	return rn, nil
}

func toNeigh(n netlink.Neigh, intfIndex map[int]string) Neighbour {
	return Neighbour{
		IP:        n.IP,
		MAC:       n.HardwareAddr,
		Interface: intfIndex[n.LinkIndex],
	}
}

// GetMAC retrieves the hardware address for the interface named intf.
func GetMAC(intf string) (net.HardwareAddr, error) {
	l, err := getLink(intf)
	if err != nil {
		return nil, fmt.Errorf("cannot get MAC address for interface %s, err: %v", intf, err)
	}
	return l.Attrs().HardwareAddr, nil
}
