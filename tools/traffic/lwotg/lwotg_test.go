package lwotg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi/otg"
	"google.golang.org/protobuf/proto"
)

func TestPortsToLinux(t *testing.T) {
	tests := []struct {
		desc      string
		inPorts   []*otg.Port
		inDevices []*otg.Device
		want      map[string]*linuxIntf
		wantErr   bool
	}{{
		desc: "port with no location",
		inPorts: []*otg.Port{{
			Name: "eth1",
		}},
		wantErr: true,
	}, {
		desc: "port with location, no devices",
		inPorts: []*otg.Port{{
			Name:     "port42",
			Location: proto.String("ethernet32"),
		}},
		want: map[string]*linuxIntf{},
	}, {
		desc: "port with a single Ethernet device",
		inPorts: []*otg.Port{{
			Name:     "port1",
			Location: proto.String("eth1"),
		}},
		inDevices: []*otg.Device{{
			Ethernets: []*otg.DeviceEthernet{{
				PortName: proto.String("port1"),
				Ipv4Addresses: []*otg.DeviceIpv4{{
					Address: "192.0.2.1",
					Prefix:  proto.Int32(32),
				}},
			}},
		}},
		want: map[string]*linuxIntf{
			"eth1": {
				IPv4: map[string]int{
					"192.0.2.1": 32,
				},
			},
		},
	}, {
		desc: "port that is not mapped to hardware",
		inPorts: []*otg.Port{{
			Name:     "port1",
			Location: proto.String("eth2"),
		}},
		inDevices: []*otg.Device{{
			Ethernets: []*otg.DeviceEthernet{{
				PortName: proto.String("port42"),
			}},
		}},
		wantErr: true,
	}, {
		desc: "port that does not have a port name",
		inPorts: []*otg.Port{{
			Name:     "port1",
			Location: proto.String("eth2"),
		}},
		inDevices: []*otg.Device{{
			Ethernets: []*otg.DeviceEthernet{{
				PortName: proto.String("port1"),
				Ipv4Addresses: []*otg.DeviceIpv4{{
					Address: "192.0.2.2",
				}},
			}},
		}},
		wantErr: true,
	}, {
		desc: "invalid prefix length",
		inPorts: []*otg.Port{{
			Name:     "port1",
			Location: proto.String("eth2"),
		}},
		inDevices: []*otg.Device{{
			Ethernets: []*otg.DeviceEthernet{{
				PortName: proto.String("port1"),
				Ipv4Addresses: []*otg.DeviceIpv4{{
					Address: "192.0.2.2",
				}},
			}},
		}},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := portsToLinux(tt.inPorts, tt.inDevices)
			if (err != nil) != tt.wantErr {
				t.Fatalf("did not get expected error, got: %v, wantErr? %v", got, tt.wantErr)
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatalf("did not get expected ports, diff(-got,+want):\n%s", diff)
			}
		})
	}
}
