// Copyright 2024 Google LLC
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

package mdocspec

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	ppb "github.com/openconfig/featureprofiles/proto/ocpaths_go_proto"
	rpb "github.com/openconfig/featureprofiles/proto/ocrpcs_go_proto"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"
)

func mustOCPaths(t *testing.T, textproto string) *ppb.OCPaths {
	ocPaths := &ppb.OCPaths{}
	if err := prototext.Unmarshal([]byte(textproto), ocPaths); err != nil {
		t.Fatal(err)
	}
	return ocPaths
}

func mustOCRPCs(t *testing.T, textproto string) *rpb.OCRPCs {
	ocRPCs := &rpb.OCRPCs{}
	if err := prototext.Unmarshal([]byte(textproto), ocRPCs); err != nil {
		t.Fatal(err)
	}
	return ocRPCs
}

func TestParse(t *testing.T) {
	tests := []struct {
		desc            string
		inMD            string
		wantOCPaths     *ppb.OCPaths
		wantOCRPCs      *rpb.OCRPCs
		wantNotFoundErr bool
		wantErr         bool
	}{{
		desc: "good",
		inMD: `---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

# Instructions for this template

Below is the required template for writing test requirements.  Good examples of test
requirements include:

* [TE-3.7: Base Hierarchical NHG Update](/feature/gribi/otg_tests/base_hierarchical_nhg_update/README.md)
* [gNMI-1.13: Telemetry: Optics Power and Bias Current](https://github.com/openconfig/featureprofiles/blob/main/feature/platform/tests/optics_power_and_bias_current_test/README.md)
* [RT-5.1: Singleton Interface](https://github.com/openconfig/featureprofiles/blob/main/feature/interface/singleton/otg_tests/singleton_test/README.md)

# TestID-x.y: Short name of test here

## Summary

Write a few sentences or paragraphs describing the purpose and scope of the test.

## Testbed type

* Specify the .testbed topology file from the [topologies](https://github.com/openconfig/featureprofiles/tree/main/topologies) folder to be used with this test

## Procedure

* Test environment setup
  * Description of procedure to configure ATE and DUT with pre-requisites making it possible to cover the intended paths and RPC's.

* TestID-x.y.z - Name of subtest
  * Step 1
  * Step 2
  * Validation and pass/fail criteria

* TestID-x.y.z - Name of subtest
  * Step 1
  * Step 2
  * Validation and pass/fail criteria

## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

` + "```" + `yaml
paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: "CHASSIS"

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
  gnoi:
      healthz.Healthz.Get:
      healthz.Healthz.List:
      healthz.Healthz.Acknowledge:
      healthz.Healthz.Artifact:
      healthz.Healthz.Check:
      bgp.BGP.ClearBGPNeighbor:
` + "```" + `

## Required DUT platform

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`,
		wantOCPaths: mustOCPaths(t, `
ocpaths: {
  name: "/components/component/state/name"
  ocpath_constraint: {
    platform_type: "CHASSIS"
  }
}
ocpaths: {
  name: "/interfaces/interface/config/description"
}
ocpaths: {
  name: "/interfaces/interface/config/enabled"
}
`),
		wantOCRPCs: mustOCRPCs(t, `
oc_protocols: {
  key: "gnmi"
  value: {
    method_name: "gnmi.gNMI.Set"
    method_name: "gnmi.gNMI.Subscribe"
  }
}
oc_protocols: {
  key: "gnoi"
  value: {
    method_name: "gnoi.bgp.BGP.ClearBGPNeighbor"
    method_name: "gnoi.healthz.Healthz.Acknowledge"
    method_name: "gnoi.healthz.Healthz.Artifact"
    method_name: "gnoi.healthz.Healthz.Check"
    method_name: "gnoi.healthz.Healthz.Get"
    method_name: "gnoi.healthz.Healthz.List"
  }
}
`),
	}, {
		desc:            "empty",
		inMD:            ``,
		wantNotFoundErr: true,
		wantErr:         true,
	}, {
		desc: "no-heading",
		inMD: `
` + "```" + `yaml
paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: "CHASSIS"

` + "```" + `
		`,
		wantNotFoundErr: true,
		wantErr:         true,
	}, {
		desc: "zero-rpcs",
		inMD: `---
name: New featureprofiles test requirement
---

## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

` + "```" + `yaml
paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: "CHASSIS"
rpcs:
` + "```" + `

## Required DUT platform
`,
		wantErr: true,
	}, {
		desc: "no-rpcs",
		inMD: `---
name: New featureprofiles test requirement
---

## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

` + "```" + `yaml
paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: "CHASSIS"

` + "```" + `

## Required DUT platform
`,
		wantErr: true,
	}, {
		desc: "zero-paths-one-rpc",
		inMD: `
## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

` + "```" + `yaml
paths:
rpcs:
  gnoi:
    healthz.Healthz.Get:
` + "```" + `

## Required DUT platform
`,
		wantOCPaths: mustOCPaths(t, ``),
		wantOCRPCs: mustOCRPCs(t, `
oc_protocols: {
  key: "gnoi"
  value: {
    method_name: "gnoi.healthz.Healthz.Get"
  }
}
`),
	}, {
		desc: "no-paths-one-rpc",
		inMD: `
## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

` + "```" + `yaml
rpcs:
  gnoi:
    healthz.Healthz.Get:
` + "```" + `

## Required DUT platform
`,
		wantOCPaths: mustOCPaths(t, ``),
		wantOCRPCs: mustOCRPCs(t, `
oc_protocols: {
  key: "gnoi"
  value: {
    method_name: "gnoi.healthz.Healthz.Get"
  }
}
`),
	}, {
		desc: "zero-paths-one-rpc-zero-methods",
		inMD: `
## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

` + "```" + `yaml
paths:
rpcs:
  gnoi:
` + "```" + `

## Required DUT platform
`,
		wantErr: true,
	}, {
		desc: "zero-paths-zero-rpcs",
		inMD: `
## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

` + "```" + `yaml
paths:
rpcs:
` + "```" + `

## Required DUT platform
`,
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			gotOCPaths, gotOCRPCs, err := Parse([]byte(tt.inMD))
			if gotNotFoundErr := errors.Is(err, ErrNotFound); gotNotFoundErr != tt.wantNotFoundErr {
				t.Fatalf("Parse gotNotFoundErr: %v, wantNotFoundErr: %v", gotNotFoundErr, tt.wantNotFoundErr)
			}
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("Parse gotErr: %v, wantErr: %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.wantOCPaths, gotOCPaths, protocmp.Transform()); diff != "" {
				t.Errorf("Parse OCPaths (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantOCRPCs, gotOCRPCs, protocmp.Transform()); diff != "" {
				t.Errorf("Parse OCRPCs (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestParseYAML(t *testing.T) {
	tests := []struct {
		desc        string
		inYAML      string
		wantOCPaths *ppb.OCPaths
		wantOCRPCs  *rpb.OCRPCs
		wantErr     bool
	}{{
		desc: "good",
		inYAML: `paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: "CHASSIS"

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
  gnoi:
      healthz.Healthz.Get:
      healthz.Healthz.List:
      healthz.Healthz.Acknowledge:
      healthz.Healthz.Artifact:
      healthz.Healthz.Check:
      bgp.BGP.ClearBGPNeighbor:
`,
		wantOCPaths: mustOCPaths(t, `
ocpaths: {
  name: "/components/component/state/name"
  ocpath_constraint: {
    platform_type: "CHASSIS"
  }
}
ocpaths: {
  name: "/interfaces/interface/config/description"
}
ocpaths: {
  name: "/interfaces/interface/config/enabled"
}
`),
		wantOCRPCs: mustOCRPCs(t, `
oc_protocols: {
  key: "gnmi"
  value: {
    method_name: "gnmi.gNMI.Set"
    method_name: "gnmi.gNMI.Subscribe"
  }
}
oc_protocols: {
  key: "gnoi"
  value: {
    method_name: "gnoi.bgp.BGP.ClearBGPNeighbor"
    method_name: "gnoi.healthz.Healthz.Acknowledge"
    method_name: "gnoi.healthz.Healthz.Artifact"
    method_name: "gnoi.healthz.Healthz.Check"
    method_name: "gnoi.healthz.Healthz.Get"
    method_name: "gnoi.healthz.Healthz.List"
  }
}
`),
	}, {
		desc: "missing-rpcs",
		inYAML: `paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: "CHASSIS"
`,
		wantErr: true,
	}, {
		desc: "missing-paths",
		inYAML: `rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
  gnoi:
      healthz.Healthz.Get:
      healthz.Healthz.List:
      healthz.Healthz.Acknowledge:
      healthz.Healthz.Artifact:
      healthz.Healthz.Check:
      bgp.BGP.ClearBGPNeighbor:
`,
		wantOCPaths: mustOCPaths(t, ``),
		wantOCRPCs: mustOCRPCs(t, `
oc_protocols: {
  key: "gnmi"
  value: {
    method_name: "gnmi.gNMI.Set"
    method_name: "gnmi.gNMI.Subscribe"
  }
}
oc_protocols: {
  key: "gnoi"
  value: {
    method_name: "gnoi.bgp.BGP.ClearBGPNeighbor"
    method_name: "gnoi.healthz.Healthz.Acknowledge"
    method_name: "gnoi.healthz.Healthz.Artifact"
    method_name: "gnoi.healthz.Healthz.Check"
    method_name: "gnoi.healthz.Healthz.Get"
    method_name: "gnoi.healthz.Healthz.List"
  }
}
`),
	}, {
		desc:    "empty",
		inYAML:  ``,
		wantErr: true,
	}, {
		desc: "extra-spaces",
		inYAML: `
paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: "CHASSIS"



rpcs:


  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
  gnoi:
      healthz.Healthz.Get:
      healthz.Healthz.List:
      healthz.Healthz.Acknowledge:
      healthz.Healthz.Artifact:
      healthz.Healthz.Check:
      bgp.BGP.ClearBGPNeighbor:

`,
		wantOCPaths: mustOCPaths(t, `
ocpaths: {
  name: "/components/component/state/name"
  ocpath_constraint: {
    platform_type: "CHASSIS"
  }
}
ocpaths: {
  name: "/interfaces/interface/config/description"
}
ocpaths: {
  name: "/interfaces/interface/config/enabled"
}
`),
		wantOCRPCs: mustOCRPCs(t, `
oc_protocols: {
  key: "gnmi"
  value: {
    method_name: "gnmi.gNMI.Set"
    method_name: "gnmi.gNMI.Subscribe"
  }
}
oc_protocols: {
  key: "gnoi"
  value: {
    method_name: "gnoi.bgp.BGP.ClearBGPNeighbor"
    method_name: "gnoi.healthz.Healthz.Acknowledge"
    method_name: "gnoi.healthz.Healthz.Artifact"
    method_name: "gnoi.healthz.Healthz.Check"
    method_name: "gnoi.healthz.Healthz.Get"
    method_name: "gnoi.healthz.Healthz.List"
  }
}
`),
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			gotOCPaths, gotOCRPCs, err := parseYAML([]byte(tt.inYAML))
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("parseYAML gotErr: %v, wantErr: %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.wantOCPaths, gotOCPaths, protocmp.Transform()); diff != "" {
				t.Errorf("parseYAML OCPaths (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantOCRPCs, gotOCRPCs, protocmp.Transform()); diff != "" {
				t.Errorf("parseYAML OCRPCs (-want, +got):\n%s", diff)
			}
		})
	}
}
