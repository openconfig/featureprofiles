// Copyright 2023 Google LLC
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

// Package main generates example textprotos of the format specified by
// nosimage.proto.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	npb "github.com/openconfig/featureprofiles/proto/nosimage_go_proto"
	ppb "github.com/openconfig/featureprofiles/proto/ocpaths_go_proto"
	rpb "github.com/openconfig/featureprofiles/proto/ocrpcs_go_proto"
	opb "github.com/openconfig/ondatra/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

// Config is the set of flags for this binary.
type Config struct {
	FilePath string
	Invalid  bool
}

// New registers a flagset with the configuration needed by this binary.
func New(fs *flag.FlagSet) *Config {
	c := &Config{}

	if fs == nil {
		fs = flag.CommandLine
	}
	fs.StringVar(&c.FilePath, "file-path", "example_nosimage.textproto", "generate an example file at the given path rather than validating a file.")
	fs.BoolVar(&c.Invalid, "invalid", false, "generate an invalid example")

	return c
}

var (
	config *Config
)

func init() {
	config = New(nil)
}

func generateExample(filepath string, valid bool) error {
	componentPrefix := "/components/component"
	softwareComponent := "OPERATING_SYSTEM"
	interfaceLeafName := "name"
	if !valid {
		componentPrefix = "/componentsssssssssss/component"
		softwareComponent = "JOVIAN_ATMOSPHERE"
		interfaceLeafName = "does-not-exist"
	}
	bs, err := formatTxtpb(&npb.NOSImageProfile{
		VendorId:        opb.Device_OPENCONFIG,
		Nos:             "lemming",
		SoftwareVersion: "7a1cb734c83f0d9ba5b273f920bc002ad0056178",
		ReleaseDate:     timestamppb.New(time.Date(2023, time.November, 16, 16, 20, 0, 0, time.FixedZone("UTC-8", -8*60*60))),
		Ocpaths: &ppb.OCPaths{
			Version: "2.5.0",
			Ocpaths: []*ppb.OCPath{{
				Name: "/interfaces/interface/config/" + interfaceLeafName,
				// Featureprofileid is not required when specifying OCPath support.
			}, {
				Name: componentPrefix + "/state/location",
				// OcpathConstraint MUST be specified if and only if the path belongs to /components/component.
				OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "PORT"}},
			}, {
				Name:             componentPrefix + "/state/serial-no",
				OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: "STORAGE"}},
			}, {
				Name:             "/components/component/state/software-version",
				OcpathConstraint: &ppb.OCPathConstraint{Constraint: &ppb.OCPathConstraint_PlatformType{PlatformType: softwareComponent}},
			}, {
				Name: "/network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as",
			}},
		},
		Ocrpcs: &rpb.OCRPCs{
			OcProtocols: func() map[string]*rpb.OCProtocol {
				if valid {
					return map[string]*rpb.OCProtocol{
						"gnmi": {
							Version: "0.10.0",
							MethodName: []string{
								"gnmi.gNMI.Set",
								"gnmi.gNMI.Subscribe",
							},
						},
						"gnoi": {
							Version: "0.3.0",
							MethodName: []string{
								"gnoi.healthz.Healthz.Get",
								"gnoi.healthz.Healthz.List",
								"gnoi.healthz.Healthz.Acknowledge",
								"gnoi.healthz.Healthz.Artifact",
								"gnoi.healthz.Healthz.Check",
								"gnoi.bgp.BGP.ClearBGPNeighbor",
							},
						},
					}
				}
				return map[string]*rpb.OCProtocol{
					"gnmi.gNMI": {
						Version: "0.10.0",
						MethodName: []string{
							"Set",
							"Subscribe",
						},
					},
					"gnoi.healthz.Healthz": {
						Version: "1.3.0",
						MethodName: []string{
							"Get",
							"List",
							"Acknowledge",
							"Artifact",
							"Check",
						},
					},
					"gnoi.bgp.BGP": {
						Version: "0.1.0",
						MethodName: []string{
							"ClearBGPNeighbor",
						},
					},
				}
			}(),
		},
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, bs, 0664)
}

func formatTxtpb(msg proto.Message) ([]byte, error) {
	out := bytes.NewBuffer(nil)
	desc := msg.ProtoReflect().Descriptor()
	fmt.Fprintln(out, "# proto-file: github.com/openconfig/featureprofiles/proto/"+desc.ParentFile().Path())
	fmt.Fprintln(out, "# proto-message:", desc.Name())
	fmt.Fprintln(out, "# txtpbfmt: expand_all_children")
	fmt.Fprintln(out, "# txtpbfmt: sort_repeated_fields_by_content")
	b, err := prototext.Marshal(msg)
	if err != nil {
		return nil, err
	}
	out.Write(b)
	b, err = parser.Format(out.Bytes())
	if err != nil {
		return nil, err
	}
	return b, nil
}

func main() {
	flag.Parse()

	if config.FilePath == "" {
		fmt.Println("must provide example file path to write")
		os.Exit(1)
	}

	if err := generateExample(config.FilePath, !config.Invalid); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
