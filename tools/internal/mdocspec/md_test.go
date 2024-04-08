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
    platform_type: "CHASSIS"

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
    platform_type: "CHASSIS"

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
    platform_type: "CHASSIS"

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
    platform_type: "CHASSIS"

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
    platform_type: "CHASSIS"

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
    platform_type: "CHASSIS"

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
    platform_type: "CHASSIS"

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
