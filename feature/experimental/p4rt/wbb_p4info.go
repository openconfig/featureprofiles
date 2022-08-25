// Package wbbp4info returns the static P4Info struct for configuring P4 Runtime
// on WBB devices.
package wbbp4info

import (
	"errors"
	"sync"

	"google3/third_party/golang/protobuf/v2/encoding/prototext/prototext"
	p4pb "google3/third_party/p4lang_p4runtime/proto/p4info_go_proto"
)

var (
	// p4info is cached P4Info protobuf data
	p4info   *p4pb.P4Info
	initOnce sync.Once
)

// initP4info initializes the global p4info by unmarshaling the Go embed
// data in memory.
func initP4info() error {
	if p4info != nil {
		return nil
	}

	if len(p4infoRaw) == 0 {
		return errors.New("bad p4info embed data")
	}

	// p4infoRaw embed data is available as a map with a single
	// key and the value contains the p4info protobuf text.
	for _, v := range p4infoRaw {
		p4info = new(p4pb.P4Info)
		if err := prototext.Unmarshal(v, p4info); err != nil {
			return err
		}
		break
	}
	return nil
}

// Get returns the static P4Info after unmarshaling raw Go embed data from file
func Get() (*p4pb.P4Info, error) {
	var err error
	initOnce.Do(func() { err = initP4info() })
	return p4info, err
}
