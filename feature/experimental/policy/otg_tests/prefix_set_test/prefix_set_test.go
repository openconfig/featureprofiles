// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prefix_set_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	prefixSetA = "PFX_SET_A"
	pfx1       = "10.240.31.48/28"
	pfx2       = "173.36.128.0/20"
	pfx3       = "173.36.144.0/20"
	pfx4       = "10.240.31.64/28"
	mskLen     = "exact"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestPrefixSet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	dutOcRoot := &oc.Root{}
	rp := dutOcRoot.GetOrCreateRoutingPolicy()
	ds := rp.GetOrCreateDefinedSets()

	// create a prefix-set with 2 prefixes
	v4PrefixSet := ds.GetOrCreatePrefixSet(prefixSetA)
	v4PrefixSet.SetMode(oc.PrefixSet_Mode_IPV4)
	v4PrefixSet.GetOrCreatePrefix(pfx1, mskLen)
	v4PrefixSet.GetOrCreatePrefix(pfx2, mskLen)

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetA).Config(), v4PrefixSet)
	prefixSet := gnmi.Get[*oc.RoutingPolicy_DefinedSets_PrefixSet](t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetA).State())
	if len(prefixSet.Prefix) != 2 {
		t.Errorf("Prefix set has %v prefixes, want 2", len(prefixSet.Prefix))
	}
	for _, pfx := range []string{pfx1, pfx2} {
		if x := prefixSet.GetPrefix(pfx, mskLen); x == nil {
			t.Errorf("%s not found in prefix-set %s", pfx, prefixSetA)
		}
	}

	// replace the prefix-set by replacing an existing prefix with new prefix
	v4PrefixSet = ds.GetOrCreatePrefixSet(prefixSetA)
	v4PrefixSet.SetMode(oc.PrefixSet_Mode_IPV4)
	v4PrefixSet.GetOrCreatePrefix(pfx1, mskLen)
	v4PrefixSet.GetOrCreatePrefix(pfx3, mskLen)

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetA).Config(), v4PrefixSet)
	prefixSet = gnmi.Get[*oc.RoutingPolicy_DefinedSets_PrefixSet](t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetA).State())
	if len(prefixSet.Prefix) != 2 {
		t.Errorf("Prefix set has %v prefixes, want 2", len(prefixSet.Prefix))
	}
	for _, pfx := range []string{pfx1, pfx3} {
		if x := prefixSet.GetPrefix(pfx, mskLen); x == nil {
			t.Errorf("%s not found in prefix-set %s", pfx, prefixSetA)
		}
	}

	// replace the prefix-set with 2 existing and a new prefix
	v4PrefixSet = ds.GetOrCreatePrefixSet(prefixSetA)
	v4PrefixSet.SetMode(oc.PrefixSet_Mode_IPV4)
	v4PrefixSet.GetOrCreatePrefix(pfx1, mskLen)
	v4PrefixSet.GetOrCreatePrefix(pfx3, mskLen)
	v4PrefixSet.GetOrCreatePrefix(pfx4, mskLen)

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetA).Config(), v4PrefixSet)
	prefixSet = gnmi.Get[*oc.RoutingPolicy_DefinedSets_PrefixSet](t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(prefixSetA).State())
	if len(prefixSet.Prefix) != 3 {
		t.Errorf("Prefix set has %v prefixes, want 3", len(prefixSet.Prefix))
	}
	for _, pfx := range []string{pfx1, pfx3, pfx4} {
		if x := prefixSet.GetPrefix(pfx, mskLen); x == nil {
			t.Errorf("%s not found in prefix-set %s", pfx, prefixSetA)
		}
	}
}
