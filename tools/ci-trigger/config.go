// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	opb "github.com/openconfig/ondatra/proto"
)

const (
	// githubProjectOwner is the path the repository exists under.  This
	// must be a GitHub organization for authorization to work.
	githubProjectOwner = "openconfig"

	// githubProjectRepo is the GitHub repository to work with.
	githubProjectRepo = "featureprofiles"

	// githubBotName is the GitHub username for API-generated issue updates.
	githubBotName = "OpenConfigBot"

	// gcpProjectID is the ID for Cloud Build and Cloud Storage buckets
	gcpProjectID = "disco-idea-817"

	// gcpCloudBuildBucketName is the storage bucket containing Cloud Build source code.
	gcpCloudBuildBucketName = gcpProjectID + "_cloudbuild"

	// gcpBucket is the object storage bucket name for badges and logs
	gcpBucket = "featureprofiles-ci"

	// gcpBucketPrefix is the prefix directory name for all objects stored in the gcpBucket.
	gcpBucketPrefix = "badges"

	// gcpBadgeTopic is the name of the pubsub topic in gcpProjectID for receiving badge status updates.
	gcpBadgeTopic = "featureprofiles-badge-status"

	// gcpPhysicalTestTopic is the name of the pubsub topic in gcpProjectID for launching physical tests.
	gcpPhysicalTestTopic = "featureprofiles-physical-tests"
)

// authorizedTeams is the list of GitHub organization teams authorized to launch Cloud Build jobs.
var authorizedTeams = []string{
	"featureprofiles-maintainers",
	"featureprofiles-quattro-tl",
}

