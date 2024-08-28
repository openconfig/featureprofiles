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

// package main generate  data structure and skeleton function for all rpc related to fp.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/protobuf/reflect/protoreflect"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	accpb "github.com/openconfig/gnsi/acctz"
	authzpb "github.com/openconfig/gnsi/authz"
	certzpb "github.com/openconfig/gnsi/certz"
	credpb "github.com/openconfig/gnsi/credentialz"
	pathzpb "github.com/openconfig/gnsi/pathz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"

	bpb "github.com/openconfig/gnoi/bgp"
	dpb "github.com/openconfig/gnoi/diag"
	frpb "github.com/openconfig/gnoi/factory_reset"
	fpb "github.com/openconfig/gnoi/file"
	hpb "github.com/openconfig/gnoi/healthz"
	lpb "github.com/openconfig/gnoi/layer2"
	mpb "github.com/openconfig/gnoi/mpls"
	ospb "github.com/openconfig/gnoi/os"
	otpb "github.com/openconfig/gnoi/otdr"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	spb "github.com/openconfig/gnoi/system"
	wpb "github.com/openconfig/gnoi/wavelength_router"
	p4rtpb "github.com/p4lang/p4runtime/go/p4/v1"

	log "github.com/golang/glog"
)

var (
	srcFolder   = flag.String("src_folder", ".", "The directory where the generated source code will be saved")
	genExecFunc = flag.Bool("gen_exec_func", true, "if set to true, the skeleton for exec function with be generated")
	pkgName     = flag.String("pkg_name", "gnxi", "The name of the package for the generated source code")
)

var (
	services = map[string]protoreflect.ServiceDescriptors{
		"gnmi":                    gnmipb.File_proto_gnmi_gnmi_proto.Services(),
		"gribi":                   gribipb.File_v1_proto_service_gribi_proto.Services(),
		"gnsi.authz":              authzpb.File_github_com_openconfig_gnsi_authz_authz_proto.Services(),
		"gnsi.certz":              certzpb.File_github_com_openconfig_gnsi_certz_certz_proto.Services(),
		"gnsi.pathz":              pathzpb.File_github_com_openconfig_gnsi_pathz_pathz_proto.Services(),
		"gnsi.cred":               credpb.File_github_com_openconfig_gnsi_credentialz_credentialz_proto.Services(),
		"gnsi.acc":                accpb.File_github_com_openconfig_gnsi_acctz_acctz_proto.Services(),
		"gnoi.bgp":                bpb.File_bgp_bgp_proto.Services(),
		"gnoi.diag":               dpb.File_diag_diag_proto.Services(),
		"gnoi.factory_reset":      frpb.File_factory_reset_factory_reset_proto.Services(),
		"gnoi.file":               fpb.File_file_file_proto.Services(),
		"gnoi.healthz":            hpb.File_healthz_healthz_proto.Services(),
		"gnoi.layer2":             lpb.File_layer2_layer2_proto.Services(),
		"gnoi.mppls":              mpb.File_mpls_mpls_proto.Services(),
		"gnoi.os":                 ospb.File_os_os_proto.Services(),
		"gnoi.otdr":               otpb.File_otdr_otdr_proto.Services(),
		"gnoi.link_qualification": plqpb.File_packet_link_qualification_packet_link_qualification_proto.Services(),
		"gnoi.system":             spb.File_system_system_proto.Services(),
		"gnoi.wpb":                wpb.File_wavelength_router_wavelength_router_proto.Services(),
		"p4rt":                    p4rtpb.File_p4_v1_p4runtime_proto.Services(),
	}
)

