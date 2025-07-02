package main

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// This is an example program that outputs specific JSON for an OpenConfig
// schema. It requires the YANG for a particular field to be defined such
// that it can be used.
//
// You can find documentation of the schema on openconfig.net.
// Particularly:
//
//   - https://openconfig.net/projects/models/paths/ - searchable list of paths.
//   - https://openconfig.net/projects/models/schemadocs/ - documentation for each path.
//
// Expected output for the program run:
//
//	{
//	  "interfaces": {
//	    "interface": [
//	      {
//	        "config": {
//	          "description": "a description",
//	          "mtu": 1500,
//	          "name": "eth0",
//	          "type": "ethernetCsmacd"
//	        },
//	        "hold-time": {
//	          "config": {
//	            "up": 42
//	          }
//	        },
//	        "name": "eth0"
//	      }
//	    ]
//	  },
//	  "system": {
//	    "config": {
//	      "hostname": "a hostname"
//	    }
//	  }
//	}
func main() {
	d := &oc.Root{}

	d.GetOrCreateInterface("eth0").GetOrCreateHoldTime().Up = ygot.Uint32(42)
	d.GetOrCreateInterface("eth0").Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	d.GetOrCreateInterface("eth0").Description = ygot.String("a description")
	d.GetOrCreateInterface("eth0").Mtu = ygot.Uint16(1500)

	d.GetOrCreateSystem().Hostname = ygot.String("a hostname")

	fmt.Printf("%v\n", renderJSON(d))
}

func renderJSON(s ygot.GoStruct) string {
	bs, err := ygot.Marshal7951(s, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{PreferShadowPath: true})
	if err != nil {
		glog.Exitf("cannot marshal JSON, %v", err)
		return ""
	}
	return string(bs)
}
