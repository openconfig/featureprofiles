// package main is a utility function to generate base config
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr       = flag.String("addr", "10.85.84.159:47402", "address of the gRIBI server in the format hostname:port")
	insec      = flag.Bool("insecure", false, "dial insecure gRPC (no TLS)")
	skipVerify = flag.Bool("skip_verify", true, "allow self-signed TLS certificate; not needed for -insecure")
	username   = flag.String("username", "cafyauto", "username to be sent as gRPC metadata")
	password   = flag.String("password", "cisco123", "password to be sent as gRPC metadata")
	grpcStub   *grpc.ClientConn
)

// flagCred implements credentials.PerRPCCredentials by populating the
// username and password metadata from flags.
type flagCred struct{}

// GetRequestMetadata is needed by credentials.PerRPCCredentials.
func (flagCred) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": *username,
		"password": *password,
	}, nil
}

// RequireTransportSecurity is needed by credentials.PerRPCCredentials.
func (flagCred) RequireTransportSecurity() bool {
	return false
}

func init() {
	flag.Parse()
	dialOpts := []grpc.DialOption{grpc.WithBlock()}
	if *insec {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if *skipVerify {
		tlsc := credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: *skipVerify,
		})
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(tlsc))
	}

	if *password != "" {
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(flagCred{}))
	}
	retryOpt := grpc_retry.WithPerRetryTimeout(60 * time.Second)
	dialOpts = append(dialOpts,
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpt)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpt)),
	)
	ctx := context.Background()
	var err error
	grpcStub, err = grpc.DialContext(ctx, *addr, dialOpts...)
	if err != nil {
		fmt.Printf("Could not dial gRPC: %v", err)
		os.Exit(2)
	}

}

func getBaseCLIConfig(ctx context.Context, gnmiC gpb.GNMIClient) string {
	getRequest := &gpb.GetRequest{
		Prefix: &gpb.Path{
			Origin: "cli",
		},
		Path: []*gpb.Path{
			{
				Elem: []*gpb.PathElem{{
					Name: "show running-config\n",
				}},
			},
		},
		Encoding: gpb.Encoding_ASCII,
	}
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Get(ctx, getRequest)
	if err != nil {
		fmt.Printf("Could not connect to GNMI service: %v", err)
		os.Exit(2)
	}
	return string(resp.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
}

func main() {
	gnmiC := gpb.NewGNMIClient(grpcStub)
	baseConfig := getBaseCLIConfig(context.Background(), gnmiC)
	baseConfigLines := strings.Split(baseConfig, "\n")
	annotatedBaseConfig := []string{}
	configuredInterfaces := map[string]bool{}
	for idx, line := range baseConfigLines {
		if strings.HasPrefix(line, "interface") {
			annotatedBaseConfig = append(annotatedBaseConfig, line)
			configuredInterfaces[strings.Split(line, " ")[1]] = true
			if strings.Contains(line, "MgmtEth") {
				continue
			}
			intfConfgidx := idx + 1
			descFound := false
			for {
				if baseConfigLines[intfConfgidx] == "!" {
					break
				}
				if strings.HasPrefix(baseConfigLines[intfConfgidx], " description") {
					descFound = true
					break
				}
				intfConfgidx++
			}
			if !descFound {
				annotatedBaseConfig = append(annotatedBaseConfig, " description not connected interface")
			}
		} else {
			annotatedBaseConfig = append(annotatedBaseConfig, line)
		}
	}
	ygnmiCli, err := ygnmi.NewClient(gnmiC)
	if err != nil {
		fmt.Printf("Could not connect to GNMI service: %v", err)
		os.Exit(2)
	}
	interfaces, _ := ygnmi.GetAll(context.Background(), ygnmiCli, gnmi.OC().InterfaceAny().State())
	addedBaseConfig := []string{}
	for _, intf := range interfaces {
		ok := configuredInterfaces[intf.GetName()]
		if !ok {
			addedBaseConfig = append(addedBaseConfig, fmt.Sprintf("interface %s", intf.GetName()))
			addedBaseConfig = append(addedBaseConfig, " description not connected interface")
			addedBaseConfig = append(addedBaseConfig, " shutdown")
			addedBaseConfig = append(addedBaseConfig, "!")
		}
	}
	fmt.Println("----------- Here is the FP base config -----------")
	for _, line := range annotatedBaseConfig {
		if line != "end" {
			fmt.Printf("\"%s\\n\"\n", line)
		} else {
			for _, addedLine := range addedBaseConfig {
				fmt.Printf("\"%s\\n\"\n", addedLine)
			}
			fmt.Printf("\"%s\\n\"\n", "end")
		}
	}
}
