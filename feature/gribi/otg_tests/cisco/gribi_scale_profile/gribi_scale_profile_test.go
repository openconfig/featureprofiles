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

// Package setup is scoped only to be used for scripts in path
// feature/experimental/system/gnmi/benchmarking/otg_tests/
// Do not use elsewhere.
package gribi_scale_profile

import (
	// "context"
	// "slices"
	// "strconv"
	"testing"

	// "github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	// "github.com/openconfig/featureprofiles/internal/gribi"
	// "github.com/openconfig/gribigo/fluent"
	// "github.com/openconfig/ondatra"
	// "github.com/openconfig/ondatra/gnmi"
)

const (
	nh1ID                     = 120
	nhg1ID                    = 20
	ipv4OuterDest             = "192.51.100.65"
	innerV4DstIP              = "198.18.1.1"
	innerV4SrcIP              = "198.18.0.255"
	innerV6SrcIP              = "2001:DB8::198:1"
	innerV6DstIP              = "2001:DB8:2:0:192::10"
	transitVrfIP              = "203.0.113.1"
	repairedVrfIP             = "203.0.113.100"
	noMatchSrcIP              = "198.100.200.123"
	decapMixPrefix1           = "192.51.128.0/22"
	decapMixPrefix2           = "192.55.200.3/32"
	src111TeDstFlowFilter     = "4043" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.51.100.111 + First 8 bits of first octet of TE DA 203.0.113.1
	src222TeDstFlowFilter     = "3787" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.51.100.222 + First 8 bits of first octet of TE DA 203.0.113.100
	noMatchSrcEncapDstFilter  = "2954" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.100.200.123 + First 8 bits of first octet of TE DA 138.0.11.8
	IPinIPProtocolFieldOffset = 184
	IPinIPProtocolFieldWidth  = 8
	IPinIPpSrcDstIPOffset     = 236
	IPinIPpSrcDstIPWidth      = 12
	IPinIPpDscpOffset         = 120
	IPinIPpDscpWidth          = 8
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGribiScaleProfile(t *testing.T) {
	configureBaseProfile(t)
}
