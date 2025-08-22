// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgplugins

import (
	"context"
	"strings"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
)

type FeatureType int

const (
	FeatureMplsTracking FeatureType = iota
	FeatureVrfSelectionExtended
	FeaturePolicyForwarding

	aristaTcamProfileMplsTracking = `
hardware counter feature traffic-policy in
!
hardware tcam
  profile ancx
    feature acl port ip
        sequence 45
        key size limit 160
        key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops l4-src-port src-ip tcp-control ttl
        action count drop mirror
        packet ipv4 forwarding bridged
        packet ipv4 forwarding routed
        packet ipv4 forwarding routed multicast
        packet ipv4 mpls ipv4 forwarding mpls decap
        packet ipv4 mpls ipv6 forwarding mpls decap
        packet ipv4 non-vxlan forwarding routed decap
        packet ipv4 vxlan eth ipv4 forwarding routed decap
        packet ipv4 vxlan forwarding bridged decap
    feature acl port ip egress mpls-tunnelled-match
        sequence 95
    feature acl port ipv6
        sequence 25
        key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-ops-3b l4-src-port src-ipv6-high src-ipv6-low tcp-control
        action count drop mirror
        packet ipv6 forwarding bridged
        packet ipv6 forwarding routed
        packet ipv6 forwarding routed multicast
        packet ipv6 ipv6 forwarding routed decap
    feature acl port ipv6 egress
        sequence 105
        key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-src-port src-ipv6-high src-ipv6-low tcp-control
        action count drop mirror
        packet ipv6 forwarding bridged
        packet ipv6 forwarding routed
    feature acl port mac
        sequence 55
        key size limit 160
        key field dst-mac ether-type src-mac
        action count drop mirror
        packet ipv4 forwarding bridged
        packet ipv4 forwarding routed
        packet ipv4 forwarding routed multicast
        packet ipv4 mpls ipv4 forwarding mpls decap
        packet ipv4 mpls ipv6 forwarding mpls decap
        packet ipv4 non-vxlan forwarding routed decap
        packet ipv4 vxlan forwarding bridged decap
        packet ipv6 forwarding bridged
        packet ipv6 forwarding routed
        packet ipv6 forwarding routed decap
        packet ipv6 forwarding routed multicast
        packet ipv6 ipv6 forwarding routed decap
        packet mpls forwarding bridged decap
        packet mpls ipv4 forwarding mpls
        packet mpls ipv6 forwarding mpls
        packet mpls non-ip forwarding mpls
        packet non-ip forwarding bridged
    feature acl vlan ipv6 egress
        sequence 20
        key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-src-port src-ipv6-high src-ipv6-low tcp-control
        action count drop mirror
        packet ipv6 forwarding bridged
        packet ipv6 forwarding routed
    feature counter lfib
        sequence 85
    feature forwarding-destination mpls
        sequence 100
    feature mirror ip
        sequence 80
        key size limit 160
        key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops l4-src-port src-ip tcp-control
        action count mirror set-policer
        packet ipv4 forwarding bridged
        packet ipv4 forwarding routed
        packet ipv4 forwarding routed multicast
        packet ipv4 non-vxlan forwarding routed decap
    feature mpls
        sequence 5
        key size limit 160
        action drop redirect set-ecn
        packet ipv4 mpls ipv4 forwarding mpls decap
        packet ipv4 mpls ipv6 forwarding mpls decap
        packet mpls ipv4 forwarding mpls
        packet mpls ipv6 forwarding mpls
        packet mpls non-ip forwarding mpls
    feature mpls pop ingress
        sequence 90
    feature pbr mpls
        sequence 65
        key size limit 160
        key field mpls-inner-ip-tos
        action count drop redirect
        packet mpls ipv4 forwarding mpls
        packet mpls ipv6 forwarding mpls
        packet mpls non-ip forwarding mpls
    feature qos ip
        sequence 75
        key size limit 160
        key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops l4-src-port src-ip tcp-control
        action set-dscp set-policer set-tc
        packet ipv4 forwarding routed
        packet ipv4 forwarding routed multicast
        packet ipv4 mpls ipv4 forwarding mpls decap
        packet ipv4 mpls ipv6 forwarding mpls decap
        packet ipv4 non-vxlan forwarding routed decap
    feature qos ipv6
        sequence 70
        key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-src-port src-ipv6-high src-ipv6-low
        action set-dscp set-policer set-tc
        packet ipv6 forwarding routed
    feature traffic-policy port ipv4
        sequence 45
        key size limit 160
        key field dscp dst-ip-label ip-frag ip-fragment-offset ip-length ip-protocol l4-dst-port-label l4-src-port-label src-ip-label tcp-control ttl
        action count drop redirect set-dscp set-tc
        packet ipv4 forwarding routed
    feature traffic-policy port ipv4 egress
        key size limit 160
        key field dscp dst-ip-label ip-frag ip-protocol l4-dst-port-label l4-src-port-label src-ip-label
        action count drop
        packet ipv4 forwarding routed
    feature traffic-policy port ipv6
        sequence 25
        key size limit 160
        key field dst-ipv6-label hop-limit ipv6-length ipv6-next-header ipv6-traffic-class l4-dst-port-label l4-src-port-label src-ipv6-label tcp-control
        action count drop redirect set-dscp set-tc
        packet ipv6 forwarding routed
    feature traffic-policy port ipv6 egress
        key size limit 160
        key field dscp dst-ipv6-label ipv6-next-header l4-dst-port-label l4-src-port-label src-ipv6-label
        action count drop
        packet ipv6 forwarding routed
    feature tunnel vxlan
        sequence 50
        key size limit 160
        packet ipv4 vxlan eth ipv4 forwarding routed decap
        packet ipv4 vxlan forwarding bridged decap
  system profile ancx
!
`

	aristaTcamProfileVrfSelectionExtended = `
hardware tcam
   profile vrf-selection-with-ip6-sip
      feature acl port ip
         sequence 45
         key size limit 160
         key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops l4-src-port src-ip tcp-control ttl
         action count drop mirror
         packet ipv4 forwarding bridged
         packet ipv4 forwarding routed
         packet ipv4 forwarding routed multicast
         packet ipv4 mpls ipv4 forwarding mpls decap
         packet ipv4 mpls ipv6 forwarding mpls decap
         packet ipv4 non-vxlan forwarding routed decap
         packet ipv4 vxlan eth ipv4 forwarding routed decap
         packet ipv4 vxlan forwarding bridged decap
      feature acl port ip egress mpls-tunnelled-match
         sequence 95
      feature acl port ipv6
         sequence 25
         key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-ops-3b l4-src-port src-ipv6-high src-ipv6-low tcp-control
         action count drop mirror
         packet ipv6 forwarding bridged
         packet ipv6 forwarding routed
         packet ipv6 forwarding routed multicast
         packet ipv6 ipv6 forwarding routed decap
      feature acl port ipv6 egress
         sequence 105
         key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-src-port src-ipv6-high src-ipv6-low tcp-control
         action count drop mirror
         packet ipv6 forwarding bridged
         packet ipv6 forwarding routed
      feature acl port mac
         sequence 55
         key size limit 160
         key field dst-mac ether-type src-mac
         action count drop mirror
         packet ipv4 forwarding bridged
         packet ipv4 forwarding routed
         packet ipv4 forwarding routed multicast
         packet ipv4 mpls ipv4 forwarding mpls decap
         packet ipv4 mpls ipv6 forwarding mpls decap
         packet ipv4 non-vxlan forwarding routed decap
         packet ipv4 vxlan forwarding bridged decap
         packet ipv6 forwarding bridged
         packet ipv6 forwarding routed
         packet ipv6 forwarding routed decap
         packet ipv6 forwarding routed multicast
         packet ipv6 ipv6 forwarding routed decap
         packet mpls forwarding bridged decap
         packet mpls ipv4 forwarding mpls
         packet mpls ipv6 forwarding mpls
         packet mpls non-ip forwarding mpls
         packet non-ip forwarding bridged
      feature acl subintf ip
         sequence 40
         key size limit 160
         key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops-18b l4-src-port src-ip tcp-control ttl
         action count drop
         packet ipv4 forwarding routed
      feature acl subintf ipv6
         sequence 15
         key field dst-ipv6 ipv6-next-header l4-dst-port l4-src-port src-ipv6-high src-ipv6-low tcp-control
         action count drop
         packet ipv6 forwarding routed
      feature acl vlan ip
         sequence 35
         key size limit 160
         key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops-18b l4-src-port src-ip tcp-control ttl
         action count drop
         packet ipv4 forwarding routed
         packet ipv4 mpls ipv4 forwarding mpls decap
         packet ipv4 mpls ipv6 forwarding mpls decap
         packet ipv4 non-vxlan forwarding routed decap
         packet ipv4 vxlan eth ipv4 forwarding routed decap
      feature acl vlan ipv6
         sequence 10
         key field dst-ipv6 ipv6-next-header l4-dst-port l4-src-port src-ipv6-high src-ipv6-low tcp-control
         action count drop
         packet ipv6 forwarding routed
         packet ipv6 ipv6 forwarding routed decap
      feature acl vlan ipv6 egress
         sequence 20
         key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-src-port src-ipv6-high src-ipv6-low tcp-control
         action count drop mirror
         packet ipv6 forwarding bridged
         packet ipv6 forwarding routed
      feature counter lfib
         sequence 85
      feature forwarding-destination mpls
         sequence 100
      feature mirror ip
         sequence 80
         key size limit 160
         key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops l4-src-port src-ip tcp-control
         action count mirror set-policer
         packet ipv4 forwarding bridged
         packet ipv4 forwarding routed
         packet ipv4 forwarding routed multicast
         packet ipv4 non-vxlan forwarding routed decap
      feature mpls
         sequence 5
         key size limit 160
         action drop redirect set-ecn
         packet ipv4 mpls ipv4 forwarding mpls decap
         packet ipv4 mpls ipv6 forwarding mpls decap
         packet mpls ipv4 forwarding mpls
         packet mpls ipv6 forwarding mpls
         packet mpls non-ip forwarding mpls
      feature mpls pop ingress
         sequence 90
      feature pbr ip
         sequence 60
         key size limit 160
         key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops-18b l4-src-port src-ip tcp-control
         action count redirect
         packet ipv4 forwarding routed
         packet ipv4 mpls ipv4 forwarding mpls decap
         packet ipv4 mpls ipv6 forwarding mpls decap
         packet ipv4 non-vxlan forwarding routed decap
         packet ipv4 vxlan forwarding bridged decap
      feature pbr ipv6
         sequence 30
         key field dst-ipv6 ipv6-next-header l4-dst-port l4-src-port src-ipv6-high src-ipv6-low tcp-control
         action count redirect
         packet ipv6 forwarding routed
      feature pbr mpls
         sequence 65
         key size limit 160
         key field mpls-inner-ip-tos
         action count drop redirect
         packet mpls ipv4 forwarding mpls
         packet mpls ipv6 forwarding mpls
         packet mpls non-ip forwarding mpls
      feature qos ip
         sequence 75
         key size limit 160
         key field dscp dst-ip ip-frag ip-protocol l4-dst-port l4-ops l4-src-port src-ip tcp-control
         action set-dscp set-policer set-tc
         packet ipv4 forwarding routed
         packet ipv4 forwarding routed multicast
         packet ipv4 mpls ipv4 forwarding mpls decap
         packet ipv4 mpls ipv6 forwarding mpls decap
         packet ipv4 non-vxlan forwarding routed decap
      feature qos ipv6
         sequence 70
         key field dst-ipv6 ipv6-next-header ipv6-traffic-class l4-dst-port l4-src-port src-ipv6-high src-ipv6-low
         action set-dscp set-policer set-tc
         packet ipv6 forwarding routed
      feature tunnel vxlan
         sequence 50
         key size limit 160
         packet ipv4 vxlan eth ipv4 forwarding routed decap
         packet ipv4 vxlan forwarding bridged decap
      feature vrf selection
         port qualifier size 8 bits
      feature vrf selection extended
	  !
	system profile vrf-selection-with-ip6-sip
`
	aristaTcamProfilePolicyForwarding = `
    hardware tcam
  	profile tcam-policy-forwarding
      feature traffic-policy port ipv4
         sequence 45
         key size limit 160
         key field dscp dst-ip-label ip-frag ip-fragment-offset ip-length ip-protocol l4-dst-port-label l4-src-port-label src-ip-label tcp-control ttl
         action count drop redirect set-dscp set-tc
         packet ipv4 forwarding routed
      !
      feature traffic-policy port ipv6
         sequence 25
         key size limit 160
         key field dst-ipv6-label hop-limit ipv6-length ipv6-next-header ipv6-traffic-class l4-dst-port-label l4-src-port-label src-ipv6-label tcp-control
         action count drop redirect set-dscp set-tc
         packet ipv6 forwarding routed
      !
   system profile tcam-policy-forwarding
    !
    hardware counter feature gre tunnel interface out
    !
    hardware counter feature traffic-policy in
    !
    hardware counter feature traffic-policy out
    !
    hardware counter feature route ipv4
    !
    hardware counter feature nexthop
    !
    `
)

