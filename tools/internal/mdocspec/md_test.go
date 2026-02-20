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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
)

func TestRenderer(t *testing.T) {
	tests := []struct {
		desc     string
		inSource []byte
		want     string
	}{{
		desc: "basic",
		inSource: []byte(`---
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
    platform_type: ["CHASSIS"]

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
` + "```" + `

## Required DUT platform

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`),
		want: `paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: ["CHASSIS"]

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
`,
	}, {
		desc: "second-yaml-block-in-separate-heading",
		inSource: []byte(`---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

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
    platform_type: ["CHASSIS"]

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
` + "```" + `

## Required DUT platform

` + "```" + `yaml
paths:
  # interface configuration
  /a/b/c:
  /d/e/f:

rpcs:
  fooi:
    fooi.Set:
      union_replace: true
    fooi.Subscribe:
      on_change: true
` + "```" + `

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`),
		want: `paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: ["CHASSIS"]

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
`,
	}, {
		desc: "two-yaml-blocks-same-heading",
		inSource: []byte(`---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

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
  /a/b/c:
  /d/e/f:

rpcs:
  fooi:
    fooi.Set:
      union_replace: true
    fooi.Subscribe:
      on_change: true
` + "```" + `

` + "```" + `yaml
paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: ["CHASSIS"]

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
` + "```" + `

## Required DUT platform

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`),
		want: `paths:
  # interface configuration
  /a/b/c:
  /d/e/f:

rpcs:
  fooi:
    fooi.Set:
      union_replace: true
    fooi.Subscribe:
      on_change: true
`,
	}, {
		desc: "yaml-block-after-next-heading-ignored",
		inSource: []byte(`---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

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

## Required DUT platform

` + "```" + `yaml
paths:
  # interface configuration
  /a/b/c:
  /d/e/f:

rpcs:
  fooi:
    fooi.Set:
      union_replace: true
    fooi.Subscribe:
      on_change: true
` + "```" + `

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`),
		want: ``,
	}, {
		desc: "yaml-block-after-next-higher-heading-ignored",
		inSource: []byte(`---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

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

# Required DUT platform

Some text

` + "```" + `yaml
paths:
  # interface configuration
  /a/b/c:
  /d/e/f:

rpcs:
  fooi:
    fooi.Set:
      union_replace: true
    fooi.Subscribe:
      on_change: true
` + "```" + `

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`),
		want: ``,
	}, {
		desc: "yaml-block-after-next-lower-heading-accepted",
		inSource: []byte(`---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

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

### Required DUT platform

Some text

` + "```" + `yaml
paths:
  # interface configuration
  /a/b/c:
  /d/e/f:

rpcs:
  fooi:
    fooi.Set:
      union_replace: true
    fooi.Subscribe:
      on_change: true
` + "```" + `

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`),
		want: `paths:
  # interface configuration
  /a/b/c:
  /d/e/f:

rpcs:
  fooi:
    fooi.Set:
      union_replace: true
    fooi.Subscribe:
      on_change: true
`,
	}, {
		desc: "two-blocks-same-heading-first-language-not-specified-and-ignored",
		inSource: []byte(`---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

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

` + "```" + `
paths:
  # interface configuration
  /a/b/c:
  /d/e/f:

rpcs:
  fooi:
    fooi.Set:
      union_replace: true
    fooi.Subscribe:
      on_change: true
` + "```" + `

` + "```" + `yaml
paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: ["CHASSIS"]

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
` + "```" + `

## Required DUT platform

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`),
		want: `paths:
  # interface configuration
  /interfaces/interface/config/description:
  /interfaces/interface/config/enabled:
  # name of chassis component
  /components/component/state/name:
    platform_type: ["CHASSIS"]

rpcs:
  gnmi:
    gNMI.Set:
      union_replace: true
    gNMI.Subscribe:
      on_change: true
`,
	}, {
		desc: "no-yaml-blocks",
		inSource: []byte(`---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

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

## Required DUT platform

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`),
		want: ``,
	}, {
		desc: "no-yaml-blocks-last-heading",
		inSource: []byte(`---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

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

* Specify the minimum DUT-type:
  * MFF - A modular form factor device containing LINECARDs, FABRIC and redundant CONTROLLER_CARD components
  * FFF - fixed form factor
  * vRX - virtual router device
`),
		want: ``,
	}, {
		desc: "yaml-block-empty",
		inSource: []byte(`---
name: New featureprofiles test requirement
about: Use this template to document the requirements for a new test to be implemented.
title: ''
labels: enhancement
assignees: ''
---

## OpenConfig Path and RPC Coverage

This example yaml defines the OC paths intended to be covered by this test.  OC paths used for test environment setup are not required to be listed here.

` + "```" + `yaml
` + "```" + `
`),
		want: ``,
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var buf strings.Builder
			md := goldmark.New(
				goldmark.WithExtensions(MDOCSpecs),
			)
			if err := md.Convert(tt.inSource, &buf); err != nil {
				t.Fatalf("MDOCSpecs.Convert: %v", err)
			}
			if diff := cmp.Diff(tt.want, buf.String()); diff != "" {
				t.Errorf("MDOCSpecs.Convert (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestJSONRenderer(t *testing.T) {
	tests := []struct {
		desc     string
		inSource []byte
		want     []string
	}{{
		desc: "valid-readme",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{`
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}`},
	}, {
		desc: "second-json-block-in-separate-heading",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## Second JSON Block
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 49
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{`
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}`},
	}, {
		desc: "two-json-blocks-same-heading",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 47
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + "\n```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 49
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{`
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 47
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}`},
	}, {
		desc: "json-block-after-next-heading-ignored",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC

## Some other OC Heading
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{},
	}, {
		desc: "no-json-blocks-last-heading",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC

## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{},
	}, {
		desc: "empty-json-codeblock",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC
` + "```" + `json` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{},
	}, {
		desc: "json-block-after-next-higher-heading-ignored",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC

# Higher Heading
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{},
	}, {
		desc: "json-block-after-next-lower-heading-accepted",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC

### Lower Heading
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{`
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}`},
	}, {
		desc: "two-blocks-same-heading-first-language-not-specified-and-ignored",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC
` + "```" +
			`{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + "\n```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 47
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{`
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 47
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}`},
	}, {
		desc: "valid-readme-with-two-canonical-ocs",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

## Canonical OC
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 48
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `

## Canonical OC
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 47
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{`
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 48
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}
`, `
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 47
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}`},
	}, {
		desc: "valid-readme-with-todo",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

#### TODO: https://github.com/openconfig/public/pull/1234 - Add new leaf to scheduler-policy
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{},
	}, {
		desc: "readme-with-todo-and-valid-oc",
		inSource: []byte(`
# RT-1.7: Local BGP Test

## Summary

The local\_bgp\_test brings up two OpenConfig controlled devices and tests that for an eBGP session

* Established between them.
* Disconnected between them.
* Verify BGP neighbor parameters

Enable an Accept-route all import-policy/export-policy for eBGP session under the BGP peer-group AFI/SAFI.

This test is suitable for running in a KNE environment.

#### TODO: https://github.com/openconfig/public/pull/1234 - Add new leaf to scheduler-policy
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `

## Canonical OC
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}` + "\n```" + `
## OpenConfig Path and RPC Coverage
` + "```" + `yaml
paths:
  ## Parameter Coverage

  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/apply-policy/config/export-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/import-policy:
  /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/apply-policy/config/export-policy:

rpcs:
  gnmi:
    gNMI.Subscribe:
    gNMI.Set:
` + "```"),
		want: []string{`
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsmacd"
        },
        "hold-time": {
          "config": {
            "up": 42
          }
        },
        "name": "eth0"
      }
    ]
  },
  "system": {
    "config": {
      "hostname": "a hostname"
    }
  }
}`},
	}}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var buf strings.Builder
			md := goldmark.New(
				goldmark.WithExtensions(MDJSONSpecs),
			)
			if err := md.Convert(tt.inSource, &buf); err != nil {
				t.Fatalf("MDJSONSpecs.Convert(%v, &buf): %v", tt.inSource, err)
			}
			if len(tt.want) != len(MDJSONSpecs.CanonicalOCs) {
				t.Fatalf("MDJSONSpecs.Convert(%s, &buf): got %v, want %v", string(tt.inSource), MDJSONSpecs.CanonicalOCs, tt.want)
			}
			for idx, got := range MDJSONSpecs.CanonicalOCs {
				if diff := cmp.Diff(strings.TrimSpace(tt.want[idx]), strings.TrimSpace(got)); diff != "" {
					t.Errorf("MDJSONSpecs.Convert(%s, &buf): at idx: %d, (-want, +got):\n%s", string(tt.inSource), idx, diff)
				}
			}
		})
	}
}
