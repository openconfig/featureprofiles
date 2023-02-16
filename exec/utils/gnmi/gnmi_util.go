// Package main implements a utility functions to load and unload oc config using gnmi from a router
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr = flag.String("addr", "10.85.84.159:47402", "address of the gRIBI server in the format hostname:port")
	//addr       = flag.String("addr", "10.85.84.159:57900", "address of the gRIBI server in the format hostname:port")
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

func filterContainer(s map[string]interface{}, containerName string) {
	for key, value := range s {
		if key == containerName {
			delete(s, key)
		} else if reflect.ValueOf(value).Kind() == reflect.Map {
			filterContainer(value.(map[string]interface{}), containerName)
		} else if reflect.ValueOf(value).Kind() == reflect.Slice {
			for _, value2 := range value.([]interface{}) {
				if reflect.ValueOf(value2).Kind() == reflect.Map {
					filterContainer(value2.(map[string]interface{}), containerName)
				} else {
					fmt.Printf("kind is %v\n", reflect.ValueOf(value2).Kind())
				}

			}
		}
	}
}

func resetLeaf(s map[string]interface{}, leafName string) {
	for key, value := range s {
		if key == leafName && reflect.ValueOf(value).Kind() == reflect.Ptr {
			s[key] = nil
		} else if reflect.ValueOf(value).Kind() == reflect.Map {
			filterContainer(value.(map[string]interface{}), leafName)
		} else if reflect.ValueOf(value).Kind() == reflect.Slice {
			for _, value2 := range value.([]interface{}) {
				if reflect.ValueOf(value2).Kind() == reflect.Map {
					filterContainer(value2.(map[string]interface{}), leafName)
				} else {
					fmt.Printf("kind is %v\n", reflect.ValueOf(value2).Kind())
				}
			}
		}
	}
}

// load oc from a file
func loadJSONOC(path string) *oc.Root {
	var ocRoot oc.Root
	jsonConfig, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("Cannot load base config: %v", err))
	}
	opts := []ytypes.UnmarshalOpt{
		&ytypes.PreferShadowPath{},
	}
	if err := oc.Unmarshal(jsonConfig, &ocRoot, opts...); err != nil {
		panic(fmt.Sprintf("Cannot unmarshal base config: %v", err))
	}
	return &ocRoot
}

// load oc from a file
func saveOCJSON(val *oc.Root, path string, containerFilter []string, leafFilter []string) {
	var jsonConfig []byte
	marshalCFG := ygot.RFC7951JSONConfig{
		AppendModuleName:             true,
		PrependModuleNameIdentityref: true,
		PreferShadowPath:             true,
	}
	marshalCFG.AppendModuleName = true
	mapVal, err := ygot.ConstructIETFJSON(val, &marshalCFG)
	if err != nil {
		panic(fmt.Sprintf("Cannot Construct IETF JSON from oc struct : %v", err))
	}
	for _, filter := range containerFilter {
		filterContainer(mapVal, filter)
	}
	for _, filter := range leafFilter {
		resetLeaf(mapVal, filter)
	}
	jsonConfig, err = json.MarshalIndent(mapVal, "", "	")
	if err != nil {
		panic(fmt.Sprintf("Cannot marshal oc struct in to json : %v", err))
	}
	err = os.WriteFile(path, jsonConfig, 0644)
	if err != nil {
		panic(fmt.Sprintf("Cannot write to file %s : %v", path, err))
	}
}

func main() {
	gnmiC := gpb.NewGNMIClient(grpcStub)
	ygnmiCli, err := ygnmi.NewClient(gnmiC)
	if err != nil {
		fmt.Printf("Could not connect to GNMI service: %v", err)
		os.Exit(2)
	}

	/// ocRoot is the key to create all oc struct
	ocRoot := &oc.Root{}
	// read interface config save them in json file for edit and push
	interfaces, _ := ygnmi.GetAll(context.Background(), ygnmiCli, gnmi.OC().InterfaceAny().State())
	ocRoot.Interface = make(map[string]*oc.Interface)
	for _, intf := range interfaces {
		if intf.GetName() == "Null0" || strings.HasPrefix(intf.GetName(), "Loopback") ||
			strings.HasPrefix(intf.GetName(), "PTP") || strings.HasPrefix(intf.GetName(), "MgmtEth") { // skip nul
			continue
		}
		ocRoot.Interface[intf.GetName()] = intf
	}
	saveOCJSON(ocRoot, "device_interfaces.json", []string{"state", "openconfig-if-ethernet:ethernet"}, []string{"port-speed"})

	//// using go sturct  to update
	ocRoot = &oc.Root{}
	intfVal := ocRoot.GetOrCreateInterface("FourHundredGigE0/0/0/35")
	intfVal.Enabled = ygot.Bool(true)
	intfVal.Description = ygot.String("test interface")
	_, err = ygnmi.Update(context.Background(), ygnmiCli, gnmi.OC().Interface("FourHundredGigE0/0/0/35").Config(), intfVal)
	if err != nil {
		fmt.Printf("error updating interface FourHundredGigE0/0/0/35: %v \n", err)
	}

	// gnmi push  from json file
	ocRoot = loadJSONOC("device_interfaces.json")
	// push gnmi replaces one by one
	for _, intf := range ocRoot.Interface {
		_, err = ygnmi.Replace(context.Background(), ygnmiCli, gnmi.OC().Interface(intf.GetName()).Config(), intf)
		if err != nil {
			fmt.Printf("error replacing %s : %v \n", intf.GetName(), err)
		}
	}
	// push one update at root level for all interfaces
	_, err = ygnmi.Update(context.Background(), ygnmiCli, gnmi.OC().Config(), ocRoot)
	if err != nil {
		fmt.Printf("error updating oc root: %v \n", err)
	}

	// puhs one request with multiple replaces (batch request)
	batchRep := &ygnmi.SetBatch{}
	for _, intf := range ocRoot.Interface {
		ygnmi.BatchReplace(batchRep, gnmi.OC().Interface(intf.GetName()).Config(), intf)
	}
	_, err = batchRep.Set(context.Background(), ygnmiCli)
	if err != nil {
		fmt.Printf("error batch replace: %v \n", err)
	}
}
