// Copyright 2022 Google LLC
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

package binding

import (
	"context"
	"os"
	"strings"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gribi/v1/proto/service"
	"google.golang.org/protobuf/encoding/prototext"
)

func readCLI(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func resetCLI(ctx context.Context, dut *staticDUT) error {
	vendorConfig := []string{}
	for _, conf := range dut.dev.GetConfig().GetCli() {
		vendorConfig = append(vendorConfig, string(conf))
	}
	for _, file := range dut.dev.GetConfig().GetCliFile() {
		conf, err := readCLI(file)
		if err != nil {
			return err
		}
		vendorConfig = append(vendorConfig, conf)
	}
	conf := strings.Join(vendorConfig, "\n")

	if conf == "" {
		return nil
	}

	cli, err := dut.DialCLI(ctx)
	if err != nil {
		return err
	}

	if _, err := cli.RunCommand(ctx, conf); err != nil {
		return err
	}
	return nil
}

func readGNMI(path string) (*gpb.SetRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	req := &gpb.SetRequest{}
	if err := prototext.Unmarshal(data, req); err != nil {
		return nil, err
	}
	return req, nil
}

func resetGNMI(ctx context.Context, dut *staticDUT) error {
	setReq := []*gpb.SetRequest{}
	for _, file := range dut.dev.GetConfig().GetGnmiSetFile() {
		conf, err := readGNMI(file)
		if err != nil {
			return err
		}
		setReq = append(setReq, conf)
	}
	if len(setReq) == 0 {
		return nil
	}

	gnmi, err := dut.DialGNMI(ctx)
	if err != nil {
		return err
	}

	for _, req := range setReq {
		if _, err := gnmi.Set(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

func resetGRIBI(ctx context.Context, dut *staticDUT) error {
	if !dut.dev.GetConfig().GetGribiFlush() {
		return nil
	}

	gribi, err := dut.DialGRIBI(ctx)
	if err != nil {
		return err
	}
	req := &spb.FlushRequest{
		NetworkInstance: &spb.FlushRequest_All{
			All: &spb.Empty{},
		},
		Election: &spb.FlushRequest_Override{
			Override: &spb.Empty{},
		},
	}
	_, err = gribi.Flush(ctx, req)
	return err
}
