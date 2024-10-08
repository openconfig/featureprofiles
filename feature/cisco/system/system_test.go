package basetest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	device1  = "dut"
	observer = fptest.NewObserver("System").AddCsvRecorder("ocreport").
			AddCsvRecorder("System")
	systemContainers = []system{
		{
			hostname: ygot.String("tempHost1"),
		},
	}
)

type system struct {
	hostname *string
}

// func TestMain(m *testing.M) {
// 	fptest.RunTests(m)
// }

func testSystemContainerUpdate(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	path := gnmi.OC().System()
	for _, system := range systemContainers {
		container := &oc.System{}
		container.Hostname = system.hostname
		gnmi.Update(t, dut, path.Config(), container)
	}
	defer observer.RecordYgot(t, "UPDATE", path)
}

func testSysGrpcState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Subscribe /system/grpc-servers/grpc-server/state/port", func(t *testing.T) {
		portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
		// we are setting port 57777 via base config file of DUT
		if portNum != uint16(57777) {
			t.Errorf("wrong port: %d", portNum)
		}
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server/state/name", func(t *testing.T) {
		grpcName := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Name().State())
		if grpcName != "DEFAULT" {
			t.Errorf("wrong grpc name. got: %s, want: %s", grpcName, "DEFAULT")
		}
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server/state/enable", func(t *testing.T) {
		grpcEn := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Enable().State())
		if grpcEn != true {
			t.Errorf("wrong grpc enable value. got: %t, want: %t", grpcEn, true)
		}
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server/state/transport-security", func(t *testing.T) {
		grpcTs := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").TransportSecurity().State())
		// true or false depending on tls used in binding
		// FIXME: primitive logic - boolean is always either true || false. need to check if TLS is enabled from binding file and then verify that via this value
		if grpcTs == false || grpcTs == true {
			t.Logf("Got the expected grpc transport security")
		} else {
			t.Errorf("Unexpected value for transport security: %v", grpcTs)
		}
	})
	t.Run("Subscribe /system/", func(t *testing.T) {
		v := gnmi.Lookup(t, dut, gnmi.OC().System().State())
		if sysData, pres := v.Val(); !pres {
			t.Fatalf("Got nil system state data")
		} else {
			grpcPort := sysData.GetGrpcServer("DEFAULT").GetPort()
			grpcName := sysData.GetGrpcServer("DEFAULT").GetName()
			grpcEn := sysData.GetGrpcServer("DEFAULT").GetEnable()
			grpcTs := sysData.GetGrpcServer("DEFAULT").GetTransportSecurity()
			sysGrpcVerify(t, grpcPort, grpcName, grpcTs, grpcEn)
		}
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server['DEFAULT']", func(t *testing.T) {
		v := gnmi.Lookup(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").State())
		if sysGrpc, ok := v.Val(); !ok {
			t.Errorf("Got nil system grpc server data")
		} else {
			grpcPort := sysGrpc.GetPort()
			grpcName := sysGrpc.GetName()
			grpcEn := sysGrpc.GetEnable()
			grpcTs := sysGrpc.GetTransportSecurity()
			sysGrpcVerify(t, grpcPort, grpcName, grpcTs, grpcEn)
		}
	})
	t.Run("Subscribe /system/grpc-servers", func(t *testing.T) {
		v := gnmi.LookupAll(t, dut, gnmi.OC().System().GrpcServerAny().State())
		if sysGrpcCont, pres := v[0].Val(); !pres {
			t.Fatalf("Got nil system grpc server data")
		} else {
			grpcPort := sysGrpcCont.GetPort()
			grpcName := sysGrpcCont.GetName()
			grpcEn := sysGrpcCont.GetEnable()
			grpcTs := sysGrpcCont.GetTransportSecurity()
			sysGrpcVerify(t, grpcPort, grpcName, grpcTs, grpcEn)
		}
	})
}

func testSysGrpcConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// configure "DEFAULT" as grpc server name
	//config.TextWithGNMI(context.Background(), t, dut, "vty-pool default 0 99 line-template default")
	//config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc name DEFAULT\n commit \n", 10*time.Second)
	//defer config.TextWithSSH(context.Background(), t, dut, "configure \n  no grpc name DEFAULT\n commit \n", 10*time.Second)

	t.Run("Update //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), "DEFAULT")
	})
	t.Run("Replace //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Name()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), "DEFAULT")
	})
	t.Run("Update //system/grpc-servers/grpc-server/config/port", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Port()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), 57777)
	})
	t.Run("Replace //system/grpc-servers/grpc-server/config/port", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Port()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), 57777)
	})
	t.Run("Update //system/grpc-servers/grpc-server/config/enable", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Enable()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), true)

	})
	t.Run("Replace //system/grpc-servers/grpc-server/config/enable", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Enable()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), true)

	})
	t.Run("Update //system/grpc-servers/grpc-server/config/transport-security", func(t *testing.T) {
		t.Skip()
		path := gnmi.OC().System().GrpcServer("DEFAULT").TransportSecurity()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), false)

	})
	t.Run("Replace //system/grpc-servers/grpc-server/config/transport-security", func(t *testing.T) {
		t.Skip()
		path := gnmi.OC().System().GrpcServer("DEFAULT").TransportSecurity()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), false)

	})

}

func testSysNonDefaultGrpcConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// set non-default name for grpc server
	t.Run("Update //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("TEST").Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), "TEST")
	})
	t.Run("Replace //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("TEST").Name()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), "TEST")
	})
}