// triggerKeywords is the list of authorized keywords to launch a test.  The
// device types reference the platforms that the keyword will launch tests
// against.
var triggerKeywords = map[string][]deviceType{
	"/fptest all": {
		{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"},
		{Vendor: opb.Device_CISCO, HardwareModel: "8000E"},
		{Vendor: opb.Device_CISCO, HardwareModel: "XRd"},
		{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"},
		{Vendor: opb.Device_JUNIPER, HardwareModel: "ncPTX"},
		{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"},
		{Vendor: opb.Device_OPENCONFIG, HardwareModel: "Lemming"},
	},
	"/fptest physical": {
		{Vendor: opb.Device_ARISTA, HardwareModel: "7808"},
		{Vendor: opb.Device_CISCO, HardwareModel: "8808"},
		{Vendor: opb.Device_JUNIPER, HardwareModel: "PTX10008"},
		{Vendor: opb.Device_NOKIA, HardwareModel: "7250 IXR-10e"},
	},
	"/fptest virtual": {
		{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"},
		{Vendor: opb.Device_CISCO, HardwareModel: "8000E"},
		{Vendor: opb.Device_CISCO, HardwareModel: "XRd"},
		{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"},
		{Vendor: opb.Device_JUNIPER, HardwareModel: "ncPTX"},
		{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"},
		{Vendor: opb.Device_OPENCONFIG, HardwareModel: "Lemming"},
	},
	"/fptest arista-7808":        {{Vendor: opb.Device_ARISTA, HardwareModel: "7808"}},
	"/fptest arista-ceos":        {{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"}},
	"/fptest cisco-8000e":        {{Vendor: opb.Device_CISCO, HardwareModel: "8000E"}},
	"/fptest cisco-8808":         {{Vendor: opb.Device_CISCO, HardwareModel: "8808"}},
	"/fptest cisco-xrd":          {{Vendor: opb.Device_CISCO, HardwareModel: "XRd"}},
	"/fptest juniper-cptx":       {{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"}},
	"/fptest juniper-ncptx":      {{Vendor: opb.Device_JUNIPER, HardwareModel: "ncPTX"}},
	"/fptest juniper-ptx10008":   {{Vendor: opb.Device_JUNIPER, HardwareModel: "PTX10008"}},
	"/fptest nokia-7250":         {{Vendor: opb.Device_NOKIA, HardwareModel: "7250 IXR-10e"}},
	"/fptest nokia-srl":          {{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"}},
	"/fptest openconfig-lemming": {{Vendor: opb.Device_OPENCONFIG, HardwareModel: "Lemming"}},

	// TODO: Deprecate the short device keywords.  The longer vendor-device form prevents overlap.
	"/fptest ceos":    {{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"}},
	"/fptest 8000e":   {{Vendor: opb.Device_CISCO, HardwareModel: "8000E"}},
	"/fptest xrd":     {{Vendor: opb.Device_CISCO, HardwareModel: "XRd"}},
	"/fptest cptx":    {{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"}},
	"/fptest srl":     {{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"}},
	"/fptest lemming": {{Vendor: opb.Device_OPENCONFIG, HardwareModel: "Lemming"}},
}

// virtualDeviceTypes is a list of device types that can execute tests in virtual machines
var virtualDeviceTypes = []deviceType{
	{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"},
	{Vendor: opb.Device_CISCO, HardwareModel: "8000E"},
	{Vendor: opb.Device_CISCO, HardwareModel: "XRd"},
	{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"},
	{Vendor: opb.Device_JUNIPER, HardwareModel: "ncPTX"},
	{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"},
	{Vendor: opb.Device_OPENCONFIG, HardwareModel: "Lemming"},
}

// virtualDeviceMachineType is a map of virtual machines to their expected machine type requirement.
var virtualDeviceMachineType = map[deviceType]string{
	{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"}:        "e2-standard-16",
	{Vendor: opb.Device_CISCO, HardwareModel: "8000E"}:        "n2-standard-32",
	{Vendor: opb.Device_CISCO, HardwareModel: "XRd"}:          "e2-standard-16",
	{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"}:       "n2-standard-32",
	{Vendor: opb.Device_JUNIPER, HardwareModel: "ncPTX"}:      "e2-standard-16",
	{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"}:     "e2-standard-16",
	{Vendor: opb.Device_OPENCONFIG, HardwareModel: "Lemming"}: "e2-standard-16",
}

// physicalDeviceTypes is a list of device types that can execute tests on real hardware.
var physicalDeviceTypes = []deviceType{
	{Vendor: opb.Device_ARISTA, HardwareModel: "7808"},
	{Vendor: opb.Device_CISCO, HardwareModel: "8808"},
	{Vendor: opb.Device_JUNIPER, HardwareModel: "PTX10008"},
	{Vendor: opb.Device_NOKIA, HardwareModel: "7250 IXR-10e"},
}

func titleCase(input string) string {
	return cases.Title(language.English).String(input)
}

var commentTpl = template.Must(template.New("commentTpl").Funcs(template.FuncMap{"titleCase": titleCase}).Parse(`## Pull Request Functional Test Report for #{{.ID}} / {{.HeadSHA}}

{{ if .Virtual }}
### Virtual Devices

| Device | Test | Test Documentation | Job | Raw Log |
| --- | --- | --- | --- | --- |
{{ range .Virtual }}| {{ .Type.Vendor.String | titleCase }} {{ .Type.HardwareModel }} | {{ range .Tests }}[![status]({{ .BadgeURL }})]({{ .TestURL }})<br />{{ end }} | {{ range .Tests }}[{{ .Name }}: {{ .Description }}]({{ .DocURL }})<br />{{ end }} | {{ if and .CloudBuildLogURL .CloudBuildID }}[{{ printf "%.8s" .CloudBuildID }}]({{ .CloudBuildLogURL }}){{ end }} | {{ if .CloudBuildRawLogURL }}[Log]({{ .CloudBuildRawLogURL }}){{ end }} |
{{ end }}{{ end }}{{ if .Physical }}
### Hardware Devices

| Device | Test | Test Documentation | Raw Log |
| --- | --- | --- | --- |
{{ range .Physical }}| {{ .Type.Vendor.String | titleCase }} {{ .Type.HardwareModel }} | {{ range .Tests }}[![status]({{ .BadgeURL }})]({{ .TestURL }})<br />{{ end }} | {{ range .Tests }}[{{ .Name }}: {{ .Description }}]({{ .DocURL }})<br />{{ end }} | {{ if .CloudBuildRawLogURL }}[Log]({{ .CloudBuildRawLogURL }}){{ end }} |
{{ end }}{{ end }}{{ if and (not .Virtual) (not .Physical) }}
No tests identified for validation.
{{ end }}
[Help](https://gist.github.com/OpenConfigBot/7dadd09b7c3133c9d8bc0d08fcb19b46)`))
