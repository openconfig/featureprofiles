// Copyright 2024 Google LLC
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
	"context"
	"errors"
	"fmt"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/binding/portgraph"
	opb "github.com/openconfig/ondatra/proto"
	"github.com/pborman/uuid"
)

func dynamicReservation(ctx context.Context, tb *opb.Testbed, r resolver) (*binding.Reservation, error) {
	abstractGraph, absNode2Dev, absPort2BindPort, err := portgraph.TestbedToAbstractGraph(tb, nil)
	if err != nil {
		return nil, fmt.Errorf("could not parse specified testbed: %w", err)
	}
	superGraph, conNode2Dev, conPort2BindPort, err := protoToConcreteGraph(r.Binding)
	if err != nil {
		return nil, fmt.Errorf("could not solve for specified testbed: %w", err)
	}
	assign, err := portgraph.Solve(ctx, abstractGraph, superGraph)
	if err != nil {
		return nil, fmt.Errorf("could not solve for specified testbed: %w", err)
	}
	res, err := assignmentToReservation(assign, r, tb, absNode2Dev, conNode2Dev, absPort2BindPort, conPort2BindPort)
	if err != nil {
		return nil, fmt.Errorf("could not solve for specified testbed: %w", err)
	}
	return res, nil
}

func protoToConcreteGraph(bpb *bindpb.Binding) (*portgraph.ConcreteGraph, map[*portgraph.ConcreteNode]*bindpb.Device, map[*portgraph.ConcretePort]*bindpb.Port, error) {
	cg := &portgraph.ConcreteGraph{Desc: "FeatureProfiles binding.proto"}
	qualName2Port := make(map[string]*portgraph.ConcretePort)
	conNode2Dev := make(map[*portgraph.ConcreteNode]*bindpb.Device)
	conPort2BindPort := make(map[*portgraph.ConcretePort]*bindpb.Port)

	addDevice := func(dev *bindpb.Device, devRole string) {
		var ports []*portgraph.ConcretePort
		for _, ap := range dev.GetPorts() {
			port := &portgraph.ConcretePort{
				Desc:  ap.Name,
				Attrs: make(map[string]string),
			}
			if name := ap.GetName(); name != "" {
				port.Attrs[portgraph.NameAttr] = name
			}
			if speed := ap.GetSpeed(); speed != opb.Port_SPEED_UNSPECIFIED {
				port.Attrs[portgraph.SpeedAttr] = speed.String()
			}
			if pmd := ap.GetPmd(); pmd != opb.Port_PMD_UNSPECIFIED {
				port.Attrs[portgraph.PMDAttr] = pmd.String()
			}
			ports = append(ports, port)
			qualName2Port[dev.Name+":"+ap.Name] = port
			conPort2BindPort[port] = ap
		}

		node := &portgraph.ConcreteNode{
			Desc:  dev.Name,
			Ports: ports,
			Attrs: map[string]string{portgraph.RoleAttr: devRole},
		}
		if name := dev.GetName(); name != "" {
			node.Attrs[portgraph.NameAttr] = name
		}
		if hw := dev.GetHardwareModel(); hw != "" {
			node.Attrs[portgraph.HWAttr] = hw
		}
		if sw := dev.GetSoftwareVersion(); sw != "" {
			node.Attrs[portgraph.SWAttr] = sw
		}
		cg.Nodes = append(cg.Nodes, node)
		conNode2Dev[node] = dev
	}
	for _, dut := range bpb.GetDuts() {
		addDevice(dut, portgraph.RoleDUT)
	}
	for _, ate := range bpb.GetAtes() {
		addDevice(ate, portgraph.RoleATE)
	}

	for _, link := range bpb.GetLinks() {
		pa, ok := qualName2Port[link.GetA()]
		if !ok {
			return nil, nil, nil, fmt.Errorf("no known port %q in link %v", link.GetA(), link)
		}
		pb, ok := qualName2Port[link.GetB()]
		if !ok {
			return nil, nil, nil, fmt.Errorf("no known port %q in link %v", link.GetB(), link)
		}
		cg.Edges = append(cg.Edges, &portgraph.ConcreteEdge{Src: pa, Dst: pb})
	}

	return cg, conNode2Dev, conPort2BindPort, nil
}

func assignmentToReservation(
	assign *portgraph.Assignment,
	r resolver,
	tb *opb.Testbed,
	absNode2Dev map[*portgraph.AbstractNode]*opb.Device,
	conNode2Dev map[*portgraph.ConcreteNode]*bindpb.Device,
	absPort2BindPort map[*portgraph.AbstractPort]*opb.Port,
	conPort2BindPort map[*portgraph.ConcretePort]*bindpb.Port,
) (*binding.Reservation, error) {
	res := &binding.Reservation{
		ID:   uuid.New(),
		DUTs: make(map[string]binding.DUT),
		ATEs: make(map[string]binding.ATE),
	}

	tbDev2BindDev := make(map[*opb.Device]*bindpb.Device)
	for absNode, conNode := range assign.Node2Node {
		tbDev2BindDev[absNode2Dev[absNode]] = conNode2Dev[conNode]
	}

	tbPort2BindPort := make(map[*opb.Port]*bindpb.Port)
	for absPort, conPort := range assign.Port2Port {
		tbPort2BindPort[absPort2BindPort[absPort]] = conPort2BindPort[conPort]
	}

	var errs []error
	for _, tdut := range tb.GetDuts() {
		bdut := tbDev2BindDev[tdut]
		d := dynDims(tdut, bdut, tbPort2BindPort)
		res.DUTs[tdut.Id] = &staticDUT{
			AbstractDUT: &binding.AbstractDUT{Dims: d},
			r:           r,
			dev:         bdut,
		}
	}
	for _, tate := range tb.GetAtes() {
		bate := tbDev2BindDev[tate]
		d := dynDims(tate, bate, tbPort2BindPort)
		res.ATEs[tate.Id] = &staticATE{
			AbstractATE: &binding.AbstractATE{Dims: d},
			r:           r,
			dev:         bate,
		}
	}
	return res, errors.Join(errs...)
}

func dynDims(td *opb.Device, bd *bindpb.Device, tbPort2BindPort map[*opb.Port]*bindpb.Port) *binding.Dims {
	dims := &binding.Dims{
		Name:            bd.Name,
		Vendor:          bd.GetVendor(),
		HardwareModel:   bd.GetHardwareModel(),
		SoftwareVersion: bd.GetSoftwareVersion(),
		Ports:           make(map[string]*binding.Port),
	}
	for _, tport := range td.GetPorts() {
		bport := tbPort2BindPort[tport]
		dims.Ports[tport.Id] = &binding.Port{
			Name:  bport.Name,
			PMD:   bport.Pmd,
			Speed: bport.Speed,
		}
	}
	return dims
}
