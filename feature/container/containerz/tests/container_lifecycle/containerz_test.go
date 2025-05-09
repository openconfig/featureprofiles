package container_lifecycle_test

import (
	"context"
	"flag"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/containerz/client"

	cpb "github.com/openconfig/gnoi/containerz"
)

var (
	containerTar        = flag.String("container_tar", "/tmp/cntrsrv.tar", "The container tarball to deploy.")
	containerUpgradeTar = flag.String("container_upgrade_tar", "/tmp/cntrsrv-upgrade.tar", "The container tarball to upgrade to.")
)

const (
	instanceName = "test-instance"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func containerzClient(ctx context.Context, t *testing.T) *client.Client {

	dut := ondatra.DUT(t, "dut")
	switch dut.Vendor() {
	case ondatra.ARISTA:
		if deviations.ContainerzOCUnsupported(dut) {
			dut.Config().New().WithAristaText(`
				management api gnoi
				service containerz
				  transport gnmi default
				  !
				  container runtime
					 vrf default
				!
			`).Append(t)
		}
	default:
		t.Fatalf("dut %s does not support containerz", dut.Name())
	}

	t.Logf("Waiting for device to ingest its config.")
	time.Sleep(time.Minute)

	return client.NewClientFromStub(dut.RawAPIs().GNOI(t).Containerz())
}

func startContainer(ctx context.Context, t *testing.T) *client.Client {
	cli := containerzClient(ctx, t)

	// Call RemoveContainer here to make sure no other container is using the instanceName name or that instanceName
	// is a leftover from a previous failed test.
	if err := cli.RemoveContainer(ctx, instanceName, true); err != nil {
		if status.Code(err) != codes.NotFound {
			t.Fatalf("failed to remove container: %v", err)
		}
	}

	progCh, err := cli.PushImage(ctx, "cntrsrv", "latest", *containerTar, false)
	if err != nil {
		t.Fatalf("unable to push image: %v", err)
	}

	for prog := range progCh {
		switch {
		case prog.Error != nil:
			t.Fatalf("failed to push image: %v", prog.Error)
		case prog.Finished:
			t.Logf("Pushed %s/%s\n", prog.Image, prog.Tag)
		default:
			t.Logf(" %d bytes pushed", prog.BytesReceived)
		}
	}

	ret, err := cli.StartContainer(ctx, "cntrsrv", "latest", "./cntrsrv", instanceName, client.WithPorts([]string{"60061:60061"}))
	if err != nil {
		t.Fatalf("unable to start container: %v", err)
	}

	t.Logf("Started %s", ret)

	time.Sleep(5 * time.Second)
	return cli
}

func stopContainer(ctx context.Context, t *testing.T, cli *client.Client) {
	if err := cli.StopContainer(ctx, instanceName, true); err != nil {
		t.Logf("container already stopping: %v", err)
	}
}

// TestDeployAndStartContainer implements CNTR-1.1 validating that it is
// possible deploy and start a container via containerz.
func TestDeployAndStartContainer(t *testing.T) {
	ctx := context.Background()
	cli := startContainer(ctx, t)
	defer stopContainer(ctx, t, cli)

	for i := 0; i < 5; i++ {
		ch, err := cli.ListContainer(ctx, true, 0, map[string][]string{
			"name": {instanceName},
		})
		if err != nil {
			t.Fatalf("unable to list container state for %s", instanceName)
		}

		for info := range ch {
			if info.Error != nil {
				t.Fatalf("unable to list containers: %v", info.Error)
			}
			if info.State == cpb.ListContainerResponse_RUNNING.String() {

				return
			}
		}
		// wait for cntr container to come up.
		time.Sleep(5 * time.Second)
	}

	t.Fatalf("test-instance was not started.")
}

// TestRetrieveLogs implements CNTR-1.2 validating that logs can be retrieved from a
// running container.
func TestRetrieveLogs(t *testing.T) {
	ctx := context.Background()
	cli := startContainer(ctx, t)
	defer stopContainer(ctx, t, cli)

	logCh, err := cli.Logs(ctx, instanceName, false)
	if err != nil {
		t.Errorf("unable to obtain logs for %s: %v", instanceName, err)
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
}

// TestListContainers implements CNTR-1.3 validating listing running containers.
func TestListContainers(t *testing.T) {
	ctx := context.Background()
	cli := startContainer(ctx, t)
	defer stopContainer(ctx, t, cli)

	listCh, err := cli.ListContainer(ctx, true, 0, nil)
	if err != nil {
		t.Errorf("unable to list containers: %v", err)
	}

	wantCntrs := []string{"cntrsrv:latest"}
	var gotCntrs []string

	for cnt := range listCh {
		gotCntrs = append(gotCntrs, cnt.ImageName)
	}

	if diff := cmp.Diff(wantCntrs, gotCntrs, cmpopts.SortSlices(func(l, r string) bool { return l < r })); diff != "" {
		t.Errorf("ListContainer() returned diff (-want, +got):\n%s", diff)
	}
}

// TestStopContainer implements CNTR-1.4 validating that stopping a container works as expected.
func TestStopContainer(t *testing.T) {
	ctx := context.Background()
	cli := startContainer(ctx, t)
	stopContainer(ctx, t, cli)

	// wait for container to stop
	time.Sleep(2 * time.Second)

	listCh, err := cli.ListContainer(ctx, true, 0, nil)
	if err != nil {
		t.Errorf("unable to list containers: %v", err)
	}

	for cntr := range listCh {
		t.Errorf("StopContainer did not stop the container: %v", cntr)
	}
}

// TestVolumes checks that volumes can be created or removed, it does not test
// if they can actually be used.
func TestVolumes(t *testing.T) {
	ctx := context.Background()
	cli := containerzClient(ctx, t)

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
			// Options used by linux mounts. See mount(8).
			Options: map[string]string{"device": "", "o": "", "type": "none"},
		},
	}
	var gotVolumes []*client.VolumeInfo
	for vol := range volCh {
		gotVolumes = append(gotVolumes, vol)
	}

	if diff := cmp.Diff(wantVolumes, gotVolumes, cmpopts.IgnoreFields(client.VolumeInfo{}, "CreationTime")); diff != "" {
		t.Errorf("Volumes returned diff (-want, +got):\n%s", diff)
	}
}

