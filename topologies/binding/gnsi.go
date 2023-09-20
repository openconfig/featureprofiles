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

package binding

import (
	"github.com/openconfig/ondatra/binding"
	"google.golang.org/grpc"

	accpb "github.com/openconfig/gnsi/acctz"
	authzpb "github.com/openconfig/gnsi/authz"
	certzpb "github.com/openconfig/gnsi/certz"
	credpb "github.com/openconfig/gnsi/credentialz"
	pathzpb "github.com/openconfig/gnsi/pathz"
)

// gnsiConn implements the stub builder needed by the Ondatra
// binding.Binding interface.
type gnsiConn struct {
	*binding.AbstractGNSIClients
	conn *grpc.ClientConn
}

func (g gnsiConn) Authz() authzpb.AuthzClient { return authzpb.NewAuthzClient(g.conn) }
func (g gnsiConn) Pathz() pathzpb.PathzClient {
	return pathzpb.NewPathzClient(g.conn)
}
func (g gnsiConn) Certz() certzpb.CertzClient { return certzpb.NewCertzClient(g.conn) }
func (g gnsiConn) Credentialz() credpb.CredentialzClient {
	return credpb.NewCredentialzClient(g.conn)
}
func (g gnsiConn) Acctz() accpb.AcctzClient {
	return accpb.NewAcctzClient(g.conn)
}

var _ = binding.GNSIClients(gnsiConn{})
