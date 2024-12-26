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
	"path/filepath"
	"time"

	log "github.com/golang/glog"
	"github.com/protocolbuffers/txtpbfmt/parser"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	npb "github.com/openconfig/featureprofiles/proto/nosimage_go_proto"
	ppb "github.com/openconfig/featureprofiles/proto/ocpaths_go_proto"
	rpb "github.com/openconfig/featureprofiles/proto/ocrpcs_go_proto"
	opb "github.com/openconfig/ondatra/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

//go:generate go run generate_example.go -folder-path .
//go:generate go run generate_example.go -folder-path . -invalid

// Config is the set of flags for this binary.
type Config struct {
	FolderPath string
	Invalid    bool
}

// New registers a flagset with the configuration needed by this binary.
func New(fs *flag.FlagSet) *Config {
	c := &Config{}

	if fs == nil {
		fs = flag.CommandLine
	}
	fs.StringVar(&c.FolderPath, "folder-path", "", "generate an example file in the given folder rather than validating a file.")
	fs.BoolVar(&c.Invalid, "invalid", false, "generate invalid examples")

	return c
}

var (
	config *Config
)

func init() {
	config = New(nil)
}

func generateExample(invalidPaths, invalidProtocols, invalidSoftwareName, invalidHardwareName bool) ([]byte, error) {
	componentPrefix := "/components/component"
	softwareComponent := "OPERATING_SYSTEM"
	interfaceLeafName := "name"
	if invalidPaths {
		componentPrefix = "/componentsssssssssss/component"
		softwareComponent = "JOVIAN_ATMOSPHERE"
		interfaceLeafName = "does-not-exist"
	}
	var (
		softwareVersion string
		hardwareName    string
	)
	if !invalidSoftwareName {
		softwareVersion = "7a1cb734c83f0d9ba5b273f920bc002ad0056178"
	}
	if !invalidHardwareName {
		hardwareName = "lemming"
	}
	return formatTxtpb(&npb.NOSImageProfile{
		VendorId:        opb.Device_OPENCONFIG,
		Nos:             "lemming",
		SoftwareVersion: softwareVersion,
		HardwareName:    hardwareName,
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
				if !invalidProtocols {
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
					"gnmi": {
						Version: "0.10.0",
						MethodName: []string{
							"gnmi.gNMI.Set",
							"gnmi.gNMI.Subscribe",
							"gnmi.gNMI.Whatsup",
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
							"gnmi.gNMI.Get",
						},
					},
				}
			}(),
		},
	})
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

	if config.FolderPath == "" {
		log.Exitln("must provide example folder path to write to")
	}

	fileSpecs := []struct {
		name                string
		isInvalid           bool
		invalidPaths        bool
		invalidProtocols    bool
		invalidSoftwareName bool
		invalidHardwareName bool
	}{{
		name: "valid",
	}, {
		name:         "invalid-path",
		isInvalid:    true,
		invalidPaths: true,
	}, {
		name:             "invalid-protocols",
		isInvalid:        true,
		invalidProtocols: true,
	}, {
		name:                "invalid-software-name",
		isInvalid:           true,
		invalidSoftwareName: true,
	}, {
		name:                "invalid-hw-name",
		isInvalid:           true,
		invalidHardwareName: true,
	}}

	for _, spec := range fileSpecs {
		if config.Invalid != spec.isInvalid {
			continue
		}

		bs, err := generateExample(spec.invalidPaths, spec.invalidProtocols, spec.invalidSoftwareName, spec.invalidHardwareName)
		if err != nil {
			log.Exitln(err)
		}
		path := filepath.Join(config.FolderPath, spec.name+"_example_nosimageprofile.textproto")
		fmt.Printf("writing to %q\n", path)
		if err := os.WriteFile(path, bs, 0664); err != nil {
			log.Exitln(err)
		}
	}
}
