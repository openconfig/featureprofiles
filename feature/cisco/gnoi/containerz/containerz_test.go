package containerz_test

import (
	"context"
	"flag"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/containerz"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

const (
	bonnetRunCmd = `/bonnet --uid= --use_doh --use_tap_instead_of_tun --loas2_force_full_key_exchange 
	--noqbone_tunnel_health_check_stubby_probes --qbone_region_file=/config/qbone_region --svelte_retry_interval_ms=1000000000 
	--qbone_resolver_cc_algo= --qbone_bonnet_use_custom_healthz_handler --qbone_client_config_file=$QBONE_CLIENT_CONFIG_FILE --logtostderr`
)

var (
	bonnetTarLocation = flag.String("bonnet_tar_location",
		"/auto/tftp-idt-tools/pi_infra/b4_containerz/bonnet-g3.tar.gz", "Location of the bonnet tar file")
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
		deployImage(t, ctx, cli)
	})

	t.Run("ListImage", func(t *testing.T) {
		listImage(t, ctx, cli)
	})

	t.Run("StartContainer", func(t *testing.T) {
		startContainer(t, ctx, cli)
	})

	t.Run("ListContainer", func(t *testing.T) {
		listContainer(t, ctx, cli)
	})

	t.Run("Logs", func(t *testing.T) {
		logs(t, ctx, cli)
	})

	t.Run("Volume", func(t *testing.T) {
		volume(t, ctx, cli)
	})

	t.Run("StopContainer", func(t *testing.T) {
		stopContainer(t, ctx, cli)

	})

	t.Run("RemoveImage", func(t *testing.T) {
		removeImage(t, ctx, cli)
	})

}

func TestHa(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dut := ondatra.DUT(t, "dut")
	cli, err := client.NewClient(ctx, t, dut)
	if err != nil {
		t.Fatalf("Unable to create gNOI containerz client: %v", err)
	}

	t.Run("Deploy bonnet Image", func(t *testing.T) {
		deployImage(t, ctx, cli)
	})

	t.Run("Start bonnet container", func(t *testing.T) {
		startContainer(t, ctx, cli)
		listContainer(t, ctx, cli)
	})

	t.Run("Reload", func(t *testing.T) {
		util.RebootDevice(t)
		listContainer(t, ctx, cli)
	})

	t.Run("RPFO", func(t *testing.T) {
		utils.Dorpfo(ctx, t, true)
		listContainer(t, ctx, cli)
	})

	// Cleanup
	t.Run("Remove bonnet Image", func(t *testing.T) {
		removeImage(t, ctx, cli)
	})
}
