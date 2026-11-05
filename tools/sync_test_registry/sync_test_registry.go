// Binary sync_test_registry goes through all the features inside the feature folder
// and read the metadata.textproto. It then generates/overwrites the testregistry.textproto.
// It keeps all entries sorted.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/protocolbuffers/txtpbfmt/parser"
	"google.golang.org/protobuf/encoding/prototext"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	tpb "github.com/openconfig/featureprofiles/proto/testregistry_go_proto"
)

var (
	registryFile = flag.String("registry", "testregistry.textproto", "Path to testregistry.textproto")
	featureDir   = flag.String("feature_dir", "feature", "Path to feature directory")
)

func main() {
	flag.Parse()

	// Preserve registry name if file exists
	registryName := "WBB Test Registry"
	r := &tpb.TestRegistry{}
	if f, err := os.ReadFile(*registryFile); err == nil {
		if err := prototext.Unmarshal(f, r); err == nil && r.GetName() != "" {
			registryName = r.GetName()
		}
	}

	tests, err := loadTestsFromDir(*featureDir)
	if err != nil {
		glog.Exitf("Failed to load tests: %v", err)
	}

	n := generateRegistry(registryName, tests)

	if err := writeRegistry(*registryFile, n); err != nil {
		glog.Exitf("Failed to write registry: %v", err)
	}

	fmt.Println("Successfully updated test registry.")
}

func githubURL(path string) string {
	return fmt.Sprintf("https://github.com/openconfig/featureprofiles/blob/main/%s", filepath.ToSlash(path))
}

func findExecURL(dir string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		glog.Warningf("cannot read directory %s: %v", dir, err)
		return ""
	}
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), "_test.go") {
			return githubURL(filepath.Join(dir, f.Name()))
		}
	}
	return ""
}

func processMetadataFile(path string, tests map[string]*tpb.Test) error {
	f, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read metadata file: %v", err)
	}

	m := &mpb.Metadata{}
	if err := prototext.Unmarshal(f, m); err != nil {
		return fmt.Errorf("invalid metadata input: %v", err)
	}

	if m.GetPlanId() == "" {
		return fmt.Errorf("plan_id is empty")
	}

	dir := filepath.Dir(path)
	readmeURL := githubURL(filepath.Join(dir, "README.md"))
	execURL := findExecURL(dir)

	if existing, ok := tests[m.GetPlanId()]; ok {
		return fmt.Errorf("duplicate plan_id %s found. Old: %v, New: %v", m.GetPlanId(), existing.GetReadme(), readmeURL)
	}

	tests[m.GetPlanId()] = &tpb.Test{
		Id:          m.GetPlanId(),
		Description: m.GetDescription(),
		Readme:      []string{readmeURL},
		Exec:        execURL,
	}

	return nil
}

func loadTestsFromDir(dir string) (map[string]*tpb.Test, error) {
	tests := make(map[string]*tpb.Test)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "metadata.textproto" {
			if err := processMetadataFile(path, tests); err != nil {
				return err
			}
		}
		return nil
	})
	return tests, err
}

func generateRegistry(name string, tests map[string]*tpb.Test) *tpb.TestRegistry {
	var ids []string
	for id := range tests {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	n := &tpb.TestRegistry{
		Name: name,
		Test: []*tpb.Test{},
	}

	for _, id := range ids {
		n.Test = append(n.Test, tests[id])
	}
	return n
}

func writeRegistry(path string, r *tpb.TestRegistry) error {
	mo := &prototext.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}

	s, err := mo.Marshal(r)
	if err != nil {
		return fmt.Errorf("cannot marshal processed proto: %v", err)
	}

	b := &bytes.Buffer{}
	b.WriteString("# proto-file: /proto/testregistry.proto\n")
	b.WriteString("# proto-message: TestRegistry\n\n")
	b.WriteString("# Auto-generated file. Use `make sync-test-registry` to re-generate.\n\n")
	b.Write(s)

	formatted, err := parser.Format(b.Bytes())
	if err != nil {
		return fmt.Errorf("cannot format txtpb: %v", err)
	}

	return os.WriteFile(path, formatted, 0644)
}
