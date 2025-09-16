package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	dpb "github.com/openconfig/featureprofiles/proto/deviations_go_proto"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	deviationsPath        = flag.String("deviations_file", "", "Path to the deviations.textproto file.")
	exemptDeviationsFile  = flag.String("exempt_deviations_file", "", "Path to a file containing a list of deviation names to exempt from validation, one per line.")
	skipCompletenessCheck = flag.Bool("skip_completeness_check", false, "If true, skips the check that all deviations in metadata.proto are present in the deviations file.")
)

// validateDeviation checks a single deviation entry according to the rules.
func validateDeviation(dev *dpb.Deviation) []error {
	var errs []error

	if dev.GetImpactedPaths() == nil {
		errs = append(errs, fmt.Errorf("deviation %q: impacted_paths must be present", dev.GetName()))
	}

	if len(dev.GetPlatforms()) == 0 {
		errs = append(errs, fmt.Errorf("deviation %q: at least one platform must be specified", dev.GetName()))
	}

	for _, p := range dev.GetPlatforms() {
		if p.GetIssueUrl() == "" {
			errs = append(errs, fmt.Errorf("deviation %q: issue_url must be present and be a valid hyperlink", dev.GetName()))
		} else if _, err := url.ParseRequestURI(p.GetIssueUrl()); err != nil {
			errs = append(errs, fmt.Errorf("deviation %q: issue_url %q is not a valid hyperlink: %v", dev.GetName(), p.GetIssueUrl(), err))
		}

		switch dev.GetType() {
		case dpb.DeviationType_DEVIATION_TYPE_VALUE:
			vals := p.GetDeviationValues()
			if vals == nil {
				errs = append(errs, fmt.Errorf("deviation %q: deviation_values must be present for DEVIATION_TYPE_VALUE", dev.GetName()))
			} else if vals.GetOcStandardValue() == nil && vals.GetVendorSpecificValue() == nil {
				errs = append(errs, fmt.Errorf("deviation %q: either oc_standard_value or vendor_specific_value must be set for DEVIATION_TYPE_VALUE", dev.GetName()))
			}
		case dpb.DeviationType_DEVIATION_TYPE_CLI:
			if p.GetClis() == nil || len(p.GetClis().GetCommands()) == 0 {
				errs = append(errs, fmt.Errorf("deviation %q: clis field with at least one command must be present for DEVIATION_TYPE_CLI", dev.GetName()))
			}
		}
	}
	return errs
}

func main() {
	flag.Parse()
	if *deviationsPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --deviations_file flag is required.")
		os.Exit(1)
	}

	// Load and parse the deviations.textproto file.
	textpb, err := ioutil.ReadFile(*deviationsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read deviations file: %v\n", err)
		os.Exit(1)
	}
	var deviationsRegistry dpb.DeviationRegistry
	if err := prototext.Unmarshal(textpb, &deviationsRegistry); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse deviations file: %v\n", err)
		os.Exit(1)
	}

	// Load the exemption list.
	exemptDevs := make(map[string]bool)
	if *exemptDeviationsFile != "" {
		file, err := os.Open(*exemptDeviationsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open exemption file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			exemptDevs[scanner.Text()] = true
		}
	}

	// Get the canonical list of deviation names from the fields of the
	// Deviations message in metadata.proto.
	canonicalDevs := make(map[string]bool)
	devsMsg := (&mpb.Metadata_Deviations{}).ProtoReflect().Descriptor()
	fields := devsMsg.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		canonicalDevs[string(field.Name())] = true
	}

	// Map the deviations from the textproto file for easy lookup. A deviation
	// name can appear multiple times, so we collect all of them.
	foundDevs := make(map[string][]*dpb.Deviation)
	for _, dev := range deviationsRegistry.GetDeviations() {
		foundDevs[dev.GetName()] = append(foundDevs[dev.GetName()], dev)
	}

	// Check for duplicate platforms within deviations of the same name.
	for name, devs := range foundDevs {
		platforms := make(map[string]bool)
		for _, dev := range devs {
			for _, p := range dev.GetPlatforms() {
				platformKey := p.GetPlatform().String()
				if _, ok := platforms[platformKey]; ok {
					fmt.Fprintf(os.Stderr, "Validation Error: Duplicate platform %q for deviation %q found in %s\n", p.GetPlatform(), name, *deviationsPath)
					os.Exit(1)
				}
				platforms[platformKey] = true
			}
		}
	}

	// Check for deviations that are in metadata.proto but not in deviations.textproto.
	if !*skipCompletenessCheck {
		var missing []string
		for name := range canonicalDevs {
			if _, ok := foundDevs[name]; !ok {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "Validation Error: The following deviations are defined in metadata.proto but are missing from %s:\n- %s\n", *deviationsPath, strings.Join(missing, "\n- "))
			os.Exit(1)
		}
	}

	// Validate each deviation in the textproto file.
	var allErrors []string
	for name, devs := range foundDevs {
		if _, ok := canonicalDevs[name]; !ok {
			allErrors = append(allErrors, fmt.Sprintf("deviation %q is in deviations.textproto but not defined in metadata.proto", name))
		}
		// Skip validation for exempted deviations.
		if exemptDevs[name] {
			continue
		}
		for _, dev := range devs {
			if errs := validateDeviation(dev); len(errs) > 0 {
				for _, err := range errs {
					allErrors = append(allErrors, err.Error())
				}
			}
		}
	}

	if len(allErrors) > 0 {
		fmt.Fprintf(os.Stderr, "Validation failed with the following errors:\n- %s\n", strings.Join(allErrors, "\n- "))
		os.Exit(1)
	}

	fmt.Println("Validation successful!")
}
