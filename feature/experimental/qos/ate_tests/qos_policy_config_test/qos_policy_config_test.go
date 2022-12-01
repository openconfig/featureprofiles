// Copyright 2022 Google LLC
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

package qos_policy_config_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// QoS policy OC config:
//  - classifiers:
//    - /qos/classifiers/classifier/config/name
//    - /qos/classifiers/classifier/config/type
//    - /qos/classifiers/classifier/terms/term/actions/config/target-group
//    - /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set
//    - /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set
//    - /qos/classifiers/classifier/terms/term/config/id
//  - forwarding-groups:
//    - /qos/forwarding-groups/forwarding-group/config/name
//    - /qos/forwarding-groups/forwarding-group/config/output-queue
//    - /qos/queues/queue/config/name
//
// Topology:
//   ate:port1 <--> port1:dut:port2 <--> ate:port2
//
// Test notes:
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestQoSPolicyConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	cases := []struct {
		desc         string
		name         string
		classType    oc.E_Qos_Classifier_Type
		termID       string
		targetGrpoup string
		dscpSet      []uint8
	}{{
		desc:         "classifier_ipv4_be1",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "0",
		targetGrpoup: "target-group-BE1",
		dscpSet:      []uint8{0, 1, 2, 3},
	}, {
		desc:         "classifier_ipv4_be0",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "1",
		targetGrpoup: "target-group-BE0",
		dscpSet:      []uint8{4, 5, 6, 7},
	}, {
		desc:         "classifier_ipv4_af1",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "2",
		targetGrpoup: "target-group-AF1",
		dscpSet:      []uint8{8, 9, 10, 11},
	}, {
		desc:         "classifier_ipv4_af2",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "3",
		targetGrpoup: "target-group-AF2",
		dscpSet:      []uint8{16, 17, 18, 19},
	}, {
		desc:         "classifier_ipv4_af3",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "4",
		targetGrpoup: "target-group-AF3",
		dscpSet:      []uint8{24, 25, 26, 27},
	}, {
		desc:         "classifier_ipv4_af4",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "5",
		targetGrpoup: "target-group-AF4",
		dscpSet:      []uint8{32, 33, 34, 35},
	}, {
		desc:         "classifier_ipv4_nc1",
		name:         "dscp_based_classifier_ipv4",
		classType:    oc.Qos_Classifier_Type_IPV4,
		termID:       "6",
		targetGrpoup: "target-group-NC1",
		dscpSet:      []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
	}, {
		desc:         "classifier_ipv6_be1",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "0",
		targetGrpoup: "target-group-BE1",
		dscpSet:      []uint8{0, 1, 2, 3},
	}, {
		desc:         "classifier_ipv6_be0",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "1",
		targetGrpoup: "target-group-BE0",
		dscpSet:      []uint8{4, 5, 6, 7},
	}, {
		desc:         "classifier_ipv6_af1",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "2",
		targetGrpoup: "target-group-AF1",
		dscpSet:      []uint8{8, 9, 10, 11},
	}, {
		desc:         "classifier_ipv6_af2",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "3",
		targetGrpoup: "target-group-AF2",
		dscpSet:      []uint8{16, 17, 18, 19},
	}, {
		desc:         "classifier_ipv6_af3",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "4",
		targetGrpoup: "target-group-AF3",
		dscpSet:      []uint8{24, 25, 26, 27},
	}, {
		desc:         "classifier_ipv6_af4",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "5",
		targetGrpoup: "target-group-AF4",
		dscpSet:      []uint8{32, 33, 34, 35},
	}, {
		desc:         "classifier_ipv6_nc1",
		name:         "dscp_based_classifier_ipv6",
		classType:    oc.Qos_Classifier_Type_IPV6,
		termID:       "6",
		targetGrpoup: "target-group-NC1",
		dscpSet:      []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
	}}

	t.Logf("qos Classifiers config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			classifier := q.GetOrCreateClassifier(tc.name)
			classifier.SetName(tc.name)
			classifier.SetType(tc.classType)
			term, err := classifier.NewTerm(tc.termID)
			if err != nil {
				t.Fatalf("Failed to create classifier.NewTerm(): %v", err)
			}

			term.SetId(tc.termID)
			action := term.GetOrCreateActions()
			action.SetTargetGroup(tc.targetGrpoup)

			condition := term.GetOrCreateConditions()
			condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
		})
	}

	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	qosClassifiers := gnmi.GetAll(t, dut, gnmi.OC().Qos().ClassifierAny().Name().State())
	t.Logf("qosClassifiers from telmetry: %v", qosClassifiers)
}

func TestQoSForwadingGroupsConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	cases := []struct {
		desc         string
		queueName    string
		targetGrpoup string
	}{{
		desc:         "forwarding-group-BE1",
		queueName:    "BE1",
		targetGrpoup: "target-group-BE1",
	}, {
		desc:         "forwarding-group-BE0",
		queueName:    "BE0",
		targetGrpoup: "target-group-BE0",
	}, {
		desc:         "forwarding-group-AF1",
		queueName:    "AF1",
		targetGrpoup: "target-group-AF1",
	}, {
		desc:         "forwarding-group-AF2",
		queueName:    "AF2",
		targetGrpoup: "target-group-AF2",
	}, {
		desc:         "forwarding-group-AF3",
		queueName:    "AF3",
		targetGrpoup: "target-group-AF3",
	}, {
		desc:         "forwarding-group-AF4",
		queueName:    "AF4",
		targetGrpoup: "target-group-AF4",
	}, {
		desc:         "forwarding-group-NC1",
		queueName:    "NC1",
		targetGrpoup: "target-group-NC1",
	}}

	t.Logf("qos forwarding groups config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			fwdGroup := q.GetOrCreateForwardingGroup(tc.targetGrpoup)
			fwdGroup.SetName(tc.targetGrpoup)
			fwdGroup.SetOutputQueue(tc.queueName)
			// queue := q.GetOrCreateQueue(tc.queueName)
			// queue.SetName(tc.queueName)
		})
	}

	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
	qosfwdGroups := gnmi.GetAll(t, dut, gnmi.OC().Qos().ForwardingGroupAny().State())
	t.Logf("qosfwdGroups from telmetry: %v", qosfwdGroups)
}
