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
	"google.golang.org/protobuf/encoding/prototext"
	"k8s.io/klog/v2"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

// Hint is a grouped key, value that is used to store metadata that is passed between
// elements of the server.
type Hint struct {
	Group    string
	Key, Val string
}

// New returns a new lightweight OTG server.
func New() *Server {
	s := &Server{
		intf: map[string]*linuxIntf{},
	}

	s.AddConfigHandler(s.baseInterfaceConfig)
	return s
}

// TXRXFn is a function that controls a specific flow. It should begin TX and RX of a flow when it
// is called. The arguments are a FlowListener per direction (Tx, Rx) that can be used by the spawned
// goroutines. They should usethe supplied empty struct channel to determine when to exit. It can write telemetry
// to the the supplied *gpb.Update channel, and errors to the supplied error channel.
type TXRXFn func(tx *FlowListener, rx *FlowListener)

// FlowGeneratorFn is a function that takes an input *otg.Flow and determines whether it can generate
// the flow. If it is able to it returns a TXRXFn, and a bool that indicates that it handled the flow
// (with true representing that it was handled). If it was unable to handle to flow, it returns no
// generator function and a bool indicating that it was unhandled (false). If any error occurs whilst
// generating the flow, it is returned.
//
// The arguments to a FlowGeneratorFn are:
//   - *otg.Flow - the flow that is being requested.
//   - map[string]string - a mapping of interfaces to their specified name in
//     the OTG configuration.
type FlowGeneratorFn func(*otg.Flow, map[string]string) (TXRXFn, bool, error)

type FlowListener struct {
	Stop chan struct{}
	GNMI chan *gpb.Update
	Err  chan error
}

// Server implements the OTG ("Openapi") server.
type Server struct {
	*otg.UnimplementedOpenapiServer

	intfMu sync.Mutex
	intf   map[string]*linuxIntf

	// hintCh is a channel that is used to sent Hints to other elements
	// of the OTG system - particularly, it is used to send hints that are needed
	// in the telemetry daemon.
	hintCh chan Hint

	// ProtocolHandler is a function called when the OTG SetProtocolState RPC
	// is called. It is used to ensure that anything that needs to be done in the
	// underlying system is performed (e.g., sending ARP responses).
	protocolHandler func(*otg.Config, otg.ProtocolState_State_Enum) error

	chMu sync.Mutex
	// configHandlers are a set of methods that are called to process the incoming
	// OTG configuration. This allows LWOTG to be extended to cover new config that
	// the base implementation does not cover.
	configHandlers []func(*otg.Config) error

	fhMu sync.Mutex
	// flowHandlers are a set of methods that are called to process specifically
	// configuration that relates to flows. Flows are special cased because they
	// return methods to start those flows.
	flowHandlers []FlowGeneratorFn

	tgMu sync.Mutex
	// trafficGenFuncs are a set of methods that are called to generate traffic
	// when set transmit state is set to true, and cancelled when it is completed.
	trafficGenFuncs []TXRXFn

	trafficListeners []*FlowListener

	cfg *otg.Config
}

// SetHintChannel sets the hint channel to the specified channel.
func (s *Server) SetHintChannel(ch chan Hint) {
	s.hintCh = ch
}

// AddFlowHandler adds a new flow handling function to the server, which generates the
// relevant methods to generate that flow.
func (s *Server) AddFlowHandler(fn FlowGeneratorFn) {
	s.fhMu.Lock()
	defer s.fhMu.Unlock()
	s.flowHandlers = append(s.flowHandlers, fn)
}

// SetProtocolHandler adds a function that is called when Start/Stop protocols is called.
func (s *Server) SetProtocolHandler(fn func(*otg.Config, otg.ProtocolState_State_Enum) error) {
	s.protocolHandler = fn
}

// AddConfigHandler adds fn to the set of configuration handler methods.
func (s *Server) AddConfigHandler(fn func(*otg.Config) error) {
	s.configHandlers = append(s.configHandlers, fn)
}

func (s *Server) cacheInterfaces(v map[string]*linuxIntf) {
	s.intfMu.Lock()
	defer s.intfMu.Unlock()
	s.intf = v
}

func (s *Server) intfHasAddr(name, addr string) bool {
	s.intfMu.Lock()
	defer s.intfMu.Unlock()
	v, ok := s.intf[name]
	if !ok {
		return false
	}
	if a, ok := v.IPv4[addr]; ok {
		return a.Configured
	}
	return false
}

