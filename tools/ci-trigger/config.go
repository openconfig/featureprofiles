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
	"/fptest virtual": {
		{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"},
		{Vendor: opb.Device_CISCO, HardwareModel: "8000E"},
		{Vendor: opb.Device_CISCO, HardwareModel: "XRd"},
		{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"},
		{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"},
	},
	"/fptest ceos":  {{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"}},
	"/fptest 8000e": {{Vendor: opb.Device_CISCO, HardwareModel: "8000E"}},
	"/fptest xrd":   {{Vendor: opb.Device_CISCO, HardwareModel: "XRd"}},
	"/fptest cptx":  {{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"}},
	"/fptest srl":   {{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"}},
	"/fptest all": {
		{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"},
		{Vendor: opb.Device_CISCO, HardwareModel: "8000E"},
		{Vendor: opb.Device_CISCO, HardwareModel: "XRd"},
		{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"},
		{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"},
	},
}

// virtualDeviceTypes is a list of device types that can execute tests in virtual machines
var virtualDeviceTypes = []deviceType{
	{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"},
	{Vendor: opb.Device_CISCO, HardwareModel: "8000E"},
	{Vendor: opb.Device_CISCO, HardwareModel: "XRd"},
	{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"},
	{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"},
}

// virtualDeviceMachineType is a map of virtual machines to their expected machine type requirement.
var virtualDeviceMachineType = map[deviceType]string{
	{Vendor: opb.Device_ARISTA, HardwareModel: "cEOS"}:    "e2-standard-4",
	{Vendor: opb.Device_CISCO, HardwareModel: "8000E"}:    "n2-standard-8",
	{Vendor: opb.Device_CISCO, HardwareModel: "XRd"}:      "e2-standard-4",
	{Vendor: opb.Device_JUNIPER, HardwareModel: "cPTX"}:   "n2-standard-16",
	{Vendor: opb.Device_NOKIA, HardwareModel: "SR Linux"}: "e2-standard-4",
}

func titleCase(input string) string {
	return cases.Title(language.English).String(input)
}

var commentTpl = template.Must(template.New("commentTpl").Funcs(template.FuncMap{"titleCase": titleCase}).Parse(`## Pull Request Functional Test Report for #{{.ID}} / {{.HeadSHA}}

{{ if .Virtual }}
### Virtual Devices

| Device | Test | Test Documentation | Job |
| --- | --- | --- | --- |
{{ range .Virtual }}| {{ .Type.Vendor.String | titleCase }} {{ .Type.HardwareModel }} | {{ range .Tests }}[![status]({{ .BadgeURL }})]({{ .TestURL }})<br />{{ end }} | {{ range .Tests }}[{{ .Name }}: {{ .Description }}]({{ .DocURL }})<br />{{ end }} | {{ if and .CloudBuildLogURL .CloudBuildID }}[{{ printf "%.8s" .CloudBuildID }}]({{ .CloudBuildLogURL }}){{ end }} |
{{ end }}{{ end }}{{ if .Physical }}
### Hardware Devices

| Device | Tests | Job |
| --- | --- | --- |
{{ range .Physical }}| {{ .Type.Vendor.String | titleCase }} {{ .Type.HardwareModel }} | {{ range .Tests }}[![status]({{ .BadgeURL }})]({{ .TestURL }}) [{{ .Name }}]({{ .DocURL }})<br />{{ end }} | {{ if and .CloudBuildLogURL .CloudBuildID }}[{{ printf "%.8s" .CloudBuildID }}]({{ .CloudBuildLogURL }}){{ end }} |{{ end }}

{{ end }}{{ if and (not .Virtual) (not .Physical) }}
No tests identified for validation.
{{ end }}
[Help](https://gist.github.com/OpenConfigBot/7dadd09b7c3133c9d8bc0d08fcb19b46)`))
