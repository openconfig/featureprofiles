package container_lifecycle_test

import (
	"context"
	"crypto/tls"
	"flag"
	"testing"

	"github.com/openconfig/containerz/client"
	cpb "github.com/openconfig/featureprofiles/internal/cntrsrv/proto/cntr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	containerTar   = flag.String("container_tar", "/tmp/cntrsrv.tar", "The container tarball to deploy.")
	containerzAddr = flag.String("containerz_addr", "localhost:19999", "containerz server address")
)

// TestDeployAndStartContainer implements CNTR-1.1 validating that it is
// possible deploy and start a container via containerz.
func TestDeployAndStartContainer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := client.NewClient(ctx, *containerzAddr)
	if err != nil {
		t.Fatalf("unable to dial containerz: %v", err)
	}

	progCh, err := cli.PushImage(ctx, "cntrsrv", "latest", *containerTar)
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

	ret, err := cli.StartContainer(ctx, "cntrsrv", "latest", "./cntrsrv", "test-instance", client.WithPorts([]string{"60061:60061"}))
	if err != nil {
		t.Fatalf("unable to start container: %v", err)
	}
	t.Logf("Started %s", ret)

	tlsc := credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, // NOLINT
	})
	conn, err := grpc.DialContext(ctx, "localhost:60061", grpc.WithTransportCredentials(tlsc), grpc.WithBlock())
	if err != nil {
		t.Fatalf("Failed to dial cntrsrv, %v", err)
	}

	cntrCli := cpb.NewCntrClient(conn)
	if _, err = cntrCli.Ping(ctx, &cpb.PingRequest{}); err != nil {
		t.Errorf("unable to reach cntrsrv: %v", err)
	}

}
