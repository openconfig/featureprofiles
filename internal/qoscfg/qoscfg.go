// Copyright 2023 Google LLC
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

// Package qoscfg provides utilities for configure QoS across vendors.
package qoscfg

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// SetForwardingGroup sets a forwarding group in the specified QoS config.
func SetForwardingGroup(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, groupName, queueName string) {
	t.Helper()
	qos.GetOrCreateForwardingGroup(groupName).SetOutputQueue(queueName)
	qos.GetOrCreateQueue(queueName)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qos)
}

// SetInputClassifier sets an input classifier in the specified QoS config.
func SetInputClassifier(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, intfID string, classType oc.E_Input_Classifier_Type, className string) {
	t.Helper()
	intf := qos.GetOrCreateInterface(intfID)
	intf.GetOrCreateInterfaceRef().SetInterface(intfID)
	if dut.Vendor() != ondatra.CISCO {
		intf.GetOrCreateInterfaceRef().SetSubinterface(0)
	}
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	intf.GetOrCreateInput().GetOrCreateClassifier(classType).SetName(className)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qos)
}
