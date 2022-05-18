package config

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/topologies/binding/cisco/config"
	"github.com/openconfig/ondatra"
)

const fullConfig = ` 
hostname DUT11
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

func TestCLIConfigViaSSH(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	_, err := config.WithSSH(context.Background(), t, dut, "config \n hostname test \n commit \n", 30*time.Second)
	if err != nil {
		t.Fatalf("TestCLIConfigViaSSH is failed: %v", err)
	}
}

func TestGNMIFullCommitReplace(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	config.GNMICommitReplace(context.Background(), t, dut, fullConfig)
}

func TestGNMIFullCommitReplaceWithOC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	_, err := config.GNMICommitReplace(context.Background(), t, dut, fullConfig)
	if err != nil {
		t.Fatalf("TestGNMIFullCommitReplace is failed: %v", err)
	}
}

func TestHWModuleWithGNMI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	_, err := config.GNMICommitReplace(context.Background(), t, dut, fullConfig)
	if err != nil {
		t.Fatalf("TestGNMIFullCommitReplace is failed: %v", err)
	}
}
