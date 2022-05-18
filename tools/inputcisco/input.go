package inputcisco

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/openconfig/featureprofiles/tools/inputcisco/feature"
	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"
	"github.com/openconfig/featureprofiles/tools/inputcisco/testinput"
	"github.com/openconfig/ondatra"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	yl "sigs.k8s.io/yaml"
)

type inputType int

const (
	json inputType = iota
	textproto
	yml
)

func unmarshall(data []byte, inType inputType) (*proto.Input, error) {
	input := &proto.Input{}
	switch inType {
	case json:
		if err := protojson.Unmarshal(data, input); err != nil {
			return input, errors.Wrap(err, "error unmarshalling test input file")
		}

	case yml:
		data, _ := yl.YAMLToJSON(data)
		if err := protojson.Unmarshal(data, input); err != nil {
			return input, errors.Wrap(err, "error unmarshalling test input file")
		}

	default:
		if err := prototext.Unmarshal(data, input); err != nil {
			return input, errors.Wrap(err, "error unmarshalling test input file")
		}
	}
	return input, nil
}

func getInput(inputFilePath string) (*proto.Input, error) {
	inputFile, err := ioutil.ReadFile(inputFilePath)
	if err != nil {
		return &proto.Input{}, errors.Wrapf(err, "failed to read input File proto %s", inputFilePath)
	}
	inType := textproto
	if filepath.Ext(inputFilePath) == "json" {
		inType = json
	}

	switch filepath.Ext(inputFilePath) {
	case ".json":
		inType = json
	case ".yaml", ".yml":
		inType = yml
	default:
		inType = textproto
	}
	return unmarshall(inputFile, inType)
}

func getTestInput(t *testing.T, input *proto.Input) *testInput {
	return &testInput{
		t:    t,
		data: input,
	}
}

// Input holds the input File parameters like filepath , err and Input variable
type Input struct {
	In            *proto.Input
	err           error
	inputFilePath string
}

// LoadInput creates an input file object
func LoadInput(inputFilePath string) *Input {
	in, err := getInput(inputFilePath)
	return &Input{
		In:            in,
		err:           err,
		inputFilePath: inputFilePath,
	}

}

// GetTestInput opens the input file and return the protObject
func (x *Input) GetTestInput(t *testing.T) (testinput.TestInput, error) {
	t.Helper()
	if x.err != nil {
		t.Logf("error, reading input file %s %v", x.inputFilePath, x.err)
	}
	return getTestInput(t, x.In), x.err

}

type testInput struct {
	data *proto.Input
	t    *testing.T
}

// ConfigInterfaces configures interfaces for the device
func (in *testInput) ConfigInterfaces(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	interfaces := dvc.Interface
	for _, intf := range interfaces {
		err := feature.ConfigInterfaces(dev, in.t, intf)
		if err != nil {
			in.t.Logf("error in Configuring Interface")
			in.t.Error(err)
		}
	}
}

// ConfigVrf configures Vrfs for the device
func (in *testInput) ConfigVrf(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	vrfs := dvc.Vrf
	for _, vrf := range vrfs {
		err := feature.ConfigVrf(dev, in.t, vrf)
		if err != nil {
			in.t.Logf("error in Configuring VRF")
			in.t.Error(err)
		}
	}
}

// ReplacesVrf Replaces Vrfs for the device
func (in *testInput) ReplaceVrf(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	vrfs := dvc.Vrf
	for _, vrf := range vrfs {
		err := feature.ReplaceVrf(dev, in.t, vrf)
		if err != nil {
			in.t.Logf("error in Replacing VRF")
			in.t.Error(err)
		}
	}
}

func getFeatureConfig(data *proto.Input, devid string) *proto.Input_Feature {
	if data.Feature == nil {

		return &proto.Input_Feature{}
	}
	dvc := data.Feature[devid]
	if dvc == nil {
		return &proto.Input_Feature{}
	}
	return data.Feature[devid]
}

// Config configures the device
func (in *testInput) Config(dev *ondatra.DUTDevice) {
	in.ConfigVrf(dev)
	in.ConfigInterfaces(dev)
	in.ConfigProtocols(dev)
}

// ConfigProtocols configures protocols for the device
func (in *testInput) ConfigProtocols(dev *ondatra.DUTDevice) {
	in.ConfigRPL(dev)
	in.ConfigBGP(dev)
	in.ConfigISIS(dev)
	in.ConfigJSON(dev)
}