func main() {

	rpcMap := make(map[string]*gnxi.RPC)
	p4rtpb.File_p4_v1_p4data_proto.Services()
	// add * that represent all grpc rpcs
	rpcMap["ALL"] = &gnxi.RPC{
		Name:    "*",
		Service: "*",
		FQN:     "*",
		Path:    "*",
	}
	for serviceName, service := range services {
		if service.Len() == 0 {
			log.Warningf("service %s has no rpc methods\n", serviceName)
		}
		for i := 0; i < service.Len(); i++ {
			log.Infof("Service %s RPCs are: \n", serviceName)
			for j := 0; j < service.Get(i).Methods().Len(); j++ {
				rpcName := fmt.Sprintf("%v", service.Get(i).Methods().Get(j).Name())
				enumName := fmt.Sprintf("%s_%s", strings.ToUpper(serviceName), strings.ToUpper(rpcName))
				rpcFQN := fmt.Sprintf("%v", service.Get(i).Methods().Get(j).FullName())
				path := ""
				lastPointIdx := strings.LastIndex(rpcFQN, ".")
				if lastPointIdx > 0 {
					path = "/" + rpcFQN[:lastPointIdx] + "/" + rpcFQN[lastPointIdx+1:]
				}
				serviceName = rpcFQN[:lastPointIdx]
				rpcMap[enumName] = &gnxi.RPC{
					Name:    rpcName,
					Service: serviceName,
					FQN:     rpcFQN,
					Path:    path,
				}
				log.Infof("\t RPC is %v\n", rpcMap[enumName])
				log.Infof("\t %s: %s \n", service.Get(i).Methods().Get(j).Name(), service.Get(i).Methods().Get(j).FullName())

			}
			// add service/* for each service that represents all RPC for the service
			rpcMap[strings.ToUpper(serviceName)+"_ALL"] = &gnxi.RPC{
				Name:    "*",
				Service: serviceName,
				FQN:     serviceName + ".*",
				Path:    "/" + serviceName + "/*",
			}
		}

	}

	rpcsCodeFile, err := os.Create(*srcFolder + "/rpcs.go")
	if err != nil {
		panic(err)
	}
	defer rpcsCodeFile.Close()
	writer := bufio.NewWriter(rpcsCodeFile)
	codeGenForEnums, err := template.New("codeGenForEnums").Funcs(funcMap).Parse(authzTemplateRPCs)
	if err == nil {
		codeGenForEnums.Execute(writer, rpcMap)
	} else {
		panic(fmt.Sprintf("Code generation for RPC enums failed: %v", err))
	}
	writer.Flush()

	if *genExecFunc {
		rpcExecCodeFile, err := os.Create(*srcFolder + "/rpcexec.go")
		if err != nil {
			panic(err)
		}
		defer rpcExecCodeFile.Close()
		writer := bufio.NewWriter(rpcExecCodeFile)
		codeGenForExecFunction, err := template.New("codeGenForExecFunction").Funcs(funcMap).Parse(authzTemplateRPCExec)
		if err == nil {
			codeGenForExecFunction.Execute(writer, rpcMap)
		} else {
			panic(fmt.Sprintf("Code generation for RPC exec functions failed: %v", err))
		}
		writer.Flush()
	}

}

var (
	funcMap = template.FuncMap{
		"pkgName": func() string {
			return *pkgName
		},
		"varName": func(service, name string) string {
			varName := ""
			if service != "*" {
				parts := strings.Split(service, ".")
				prevPart := ""
				for _, part := range parts {
					if !strings.EqualFold(part, prevPart) {
						varName += part
						prevPart = part
					}
				}
			}
			if name != "*" {
				varName += name
			} else {
				varName += "ALL"
			}
			return varName
		},
		"funcName": func(service, name string) string {
			if !*genExecFunc {
				return "nil"
			}
			caser := cases.Title(language.English)
			funcName := ""
			if service != "*" {
				parts := strings.Split(service, ".")
				if len(parts) >= 1 {
					funcName += caser.String(parts[0])
					if len(parts) > 2 {
						funcName += caser.String(parts[len(parts)-1])
					}
				}
			}
			if name != "*" {
				funcName += name
			} else {
				funcName += "AllRPC"
			}
			return funcName
		},
		"enumName": func(service, name string) string {
			varName := ""
			if service != "*" {
				parts := strings.Split(service, ".")
				prevPart := ""
				for _, part := range parts {
					if !strings.EqualFold(part, prevPart) {
						varName += part + "_"
						prevPart = part
					}
				}
			}
			if name != "*" {
				varName += name
			} else {
				varName += "ALL"
			}
			return strings.ToTitle(varName)
		},
	}

	authzTemplateRPCExec = `package {{pkgName}}

import (
	"context"

	"github.com/openconfig/ondatra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


{{- range .}}
// function {{funcName .Service .Name}} implements a sample request for service {{.Path}} to validate if authz works as expected.
func {{ funcName .Service .Name}}(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any)  error  {
	return status.Errorf(codes.Unimplemented, "exec function for RPC {{.Path}} is not implemented")
}
{{ end }}
`
	authzTemplateRPCs = `
// This following provide a list of all RPCs related to FP testing.
// The below code is generated using ../gen/generate.go. Please do not modify.
package {{pkgName}}
	type rpcs struct{
	{{- range .}}
		{{ funcName .Service .Name}}   *RPC
	{{- end }}
	}


	var (
		// ALL defines all FP related RPCs

		{{- range .}}
		{{ varName .Service .Name}} = &RPC{
			Name: "{{.Name}}",
			Service: "{{.Service}}",
			FQN: "{{.FQN}}",
			Path: "{{.Path}}",
			Exec: {{ funcName .Service .Name}},
		}
		{{- end }}

		// RPCs is a list of all FP related RPCs
		RPCs=rpcs{
		{{- range .}}
			{{ funcName .Service .Name}} : {{ varName .Service .Name}},
		{{- end }}
		}

		// RPCMAP is a helper that  maps path to RPCs data that may be needed in tests.
		RPCMAP=map[string]*RPC{
			{{- range .}}
				"{{.Path}}" : {{ varName .Service .Name}},
			{{- end }}
		}
	)
`
)
