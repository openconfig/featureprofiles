package gmeta_test

import (
	"flag"
	"os"
	"testing"
	"text/template"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	yamlTemplate = `vendor: "CISCO" 
target-type: "hardware"  
platform-name: "8800" 
architecture: "x86_64"
firmware-version: "{{.SoftwareVersion}}"	
`
)

var (
	outFile = flag.String("outFile", "metadata.yaml", "Output file")
)

func TestGenerateGoogleImageMetadata(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	swVer := gnmi.Lookup(t, dut, gnmi.OC().System().SoftwareVersion().State())
	version, ok := swVer.Val()
	if !ok && version != "" {
		t.Fatalf("System software version was not reported")
	}
	t.Logf("Got a system software version value %q", version)

	tmpl, err := template.New("yaml").Parse(yamlTemplate)
	if err != nil {
		t.Fatalf("Error parsing template: %v", err)
	}

	outFile, err := os.Create(*outFile)
	if err != nil {
		t.Fatalf("Error creating output file: %v", err)
	}
	defer outFile.Close()

	if err := tmpl.Execute(outFile, struct {
		SoftwareVersion string
	}{
		SoftwareVersion: version,
	}); err != nil {
		t.Fatalf("Error executing template: %v", err)
	}
}