// ConfigAte configures ATE
func (in *testInput) ConfigAte(dev *ondatra.ATEDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	feature.ConfigIxiaTopology(dev, in.t, dvc)

}

// StartAteProtocols starts protocols on ATE
func (in *testInput) StartAteProtocols(dev *ondatra.ATEDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	feature.StartIxiaProtocols(dev, in.t, dvc)
}

// ConfigRPL configures RPL
func (in *testInput) ConfigRPL(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	rpls := dvc.Routepolicy
	for _, rpl := range rpls {
		err := feature.ConfigRPL(dev, in.t, rpl)
		if err != nil {
			in.t.Logf("error in Configuring RPL")
			in.t.Error(err)
		}
	}
}

// ReplaceRPL Replaces RPL in the device
func (in *testInput) ReplaceRPL(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	rpls := dvc.Routepolicy
	for _, rpl := range rpls {
		err := feature.ReplaceRPL(dev, in.t, rpl)
		if err != nil {
			in.t.Logf("error in Replacing RPL")
			in.t.Error(err)
		}
	}
}

// UnConfigRPL removes RPL in the device
func (in *testInput) UnConfigRPL(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	rpls := dvc.Routepolicy
	for _, rpl := range rpls {
		err := feature.UnConfigRPL(dev, in.t, rpl)
		if err != nil {
			in.t.Logf("error in UnConfiguring RPL")
			in.t.Error(err)
		}
	}
}

// ConfigBGP configure BGP on the device
func (in *testInput) ConfigBGP(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	bgps := dvc.Bgp
	for _, bgp := range bgps {
		err := feature.ConfigBGP(dev, in.t, bgp, in)
		if err != nil {
			in.t.Logf("error in Configuring BGP")
			in.t.Error(err)
		}
	}
}

// ConfigJSON sends a Raw JSON config
func (in *testInput) ConfigJSON(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	jsonConfigs := dvc.JsonConfig
	for _, jsonConfig := range jsonConfigs {
		err := feature.ConfigJSON(dev, in.t, jsonConfig)
		if err != nil {
			in.t.Logf("error in Configuring from Json %s", jsonConfig)
			in.t.Error(err)
		}
	}
}

// ConfigISIS configures ISIS on the device
func (in *testInput) ConfigISIS(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	fmt.Println(dvc)
	isiss := dvc.Isis
	for _, isis := range isiss {
		fmt.Println(isis)
		err := feature.ConfigISIS(dev, in.t, isis)
		if err != nil {
			in.t.Logf("error in Configuring ISIS")
			in.t.Error(err)
		}
	}
}

// UnConfig Removes all device configs as in input file
func (in *testInput) UnConfig(dev *ondatra.DUTDevice) {
	in.UnConfigProtocols(dev)
	in.UnConfigInterfaces(dev)
	in.UnConfigVrf(dev)
}

// UnConfigVrf removes Vrf config from device
func (in *testInput) UnConfigVrf(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	vrfs := dvc.Vrf
	for _, vrf := range vrfs {
		err := feature.UnConfigVrf(dev, in.t, vrf)
		if err != nil {
			in.t.Logf("error in Configuring VRF")
			in.t.Error(err)
		}
	}
}

// UnConfigProtocols removes protocol configurations from device
func (in *testInput) UnConfigProtocols(dev *ondatra.DUTDevice) {
	in.UnConfigRPL(dev)
	in.UnConfigBGP(dev)
	in.UnConfigISIS(dev)
}

// UnConfigBGP removes BGP configurations from device
func (in *testInput) UnConfigBGP(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	bgps := dvc.Bgp
	for _, bgp := range bgps {
		err := feature.UnConfigBGP(dev, in.t, bgp)
		if err != nil {
			in.t.Logf("error in Removing BGP")
			in.t.Error(err)
		}
	}
}

// UnConfigISIS removes ISIS from the device
func (in *testInput) UnConfigISIS(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	isiss := dvc.Isis
	for _, isis := range isiss {
		err := feature.UnConfigISIS(dev, in.t, isis)
		if err != nil {
			in.t.Logf("error in Removing ISIS")
			in.t.Error(err)
		}
	}
}

