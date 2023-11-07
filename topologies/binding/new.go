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
	"context"
	"errors"
	"fmt"
	"os"
	"plugin"
	"time"

	"flag"

	"github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/rundata"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/knebind"
	knecreds "github.com/openconfig/ondatra/knebind/creds"
	opb "github.com/openconfig/ondatra/proto"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	pluginFile   = flag.String("plugin", "", "vendor binding as a Go plugin")
	pluginArgs   = flag.String("plugin-args", "", "arguments for the vendor binding")
	bindingFile  = flag.String("binding", "", "static binding configuration file")
	kneConfig    = flag.String("kne-config", "", "YAML configuration file")
	pushConfig   = flag.Bool("push-config", true, "push device reset config supplied to static binding")
	kneTopo      = flag.String("kne-topo", "", "KNE topology file")
	kneSkipReset = flag.Bool("kne-skip-reset", false, "skip the initial config reset phase when using KNE")
	credFlags    = knecreds.DefineFlags()
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
	b, err := newBind()
	if err != nil {
		return nil, err
	}
	return &rundataBind{Binding: b}, nil
}

func newBind() (binding.Binding, error) {
	if *pluginFile != "" {
		return loadBinding(*pluginFile, *pluginArgs)
	}
	if *bindingFile != "" {
		return staticBinding(*bindingFile)
	}
	if *kneTopo != "" {
		cred, err := credFlags.Parse()
		if err != nil {
			return nil, err
		}
		return knebind.New(&knebind.Config{
			Topology:    *kneTopo,
			Credentials: cred,
			SkipReset:   *kneSkipReset,
		})
	}
	if *kneConfig != "" {
		glog.Warning("-kne-config flag is deprecated; use -kne-topo and credentials flags instead")
		cfg, err := knebind.ParseConfigFile(*kneConfig)
		if err != nil {
			return nil, err
		}
		return knebind.New(cfg)
	}
	return nil, errors.New("one of -plugin, -binding, or -kne-topo must be provided")
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

// rundataBind wraps an Ondatra binding to report rundata.
type rundataBind struct {
	binding.Binding
}

func (b *rundataBind) Reserve(ctx context.Context, tb *opb.Testbed, runTime, waitTime time.Duration, partial map[string]string) (*binding.Reservation, error) {
	resv, err := b.Binding.Reserve(ctx, tb, runTime, waitTime, partial)
	if err != nil {
		return nil, err
	}
	b.addResvProperties(ctx, resv)
	return resv, nil
}

func (b *rundataBind) FetchReservation(ctx context.Context, id string) (*binding.Reservation, error) {
	resv, err := b.Binding.FetchReservation(ctx, id)
	if err != nil {
		return nil, err
	}
	b.addResvProperties(ctx, resv)
	return resv, nil
}

func (b *rundataBind) addResvProperties(ctx context.Context, resv *binding.Reservation) {
	for k, v := range rundata.Properties(ctx, resv) {
		ondatra.Report().AddSuiteProperty(k, v)
	}
}

func (b *rundataBind) Release(ctx context.Context) error {
	for k, v := range rundata.Timing(ctx) {
		ondatra.Report().AddSuiteProperty(k, v)
	}
	return b.Binding.Release(ctx)
}
