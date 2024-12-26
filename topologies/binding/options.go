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
	"flag"
	"fmt"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/binding/introspect"
	"google.golang.org/protobuf/proto"
)

// IANA assigns 9339 for gNxI, 9559 for P4RT and 9340 for gRIBI.
var (
	gnmiPort    = flag.Int("gnmi_port", 9339, "default gNMI port")
	gnoiPort    = flag.Int("gnoi_port", 9339, "default gNOI port")
	gnsiPort    = flag.Int("gnsi_port", 9339, "default gNSI port")
	gribiPort   = flag.Int("gribi_port", 9340, "default gRIBI port")
	p4rtPort    = flag.Int("p4rt_port", 9559, "default P4RT part")
	ateGNMIPort = flag.Int("ate_gnmi_port", 50051, "default ATE gNMI port")
	ateOTGPort  = flag.Int("ate_grpc_port", 40051, "default ATE OTG port")

	dutSvcParams = map[introspect.Service]*svcParams{
		introspect.GNMI: {
			port:   *gnmiPort,
			optsFn: (*bindpb.Device).GetGnmi,
		},
		introspect.GNOI: {
			port:   *gnoiPort,
			optsFn: (*bindpb.Device).GetGnoi,
		},
		introspect.GNSI: {
			port:   *gnsiPort,
			optsFn: (*bindpb.Device).GetGnsi,
		},
		introspect.GRIBI: {
			port:   *gribiPort,
			optsFn: (*bindpb.Device).GetGribi,
		},
		introspect.P4RT: {
			port:   *p4rtPort,
			optsFn: (*bindpb.Device).GetP4Rt,
		},
	}

	ateSvcParams = map[introspect.Service]*svcParams{
		introspect.GNMI: {
			port:   *ateGNMIPort,
			optsFn: (*bindpb.Device).GetGnmi,
		},
		introspect.OTG: {
			port:   *ateOTGPort,
			optsFn: (*bindpb.Device).GetOtg,
		},
	}
)

type svcParams struct {
	port   int
	optsFn func(*bindpb.Device) *bindpb.Options
}

// merge creates combines one or more options into one set of options.
func merge(bopts ...*bindpb.Options) *bindpb.Options {
	result := &bindpb.Options{}
	for _, bopt := range bopts {
		if bopt != nil {
			proto.Merge(result, bopt)
		}
	}
	return result
}

type resolver struct {
	*bindpb.Binding
}

func (r *resolver) grpc(dev *bindpb.Device, params *svcParams) *bindpb.Options {
	targetOpts := &bindpb.Options{Target: fmt.Sprintf("%s:%d", dev.Name, params.port)}
	return merge(targetOpts, r.Options, dev.Options, params.optsFn(dev))
}

func (r *resolver) ssh(dev *bindpb.Device) *bindpb.Options {
	targetOpts := &bindpb.Options{Target: dev.Name}
	return merge(targetOpts, r.Options, dev.Options, dev.Ssh)
}

func (r *resolver) ixnetwork(dev *bindpb.Device) *bindpb.Options {
	targetOpts := &bindpb.Options{Target: dev.Name}
	return merge(targetOpts, r.Options, dev.Options, dev.Ixnetwork)
}
