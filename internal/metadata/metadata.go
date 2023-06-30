// Package metadata makes the data in metadata.textproto available.
package metadata

import (
	"os"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"google.golang.org/protobuf/encoding/prototext"
)

var md *mpb.Metadata

// Init reads the metadata textproto data.
func Init() error {
	// When "go test" runs, the current working directory is the test
	// package directory, which is where we will find the metadata file.
	const metadataFilename = "metadata.textproto"
	bytes, err := os.ReadFile(metadataFilename)
	if err != nil {
		return err
	}
	md = new(mpb.Metadata)
	return prototext.Unmarshal(bytes, md)
}

// Get returns the metadata for the current test.
func Get() *mpb.Metadata {
	return md
}
