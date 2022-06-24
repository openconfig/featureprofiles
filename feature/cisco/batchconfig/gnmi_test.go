package config

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
)

const fullConfig = ` 
hostname %s
logging console disable
username cisco
 group root-lr
 group cisco-support
 secret 10 $6$lToan5htAPC1n...$sDDES6OVdZvfHnZ2iZf7ThFBJDoarCL05d/gR02GcjySEZ/HTeEcQ90ZoF5rY3oq3XbQRfZGzXt55JGnxOB/W1
!
grpc
 port 57777
!
line console
 exec-timeout 0 0
 absolute-timeout 0
 session-timeout 0
!
line default
 exec-timeout 0 0
 absolute-timeout 0
 session-timeout 0
!
call-home
 service active
 contact smart-licensing
 profile CiscoTAC-1
  active
  destination transport-method email disable
  destination transport-method http
 !
!
interface MgmtEth0/RP0/CPU0/0
 ipv4 address dhcp
!
interface FourHundredGigE0/0/0/0
 shutdown
!
interface FourHundredGigE0/0/0/1
 shutdown
!
interface FourHundredGigE0/0/0/2
 shutdown
!
interface FourHundredGigE0/0/0/3
 shutdown
!
interface FourHundredGigE0/0/0/4
 shutdown
!
interface FourHundredGigE0/0/0/5
 shutdown 
!         
interface FourHundredGigE0/0/0/6
 shutdown 
!         
interface FourHundredGigE0/0/0/7
 shutdown 
!         
interface FourHundredGigE0/0/0/8
 shutdown 
!         
interface FourHundredGigE0/0/0/9
 shutdown 
!         
interface FourHundredGigE0/0/0/21
 shutdown 
!         
interface FourHundredGigE0/0/0/22
 shutdown 
!         
interface FourHundredGigE0/0/0/23
 shutdown 
!         
interface FourHundredGigE0/0/0/24
 shutdown 
!         
interface FourHundredGigE0/0/0/25
 shutdown 
!         
interface FourHundredGigE0/0/0/26
 shutdown 
!         
interface FourHundredGigE0/0/0/27
 shutdown 
!         
interface FourHundredGigE0/0/0/28
 shutdown 
!         
interface FourHundredGigE0/0/0/29
 shutdown 
!         
interface FourHundredGigE0/0/0/30
 shutdown 
!         
interface FourHundredGigE0/0/0/31
 shutdown 
!         
ssh server v2
ssh server netconf vrf default
end   
`

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The following tests are sanity checks to make sure the GNMI Replace works
func TestGNMIFullCommitReplace(t *testing.T) {
	t.Skip() // skipped since the commit replace  can cause issue for other test cases
	dut := ondatra.DUT(t, "dut")
	oldHostName := dut.Telemetry().System().Hostname().Get(t)
	newHostname := oldHostName + "new"
	config.GNMICommitReplace(context.Background(), t, dut, fmt.Sprintf(fullConfig, newHostname))
	defer config.GNMICommitReplace(context.Background(), t, dut, fmt.Sprintf(fullConfig, oldHostName))
	if got := dut.Telemetry().System().Hostname().Get(t); got != newHostname {
		t.Fatalf("Expected the host name to be %s, got %s", newHostname, got)
	}
}

func TestGNMIFullCommitReplaceWithOC(t *testing.T) {
	t.Skip() // skiped since this can cuase issues for other test cases
	dut := ondatra.DUT(t, "dut")
	oldHostName := dut.Telemetry().System().Hostname().Get(t)
	newHostname := oldHostName + "new"
	hostNamePath := dut.Config().System().Hostname()
	config.GNMICommitReplaceWithOC(context.Background(), t, dut, fmt.Sprintf(fullConfig, newHostname), hostNamePath, ygot.String(oldHostName))
	if got := dut.Telemetry().System().Hostname().Get(t); got != oldHostName {
		t.Fatalf("Expected the host name to be not changed  %s, got %s", oldHostName, got)
	}

}

func TestTextConfigWithGNMI(t *testing.T) {
	// skipped since the commit replace  can cause issue for other test cases
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	oldHostName := dut.Telemetry().System().Hostname().Get(t)
	newHostname := oldHostName + "new"
	config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("hostname %s", newHostname))
	defer config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("hostname %s", oldHostName))
	if got := dut.Telemetry().System().Hostname().Get(t); got != newHostname {
		t.Fatalf("Expected the host name to be %s, got %s", newHostname, got)
	}
}

func TestBatchConfig(t *testing.T) {
	//t.Skip() // skiped since this can cuase issues for other test cases
	dut := ondatra.DUT(t, "dut")
	oldHostName := dut.Telemetry().System().Hostname().Get(t)
	newHostname := oldHostName + "new"
	hostNamePath := dut.Config().System().Hostname()
	batchSet := config.NewBatchSetRequest()
	ctx := context.Background()
	batchSet.Append(ctx, t, hostNamePath, ygot.String(newHostname), config.ReplaceOC)
	//
	batchSet.Append(ctx, t, hostNamePath, nil, config.DeleteOC)
	batchSet.Append(ctx, t, hostNamePath, ygot.String(oldHostName), config.UpdateOC)
	cli := fmt.Sprintf("hostname %s", newHostname)
	batchSet.Append(ctx, t, nil, cli, config.UpdateCLI)
	cli = fmt.Sprintf("hostname %s", oldHostName)
	batchSet.Append(ctx, t, nil, cli, config.ReplaceCLI)
	batchSet.Send(ctx, t, dut)
}
