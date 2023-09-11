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

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
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

func resetCLI(ctx context.Context, bdut *bindpb.Device, r resolver) error {
	vendorConfig := []string{}
	for _, conf := range bdut.GetConfig().GetCli() {
		vendorConfig = append(vendorConfig, string(conf))
	}
	for _, file := range bdut.GetConfig().GetCliFile() {
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

	dialer, err := r.ssh(bdut.GetName())
	if err != nil {
		return err
	}
	sc, err := dialer.dialSSH()
	if err != nil {
		return err
	}
	defer sc.Close()
	cli, err := newCLI(sc)
	if err != nil {
		return err
	}

	if _, err := cli.SendCommand(ctx, conf); err != nil {
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

func resetGNMI(ctx context.Context, bdut *bindpb.Device, r resolver) error {
	setReq := []*gpb.SetRequest{}
	for _, file := range bdut.GetConfig().GetGnmiSetFile() {
		conf, err := readGNMI(file)
		if err != nil {
			return err
		}
		setReq = append(setReq, conf)
	}
	if len(setReq) == 0 {
		return nil
	}

	dialer, err := r.gnmi(bdut.GetName())
	if err != nil {
		return err
	}
	conn, err := dialer.dialGRPC(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	gnmi := gpb.NewGNMIClient(conn)

	for _, req := range setReq {
		if _, err := gnmi.Set(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

func resetGRIBI(ctx context.Context, bdut *bindpb.Device, r resolver) error {
	if !bdut.GetConfig().GetGribiFlush() {
		return nil
	}

	dialer, err := r.gribi(bdut.GetName())
	if err != nil {
		return err
	}
	conn, err := dialer.dialGRPC(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	gribi := spb.NewGRIBIClient(conn)
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
