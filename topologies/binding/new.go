package binding

import (
	"errors"
	"flag"
	"fmt"
	"plugin"

	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/knebind"
)

var (
	pluginFile  = flag.String("plugin", "", "vendor binding as a Go plugin")
	pluginArgs  = flag.String("plugin-args", "", "arguments for the vendor binding")
	bindingFile = flag.String("binding", "", "binding configuration file")
	kneConfig   = flag.String("kne-config", "", "YAML configuration file")
)

// New creates a new binding that could be either a vendor plugin, a
// binding configuration file, or a KNE configuration file.  This
// depends on the command line flags given.
//
// The vendor plugin should be a "package main" with a New function
// that will receive the value of the --plugin-args flag as a string.
//
//   package main
//
//   import "github.com/openconfig/ondatra/binding"
//
//   func New(arg string) (binding.Binding, error) {
//     ...
//   }
//
// And the plugin should be built with:
//
//   go build -buildmode=plugin
//
// For more detail about how to write a plugin, see: https://pkg.go.dev/plugin
func New() (binding.Binding, error) {
	if *pluginFile != "" {
		return loadBinding(*pluginFile, *pluginArgs)
	}
	if *bindingFile != "" {
		return nil, errors.New("-binding is not implemented")
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
