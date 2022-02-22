// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Package absate provides an abstracted interface to automated test infrastructure
// which allows use of the ONDATRA ATE API, and the long-term target of Open
// Traffic Generator specifically for feature profiles use.
//
// The APIs provided here seek to take some of the low-level flows and protocol
// operations that ATEs within functional tests provide such that tests can
// do not need to redefine them.
package absate

import (
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
)

// TODO(robjs): implementation :-) this file serves as a notes file for what
// we need to do here.
//
// reference: wbbtest.go
//		- we use ATETopology which is provided by ONDATRA. In an OTG world,
//		  what is the plan for topology.
//			- ATETopology isn't a first class citizen of the existing API in
//			  IxNetwork, so we need to choose if this is something that ONDATRA
//			  will continue to expose, or if it's something that we need to
//			  re-implement itself.

// Environment is a type that indicates which ATE environment the test is running
// within.
type Environment int64

const (
	_ Environment = iota
	// ClassicATE indicates that the ATE should be interfaced with through the
	// "classic" ONDATRA ATE API.
	ClassicATE
	// OTG indicates that the ATE should be interfaced with through the Open
	// Traffic Generator API.
	OTG
)

var (
	// mode specifies which mode the abstract ATE should operate within to
	// start with. By default we use the classic ATE API.
	mode = ClassicATE
)

// SetMode allows the caller to determine which environment it should build
// contents for.
//
// TODO(robjs): we could discuss here whether we want to - rather than only
// building one set of contents, build both, and just have the actuation of
// pushing the config choose which to push.
func SetMode(m Environment) {
	mode = m
}

// ATE is a wrapper struct that contains a 'classic' and 'OTG' ATE device.
type ATE struct {
	// From the OTG() implementation in ONDATRA, ATEDevice will be common across the
	// different implementations, so we don't need to store anything specific
	// for each type here.
	dev *ondatra.ATEDevice

	// otg is the OTG configuration for this ATE which is global for the device.
	otg gosnappi.Config
}

// NewDevice returns a new abstract ATE device.
func NewDevice(d *ondatra.ATEDevice) *ATE {
	return &ATE{
		dev: d,
		otg: gosnappi.NewConfig(),
	}
}

// AddFlow adds a flow to the ATE with the specified name.
func (a *ATE) AddFlow(name string) *Flow {
	return newFlow(a, name)
}

func (a *ATE) WithBGP() *BGP {
	return newBGP(a)
}

// StartTraffic starts traffic flowing for the flow specified.
func (a *ATE) StartTraffic(t testing.TB) {
	switch mode {
	case ClassicATE:
		a.dev.Traffic().Start(t)
	case OTG:
		if err := a.otg.Validate(); err != nil {
			t.Fatalf("invalid OTG, got err: %v", err)
		}
		// TODO(robjs): understand how in the OTG API we start things, can we
		// start a specific flow, or do we just start everything?
	}
}

// StopTraffic stops traffic that is running on the abstract ATE.
func (a *ATE) StopTraffic(t testing.TB) {
	switch mode {
	case ClassicATE:
		a.dev.Traffic().Stop(t)
	case OTG:
		// TODO(robjs): same as above.
	}
}

// Flow is the container ATE or OTG flow.
type Flow struct {
	parent *ATE
	ate    *ondatra.Flow
	otg    gosnappi.Flow
}

// newFlow adds a new flow to the ATE device specified with the specified
// name, initialising the relevant structures required for the flow.
func newFlow(a *ATE, name string) *Flow {
	f := &Flow{parent: a}
	switch mode {
	case ClassicATE:
		f.ate = f.parent.dev.Traffic().NewFlow(name)
	case OTG:
		f.otg = f.parent.otg.Flows().Add()
		f.otg.SetName(name)
	}
	return f
}

// IPinIP tunnel defines an IP-in-IP tunnel with the specifid outer DSCP value
// that represents a flow that can be matched within functional tests.
func (f *Flow) IPinIPTunnel(dscp uint8) *Flow {
	// TODO(robjs): figure out whether how this looks if we support explicit
	// source and destination vs. between ATE ports that would be handled by the
	// topology (see open question above).
	switch mode {
	case ClassicATE:
		h := []ondatra.Header{
			ondatra.NewEthernetHeader(),
			ondatra.NewIPv4Header().WithDSCP(dscp), // TODO(robjs); set IP protocol
			ondatra.NewIPv4Header(),
		}
		f.ate.WithHeaders(h...)
	case OTG:
		f.otg.Packet().Add().Ethernet()
		f.otg.Packet().Add().Ipv4().SetProtocol(gosnappi.NewPatternFlowIpv4Protocol().SetValue(40))
		f.otg.Packet().Add().Ipv4()
	}
	return f
}

/*type BGP struct {
	parent *ATE

	otg gosnappi.DeviceBgpRouter
}

func newBGP(a *ATE, devName string) *BGP {
	b := &BGP{
		parent: a,
	}
	switch mode {
	case ClassicATE:
	case OTG:
		b.otg = a.otg.Devices().Add().SetName(devName).Bgp()
	}
	return b
}

func (b *BGP) ExternalIPv4Peer() *IPv4BGPPeer {

}


type IPv4BGPPeer struct {
	parent *ATE

	otg gosnappi.BgpV4Peer
}*/
