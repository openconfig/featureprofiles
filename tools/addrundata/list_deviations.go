package main

import (
	"encoding/json"
	"io"
	"path/filepath"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
)

func listDeviations(w io.Writer, featuredir string, ts testsuite) error {
	rootdir := filepath.Dir(featuredir)

	o := make(map[string]jsonDevsCase)
	for testdir, tc := range ts {
		reldir, err := filepath.Rel(rootdir, testdir)
		if err != nil {
			reldir = testdir
		}
		o[reldir] = newJSONDevsCase(tc.existing)
	}
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

type jsonDevsCase struct {
	UUID        string `json:"test.uuid,omitempty"`
	PlanID      string `json:"test.plan_id,omitempty"`
	Description string `json:"test.description,omitempty"`
	PlatformExceptions  []*mpb.Metadata_PlatformExceptions `json:"test.platform_exceptions,omitempty"`
}

func newJSONDevsCase(md *mpb.Metadata) jsonDevsCase {
	return jsonDevsCase{
		UUID:        md.Uuid,
		PlanID:      md.PlanId,
		Description: md.Description,
		PlatformExceptions:  md.PlatformExceptions,
	}
}
