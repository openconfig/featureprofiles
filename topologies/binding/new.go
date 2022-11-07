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

package binding

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"plugin"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/knebind"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	pluginFile  = flag.String("plugin", "", "vendor binding as a Go plugin")
	pluginArgs  = flag.String("plugin-args", "", "arguments for the vendor binding")
	bindingFile = flag.String("binding", "", "static binding configuration file")
	kneConfig   = flag.String("kne-config", "", "YAML configuration file")
	pushConfig  = flag.Bool("push-config", true, "push device reset config supplied to static binding")
)

// New creates a new binding that could be either a vendor plugin, a
// binding configuration file, or a KNE configuration file.  This
// depends on the command line flags given.
//
// The vendor plugin should be a "package main" with a New function
// that will receive the value of the --plugin-args flag as a string.
//
//	package main
//
//	import "github.com/openconfig/ondatra/binding"
//
//	func New(arg string) (binding.Binding, error) {
//	  ...
//	}
//
// And the plugin should be built with:
//
//	go build -buildmode=plugin
//
// For more detail about how to write a plugin, see: https://pkg.go.dev/plugin
func New() (binding.Binding, error) {
	if *pluginFile != "" {
		return loadBinding(*pluginFile, *pluginArgs)
	}
	if *bindingFile != "" {
		return staticBinding(*bindingFile)
	}
	if *kneConfig != "" {
		cfg, err := knebind.ParseConfigFile(*kneConfig)
		if err != nil {
			return nil, err
		}
		return knebind.New(cfg)
	}
	return nil, errors.New("one of -plugin, -binding, or -kne-config must be provided")
}

// NewFunc describes the type of the New function that a vendor
// binding plugin should provide.
type NewFunc func(arg string) (binding.Binding, error)

// loadBinding loads a binding from a plugin.
func loadBinding(path, args string) (binding.Binding, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}
	newVal, err := p.Lookup("New")
	if err != nil {
		return nil, err
	}
	newFn, ok := newVal.(NewFunc)
	if !ok {
		return nil, fmt.Errorf("func New() has the wrong type %T from plugin: %s", newVal, path)
	}
	return newFn(args)
}

// staticBinding makes a static binding from the binding configuration file.
func staticBinding(bindingFile string) (binding.Binding, error) {
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read binding file: %w", err)
	}
	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return nil, fmt.Errorf("unable to parse binding file: %w", err)
	}
	for _, ate := range b.Ates {
		if ate.Otg != nil && ate.Ixnetwork != nil {
			return nil, fmt.Errorf("otg and ixnetwork are mutually exclusive, please configure one of them in ate %s binding", ate.Name)
		}
	}
	return &staticBind{
		Binding:    nil,
		r:          resolver{b},
		pushConfig: *pushConfig,
	}, nil
}