func (s *Server) setAddrConfigured(name, addr string) {
	s.intfMu.Lock()
	defer s.intfMu.Unlock()
	v, ok := s.intf[name]
	if !ok {
		return
	}
	a, ok := v.IPv4[addr]
	if !ok {
		return
	}
	a.Configured = true
}

func (s *Server) intfMap() map[string]string {
	s.intfMu.Lock()
	defer s.intfMu.Unlock()

	r := map[string]string{}
	for linuxName, v := range s.intf {
		r[v.OTGName] = linuxName
	}
	return r
}

// SetConfig allows the configuration to be set on the server.
func (s *Server) SetConfig(ctx context.Context, req *otg.SetConfigRequest) (*otg.SetConfigResponse, error) {
	if req.Config == nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request configuration received, %v", req)
	}

	klog.Infof("got config %s\n", req)
	if s.hintCh != nil {
		select {
		case s.hintCh <- Hint{Group: "meta", Key: "SetConfig", Val: prototext.Format(req)}:
		default:
		}
	}

	for _, fn := range s.configHandlers {
		if err := fn(req.Config); err != nil {
			return nil, err
		}
	}

	flowMethods := []TXRXFn{}
	for _, flow := range req.GetConfig().GetFlows() {
		var handled bool
		for _, fn := range s.flowHandlers {
			txrx, ok, err := fn(flow, s.intfMap())
			if err != nil {
				if ok {
					// This was a flow that was handled, but an error occurred.
					return nil, status.Errorf(codes.Internal, "error generating flows, %v", err)
				}
				klog.Infof("Flow was not handled by function, %v", err)
				continue
			}
			flowMethods = append(flowMethods, txrx)
			klog.Infof("flow %s was handled.", flow)
			handled = true
			break
		}
		if !handled {
			return nil, status.Errorf(codes.Unimplemented, "no handler for flow %s", flow)
		}
	}

	s.tgMu.Lock()
	s.trafficGenFuncs = flowMethods
	s.tgMu.Unlock()

	s.cfg = req.Config

	return &otg.SetConfigResponse{StatusCode_200: &otg.ResponseWarning{ /* WTF, who knows?  */ }}, nil
}

func (s *Server) SetProtocolState(ctx context.Context, req *otg.SetProtocolStateRequest) (*otg.SetProtocolStateResponse, error) {
	klog.Infof("Setting protocol state requested, %v", req)
	if err := s.protocolHandler(s.cfg, req.GetProtocolState().GetState()); err != nil {
		return nil, err
	}

	return &otg.SetProtocolStateResponse{StatusCode_200: &otg.ResponseWarning{}}, nil
}

func (s *Server) SetTransmitState(ctx context.Context, req *otg.SetTransmitStateRequest) (*otg.SetTransmitStateResponse, error) {
	klog.Infof("Setting traffic state requested, %v", req)

	switch req.GetTransmitState().GetState() {
	case otg.TransmitState_State_start:
		if err := s.startTraffic(); err != nil {
			return nil, status.Errorf(codes.Internal, "cannot start traffic, %v", err)
		}
	case otg.TransmitState_State_stop:
		klog.Infof("stopping traffic...")
		for i, l := range s.trafficListeners {
			klog.Infof("stopping traffic listener %d...", i)
			l.Stop <- struct{}{}
		}
	default:
		return nil, status.Errorf(codes.Unimplemented, "states other than start and stop unimplemented, got %s", req.GetTransmitState().State)
	}

	return &otg.SetTransmitStateResponse{StatusCode_200: &otg.ResponseWarning{}}, nil
}

func (s *Server) startTraffic() error {
	s.trafficListeners = []*FlowListener{}
	s.tgMu.Lock()
	defer s.tgMu.Unlock()
	klog.Info("starting traffic...")
	for _, fn := range s.trafficGenFuncs {
		// TODO(robjs): Make New()
		tx := &FlowListener{
			Stop: make(chan struct{}, 2),
			GNMI: make(chan *gpb.Update),
			Err:  make(chan error),
		}
		rx := &FlowListener{
			Stop: make(chan struct{}, 2),
			GNMI: make(chan *gpb.Update),
			Err:  make(chan error),
		}
		go logger(tx.GNMI, tx.Err)
		go logger(rx.GNMI, rx.Err)
		go fn(tx, rx)
		s.trafficListeners = append(s.trafficListeners, []*FlowListener{tx, rx}...)
		klog.Infof("started listener number %d", len(s.trafficListeners))
	}
	return nil
}

