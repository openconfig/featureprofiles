package feature

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/openconfig/ondatra"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func readJSON(fp string) (string, error) {
	data, err := os.ReadFile(fp)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(data, "\n")), nil

}

// ConfigJSON configures a json request over gnmi
func ConfigJSON(dev *ondatra.DUTDevice, t *testing.T, fp string) error {
	client := dev.RawAPIs().GNMI(t)
	rawjson, err := readJSON(fp)
	if err != nil {
		t.Errorf("Unable to read json config file %s %v", fp, err)
	}
	response, err := client.Set(context.Background(), configJSON(rawjson))
	if err != nil {
		t.Log(response)
		t.Error(err)
	}
	return nil
}

func configJSON(config string) *gnmipb.SetRequest {
	return &gnmipb.SetRequest{
		Update: []*gnmipb.Update{{
			Path: &gnmipb.Path{
				Origin: "openconfig",
				Elem:   []*gnmipb.PathElem{},
			},
			Val: &gnmipb.TypedValue{
				Value: &gnmipb.TypedValue_JsonIetfVal{
					JsonIetfVal: []byte(config),
				},
			},
		}},
	}
}
