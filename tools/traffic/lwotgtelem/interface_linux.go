package lwotgtelem

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/openconfig/featureprofiles/tools/traffic/intf"
	"github.com/openconfig/featureprofiles/tools/traffic/otgyang"
	"github.com/openconfig/lemming/gnmi/gnmit"
	"github.com/openconfig/ygot/ygot"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/value"
)

const (
	NLMSG_TYPE_ADDLINK uint16 = 16
	NLMSG_TYPE_DELLINK uint16 = 17
)

func dateTime(_ gnmit.Queue, updateFn gnmit.UpdateFn, target string, cleanup func()) error {
	periodic := func() error {
		currentDatetime, err := value.FromScalar(time.Now().Format(time.RFC3339))
		if err != nil {
			return fmt.Errorf("currentDateTimeTask: %v", err)
		}
		if err := updateFn(&gpb.Notification{
			Timestamp: time.Now().UnixNano(),
			Prefix: &gpb.Path{
				Origin: "openconfig",
				Target: target,
			},
			Update: []*gpb.Update{{
				Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "time"}}},
				Val:  currentDatetime,
			}},
		}); err != nil {
			return err
		}
		return nil
	}

	tick := time.NewTicker(time.Second)
	klog.Infof("starting datetime task.")
	go func() {
		defer cleanup()
		for range tick.C {
			if err := periodic(); err != nil {
				klog.Errorf("dateTime: err: %v", err)
				return
			}
		}
	}()

	return nil

}

func makeARPNeighborTask(ctx context.Context, hints map[string]map[string]string) gnmit.TaskRoutine {
	arpUpdate := func(h net.HardwareAddr, ip net.IP, link, target string, timeFn func() int64) *gpb.Notification {
		s := &otgyang.Device{}
		n := s.GetOrCreateInterface(link).GetOrCreateIpv4Neighbor(ip.String())
		n.LinkLayerAddress = ygot.String(h.String())
		g, err := ygot.TogNMINotifications(s, timeFn(), ygot.GNMINotificationsConfig{UsePathElem: true})
		if err != nil {
			klog.Errorf("cannot serialise, %v", err)
		}
		klog.Infof("update being sent is %s", g[0])
		return addTarget(g[0], target)
	}

	arpTask := func(_ gnmit.Queue, updateFn gnmit.UpdateFn, target string, cleanup func()) error {
		initialMap := func() {
			neighs, err := intf.ARPEntries()
			if err != nil {
				klog.Errorf("returning as can't list neighbours")
				return //
				//fmt.Errorf("arpNeighbors: cannot list neighbours, %v", err)
			}

			if _, ok := hints["interface_map"]; ok {
				for _, n := range neighs {
					linkName := hints["interface_map"][n.Interface]
					if linkName == "" {
						continue
					}
					updateFn(arpUpdate(n.MAC, n.IP, linkName, target, time.Now().UnixNano))
				}
			} else {
				klog.Errorf("arpNeighbors: cannot map with nil interface mapping table.")
			}

		}
		var retErr error
		go func() {
			initialMap()
			ch := make(chan intf.Neighbour, 100)
			if err := intf.ARPUpdates(ctx, ch); err != nil {
				retErr = fmt.Errorf("cannot open ARP update channel, %v", err)
				return
			}
			for {
				select {
				case <-ctx.Done():
					return
				case u := <-ch:
					linkName := hints["interface_map"][u.Interface]
					// TODO(robjs): be smarter here - we need to do a diff.
					initialMap()
					klog.Infof("updating with %v -> linkName: %s, %v\n", u, linkName, hints)
					if linkName != "" {
						if err := updateFn(arpUpdate(u.MAC, u.IP, linkName, target, time.Now().UnixNano)); err != nil {
							klog.Errorf("got error sending ARP update, %v", err)
						}
					}
				}
			}
		}()

		return retErr
	}

	klog.Infof("returning ARP task...")
	return arpTask
}

