package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/cisco/security/gnxi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/protobuf/reflect/protoreflect"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	accpb "github.com/openconfig/gnsi/accounting"
	authzpb "github.com/openconfig/gnsi/authz"
	certzpb "github.com/openconfig/gnsi/certz"
	credpb "github.com/openconfig/gnsi/credentialz"
	pathzpb "github.com/openconfig/gnsi/pathz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"

	bpb "github.com/openconfig/gnoi/bgp"
	cpb "github.com/openconfig/gnoi/cert"
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
		"gnsi.acc":                accpb.File_github_com_openconfig_gnsi_accounting_acct_proto.Services(),
		"gnoi.bgp":                bpb.File_bgp_bgp_proto.Services(),
		"gnoi.cert":               cpb.File_cert_cert_proto.Services(),
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
	rpcMap["ALL"] = &gnxi.RPC{
		Name:    "*",
		Service: "*",
		QFN:     "*",
		Path:    "*",
	}
	for serviceName, service := range services {
		if service.Len() == 0 {
			glog.Warningf("servie %s has no rpc methds\n", serviceName)
		}
		for i := 0; i < service.Len(); i++ {
			glog.Infof("Service %s RPCes are: \n", serviceName)
			for j := 0; j < service.Get(i).Methods().Len(); j++ {
				rpcName := fmt.Sprintf("%v", service.Get(i).Methods().Get(j).Name())
				enumName := fmt.Sprintf("%s_%s", strings.ToUpper(serviceName), strings.ToUpper(rpcName))
				rpcQFN := fmt.Sprintf("%v", service.Get(i).Methods().Get(j).FullName())
				path := ""
				lastPointIdx := strings.LastIndex(rpcQFN, ".")
				if lastPointIdx > 0 {
					path = "/" + rpcQFN[:lastPointIdx] + "/" + rpcQFN[lastPointIdx+1:]
				}
				serviceName = rpcQFN[:lastPointIdx]
				rpcMap[enumName] = &gnxi.RPC{
					Name:    rpcName,
					Service: serviceName,
					QFN:     rpcQFN,
					Path:    path,
				}
				glog.Infof("\t RPC is %v\n", rpcMap[enumName])
				glog.Infof("\t %s: %s \n", service.Get(i).Methods().Get(j).Name(), service.Get(i).Methods().Get(j).FullName())

			}
			// add service/* for each service that represenst all RPC for the service
			rpcMap[strings.ToUpper(serviceName)+"_ALL"] = &gnxi.RPC{
				Name:    "*",
				Service: serviceName,
				QFN:     serviceName + ".*",
				Path:    "/" + serviceName + "/*",
			}
		}

	}

	//
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
	//
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
			funcName := ""
			if service != "*" {
				parts := strings.Split(service, ".")
				prevPart := ""
				for _, part := range parts {
					if !strings.EqualFold(part, prevPart) {
						funcName += part
						prevPart = part
					}
				}
			}
			if name != "*" {
				funcName += name
			} else {
				funcName += "AllRPc"
			}
			caser := cases.Title(language.English)
			return "Exec" + caser.String(funcName)
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
			return strings.ToUpper(varName)
		},
	}

	authzTemplateRPCExec = `package {{pkgName}}

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)


{{- range .}}
// function {{funcName .Service .Name}} implements a sample request for service {{.Path}} to validate if authz works as expected.
func {{ funcName .Service .Name}}(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC {{.Path}} is not implemented")
}
{{ end }}
`
	authzTemplateRPCs = `package {{pkgName}}
	type rpcs struct{
	{{- range .}}
		{{ enumName .Service .Name}}   *RPC
	{{- end }}
	}


	var (
		{{- range .}}
		{{ varName .Service .Name}} = &RPC{
			Name: "{{.Name}}",
			Service: "{{.Service}}",
			QFN: "{{.QFN}}",
			Path: "{{.Path}}",
			ExecFunc: {{ funcName .Service .Name}},
		}
		{{- end }}

		RPCs=rpcs{
		{{- range .}}
			{{ enumName .Service .Name}} : {{ varName .Service .Name}},
		{{- end }}
		}
		RPCMAP=map[string]*RPC{
			{{- range .}}
				"{{.Path}}" : {{ varName .Service .Name}},
			{{- end }}
		}
	)
`
)