func testGrpcListenAddress(t *testing.T) {
	activeRp := 0
	re := regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}
	dut := ondatra.DUT(t, "dut")
	var listenAdd string
	virIP := config.CMDViaGNMI(context.Background(), t, dut, "sh run ipv4 virtual address")
	val := re.FindString(virIP)
	if strings.Contains(b.Duts[0].Ssh.Target, "::") {
		listenAdd = gnmi.GetAll(t, dut, gnmi.OC().Interface("Bundle-Ether120").Subinterface(0).Ipv6().AddressAny().State())[0].GetIp()
	} else if val != "" {
		listenAdd = strings.TrimSuffix(val, "/16")
	} else {

		resp := config.CMDViaGNMI(context.Background(), t, dut, "show redundancy")
		t.Logf(resp)

		if strings.Contains(resp, "Redundancy information for node 0/RP1/CPU0") {
			activeRp = 1
		}
		mgmtIP := config.CMDViaGNMI(context.Background(), t, dut, fmt.Sprintf("sh ip int brief mgmtEth 0/RP%v/CPU0/0", activeRp))
		listenAdd = re.FindString(mgmtIP)
	}

	defer config.TextWithGNMI(context.Background(), t, dut, "no grpc listen-address")
	cases := []struct {
		addresses []oc.System_GrpcServer_ListenAddresses_Union
		want      string
	}{

		{
			addresses: []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)},
			want:      listenAdd,
		},
		{
			addresses: []oc.System_GrpcServer_ListenAddresses_Union{oc.E_GrpcServer_ListenAddresses(oc.GrpcServer_ListenAddresses_ANY)},
			want:      "ANY",
		},
	}
	for _, tc := range cases {
		serveranyobj := &oc.System_GrpcServer{
			Name:            ygot.String("DEFAULT"),
			ListenAddresses: tc.addresses,
		}
		sysobj := &oc.System{
			GrpcServer: map[string]*oc.System_GrpcServer{"DEFAULT": serveranyobj},
		}

		t.Run("Update listen address on System container level", func(t *testing.T) {
			path := gnmi.OC().System().Config()
			defer observer.RecordYgot(t, "UPDATE", path)
			gnmi.Update(t, dut, path, sysobj)

		})
		t.Run("Subscribe listen address on System Config container level", func(t *testing.T) {
			path := gnmi.OC().System()
			defer observer.RecordYgot(t, "SUBSCRIBE", path)
			systemGet, pres := gnmi.LookupConfig(t, dut, path.Config()).Val()
			if !pres {
				t.Errorf("unable to fetch config")
			}
			got := systemGet.GrpcServer["DEFAULT"].GetListenAddresses()[0]
			if got != serveranyobj.ListenAddresses[0] {
				t.Logf("Listen Address not returned as expected got : %v , want %v", got, tc.want)
			}
		})
		t.Run("Subscribe listen address on System State container level", func(t *testing.T) {
			path := gnmi.OC().System()
			defer observer.RecordYgot(t, "SUBSCRIBE", path)
			systemGet := gnmi.Get(t, dut, path.State())

			listenAddresses := systemGet.GrpcServer["DEFAULT"].GetListenAddresses()
			if len(listenAddresses) == 0 {
				t.Fatalf("No listen addresses found for DEFAULT GrpcServer")
			}

			got := listenAddresses[0]
			if got != serveranyobj.ListenAddresses[0] {
				t.Logf("Listen Address not returned as expected got : %v , want %v", got, tc.want)
			}

		})
		t.Run("Update listen address on GrpcServer container level", func(t *testing.T) {
			path := gnmi.OC().System().GrpcServer("DEFAULT").Config()
			defer observer.RecordYgot(t, "UPDATE", path)
			gnmi.Update(t, dut, path, serveranyobj)

		})
		t.Run("Subscribe listen address on GrpcServer Config container level", func(t *testing.T) {
			path := gnmi.OC().System().GrpcServer("DEFAULT")
			defer observer.RecordYgot(t, "SUBSCRIBE", path)
			systemGet, pres := gnmi.LookupConfig(t, dut, path.Config()).Val()
			if !pres {
				t.Errorf("unable to fetch config")
			}
			got := systemGet.GetListenAddresses()[0]
			if got != serveranyobj.ListenAddresses[0] {
				t.Logf("Listen Address not returned as expected got : %v , want %v", got, tc.want)
			}
		})
		t.Run("Subscribe listen address on GrpcServer State container level", func(t *testing.T) {
			path := gnmi.OC().System().GrpcServer("DEFAULT")
			defer observer.RecordYgot(t, "SUBSCRIBE", path)
			systemGet := gnmi.Get(t, dut, path.State())

			listenAddresses := systemGet.GetListenAddresses()
			if len(listenAddresses) == 0 {
				t.Fatalf("No listen addresses found for DEFAULT GrpcServer")
			}

			got := listenAddresses[0]
			if got != serveranyobj.ListenAddresses[0] {
				t.Logf("Listen Address not returned as expected got : %v , want %v", got, tc.want)
			}
		})

		t.Run("Update listen address on listen-address leaf level", func(t *testing.T) {
			path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses().Config()
			defer observer.RecordYgot(t, "UPDATE", path)
			gnmi.Update(t, dut, path, tc.addresses)

		})

		t.Run("Subscribe listen address on listen-address config leaf level", func(t *testing.T) {
			path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses()
			defer observer.RecordYgot(t, "SUBSCRIBE", path)
			if tc.want == "ANY" {
				got, pres := gnmi.LookupConfig(t, dut, path.Config()).Val()
				if !pres {
					t.Fatalf("unable to fetch config")
				}
				if got[0] != serveranyobj.ListenAddresses[0] {
					t.Logf("Listen Address not returned as expected. got : %v , want %v", got, tc.want)
				}
			} else {
				got, pres := gnmi.LookupConfig(t, dut, path.Config()).Val()
				if !pres {
					t.Fatalf("unable to fetch config")
				}
				if got[0] != serveranyobj.ListenAddresses[0] {
					t.Logf("Listen Address not returned as expected. got : %v , want %v", got, tc.want)
				}
			}
		})
		t.Run("Subscribe listen address on listen-address state leaf level", func(t *testing.T) {
			path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses()
			defer observer.RecordYgot(t, "SUBSCRIBE", path)
			listenAddresses := gnmi.Get(t, dut, path.State())
			if len(listenAddresses) == 0 {
				t.Fatalf("No listen addresses found for DEFAULT GrpcServer")
			}
			got := listenAddresses[0]
			if got != serveranyobj.ListenAddresses[0] {
				t.Logf("Listen Address not returned as expected got : %v , want %v", got, tc.want)
			}
		})

	}

	t.Run("Modify leaf-list", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses()
		address1 := []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}
		defer observer.RecordYgot(t, "SUBSCRIBE", path)
		gnmi.Update(t, dut, path.Config(), address1)
		got1 := gnmi.Get(t, dut, path.State())[0]
		if got1 != address1[0] {
			t.Logf("Listen Address not returned as expected got : %v , want %v", got1, listenAdd)
		}
		//Update second address
		gnmi.Replace(t, dut, path.Config(), append(address1, oc.UnionString("1.1.1.1")))
		got2 := gnmi.Get(t, dut, path.State())
		if len(got2) != 2 {
			t.Errorf("The Second listen address did not get appended")
		}
		//Remove second address
		gnmi.Delete(t, dut, path.Config())
		got3 := gnmi.Get(t, dut, path.State())
		if got3[0] != oc.E_GrpcServer_ListenAddresses(oc.GrpcServer_ListenAddresses_ANY) {
			t.Errorf("Delete of listen address was not successfull")
		}

	})

	t.Run("Process restart emsd and get updated listen-address", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses()
		gnmi.Update(t, dut, path.Config(), []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)})
		got1 := gnmi.Get(t, dut, path.State())[0]
		if got1 != []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}[0] {
			t.Logf("Listen Address not returned as expected got : %v , want %v", got1, listenAdd)
		}
		//Process retstart
		config.CMDViaGNMI(context.Background(), t, dut, "process restart emsd")
		got3 := gnmi.Get(t, dut, path.State())
		if got3[0] != oc.UnionString(listenAdd) {
			t.Errorf("Delete of listen address was not successfull")
		}
	})

	t.Run("Reload router and check grpc before and after ", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses()
		gnmi.Update(t, dut, path.Config(), []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)})
		gotbefore := gnmi.Get(t, dut, path.State())[0]
		if gotbefore != []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}[0] {
			t.Logf("Listen Address not returned as expected got : %v , want %v", gotbefore, listenAdd)
		}
		//Reload router
		gnoiReboot(t, dut)
		resp := config.CMDViaGNMI(context.Background(), t, dut, "show redundancy")
		t.Logf(resp)
		activeRpAfterReboot := 0
		if strings.Contains(resp, "Redundancy information for node 0/RP1/CPU0") {
			activeRpAfterReboot = 1
		}
		t.Logf("RP %v came up as Active after reboot", activeRpAfterReboot)
		if activeRp != activeRpAfterReboot {
			// this is required if RP mgmt address is used, ( instead of virtual ip )
			t.Logf("switch RP to make RP %v as Active after reboot", activeRp)
			resp := config.CMDViaGNMI(context.Background(), t, dut, "redundancy switchover force")
			t.Logf(resp)
		}
		gotafter := gnmi.Get(t, dut, path.State())[0]
		if gotafter != []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}[0] {
			t.Logf("Listen Address not returned as expected got : %v , want %v", gotafter, listenAdd)
		}
	})
}

