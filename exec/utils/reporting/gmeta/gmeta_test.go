package gmeta_test

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"strings"
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
target-type: "{{.TargetType}}"  
platform-name: "{{.PlatformName}}" 
architecture: "x86_64"
firmware-version: "{{.SoftwareVersion}}"	
`
)

var (
	imagePath = flag.String("image", "", "Image to generate metadata for")
	outDir    = flag.String("out", "", "Output directory")
)

type templateData struct {
	TargetType      string
	PlatformName    string
	SoftwareVersion string
	FileName        string
}

func TestGenerateGoogleImageMetadata(t *testing.T) {
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		t.Fatalf("Error creating output directory")
	}

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

	imageName := strings.TrimSuffix(filepath.Base(*imagePath), "]")
	for _, td := range []templateData{
		{
			TargetType:      "hardware",
			PlatformName:    "8800",
			SoftwareVersion: version,
			FileName:        imageName + ".yaml",
		},
		{
			TargetType:      "software",
			PlatformName:    "8000e",
			SoftwareVersion: version,
			FileName:        "c8202-pvt-" + version + ".tar.yaml",
		},
		{
			TargetType:      "software",
			PlatformName:    "XRD",
			SoftwareVersion: version,
			FileName:        "xrd-control-plane-container-x64.dockerv1-" + version + ".tgz.yaml",
		},
	} {
		outFile, err := os.Create(path.Join(*outDir, td.FileName))
		if err != nil {
			t.Fatalf("Error creating output file: %v", err)
		}
		defer outFile.Close()

		if err := tmpl.Execute(outFile, td); err != nil {
			t.Fatalf("Error executing template: %v", err)
		}
	}
}
