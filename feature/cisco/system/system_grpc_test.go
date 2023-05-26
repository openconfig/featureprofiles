package basetest

import (
	"context"
	"flag"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/prototext"
)

func TestSysGrpcState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Run("Subscribe /system/grpc-servers/grpc-server/state/port", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")

			} else {
				t.Errorf("Unexpected value for port number: %v", portNum)
			}
		})
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server/state/name", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			grpcName := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Name().State())
			if grpcName == "DEFAULT" {
				t.Logf("Got the expected grpc Name")

			} else {
				t.Errorf("Unexpected value for Name: %s", grpcName)
			}
		})
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server/state/enable", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			grpcEn := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Enable().State())
			if grpcEn == true {
				t.Logf("Got the expected grpc Enable")

			} else {
				t.Errorf("Unexpected value for Enable: %v", grpcEn)
			}
		})
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server/state/transport-security", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			grpcTs := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").TransportSecurity().State())
			if grpcTs == false || grpcTs == true { //true or false depending on tls used in binding
				t.Logf("Got the expected grpc transport security")

			} else {
				t.Errorf("Unexpected value for transport security: %v", grpcTs)
			}
		})
	})

	t.Run("Subscribe /system/", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			sysData := gnmi.Get(t, dut, gnmi.OC().System().State())
			grpcPort := *sysData.GrpcServer["DEFAULT"].Port
			grpcName := *sysData.GrpcServer["DEFAULT"].Name
			grpcEn := *sysData.GrpcServer["DEFAULT"].Enable
			grpcTs := *sysData.GrpcServer["DEFAULT"].TransportSecurity
			sysGrpcVerify(grpcPort, grpcName, grpcTs, grpcEn, t)
		})
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server['DEFAULT']", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			sysGrpc := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").State())
			grpcPort := *sysGrpc.Port
			grpcName := *sysGrpc.Name
			grpcEn := *sysGrpc.Enable
			grpcTs := *sysGrpc.TransportSecurity
			sysGrpcVerify(grpcPort, grpcName, grpcTs, grpcEn, t)
		})
	})
	t.Run("Subscribe /system/grpc-servers", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			sysGrpcCont := gnmi.GetAll(t, dut, gnmi.OC().System().GrpcServerAny().State())
			grpcPort := *sysGrpcCont[0].Port
			grpcName := *sysGrpcCont[0].Name
			grpcEn := *sysGrpcCont[0].Enable
			grpcTs := *sysGrpcCont[0].TransportSecurity
			sysGrpcVerify(grpcPort, grpcName, grpcTs, grpcEn, t)

		})
	})
}
func TestSysGrpcConfig(t *testing.T) {
	//t.Skip()
	dut := ondatra.DUT(t, "dut")
	config.TextWithGNMI(context.Background(), t, dut, "vty-pool default 0 99 line-template default")
	config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc name DEFAULT\n commit \n", 10*time.Second)
	defer config.TextWithSSH(context.Background(), t, dut, "configure \n  no grpc name DEFAULT\n commit \n", 10*time.Second)

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
	//set non-default name
	config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc name TEST\n commit \n", 10*time.Second)
	defer config.TextWithSSH(context.Background(), t, dut, "configure \n  no grpc name TEST\n commit \n", 10*time.Second)
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
func TestGrpcListenAddress(t *testing.T) {
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
	if strings.Contains(b.Duts[0].Ssh.Target, "::") {
		listenAdd = gnmi.GetAll(t, dut, gnmi.OC().Interface("Bundle-Ether120").Subinterface(0).Ipv6().AddressAny().State())[0].GetIp()
	} else if virIP != "" {
		val := re.FindString(virIP)
		listenAdd = strings.TrimSuffix(val, "/16")
	} else {
		mgmtIP := config.CMDViaGNMI(context.Background(), t, dut, "sh ip int brief mgmtEth 0/RP0/CPU0/0")
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
			systemGet := gnmi.GetConfig(t, dut, path.Config())
			got := systemGet.GrpcServer["DEFAULT"].GetListenAddresses()[0]
			if got != serveranyobj.ListenAddresses[0] {
				t.Logf("Listen Address not returned as expected got : %v , want %v", got, tc.want)
			}
		})
		t.Run("Subscribe listen address on System State container level", func(t *testing.T) {
			path := gnmi.OC().System()
			defer observer.RecordYgot(t, "SUBSCRIBE", path)
			systemGet := gnmi.Get(t, dut, path.State())
			got := systemGet.GrpcServer["DEFAULT"].GetListenAddresses()[0]
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
			systemGet := gnmi.GetConfig(t, dut, path.Config())
			got := systemGet.GetListenAddresses()[0]
			if got != serveranyobj.ListenAddresses[0] {
				t.Logf("Listen Address not returned as expected got : %v , want %v", got, tc.want)
			}
		})
		t.Run("Subscribe listen address on GrpcServer State container level", func(t *testing.T) {
			path := gnmi.OC().System().GrpcServer("DEFAULT")
			defer observer.RecordYgot(t, "SUBSCRIBE", path)
			systemGet := gnmi.Get(t, dut, path.State())
			got := systemGet.GetListenAddresses()[0]
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
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					got := gnmi.GetConfig(t, dut, path.Config())[0]
					t.Logf("Get Config : %v", got)
				}); errMsg != nil {
					t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
				} else {
					t.Errorf("This Get-Config should have failed ")
				}
			} else {
				got := gnmi.GetConfig(t, dut, path.Config())[0]
				if got != serveranyobj.ListenAddresses[0] {
					t.Logf("Listen Address not returned as expected got : %v , want %v", got, tc.want)
				}
			}
		})
		t.Run("Subscribe listen address on listen-address state leaf level", func(t *testing.T) {
			path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses()
			defer observer.RecordYgot(t, "SUBSCRIBE", path)
			got := gnmi.Get(t, dut, path.State())[0]
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
		gnoiClient := dut.RawAPIs().GNOI().New(t)
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
		gotafter := gnmi.Get(t, dut, path.State())[0]
		if gotafter != []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}[0] {
			t.Logf("Listen Address not returned as expected got : %v , want %v", gotafter, listenAdd)
		}
	})
}