func TestUpgrade(t *testing.T) {
	ctx := context.Background()
	cli := startContainer(ctx, t)
	defer stopContainer(ctx, t, cli)

	progCh, err := cli.PushImage(ctx, "cntrsrv", "upgrade", *containerUpgradeTar, false)
	if err != nil {
		t.Fatalf("unable to push image: %v", err)
	}

	for prog := range progCh {
		switch {
		case prog.Error != nil:
			t.Fatalf("failed to push image: %v", err)
		case prog.Finished:
			t.Logf("Pushed %s/%s\n", prog.Image, prog.Tag)
		default:
			t.Logf(" %d bytes pushed", prog.BytesReceived)
		}
	}

	if _, err := cli.UpdateContainer(ctx, "cntrsrv", "upgrade", "./cntrsrv", instanceName, false, client.WithPorts([]string{"60061:60061"})); err != nil {
		t.Errorf("unable to upgrade container: %v", err)
	}

	time.Sleep(3 * time.Second)

	listCh, err := cli.ListContainer(ctx, true, 0, nil)
	if err != nil {
		t.Errorf("unable to list containers: %v", err)
	}

	wantCntrs := []string{"cntrsrv:upgrade"}
	var gotCntrs []string

	for cnt := range listCh {
		gotCntrs = append(gotCntrs, cnt.ImageName)
	}

	if diff := cmp.Diff(wantCntrs, gotCntrs, cmpopts.SortSlices(func(l, r string) bool { return l < r })); diff != "" {
		t.Errorf("ListContainer() returned diff (-want, +got):\n%s", diff)
	}
}
