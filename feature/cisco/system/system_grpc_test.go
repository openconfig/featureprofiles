package basetest

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/ondatra"
)
func TestSysGrpcState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Run("Subscribe /system/grpc-servers/grpc-server/config/port", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			portNum := dut.Telemetry().System().GrpcServer("DEFAULT").Port().Get(t)
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")

			} else {
				t.Errorf("Unexpected value for port number: %v", portNum)
			}
		})
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			grpcName := dut.Telemetry().System().GrpcServer("DEFAULT").Name().Get(t)
			if grpcName == "DEFAULT" {
				t.Logf("Got the expected grpc Name")

			} else {
				t.Errorf("Unexpected value for Name: %s", grpcName)
			}
		})
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server/config/enable", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			grpcEn := dut.Telemetry().System().GrpcServer("DEFAULT").Enable().Get(t)
			if grpcEn == true {
				t.Logf("Got the expected grpc Enable")

			} else {
				t.Errorf("Unexpected value for Enable: %v", grpcEn)
			}
		})
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server/config/transport-security", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			grpcTs := dut.Telemetry().System().GrpcServer("DEFAULT").TransportSecurity().Get(t)
			if grpcTs == false {
				t.Logf("Got the expected grpc transport security")

			} else {
				t.Errorf("Unexpected value for transport security: %v", grpcTs)
			}
		})
	})

	t.Run("Subscribe /system/", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			sysData := dut.Telemetry().System().Get(t)
			grpcPort := *sysData.GrpcServer["DEFAULT"].Port
			grpcName := *sysData.GrpcServer["DEFAULT"].Name
			grpcEn := *sysData.GrpcServer["DEFAULT"].Enable
			grpcTs := *sysData.GrpcServer["DEFAULT"].TransportSecurity
			sysGrpcVerify(grpcPort, grpcName, grpcTs, grpcEn, t)
		})
	})
	t.Run("Subscribe /system/grpc-servers/grpc-server['DEFAULT']", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			sysGrpc := dut.Telemetry().System().GrpcServer("DEFAULT").Get(t)
			grpcPort := *sysGrpc.Port
			grpcName := *sysGrpc.Name
			grpcEn := *sysGrpc.Enable
			grpcTs := *sysGrpc.TransportSecurity
			sysGrpcVerify(grpcPort, grpcName, grpcTs, grpcEn, t)
		})
	})
	t.Run("Subscribe /system/grpc-servers", func(t *testing.T) {
		t.Run("Subscribe", func(t *testing.T) {
			sysGrpcCont := dut.Telemetry().System().GrpcServerAny().Get(t)
			grpcPort := *sysGrpcCont[0].Port
			grpcName := *sysGrpcCont[0].Name
			grpcEn := *sysGrpcCont[0].Enable
			grpcTs := *sysGrpcCont[0].TransportSecurity
			sysGrpcVerify(grpcPort, grpcName, grpcTs, grpcEn, t)

		})
	})
}
func TestSysGrpcConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Update //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("DEFAULT").Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, "DEFAULT")

	})

	t.Run("Replace //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("DEFAULT").Name()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, "DEFAULT")

	})
	t.Run("Get //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		configName := dut.Config().System().GrpcServer("DEFAULT").Name().Get(t)
		if configName == "DEFAULT" {
			t.Logf("Got the expected grpc Name")

		} else {
			t.Errorf("Unexpected value for Name: %s", configName)
		}

	})

	t.Run("Update //system/grpc-servers/grpc-server/config/port", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("DEFAULT").Port()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, 57777)

	})
	t.Run("Replace //system/grpc-servers/grpc-server/config/port", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("DEFAULT").Port()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, 57777)

	})
	t.Run("Get //system/grpc-servers/grpc-server/config/port", func(t *testing.T) {
		configPort := dut.Config().System().GrpcServer("DEFAULT").Port().Get(t)
		if configPort == uint16(0) || configPort > uint16(0) {
			t.Logf("Got the expected port number")

		} else {
			t.Errorf("Unexpected value for Port: %v", configPort)
		}

	})
	t.Run("Update //system/grpc-servers/grpc-server/config/enable", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("DEFAULT").Enable()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, true)

	})
	t.Run("Replace //system/grpc-servers/grpc-server/config/enable", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("DEFAULT").Enable()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, true)

	})
	t.Run("Get //system/grpc-servers/grpc-server/config/enable", func(t *testing.T) {
		configEn := dut.Config().System().GrpcServer("DEFAULT").Enable().Get(t)
		if configEn == true {
			t.Logf("Got the expected grpc Enable ")

		} else {
			t.Errorf("Unexpected value for Enable: %v", configEn)
		}

	})
	t.Run("Update //system/grpc-servers/grpc-server/config/transport-security", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("DEFAULT").TransportSecurity()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, false)

	})
	t.Run("Replace //system/grpc-servers/grpc-server/config/transport-security", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("DEFAULT").TransportSecurity()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, false)

	})
	t.Run("Get //system/grpc-servers/grpc-server/config/transport-security", func(t *testing.T) {
		configTs := dut.Config().System().GrpcServer("DEFAULT").TransportSecurity().Get(t)
		if configTs == true {
			t.Logf("Got the expected grpc Transport-Security ")

		} else {
			t.Errorf("Unexpected value for Transport-Security: %v", configTs)
		}

	})
	//set non-default name
	ctx := context.Background()
        config.CMDViaGNMI(ctx, t, dut, "grpc name TEST\n" )
	t.Run("Update //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("TEST").Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, "TEST")

	})

	t.Run("Replace //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		path := dut.Config().System().GrpcServer("TEST").Name()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Replace(t, "TEST")

	})
	t.Run("Get //system/grpc-servers/grpc-server/config/name", func(t *testing.T) {
		configName := dut.Config().System().GrpcServer("TEST").Name().Get(t)
		if configName != "TEST" {
			t.Errorf("Unexpected value for Name: %s", configName)

		}

	})
        defer config.CMDViaGNMI(ctx, t, dut, "no grpc name TEST\n")

}
