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

package rundata

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDUTShortVendor(t *testing.T) {
	cases := []struct {
		vendor string
		want   string
	}{
		{"Arista Networks", "ARISTA"},
		{"Cisco Systems, Inc.", "CISCO"},
		{"Juniper Networks, Inc.", "JUNIPER"},
		{"Palo Alto Networks, Inc.", "PALOALTO"}, // Not a single word.
		{"Yoyodyne Systems", "YOYODYNE"},         // Not a recognized enum.
	}
	for _, c := range cases {
		di := &DUTInfo{Vendor: c.vendor}
		got := di.shortVendor()
		if got != c.want {
			t.Errorf("Case %q got %q, want %q", c.vendor, got, c.want)
		}
	}
}

func TestDUTShortModel(t *testing.T) {
	cases := []struct {
		model string
		want  string
	}{
		{"DCS-7280CR3K-32D4", "DCS-7280CR3K-32D4"}, // No change.
		{"Cisco 9999 9-slot Chassis", "9999"},
		{"JNP10001 [PTX10001]", "PTX10001"},
	}
	for _, c := range cases {
		di := &DUTInfo{Model: c.model}
		got := di.shortModel()
		if got != c.want {
			t.Errorf("Case %q got %q, want %q", c.model, got, c.want)
		}
	}
}

func TestDUTPut(t *testing.T) {
	cases := []struct {
		name string
		di   *DUTInfo
		want map[string]string
	}{{
		name: "Arista",
		di: &DUTInfo{
			Vendor: "Arista Networks",
			Model:  "DCS-7280CR3K-32D4",
			OSVer:  "4.29.0F",
		},
		want: map[string]string{
			"dut.vendor.full": "Arista Networks",
			"dut.vendor":      "ARISTA",
			"dut.model.full":  "DCS-7280CR3K-32D4",
			"dut.model":       "DCS-7280CR3K-32D4",
			"dut.os_version":  "4.29.0F",
		},
	}, {
		name: "Cisco",
		di: &DUTInfo{
			Vendor: "Cisco Systems, Inc.",
			Model:  "Cisco 9999 9-slot Chassis",
			OSVer:  "7.7.1",
		},
		want: map[string]string{
			"dut.vendor.full": "Cisco Systems, Inc.",
			"dut.vendor":      "CISCO",
			"dut.model.full":  "Cisco 9999 9-slot Chassis",
			"dut.model":       "9999",
			"dut.os_version":  "7.7.1",
		},
	}, {
		name: "Juniper",
		di: &DUTInfo{
			Vendor: "Juniper Networks, Inc.",
			Model:  "JNP10001 [PTX10001]",
			OSVer:  "21.42-S2-EVO",
		},
		want: map[string]string{
			"dut.vendor.full": "Juniper Networks, Inc.",
			"dut.vendor":      "JUNIPER",
			"dut.model.full":  "JNP10001 [PTX10001]",
			"dut.model":       "PTX10001",
			"dut.os_version":  "21.42-S2-EVO",
		},
	}, {
		name: "Yoyodyne",
		di: &DUTInfo{
			Vendor: "Yoyodyne Systems",
			Model:  "YY1608",
			OSVer:  "6.22",
		},
		want: map[string]string{
			"dut.vendor.full": "Yoyodyne Systems",
			"dut.vendor":      "YOYODYNE",
			"dut.model.full":  "YY1608",
			"dut.model":       "YY1608",
			"dut.os_version":  "6.22",
		},
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := make(map[string]string)
			c.di.put(got, "dut")
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("di.put -want, +got:\n%s", diff)
			}
		})
	}
}