// TODO(robjs): remove as this leaks goroutines.
func logger(updCh chan *gpb.Update, errorCh chan error) {
	for {
		select {
		case upd := <-updCh:
			klog.Infof("got update from traffic: %v", upd)
		case err := <-errorCh:
			klog.Errorf("traffic error, %v", err)
		}
	}
	return
}

func (s *Server) baseInterfaceConfig(pb *otg.Config) error {
	// Working with gosnappi here seems worse than just using the proto directly.
	// gsCfg := gosnappi.NewConfig().SetMsg(pb)

	ifCfg, ethMap, err := portsToLinux(pb.Ports, pb.Devices)
	if err != nil {
		return err
	}

	s.cacheInterfaces(ifCfg)

	if s.hintCh != nil {
		for linuxIf, ethName := range ethMap {
			klog.Infof("sending hint %s -> %s", linuxIf, ethName)
			select {
			case s.hintCh <- Hint{Group: "interface_map", Key: linuxIf, Val: ethName}:
			default:
			}
		}
	}

	for intName, cfg := range ifCfg {
		if !intf.ValidInterface(intName) {
			return status.Errorf(codes.Internal, "interface %s is not configrable, %v", intName, err)
		}

		for addr, details := range cfg.IPv4 {
			mask := details.Mask
			_, ipNet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", addr, mask))
			if err != nil {
				return status.Errorf(codes.InvalidArgument, "invalid prefix %s/%d for interface %s, err: %v", addr, mask, intName, err)
			}

			// Avoid configuring an address on an interface that already has the address.
			// TODO(robjs): Handle deconfiguring IPs.
			if !s.intfHasAddr(intName, addr) {
				klog.Infof("Configuring interface %s with address %s", intName, ipNet)
				if err := intf.AddIP(intName, ipNet); err != nil {
					return status.Errorf(codes.Internal, "cannot configure address %s on interface %s, err: %v", addr, intName, err)
				}
				s.setAddrConfigured(intName, addr)
			}
		}
	}

	// Send ARP responses for the IP addresses we just configured.
	intf.SendARP(false)

	return nil
}

type address struct {
	Mask       int
	Configured bool
}

// linuxIntf describes the configuration of a specific interface in Linux.
type linuxIntf struct {
	// IPv4 is a map containing the IPv4 addresses to be configured
	// on the interface and the mask used for them.
	IPv4 map[string]*address
	// OTGName specifies the name of the interface in OTG.
	OTGName    string
	Configured bool
}

// portsToLinux takes an input set of ports in an OTG configuration and returns the information
// required to configure them on a Linux host.
func portsToLinux(ports []*otg.Port, devices []*otg.Device) (map[string]*linuxIntf, map[string]string, error) {
	physIntf := map[string]string{}
	ethMap := map[string]string{}
	for _, p := range ports {
		if p.Location == nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "invalid interface %s, does not specify a port location", p.Name)
		}
		// Location contains the name of the interface of the form 'eth0'.
		physIntf[p.Name] = *p.Location
	}

	retIntf := map[string]*linuxIntf{}
	for _, d := range devices {
		for _, e := range d.Ethernets {
			if e.GetPortName() == "" {
				return nil, nil, status.Errorf(codes.InvalidArgument, "invalid ethernet port %v, does not specify a name", e)
			}
			n, ok := physIntf[*e.PortName]
			if !ok {
				return nil, nil, status.Errorf(codes.InvalidArgument, "invalid port name for Ethernet %s, does not map to a real interface", *e.PortName)
			}

			ethMap[n] = e.Name
			retIntf[n] = &linuxIntf{OTGName: *e.PortName, IPv4: map[string]*address{}}

			for _, a := range e.Ipv4Addresses {
				if a.GetPrefix() == 0 {
					return nil, nil, status.Errorf(codes.InvalidArgument, "unsupported zero prefix length for address %s", a.Address)
				}
				retIntf[n].IPv4[a.Address] = &address{Mask: int(a.GetPrefix())}
			}
		}
	}

	return retIntf, ethMap, nil
}
