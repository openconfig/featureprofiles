package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
)

// markdownRE matches the heading line: `# XX-1.1: Foo Functional Test`
var markdownRE = regexp.MustCompile(`#(.*?):(.*)`)

// parseMarkdown reads metadata from README.md.
func parseMarkdown(r io.Reader) (*mpb.Metadata, error) {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("missing markdown heading")
	}
	line := sc.Text()
	m := markdownRE.FindStringSubmatch(line)
	if len(m) < 3 {
		return nil, fmt.Errorf("cannot parse markdown: %s", line)
	}
	return &mpb.Metadata{
		PlanId:      strings.TrimSpace(m[1]),
		Description: strings.TrimSpace(m[2]),
	}, nil
}

// parseProto reads metadata from a textproto.
func parseProto(r io.Reader) (*mpb.Metadata, error) {
	bytes, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	md := new(mpb.Metadata)
	return md, prototext.Unmarshal(bytes, md)
}

var marshaller = prototext.MarshalOptions{Multiline: true}

// writeProto generates a complete metadata.textproto to the writer.
func writeProto(w io.Writer, md *mpb.Metadata) error {
	const header = `# proto-file: github.com/openconfig/featureprofiles/proto/metadata.proto
# proto-message: Metadata

`
	if _, err := w.Write([]byte(header)); err != nil {
		return err
	}
	bytes, err := marshaller.Marshal(md)
	if err != nil {
		return err
	}
	_, err = w.Write(bytes)
	return err
}