// UnConfigInterfaces unconfigures all interface configurations
func (in *testInput) UnConfigInterfaces(dev *ondatra.DUTDevice) {
	dvc := getFeatureConfig(in.data, dev.ID())
	interfaces := dvc.Interface
	for _, intf := range interfaces {
		err := feature.UnConfigInterfaces(dev, in.t, intf)
		if err != nil {
			in.t.Logf("error in Configuring Interface")
			in.t.Error(err)
		}
	}
}

type device struct {
	dev        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	features   *proto.Input_Feature
	interfaces []*feature.IfObject
}

// Interfaces all resolved interfaces
func (in *device) Interfaces() []*feature.IfObject {
	return in.interfaces
}

// Device returns Device properties (features and interfaces)
func (in *testInput) Device(dev *ondatra.DUTDevice) testinput.Device {
	devFeatures := &proto.Input_Feature{}
	features := in.data.Feature

	if features != nil {
		devFeatures = in.data.Feature[dev.ID()]
	}

	interfaces := []*feature.IfObject{}
	if devFeatures != nil {
		for _, intf := range devFeatures.Interface {
			interfaces = append(interfaces, feature.GetIFs(dev, in.t, intf)...)
		}
	}
	return &device{
		dev:        dev,
		features:   devFeatures,
		interfaces: interfaces,
	}
}

// ATE returns ATE properties (features and interfaces)
func (in *testInput) ATE(dev *ondatra.ATEDevice) testinput.ATE {
	devFeatures := &proto.Input_Feature{}
	features := in.data.Feature

	if features != nil {
		devFeatures = in.data.Feature[dev.ID()]
	}

	interfaces := []*feature.IfObject{}

	return &device{
		ate:        dev,
		features:   devFeatures,
		interfaces: interfaces,
	}
}

type ifgroup struct {
	ifnames        []string
	v4addresses    []string
	v4addressmasks []string
	v6addresses    []string
	v6addressmasks []string
	interfaces     map[string]*feature.IfObject
}

// Features returns all features for a device
func (in *device) Features() *proto.Input_Feature {
	return in.features

}

// GetInterface returns  the required interface
func (in *device) GetInterface(name string) testinput.Intf {
	for _, intf := range in.interfaces {
		if intf.Name() == name {
			return intf
		}
	}
	for _, intf := range in.interfaces {
		if intf.ID() == name {
			return intf
		}
	}

	return nil

}

// IFGroup returns a particular interface group
func (in *device) IFGroup(groupName string) testinput.IfGroup {
	ifg := ifgroup{
		ifnames:        []string{},
		v4addresses:    []string{},
		v4addressmasks: []string{},
		v6addresses:    []string{},
		v6addressmasks: []string{},
		interfaces:     map[string]*feature.IfObject{},
	}

	for _, intf := range in.interfaces {
		if intf.Group() == groupName {
			if intf.Name() != "" {
				ifg.ifnames = append(ifg.ifnames, intf.Name())
			}
			if intf.Ipv4Address() != "" {
				ifg.v4addresses = append(ifg.v4addresses, intf.Ipv4Address())
			}
			if intf.Ipv4AddressMask() != "" {
				ifg.v4addressmasks = append(ifg.v4addressmasks, intf.Ipv4AddressMask())
			}
			if intf.Ipv6Address() != "" {
				ifg.v6addresses = append(ifg.v6addresses, intf.Ipv6Address())
			}
			if intf.Ipv6AddressMask() != "" {
				ifg.v6addressmasks = append(ifg.v6addressmasks, intf.Ipv6AddressMask())
			}
			if intf.Name() != "" {
				ifg.interfaces[intf.Name()] = intf
			}
		}
	}
	return &ifg
}

// Names return all IF names  in a Group
func (x *ifgroup) Names() []string { return x.ifnames }

// Ipv4Addresses return all IF Ipv4Addresses  in a Group
func (x *ifgroup) Ipv4Addresses() []string { return x.v4addresses }

// Ipv4AddressMasks return all IF Ipv4AddressMasks  in a Group
func (x *ifgroup) Ipv4AddressMasks() []string { return x.v4addressmasks }

// Ipv6Addresses return all IF Ipv6Addresses  in a Group
func (x *ifgroup) Ipv6Addresses() []string { return x.v6addresses }

// Ipv6AddressMasks return all IF Ipv6AddressMasks in a Group
func (x *ifgroup) Ipv6AddressMasks() []string { return x.v6addressmasks }

// nterfaces return all IF objects  in a Group
func (x *ifgroup) Interfaces() map[string]*feature.IfObject { return x.interfaces }
