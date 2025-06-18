// Copyright 2025 Google LLC
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

package canonicalocspec

import (
	"errors"
	"testing"

	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestParse(t *testing.T) {
	wantOC := &oc.Root{}
	intf := wantOC.GetOrCreateInterface("eth0")
	intf.GetOrCreateHoldTime().SetUp(42)
	intf.SetDescription("a description")
	intf.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
	intf.SetMtu(1500)
	wantOC.GetOrCreateSystem().SetHostname("a hostname")

	tests := []struct {
		desc            string
		readme          string
		wantErr         bool
		wantNotFoundErr bool
		wantOCs         []ygot.GoStruct
	}{{
		desc: "valid-canonical-oc",
		readme: `## Canonical OC
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
}` + "\n```",
		wantErr:         false,
		wantNotFoundErr: false,
		wantOCs:         []ygot.GoStruct{wantOC},
	}, {
		desc: "invalid-header",
		readme: `## Invalid Canonical OC
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
}` + "\n```",
		wantErr:         true,
		wantNotFoundErr: true,
		wantOCs:         []ygot.GoStruct{},
	}, {
		desc: "incomplete-canonical-oc",
		readme: `## Canonical OC
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
}` + "\n```",
		wantErr:         true,
		wantNotFoundErr: false,
		wantOCs:         []ygot.GoStruct{},
	}, {
		desc: "incorrect-canonical-oc",
		readme: `## Canonical OC
` + "```" + `json
{
  "interfaces": {
    "interface": [
      {
        "config": {
          "description": "a description",
          "mtu": 1500,
          "name": "eth0",
          "type": "ethernetCsd"
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
}` + "\n```",
		wantErr:         true,
		wantNotFoundErr: false,
		wantOCs:         []ygot.GoStruct{},
	}, {
		desc:            "json-codeblock-missing",
		readme:          `## Canonical OC`,
		wantErr:         true,
		wantNotFoundErr: true,
		wantOCs:         []ygot.GoStruct{},
	}, {
		desc: "multiple-json-codeblocks",
		readme: `## Canonical OC
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
}` + "\n```" + "\n```" + `json
{
  "openconfig-qos": {
    "interfaces": [
      {
        "config": {
          "interface-id": "PortChannel1.100"
        },
        "input": {
          "classifiers": [
            {
              "classifier": "dest_A",
              "config": {
                "name": "dest_A",
                "type": "IPV4"
              }
            }
          ],
          "scheduler-policy": {
            "config": {
              "name": "limit_group_A_1Gb",
              "new-leaf": "my_new_value"
            }
          }
        },
        "interface": "PortChannel1.100"
      }
    ]
  }
}` + "\n```",
		wantErr:         false,
		wantNotFoundErr: false,
		wantOCs:         []ygot.GoStruct{wantOC},
	}}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ocs, err := Parse([]byte(tc.readme))
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseCanonicalOC(%v) got error: %v, want error: %v", tc.readme, err, tc.wantErr)
			}
			if errors.Is(err, ErrNotFound) != tc.wantNotFoundErr {
				t.Fatalf("ParseCanonicalOC(%v) got ErrNotFound: %v, want ErrNotFound: %v", tc.readme, err, tc.wantNotFoundErr)
			}
			if len(ocs) != len(tc.wantOCs) {
				t.Fatalf("ParseCanonicalOC(%v) got %v, want %v", tc.readme, ocs, tc.wantOCs)
			}
			for idx, got := range ocs {
				if diff, err := ygot.Diff(tc.wantOCs[idx], got); err != nil {
					t.Errorf("ParseCanonicalOCs(%v) returned diff (-want +got):\n%s", tc.readme, diff)
				}
			}
		})
	}
}
