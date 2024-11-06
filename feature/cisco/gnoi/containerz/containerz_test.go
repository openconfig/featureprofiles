package containerz_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/feature/cisco/gnoi/containerz/client"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

const (
	bonnetTarLocation = "/auto/tftp-idt-tools/pi_infra/b4_containerz/bonnet-g3.tar.gz"
	bonnetRunCmd      = `/bonnet --uid= --use_doh --use_tap_instead_of_tun --loas2_force_full_key_exchange 
	--noqbone_tunnel_health_check_stubby_probes --qbone_region_file=/config/qbone_region --svelte_retry_interval_ms=1000000000 
	--qbone_resolver_cc_algo= --qbone_bonnet_use_custom_healthz_handler --qbone_client_config_file=$QBONE_CLIENT_CONFIG_FILE --logtostderr`
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestContainerzWorkflow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	dut := ondatra.DUT(t, "dut")
	cli, err := client.NewClient(ctx, t, dut)
	if err != nil {
		t.Fatalf("Unable to create gNOI containerz client: %v", err)
	}

	t.Run("Deploy", func(t *testing.T) {
		progCh, err := cli.PushImage(ctx, "bonnet", "g3", bonnetTarLocation)
		if err != nil {
			t.Fatalf("Unable to push bonnet image: %v", err)
		}

		for prog := range progCh {
			switch {
			case prog.Error != nil:
				t.Fatalf("Failed to push image: %v", prog.Error)
			case prog.Finished:
				t.Logf("Pushed %s/%s\n", prog.Image, prog.Tag)
			default:
				t.Logf(" %d bytes pushed", prog.BytesReceived)
			}
		}
	})

	t.Run("ListImage", func(t *testing.T) {

		listCh, err := cli.ListImage(ctx, 10, nil)
		if err != nil {
			t.Errorf("unable to list images: %v", err)
		}

		wantImgs := []string{"bonnet", "g3"}
		var gotImgs []string

		for img := range listCh {
			gotImgs = append(gotImgs, img.ImageName)
			gotImgs = append(gotImgs, img.ImageTag)
		}

		if diff := cmp.Diff(wantImgs, gotImgs, cmpopts.SortSlices(func(l, r string) bool { return l < r })); diff != "" {
			t.Errorf("ListImage() returned diff (-want, +got):\n%s", diff)
		}

	})

	t.Run("StartContainer", func(t *testing.T) {

		ret, err := cli.StartContainer(ctx, "bonnet", "g3", bonnetRunCmd, "bonnet-1",
			client.WithCapabilities([]string{"NET_ADMIN"}, nil), client.WithNetwork("host"),
			client.WithRestartPolicy("ALWAYS"), client.WithVolumes([]string{"/dev/net/tun:/dev/net/tun",
				"/misc/app_host/google/.loas:/.loas", "/misc/app_host/google/config:/config"}))

		if err != nil {
			t.Fatalf("Unable to start container: %v", err)
		}

		// wait for bonnet container to come up.
		time.Sleep(5 * time.Second)
		t.Logf("Started %s", ret)

	})

	t.Run("ListContainer", func(t *testing.T) {
		listCh, err := cli.ListContainer(ctx, true, 0, nil)
		if err != nil {
			t.Fatalf("unable to list containers: %v", err)
		}

		wantCntrs := []string{"bonnet-1", "bonnet:g3", "RUNNING"}
		var gotCntrs []string

		for cnt := range listCh {
			gotCntrs = append(gotCntrs, cnt.ImageName)
			gotCntrs = append(gotCntrs, cnt.Name)
			gotCntrs = append(gotCntrs, cnt.State)
		}

		if diff := cmp.Diff(wantCntrs, gotCntrs,
			cmpopts.SortSlices(func(l, r string) bool { return l < r })); diff != "" {
			t.Errorf("ListContainer() returned diff (-want, +got):\n%s", diff)
		}
	})

	t.Run("Logs", func(t *testing.T) {
		logCh, err := cli.Logs(ctx, "bonnet-1", false)
		if err != nil {
			t.Errorf("unable to obtain logs for %s: %v", "bonnet-1", err)
		}

		var logs []string
		for msg := range logCh {
			logs = append(logs, msg.Msg)
			if msg.Error != nil {
				t.Errorf("logs returned an error: %v", err)
			}
		}

		if len(logs) == 0 {
			t.Errorf("no logs were returned")
		}
	})

	t.Run("Volume", func(t *testing.T) {
		wantVolume := "my-vol"
		gotVolume, err := cli.CreateVolume(ctx, "my-vol", "local", nil, nil)
		if err != nil {
			t.Errorf("unable to create volume: %v", err)
		}
		defer cli.RemoveVolume(ctx, "my-vol", true)

		if wantVolume != gotVolume {
			t.Errorf("incorrect volume name: want %s, got %s", wantVolume, gotVolume)
		}

		t.Logf("created volume %s", gotVolume)

		volCh, err := cli.ListVolume(ctx, nil)
		if err != nil {
			t.Errorf("unable to list volumes: %v", err)
		}

		wantVolumes := []*client.VolumeInfo{
			{
				Name:   "my-vol",
				Driver: "local",
			},
		}
		var gotVolumes []*client.VolumeInfo
		for vol := range volCh {
			gotVolumes = append(gotVolumes, vol)
		}

		if diff := cmp.Diff(wantVolumes, gotVolumes,
			cmpopts.IgnoreFields(client.VolumeInfo{}, "CreationTime")); diff != "" {
			t.Errorf("Volumes returned diff (-want, +got):\n%s", diff)
		}
	})

	t.Run("StopContainer", func(t *testing.T) {

		if err := cli.StopContainer(ctx, "bonnet-1", true, false); err != nil {
			t.Logf("container already stopping: %v", err)
		}

	})
	
	t.Run("RemoveImage", func(t *testing.T) {
		if err := cli.RemoveImage(ctx, "bonnet", "g3", true); err != nil {
			t.Logf("Failed to remove image: %v", err)
		}
	})

}
