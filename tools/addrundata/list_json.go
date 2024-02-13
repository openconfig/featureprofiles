package main

import (
	"encoding/json"
	"io"
	"path/filepath"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
)

// listJSON writes the testsuite as a JSON map, mapping from the test package directory to
// the test rundata.
//
// Example:
//
//	{
//	  "feature/subfeature/otg_tests/foo_test": {
//	    "test.uuid": "123e4567-e89b-42d3-8456-426614174000",
//	    "test.plan_id": "XX-1.1",
//	    "test.description": "Foo Functional Test",
//	  },
//	  ...
//	}
func listJSON(w io.Writer, featuredir string, ts testsuite) error {
	rootdir := filepath.Dir(featuredir)

	o := make(map[string]jsonCase)
	for testdir, tc := range ts {
		reldir, err := filepath.Rel(rootdir, testdir)
		if err != nil {
			reldir = testdir
		}
		o[reldir] = newJSONCase(tc.existing)
	}
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

type jsonCase struct {
	UUID        string `json:"test.uuid,omitempty"`
	PlanID      string `json:"test.plan_id,omitempty"`
	Description string `json:"test.description,omitempty"`
}

func newJSONCase(md *mpb.Metadata) jsonCase {
	return jsonCase{
		UUID:        md.Uuid,
		PlanID:      md.PlanId,
		Description: md.Description,
	}
}