func interfaceState(_ gnmit.Queue, updateFn gnmit.UpdateFn, target string, cleanup func()) error {
	var (
		linkUpdCh chan netlink.LinkUpdate
		linkDone  chan struct{}
	)

	links, err := intf.GetInterfaces()
	if err != nil {
		return fmt.Errorf("interfaceState: cannot list links, %v")
	}

	for _, l := range links {
		n, err := linkAttrUpdate(l.Attrs(), target, time.Now().UnixNano)
		if err != nil {
			return fmt.Errorf("interfaceState: can't map attrs for %s, %v", l.Attrs().Name, err)
		}
		klog.Infof("sending %s\n", n)
		if err := updateFn(n); err != nil {
			return fmt.Errorf("can't send initial update for link %s, %v", l.Attrs().Name, err)
		}
	}

	if err := netlink.LinkSubscribe(linkUpdCh, linkDone); err != nil {
		klog.Errorf("unable to subscribe to link state, %v", err)
		return fmt.Errorf("interfaceState: cannot open netlink channel, %v", err)
	}

	go func() {
		defer func() {
			linkDone <- struct{}{} // close the channel to read from netlink.
			cleanup()
		}()
		klog.Infof("starting interfaceState task...")

		for {
			select {
			case upd := <-linkUpdCh:
				klog.Infof("received update from kernel, %v", upd)
				n, err := processLinkUpdate(upd, target, time.Now().UnixNano)
				if err != nil {
					klog.Errorf("interfaceStats task, cannot generate update: %v", err)
					return
				}
				klog.Infof("sending %s\n", n)
				if err := updateFn(n); err != nil {
					// fatal error for the task - should likely log here.
					klog.Errorf("interfaceState task, cannot update: %v", err)
					return
				}
			}
		}
	}()
	return nil
}

var (
	operStatusMap = map[netlink.LinkOperState]otgyang.E_Port_Link{
		netlink.OperUnknown: otgyang.Port_Link_UNSET,
		netlink.OperUp:      otgyang.Port_Link_UP,
		netlink.OperDown:    otgyang.Port_Link_DOWN,
	}
)

// Process link update handles netlink events for interface addition and removal.
func processLinkUpdate(u netlink.LinkUpdate, target string, timeFn func() int64) (*gpb.Notification, error) {
	attrs := u.Attrs()
	if u.Header.Type == NLMSG_TYPE_DELLINK {
		return &gpb.Notification{
			Timestamp: timeFn(),
			Delete: []*gpb.Path{{
				Origin: "openconfig",
				Elem: []*gpb.PathElem{{
					Name: "ports",
				}, {
					Name: "port",
					Key: map[string]string{
						"name": attrs.Name,
					},
				}},
			}},
		}, nil

	}
	return linkAttrUpdate(attrs, target, timeFn)
}

func linkAttrUpdate(attrs *netlink.LinkAttrs, target string, timeFn func() int64) (*gpb.Notification, error) {
	d := &otgyang.Device{} // start at the root to allow for full paths in marshalling.
	upd := d.GetOrCreatePort(attrs.Name)
	upd.Link = operStatusMap[attrs.OperState]
	// OTG does not let us currently report:
	//  - admin status
	//  - mac address

	ns, err := ygot.TogNMINotifications(d, timeFn(), ygot.GNMINotificationsConfig{
		UsePathElem: true,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot generate gNMI notifications, %v", err)
	}

	if len(ns) == 0 {
		return nil, fmt.Errorf("no notifications returned from update", err)
	}
	for _, n := range ns {
		addTarget(n, target)
	}
	return ns[0], nil
}

func addTarget(n *gpb.Notification, target string) *gpb.Notification {
	if n.Prefix == nil {
		n.Prefix = &gpb.Path{}
	}
	n.Prefix.Target = target
	n.Prefix.Origin = "openconfig"
	return n
}
