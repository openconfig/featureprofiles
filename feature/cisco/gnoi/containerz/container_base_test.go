package containerz_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/cisco/containerz"
)

func deployImage(t *testing.T, ctx context.Context, cli *client.Client) {
	progCh, err := cli.PushImage(ctx, "bonnet", "g3", *bonnetTarLocation)
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
}

func startContainer(t *testing.T, ctx context.Context, cli *client.Client) {
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
}

func listImage(t *testing.T, ctx context.Context, cli *client.Client) {
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
}

func listContainer(t *testing.T, ctx context.Context, cli *client.Client) {
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
}

func logs(t *testing.T, ctx context.Context, cli *client.Client) {
	logCh, err := cli.Logs(ctx, "bonnet-1", false)
	if err != nil {
		t.Errorf("unable to obtain logs for %s: %v", "bonnet-1", err)
	}

	var logs []string
	for msg := range logCh {
		logs = append(logs, msg.Msg)
		if msg.Error != nil {
			t.Logf("Message: %v", msg.Msg)
			t.Errorf("logs returned an error: %v", msg.Error)
		}
	}

	if len(logs) == 0 {
		t.Errorf("no logs were returned")
	}
}

func volume(t *testing.T, ctx context.Context, cli *client.Client) {
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
}

func stopContainer(t *testing.T, ctx context.Context, cli *client.Client) {
	if err := cli.StopContainer(ctx, "bonnet-1", true, false); err != nil {
		t.Logf("Failed to stop container: %v", err)
	}
}

func removeImage(t *testing.T, ctx context.Context, cli *client.Client) {
	if err := cli.RemoveImage(ctx, "bonnet", "g3", true); err != nil {
		t.Logf("Failed to remove image: %v", err)
	}
}
