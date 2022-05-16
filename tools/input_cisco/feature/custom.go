package feature

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/openconfig/ondatra"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func readJson(fp string) (string, error) {
	data, err := os.ReadFile(fp)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(data, "\n")), nil

}
func ConfigJson(dev *ondatra.DUTDevice, t *testing.T, fp string) error {
	client := dev.RawAPIs().GNMI().New(t)
	rawjson, err := readJson(fp)
	if err != nil {
		t.Errorf("Unable to read json config file %s %v", fp, err)
	}
	response, err := client.Set(context.Background(), configJson(rawjson))
	if err != nil {
		t.Log(response)
		t.Error(err)
	}
	return nil
}
func UnConfigJson(dev *ondatra.DUTDevice, t *testing.T, fp string) error {
	client := dev.RawAPIs().GNMI().New(t)
	rawjson, err := readJson(fp)
	if err != nil {
		t.Errorf("Unable to read json config file %s %v", fp, err)
	}
	client.Set(context.Background(), configJson(rawjson))
	return nil
}
func configJson(config string) *gnmipb.SetRequest {
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
func unconfigJson(config string) *gnmipb.SetRequest {
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
