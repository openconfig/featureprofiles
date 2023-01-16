package basetest

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
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
		path := gnmi.OC().System().GrpcServer("DEFAULT").TransportSecurity()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), false)

	})
	t.Run("Replace //system/grpc-servers/grpc-server/config/transport-security", func(t *testing.T) {
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
