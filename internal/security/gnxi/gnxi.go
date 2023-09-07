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

// Package gnxi populates a list of all RPCs related for featuresprofile tests. It also
// add additional data (such as paths for each rpc) to simplify security testing.
// Having all rpc in a list also allow us to  write tests that cover all RPCs.
// Package also contains function skeleton for all  RPCs.
// By adding an implementation here, all tests can use the code. This can prevent the duplication and unify the testing.
package gnxi

import (
	"context"

	"github.com/openconfig/ondatra"
	"google.golang.org/grpc"
)

// ExecRPCFunction is a function that is used to provide an implementation for an RPC.
// The focus here is security testing, so in most case a simple call of the RPC should suffice.
type ExecRPCFunction func(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error

// RPC is a data structure for populating information for all FP related RPCs.
type RPC struct {
	// name of the grpc service that provides the RPC
	Service string
	// name of the RPC
	Name string
	// fully qualified name of the RPC
	FQN string
	// Path of the rpc that is used by authz to refer to the rpc
	Path string
	// a function that takes an grpc config (must include mtls cfg) and dut and executes the RPC against the dut.
	Exec ExecRPCFunction
}