func sysGrpcVerify(t *testing.T, grpcPort uint16, grpcName string, grpcTs bool, grpcEn bool) {
	t.Helper()
	if grpcPort != uint16(57777) {
		t.Errorf("wrong grpc port value. got: %v, want: %v", grpcPort, uint16(57777))
	}
	if grpcName != "DEFAULT" {
		t.Errorf("wrong grpc name. got: %v, want: %v", grpcName, "DEFAULT")
	}
	if grpcEn != true {
		t.Errorf("wrong grpc enabled value. got: %v, want: %v", grpcEn, true)
	}
	if grpcTs == false || grpcTs == true {
		// FIXME: primitive logic. see similar note above
		t.Logf("Got the expected grpc Transport-Security")
	} else {
		t.Errorf("Unexpected value for Transport-Security: %v", grpcTs)
	}
}

func gnoiReboot(t *testing.T, dut *ondatra.DUTDevice) {
	//Reload router
	gnoiClient := dut.RawAPIs().GNOI(t)
	_, err := gnoiClient.System().Reboot(context.Background(), &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Reboot failed %v", err)
	}
	startReboot := time.Now()
	const maxRebootTime = 30
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
	// same variable name, gnoiClient, is used for the gNOI connection during the second reboot.
	// This causes the cache to reuse the old connection for the second reboot, which is no longer active due to a timeout from the previous reboot.
	// The retry mechanism clears the previous cache connection and re-establishes new connection.
	for {
		gnoiClient := dut.RawAPIs().GNOI(t)
		ctx := context.Background()
		response, err := gnoiClient.System().Time(ctx, &spb.TimeRequest{})

		// Log the error if it occurs
		if err != nil {
			t.Logf("Error fetching device time: %v", err)
		}

		// Check if the error code indicates that the service is unavailable
		if status.Code(err) == codes.Unavailable {
			// If the service is unavailable, wait for 30 seconds before retrying
			t.Logf("Service unavailable, retrying in 30 seconds...")
			time.Sleep(30 * time.Second)
		} else {
			// If the device time is fetched successfully, log the success message
			t.Logf("Device Time fetched successfully: %v", response)
			break
		}
		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device gnoi ready time: %.2f minutes", time.Since(startReboot).Minutes())
}