var (
	aristaTcamProfileMap = map[FeatureType]string{
		FeatureMplsTracking:         aristaTcamProfileMplsTracking,
		FeatureVrfSelectionExtended: aristaTcamProfileVrfSelectionExtended,
		FeaturePolicyForwarding:     aristaTcamProfilePolicyForwarding,
	}
)

func buildCliSetRequest(config string) *gpb.SetRequest {
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{
					Origin: "cli",
					Elem:   []*gpb.PathElem{},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_AsciiVal{
						AsciiVal: config,
					},
				},
			},
		},
	}
	return gpbSetRequest
}

func NewDUTHardwareInit(t *testing.T, dut *ondatra.DUTDevice, feature FeatureType) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		if strings.ToLower(dut.Model()) == "ceos" {
			return ""
		}
		return aristaTcamProfileMap[feature]
	default:
		return ""
	}
}

func PushDUTHardwareInitConfig(t *testing.T, dut *ondatra.DUTDevice, hardwareInitConf string) {
	if hardwareInitConf == "" {
		t.Logf("No hardware init config provided")
		return
	}
	gnmiClient := dut.RawAPIs().GNMI(t)
	t.Log("Pushing hardware init config")
	gpbSetRequest := buildCliSetRequest(hardwareInitConf)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("Failed to set hardware init config: %v", err)
	}
}
