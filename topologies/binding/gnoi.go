// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package binding

import (
	"github.com/openconfig/ondatra/binding"
	"google.golang.org/grpc"

	bpb "github.com/openconfig/gnoi/bgp"
	cpb "github.com/openconfig/gnoi/cert"
	dpb "github.com/openconfig/gnoi/diag"
	frpb "github.com/openconfig/gnoi/factory_reset"
	fpb "github.com/openconfig/gnoi/file"
	hpb "github.com/openconfig/gnoi/healthz"
	lpb "github.com/openconfig/gnoi/layer2"
	mpb "github.com/openconfig/gnoi/mpls"
	ospb "github.com/openconfig/gnoi/os"
	otpb "github.com/openconfig/gnoi/otdr"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	spb "github.com/openconfig/gnoi/system"
	wpb "github.com/openconfig/gnoi/wavelength_router"
)

// gnoiConn implements the stub builder needed by the Ondatra
// binding.Binding interface.
type gnoiConn struct {
	*binding.AbstractGNOIClients
	conn *grpc.ClientConn
}

func (g gnoiConn) BGP() bpb.BGPClient { return bpb.NewBGPClient(g.conn) }
func (g gnoiConn) CertificateManagement() cpb.CertificateManagementClient {
	return cpb.NewCertificateManagementClient(g.conn)
}
func (g gnoiConn) Diag() dpb.DiagClient { return dpb.NewDiagClient(g.conn) }
func (g gnoiConn) FactoryReset() frpb.FactoryResetClient {
	return frpb.NewFactoryResetClient(g.conn)
}
func (g gnoiConn) File() fpb.FileClient { return fpb.NewFileClient(g.conn) }
func (g gnoiConn) Healthz() hpb.HealthzClient {
	return hpb.NewHealthzClient(g.conn)
}
func (g gnoiConn) Layer2() lpb.Layer2Client {
	return lpb.NewLayer2Client(g.conn)
}
func (g gnoiConn) LinkQualification() plqpb.LinkQualificationClient {
	return plqpb.NewLinkQualificationClient(g.conn)
}
func (g gnoiConn) MPLS() mpb.MPLSClient  { return mpb.NewMPLSClient(g.conn) }
func (g gnoiConn) OS() ospb.OSClient     { return ospb.NewOSClient(g.conn) }
func (g gnoiConn) OTDR() otpb.OTDRClient { return otpb.NewOTDRClient(g.conn) }
func (g gnoiConn) System() spb.SystemClient {
	return spb.NewSystemClient(g.conn)
}
func (g gnoiConn) WavelengthRouter() wpb.WavelengthRouterClient {
	return wpb.NewWavelengthRouterClient(g.conn)
}

var _ = binding.GNOIClients(gnoiConn{})
