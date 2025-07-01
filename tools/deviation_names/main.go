// Binary deviation_names prints the names of the fields within the
// metadata Deviations message.
package main

import (
	"fmt"

	"github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Modify the Range function for a protoreflect.Message to be able to cover fields that
// are not populated, since we need to be able to support scalar fields in our ranges.
//
// This code is taken from the updated protojson package - and is used because we need
// to range over all scalar fields within the populated key messages for a list - since
// we should include the values even if they are set to the Go default value (e.g., a uint32
// is set to 0).
type unpopRange struct{ protoreflect.Message }

// Range wraps the protomessage.Range, and sets fields to be marked as non-nil even if they
// are set to the Go default value. This means that we will output fields that are unset as
// their nil values, which is required for list keys within these messages.
func (m unpopRange) Range(f func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	fds := m.Descriptor().Fields()
	for i := 0; i < fds.Len(); i++ {
		fd := fds.Get(i)
		if m.Has(fd) || fd.ContainingOneof() != nil {
			continue // ignore populated fields and fields within a oneofs
		}

		v := m.Get(fd)
		if fd.HasPresence() {
			v = protoreflect.Value{} // use invalid value to emit null
		}
		if !f(fd, v) {
			return
		}
	}
	m.Message.Range(f)
}

func main() {
	m := &metadata_go_proto.Metadata_Deviations{}

	// Wrap with unpopRange so that we don't need to populate all
	// fields. Range on a protoreflect.Message skips unpopulated fields
	// by default.
	unpopRange{m.ProtoReflect()}.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		fmt.Printf("%s\n", fd.Name())
		return true
	})

}
