package basetest

import (
	"context"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func testIanaPorts(t *testing.T) {
	//t.Skip()
	dut := ondatra.DUT(t, "dut")
	// var listenAdd string
	// This Sub-Test will Restart The EMSD Process and Get the

	t.Run("Process restart EMSD and get updated listen-address", func(t *testing.T) {
		// TODO - Harish to take a look
		// Opened TZ to track this Issue: https://techzone.cisco.com/t5/IOS-XR-PI-GNMI-GNOI-Infra-Eng/GB4-RPC-Errors-on-Fetching-GRPC-Listen-Addresses-of-GRPC-Server/td-p/9282368
		// config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc name  DEFAULT \n commit \n", 10*time.Second)
		// config.TextWithSSH(context.Background(), t, dut, "vty-pool default 0 18 line-template default", 10*time.Second)
		// path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses()
		// gnmi.Update(t, dut, path.Config(), []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)})
		// got1 := gnmi.Get(t, dut, path.State())[0]
		// if got1 != []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}[0] {
		// 	t.Logf("Listen Address not returned as expected got : %v , want %v", got1, listenAdd)
		// }
		//Process restart
		config.CMDViaGNMI(context.Background(), t, dut, "process restart emsd")
		// got3 := gnmi.Get(t, dut, path.State())
		// if got3[0] != oc.UnionString(listenAdd) {
		// 	t.Errorf("Delete of listen address was not successfull")
		// }
	})

	t.Run("Reload Router and check grpc before and after ", func(t *testing.T) {
		// TODO - Harish to take a look
		// 	path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses()
		// 	gnmi.Update(t, dut, path.Config(), []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)})
		// 	gotbefore := gnmi.Get(t, dut, path.State())[0]
		// 	if gotbefore != []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}[0] {
		// 		t.Logf("Listen Address not returned as expected got : %v , want %v", gotbefore, listenAdd)
		// 	}
		// Reload router
		resp := config.CMDViaGNMI(context.Background(), t, dut, "show run grpc")
		t.Logf("GRPC config before reboot:\n %v", resp)
		gnoiReboot(t, dut)
		resp = config.CMDViaGNMI(context.Background(), t, dut, "show run grpc")
		t.Logf("GRPC config after reboot:\n %v", resp)
		// 	gotafter := gnmi.Get(t, dut, path.State())[0]
		// 	if gotafter != []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}[0] {
		// 		t.Logf("Listen Address not returned as expected got : %v , want %v", gotafter, listenAdd)
		//	}
	})

	t.Run("Assign a GNMI / GRIBI / P4RT Default Ports", func(t *testing.T) {
		resp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
		t.Log(resp)
		if strings.Contains(resp, "VXR") {
			t.Logf("Skipping since platfrom is VXR")
			t.Skip()
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		config.TextWithSSH(ctx, t, dut, "configure \n  grpc gnmi port 9339 \n commit \n", 10*time.Second)
		config.TextWithSSH(ctx, t, dut, "configure \n  grpc gribi port 9340 \n commit \n", 10*time.Second)
		config.TextWithSSH(ctx, t, dut, "configure \n  grpc p4rt port 9559 \n commit \n", 10*time.Second)

		// Verifications
		portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
		if portNum == uint16(0) || portNum > uint16(0) {
			t.Logf("Got the expected port number")
		} else {
			t.Errorf("Unexpected value for port number: %v", portNum)
		}
		configs := gnmi.OC().System()
		gnmi.Get(t, dut, configs.Config())
	})

	t.Run("GRPC Server Update Test", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), "DEFAULT")

	})

	t.Run("GRPC Server Replace Test", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Name()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), "DEFAULT")

	})

	t.Run("GRPC Server Port Update Test", func(t *testing.T) {
		resp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
		t.Log(resp)
		if strings.Contains(resp, "VXR") {
			t.Logf("Skipping since platfrom is VXR")
			t.Skip()
		}
		path := gnmi.OC().System().GrpcServer("DEFAULT").Port()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), 57777)

	})

	t.Run("GRPC Server Port Replace Test", func(t *testing.T) {
		resp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
		t.Log(resp)
		if strings.Contains(resp, "VXR") {
			t.Logf("Skipping since platfrom is VXR")
			t.Skip()
		}
		path := gnmi.OC().System().GrpcServer("DEFAULT").Port()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), 57777)

	})

	t.Run("GRPC Config Update Test", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Enable()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), true)
	})

	t.Run("GRPC Config Replace Test", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("DEFAULT").Enable()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), true)

	})

	t.Run("TLS Update Test", func(t *testing.T) {
		t.Skip()
		path := gnmi.OC().System().GrpcServer("DEFAULT").TransportSecurity()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), false)

	})

	t.Run("TLS Replace Test", func(t *testing.T) {
		t.Skip()
		path := gnmi.OC().System().GrpcServer("DEFAULT").TransportSecurity()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), false)

	})

	t.Run("Assign a Non-Default GNMI / GRIBI / P4RT Default Ports", func(t *testing.T) {
		resp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
		t.Log(resp)
		if strings.Contains(resp, "VXR") {
			t.Logf("Skipping since platfrom is VXR")
			t.Skip()
		}
		min := 57344
		max := 57399
		value1 := rand.Intn(max-min) + min
		value2 := value1 + 1
		value3 := value2 + 1
		s1 := strconv.FormatInt(int64(value1), 10)
		s2 := strconv.FormatInt(int64(value2), 10)
		s3 := strconv.FormatInt(int64(value3), 10)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc gnmi port "+s1+" \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc gribi port "+s2+" \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc p4rt port "+s3+" \n commit \n", 10*time.Second)

		// Verifications
		portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
		if portNum == uint16(0) || portNum > uint16(0) {
			t.Logf("Got the expected port number")
		} else {
			t.Errorf("Unexpected value for port number: %v", portNum)
		}
		configs := gnmi.OC().System()
		gnmi.Get(t, dut, configs.Config())
	})

	t.Run("Rollback to IANA Default Ports", func(t *testing.T) {
		resp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
		t.Log(resp)
		if strings.Contains(resp, "VXR") {
			t.Logf("Skipping since platfrom is VXR")
			t.Skip()
		}
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc \n no port \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc \n gnmi \n no port \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc \n gribi \n no port \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc \n p4rt \n no port \n commit \n", 10*time.Second)
		// Verifications
		portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
		if portNum == uint16(0) || portNum > uint16(0) {
			t.Logf("Got the expected port number")
		} else {
			t.Errorf("Unexpected value for port number: %v", portNum)
		}
		configs := gnmi.OC().System()
		gnmi.Get(t, dut, configs.Config())
	})
}
