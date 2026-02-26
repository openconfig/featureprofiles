package main

import (
	"testing"

	dpb "github.com/openconfig/featureprofiles/proto/deviations_go_proto"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	ocpb "github.com/openconfig/featureprofiles/proto/ocpaths_go_proto"
	opb "github.com/openconfig/ondatra/proto"
)

func TestValidateDeviation(t *testing.T) {
	tests := []struct {
		name    string
		dev     *dpb.Deviation
		wantErr bool
	}{
		{
			name: "valid deviation",
			dev: &dpb.Deviation{
				Name: "valid-deviation",
				ImpactedPaths: &ocpb.OCPaths{
					Ocpaths: []*ocpb.OCPath{
						{Name: "/interfaces/interface/state"},
					},
				},
				Platforms: []*dpb.PlatformData{
					{
						Platform: &mpb.Metadata_Platform{Vendor: opb.Device_CISCO},
						IssueUrl: "http://valid-url.com",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing impacted paths",
			dev: &dpb.Deviation{
				Name: "missing-impacted-paths",
				Platforms: []*dpb.PlatformData{
					{
						Platform: &mpb.Metadata_Platform{Vendor: opb.Device_CISCO},
						IssueUrl: "http://valid-url.com",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing issue url",
			dev: &dpb.Deviation{
				Name: "missing-issue-url",
				ImpactedPaths: &ocpb.OCPaths{
					Ocpaths: []*ocpb.OCPath{
						{Name: "/interfaces/interface/state"},
					},
				},
				Platforms: []*dpb.PlatformData{
					{
						Platform: &mpb.Metadata_Platform{Vendor: opb.Device_CISCO},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid issue url",
			dev: &dpb.Deviation{
				Name: "invalid-issue-url",
				ImpactedPaths: &ocpb.OCPaths{
					Ocpaths: []*ocpb.OCPath{
						{Name: "/interfaces/interface/state"},
					},
				},
				Platforms: []*dpb.PlatformData{
					{
						Platform: &mpb.Metadata_Platform{Vendor: opb.Device_CISCO},
						IssueUrl: "not a valid url",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if errs := validateDeviation(tt.dev); len(errs) > 0 && !tt.wantErr {
				t.Errorf("validateDeviation() for %q got unexpected errors: %v", tt.name, errs)
			} else if len(errs) == 0 && tt.wantErr {
				t.Errorf("validateDeviation() for %q did not get an expected error", tt.name)
			}
		})
	}
}
