// Package mdocspec parses yaml OC requirements from functional test READMEs.
package mdocspec

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/yuin/goldmark"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"

	ppb "github.com/openconfig/featureprofiles/proto/ocpaths_go_proto"
	rpb "github.com/openconfig/featureprofiles/proto/ocrpcs_go_proto"
)

// ErrNotFound indicates the user was not found or is unknown.
var ErrNotFound = errors.New(`did not detect valid "OpenConfig Path and RPC Coverage" yaml, please see https://github.com/openconfig/featureprofiles/blob/main/doc/test-requirements-template.md for example`)

// Parse extracts sorted OpenConfig Path and RPC Coverage from an FNT README.
//
// The first yaml code block after a heading line named exactly as
// "OpenConfig Path and RPC Coverage" will be used. Any other code blocks are
// ignored.
//
// Expected markdown format:
//
//	## OpenConfig Path and RPC Coverage
//
//	```yaml
//	paths:
//	  /interfaces/interface/config/description:
//	  /interfaces/interface/config/enabled:
//	  /components/component/state/name:
//	    platform_type: "CHASSIS"
//
//	rpcs:
//	  gnmi:
//	    gNMI.Set:
//	      union_replace: true
//	    gNMI.Subscribe:
//	      on_change: true
//	```
//
// Note: For `rpcs`, only the RPC name and methods are validated. Any
// attributes defined below RPC methods (e.g. union_replace) are not validated.
func Parse(source []byte) (*ppb.OCPaths, *rpb.OCRPCs, error) {
	var buf bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(MDOCSpecs),
	)
	if err := md.Convert(source, &buf); err != nil {
		return nil, nil, fmt.Errorf("MDOCSpec.Convert: %v", err)
	}
	if buf.Len() == 0 {
		return nil, nil, ErrNotFound
	}

	return parseYAML(buf.Bytes())
}

func parseYAML(source []byte) (*ppb.OCPaths, *rpb.OCRPCs, error) {
	s := map[string]map[string]map[string]any{}
	if err := yaml.Unmarshal(source, &s); err != nil {
		return nil, nil, fmt.Errorf("mdocspec: error parsing YAML: %v", err)
	}

	protoPaths := &ppb.OCPaths{}

	paths, ok := s["paths"]
	if !ok {
		return nil, nil, fmt.Errorf("mdocspec: YAML does not have mandatory top-level `paths` attribute")
	}
	pathNames := maps.Keys(paths)
	sort.Strings(pathNames)
	for _, name := range pathNames {
		var platformType string
		for propertyName, property := range paths[name] {
			switch propertyName {
			case "platform_type":
				p, ok := property.(string)
				if !ok {
					return nil, nil, fmt.Errorf("mdocspec: only string values expected for `platform_type` attribute, got (%T, %v)", property, property)
				}
				platformType = p
			default:
				return nil, nil, fmt.Errorf("mdocspec: only `platform_type` is expected as a valid attribute for paths, got %q", propertyName)
			}
		}
		ocPath := &ppb.OCPath{
			Name: name,
		}
		if platformType != "" {
			ocPath.OcpathConstraint = &ppb.OCPathConstraint{
				Constraint: &ppb.OCPathConstraint_PlatformType{
					PlatformType: platformType,
				},
			}
		}
		protoPaths.Ocpaths = append(protoPaths.Ocpaths, ocPath)
	}

	protoRPCs := &rpb.OCRPCs{
		OcProtocols: map[string]*rpb.OCProtocol{},
	}

	rpcs, ok := s["rpcs"]
	if !ok {
		return nil, nil, fmt.Errorf("mdocspec: YAML does not have mandatory top-level \"rpcs\" attribute")
	}
	rpcNames := maps.Keys(rpcs)
	sort.Strings(rpcNames)
	for _, name := range rpcNames {
		methods := maps.Keys(rpcs[name])
		sort.Strings(methods)
		for i, method := range methods {
			methods[i] = name + "." + method
		}
		protoRPCs.OcProtocols[name] = &rpb.OCProtocol{
			MethodName: methods,
		}
	}

	return protoPaths, protoRPCs, nil
}
