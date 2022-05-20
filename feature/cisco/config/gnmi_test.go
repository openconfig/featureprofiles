package config

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/topologies/binding/cisco/config"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
)

const fullConfig = ` 
!! IOS XR Configuration 7.8.1.11I
!! Last configuration change at Thu May 19 18:39:42 2022 by cisco
!
hostname %s
logging console disable
username cisco
 group root-lr
 group cisco-support
 secret 10 $6$EkwCU1Aj1sv.CU1.$GwgVbWTCCs1frOB2n0HnVEHEDvxzCdPE58ZTQflCOZARap7UCofmTY2CdmYJoRY2mHKCkAH/Qwu7cRDqiBFo.0
!
grpc
 port 57777
!
vrf TE
!
vrf VRF1
!
vrf cust0
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
!
class-map type traffic match-all 1_PBR
 match ethertype ipv4 
 match protocol ipinip 
 end-class-map
! 
class-map type traffic match-all 2_PBR
 match dscp ipv4 16 
 end-class-map
! 
class-map type traffic match-all 3_PBR
 match dscp ipv4 18 
 end-class-map
!         
class-map type traffic match-all 4_PBR
 match dscp ipv4 48 
 end-class-map
!         
class-map type traffic match-all DSCP10
 match protocol ipinip 
 end-class-map
!         
class-map type traffic match-all DSCP16
 match dscp cs2 
 end-class-map
!         
class-map type traffic match-all DSCP18
 match dscp af21 
 end-class-map
!         
class-map type traffic match-all DSCP48
 match dscp cs6 
 end-class-map
!         
policy-map type pbr PBR
 class type traffic 1_PBR 
  redirect ipv4 nexthop vrf TE 
 !        
 class type traffic 2_PBR 
  redirect ipv4 nexthop vrf TE 
 !        
 class type traffic 3_PBR 
  redirect ipv4 nexthop vrf VRF1 
 !        
 class type traffic 4_PBR 
  redirect ipv4 nexthop vrf TE 
 !        
 class type traffic class-default 
 !        
 end-policy-map
!         
policy-map type pbr Transit
 class type traffic DSCP10 
  redirect ipv4 nexthop vrf TE 
 !        
 class type traffic DSCP16 
  redirect ipv4 nexthop vrf TE 
 !        
 class type traffic DSCP18 
  redirect ipv4 nexthop vrf VRF1 
 !        
 class type traffic DSCP48 
  redirect ipv4 nexthop vrf TE 
 !        
 class type traffic class-default 
 !        
 end-policy-map
!         
interface Bundle-Ether1
!         
interface Bundle-Ether1.1
!         
interface Bundle-Ether120
 service-policy type pbr input PBR
 ipv4 address 100.120.1.1 255.255.255.0
 mac-address 1.2.0
!         
interface Bundle-Ether121
 ipv4 address 100.121.1.1 255.255.255.0
!         
interface Bundle-Ether122
 ipv4 address 100.122.1.1 255.255.255.0
!         
interface Bundle-Ether123
 ipv4 address 100.123.1.1 255.255.255.0
!         
interface Bundle-Ether124
 ipv4 address 100.124.1.1 255.255.255.0
!         
interface Bundle-Ether125
 ipv4 address 100.125.1.1 255.255.255.0
!         
interface Bundle-Ether126
 ipv4 address 100.126.1.1 255.255.255.0
!         
interface Bundle-Ether127
 ipv4 address 100.127.1.1 255.255.255.0
!         
interface Bundle-Ether128
 ipv4 address 100.128.1.1 255.255.255.0
!         
interface Loopback0
 ipv4 address 1.1.1.1 255.255.255.255
 ipv6 address 1::1/128
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
interface FourHundredGigE0/0/0/10
 bundle id 120 mode on
!         
interface FourHundredGigE0/0/0/11
 bundle id 121 mode on
!         
interface FourHundredGigE0/0/0/12
 bundle id 122 mode on
!         
interface FourHundredGigE0/0/0/13
 bundle id 123 mode on
!         
interface FourHundredGigE0/0/0/14
 bundle id 124 mode on
!         
interface FourHundredGigE0/0/0/15
 bundle id 125 mode on
!         
interface FourHundredGigE0/0/0/16
 bundle id 126 mode on
!         
interface FourHundredGigE0/0/0/17
 bundle id 127 mode on
!         
interface FourHundredGigE0/0/0/18
 bundle id 128 mode on
!         
interface FourHundredGigE0/0/0/19
 shutdown 
!         
interface FourHundredGigE0/0/0/20
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
!         
route-policy ALLOW
  pass    
end-policy
!         
router isis B4
 is-type level-2-only
 net 47.0001.0000.0000.0001.00
 address-family ipv4 unicast
  metric-style wide
 !        
 address-family ipv6 unicast
  metric-style wide
 !        
 interface Bundle-Ether120
  circuit-type level-2-only
  point-to-point
  address-family ipv4 unicast
   metric 10
  !       
  address-family ipv6 unicast
   metric 10
  !       
 !        
 interface Bundle-Ether121
  circuit-type level-2-only
  point-to-point
  address-family ipv4 unicast
   metric 10
  !       
  address-family ipv6 unicast
   metric 10
  !       
 !        
 interface Loopback0
  passive 
  address-family ipv4 unicast
  !       
  address-family ipv6 unicast
  !       
 !        
!         
router bgp 65000
 nsr      
 bgp router-id 1.1.1.1
 bgp graceful-restart
 address-family ipv4 unicast
  additional-paths receive
  additional-paths send
 !        
 neighbor 100.120.1.2
  remote-as 64001
  local-as 63001
  ebgp-multihop 255
  description BGP_TEST
  address-family ipv4 unicast
   route-policy ALLOW in
   route-policy ALLOW out
  !       
 !        
 neighbor 100.121.1.2
  remote-as 64001
  local-as 63001
  ebgp-multihop 255
  description BGP_TEST
  address-family ipv4 unicast
   route-policy ALLOW in
   route-policy ALLOW out
  !       
 !        
!         
cef proactive-arp-nd enable
ssh server v2
ssh server netconf vrf default
hw-module profile pbr vrf-redirect
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
	//t.Skip() // skiped since this can cuase issues for other test cases
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
