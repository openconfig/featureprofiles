package lwotg

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/open-traffic-generator/snappi/gosnappi/otg"
	"github.com/openconfig/featureprofiles/tools/traffic/intf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// New returns a new lightweight OTG server.
func New() *Server {
	return &Server{
		intfMu: map[string]*linuxIntf{},
	}
}

// Server implements the OTG ("Openapi") server.
type Server struct {
	*otg.UnimplementedOpenapiServer

	intfMu sync.Mutex
	intf   map[string]*linuxIntf
}

func (s *Server) cacheInterfaces(v map[string]*linuxIntf) {
	s.intfMu.Lock()
	defer s.intfMu.Unlock()
	s.intf = v
}

func (s *Server) intfHasAddr(name, addr string) bool {
	s.intfMu.Lock()
	defer s.intfMu.Unlock()
	v, ok := s.intfMu[name]
	if !ok {
		return false
	}
	_, configured := v[addr]
	return configured
}

// SetConfig allows the configuration to be set on the server.
func (s *Server) SetConfig(ctx context.Context, req *otg.SetConfigRequest) (*otg.SetConfigResponse, error) {
	if req.Config == nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request configuration received, %v", req)
	}

	if len(req.Config.Lags) != 0 || len(req.Config.Layer1) != 0 || len(req.Config.Captures) != 0 || req.Config.Options != nil {
		return nil, status.Errorf(codes.Unimplemented, "request contained fields that are unimplemented, %v", req)
	}
	return handleConfig(req.Config)
}

func (s *Server) SetProtocolState(ctx context.Context, req *otg.SetProtocolStateRequest) (*otg.SetProtocolStateResponse, error) {
	klog.Infof("Setting protocol state requested, %v", req)
	return &otg.SetProtocolStateResponse{StatusCode_200: &otg.ResponseWarning{}}, nil
}

func (s *Server) SetTransmitState(ctx context.Context, req *otg.SetTransmitStateRequest) (*otg.SetTransmitStateResponse, error) {
	klog.Infof("Setting traffic state requested, %v", req)
	return &otg.SetTransmitStateResponse{StatusCode_200: &otg.ResponseWarning{}}, nil
}

func handleConfig(pb *otg.Config) (*otg.SetConfigResponse, error) {
	// Working with gosnappi here seems worse than just using the proto directly.
	// gsCfg := gosnappi.NewConfig().SetMsg(pb)

	ifCfg, err := portsToLinux(pb.Ports, pb.Devices)
	if err != nil {
		return nil, err
	}

	for intName, cfg := range ifCfg {
		if !intf.ValidInterface(intName) {
			return nil, status.Errorf(codes.Internal, "interface %s is not configrable, %v", intName, err)
		}

		for addr, mask := range cfg.IPv4 {
			_, ipNet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", addr, mask))
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid prefix %s/%d for interface %s, err: %v", addr, mask, intName, err)
			}

			// Avoid configuring an address on an interface that already has the address.
			if !intfHasAddr(intName, addr) {
				klog.Infof("Configuring interface %s with address %s", intName, ipNet)
				if err := intf.AddIP(intName, ipNet); err != nil {
					return nil, status.Errorf(codes.Internal, "cannot configure address %s on interface %s, err: %v", addr, intName, err)
				}
			}
		}
	}

	return &otg.SetConfigResponse{StatusCode_200: &otg.ResponseWarning{ /* WTF, who knows?  */ }}, nil
}

// linuxIntf describes the configuration of a specific interface in Linux.
type linuxIntf struct {
	// IPv4 is a map containing the IPv4 addresses to be configured
	// on the interface and the mask used for them.
	IPv4 map[string]int
}

// portsToLinux takes an input set of ports in an OTG configuration and returns the information
// required to configure them on a Linux host.
func portsToLinux(ports []*otg.Port, devices []*otg.Device) (map[string]*linuxIntf, error) {
	physIntf := map[string]string{}
	for _, p := range ports {
		if p.Location == nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid interface %s, does not specify a port location", p.Name)
		}
		// Location contains the name of the interface of the form 'eth0'.
		physIntf[p.Name] = *p.Location
	}

	retIntf := map[string]*linuxIntf{}
	for _, d := range devices {
		for _, e := range d.Ethernets {
			if e.GetPortName() == "" {
				return nil, status.Errorf(codes.InvalidArgument, "invalid ethernet port %v, does not specify a name", e)
			}
			n, ok := physIntf[*e.PortName]
			if !ok {
				return nil, status.Errorf(codes.InvalidArgument, "invalid port name for Ethernet %s, does not map to a real interface", *e.PortName)
			}
			retIntf[n] = &linuxIntf{IPv4: map[string]int{}}

			for _, a := range e.Ipv4Addresses {
				if a.GetPrefix() == 0 {
					return nil, status.Errorf(codes.InvalidArgument, "unsupported zero prefix length for address %s", a.Address)
				}
				retIntf[n].IPv4[a.Address] = int(a.GetPrefix())
			}
		}
	}

	return retIntf, nil
}
