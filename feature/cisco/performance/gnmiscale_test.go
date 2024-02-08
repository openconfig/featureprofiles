package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"

	//"github.com/openconfig/ondatra/gnmi"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// var (
// 	grpcStub *grpc.ClientConn
// )

var (
	addr = flag.String("addr", "10.85.232.104:15473", "address of the gRIBI server in the format hostname:port")
	//addr       = flag.String("addr", "10.85.84.159:57900", "address of the gRIBI server in the format hostname:port")
	insec      = flag.Bool("insecure", true, "dial insecure gRPC (no TLS)")
	skipVerify = flag.Bool("skip_verify", true, "allow self-signed TLS certificate; not needed for -insecure")
	username   = flag.String("username", "cisco", "username to be sent as gRPC metadata")
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

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// func TestGNMIUpdateScale(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	beforeTime := time.Now()
// 	for i := 0; i <= 10; i++ {
// 		gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test"+strconv.Itoa(i))
// 	}
// 	t.Logf("Time to do 100 gnmi uodate is %s", time.Since(beforeTime).String())
// 	if int(time.Since(beforeTime).Seconds()) >= 180 {
// 		t.Fatalf("GNMI Scale Took too long")
// 	}
// }

// func TestGNMIBigSetRequest(t *testing.T) {
// 	// Perform a gNMI Set Request with 20 MB of Data
// 	dut := ondatra.DUT(t, "dut")
// 	RestartEmsd(t, dut)
// 	ControlPlaneVerification(ygnmiCli)
// }

// beforeEach invoke getcpuusage and defer wait additional 60s 
func TestEmsdRestart(t *testing.T) {
	gnmiC := gpb.NewGNMIClient(grpcStub)
	ygnmiCli, err := ygnmi.NewClient(gnmiC)
	if err != nil {
		fmt.Printf("Could not connect to GNMI service: %v", err)
		os.Exit(2)
	}
	// get the actual value of isRunning
	isRunning := true

	// if client was successfully connected, does this imply isRunning?
	cpuChannel := CCpuVerify(ygnmiCli, &isRunning)

	//maybe move to different func
	go func(cpuChannel chan []*oc.System_Cpu) {
		for cpuData := range cpuChannel {
			fmt.Printf("CPU INFO:\n%s\n", PrettyPrint(cpuData))
			// pass each memUse to channel
		}
		
	}(cpuChannel)

	
	//dut := ondatra.DUT(t, "dut")
	//RestartEmsd(t, dut)
	//ControlPlaneVerification(ygnmiCli)
	//ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
	//b, err := gnmi.GetAll(ctx, t, dut, gnmi.OC().System().CpuAny().Total().State())
	// t.Log(b)
	// if err != nil {
	// 	fmt.Printf("Error /n")
	// }
	// if cancel != nil {
	// 	fmt.Printf("Error /n")
	// }
	//GetCpuInfoEvery60Seconds(t, &isRunning)
	//CpuVerify(gNMI, &isRunning)
	//MemoryVerify(t, ygnmiCli, &isRunning)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Log(ygnmiCli)
	t.Log(ctx)
	data, err := ygnmi.CollectAll(ctx, ygnmiCli, gnmi.OC().System().CpuAny().State()).Await()
	t.Log("hi there")
	t.Log(data)
	if cancel != nil {
		fmt.Printf("Error %v /n", err)
	}
	if err != nil {
		fmt.Printf("Error %v /n", err)
	}
	//time.Sleep(150 * time.Second)
	//isRunning = false
}

// func TestRouterReload(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	RestartEmsd(t, dut)
// 	ControlPlaneVerification(ygnmiCli)
// }

func MemoryVerify(t *testing.T, ygnmiCli *ygnmi.Client, isRunning *bool) {
	// oc leaves for memory do not work!! and cpu information require extra analysis, commenting this code for now
	go func() {
		for *isRunning {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			//defer cancel()
			data, err := ygnmi.CollectAll(ctx, ygnmiCli, gnmi.OC().System().CpuAny().State()).Await()
			t.Log("hi there")
			t.Log(data)
			if cancel != nil {
				fmt.Printf("Error %v /n", err)
			}
			if err != nil {
				fmt.Printf("Error %v /n", err)
			}
			for _, memUse := range data {
				usedMem, _ := memUse.Val()
				fmt.Printf("Cpu info at %v : %v\n", memUse.Timestamp, PrettyPrint(usedMem))
			}
		}
	}()
}

func GetCpuInfoEvery60Seconds(t *testing.T, isRunning *bool) {
	go func() {
		for *isRunning {
			dut := ondatra.DUT(t, "dut")
			b := gnmi.GetAll(t, dut, gnmi.OC().System().CpuAny().Total().State())
			t.Log("hello there")
			t.Log(b)
			time.Sleep(30 * time.Second) // Wait for 60 seconds before the next call
			// for _, memUse := range b {
			// 	usedMem, _ := memUse.()
			// 	fmt.Printf("Cpu info at %v : %v\n", memUse.Timestamp, PrettyPrint(usedMem))
			// }
		}
	}()
}
