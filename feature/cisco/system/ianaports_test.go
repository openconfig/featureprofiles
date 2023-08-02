package basetest

import (
	"context"
	"strconv"
	"testing"
	"time"

	"math/rand"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
)

func TestIanaPorts(t *testing.T) {
	//t.Skip()
	dut := ondatra.DUT(t, "dut")
	// var listenAdd string
	// This Sub-Test will Restart The EMSD Process and Get the

	t.Run("Process restart EMSD and get updated listen-address", func(t *testing.T) {
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
		// 	path := gnmi.OC().System().GrpcServer("DEFAULT").ListenAddresses()
		// 	gnmi.Update(t, dut, path.Config(), []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)})
		// 	gotbefore := gnmi.Get(t, dut, path.State())[0]
		// 	if gotbefore != []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}[0] {
		// 		t.Logf("Listen Address not returned as expected got : %v , want %v", gotbefore, listenAdd)
		// 	}
		// Reload router
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
		// 	gotafter := gnmi.Get(t, dut, path.State())[0]
		// 	if gotafter != []oc.System_GrpcServer_ListenAddresses_Union{oc.UnionString(listenAdd)}[0] {
		// 		t.Logf("Listen Address not returned as expected got : %v , want %v", gotafter, listenAdd)
		//	}
	})

	t.Run("Assign a GNMI / GRIBI / P4RT Default Ports", func(t *testing.T) {
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc gnmi port 9339 \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc gnmi port 9340 \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc gnmi port 9559 \n commit \n", 10*time.Second)

		// Verifications
		config.TextWithGNMI(context.Background(), t, dut, "vty-pool default 0 99 line-template default")
		config.TextWithSSH(context.Background(), t, dut, "bash ip netns exec 'vrf-mgmt' netstat -anp", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "show lpts bidings", 10*time.Second)

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
		path := gnmi.OC().System().GrpcServer("DEFAULT").Port()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), 57777)

	})

	t.Run("GRPC Server Port Replace Test", func(t *testing.T) {
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

	//set non-default name
	config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc name TEST\n commit \n", 10*time.Second)
	defer config.TextWithSSH(context.Background(), t, dut, "configure \n  no grpc name TEST\n commit \n", 10*time.Second)
	t.Run("GRPC Name Update Test", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("TEST").Name()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), "TEST")

	})

	t.Run("GRPC Name Replace Test", func(t *testing.T) {
		path := gnmi.OC().System().GrpcServer("TEST").Name()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Replace(t, dut, path.Config(), "TEST")

	})

	t.Run("Assign a Non-Default GNMI / GRIBI / P4RT Default Ports", func(t *testing.T) {
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
		// config.TextWithGNMI(context.Background(), t, dut, "vty-pool default 0 99 line-template default")
		config.TextWithSSH(context.Background(), t, dut, "bash ip netns exec 'vrf-mgmt' netstat -anp", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "show lpts bidings", 10*time.Second)
	})

	t.Run("Rollback to IANA Default Ports", func(t *testing.T) {
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc \n no port \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc \n gnmi \n no port \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc \n gribi \n no port \n commit \n", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "configure \n  grpc \n p4rt \n no port \n commit \n", 10*time.Second)
		// Verifications
		config.TextWithGNMI(context.Background(), t, dut, "vty-pool default 0 99 line-template default")
		config.TextWithSSH(context.Background(), t, dut, "bash ip netns exec 'vrf-mgmt' netstat -anp", 10*time.Second)
		config.TextWithSSH(context.Background(), t, dut, "show lpts bidings", 10*time.Second)
	})
}
