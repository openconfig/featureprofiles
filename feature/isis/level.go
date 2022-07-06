/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package isis

import (
	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// Level struct to hold ISIS level OC attributes.
type Level struct {
	oc fpoc.NetworkInstance_Protocol_Isis_Level
}

// NewLevel returns a new Level object.
func NewLevel(level int) *Level {
	return &Level{
		oc: fpoc.NetworkInstance_Protocol_Isis_Level{
			Enabled:     ygot.Bool(true),
			LevelNumber: ygot.Uint8(uint8(level)),
		},
	}
}

// AugmentGlobal implements the isis.GlobalFeature interface.
// This method augments the ISIS OC with level configuration.
// Use isis.WithFeature(l) instead of calling this method directly.
func (l *Level) AugmentGlobal(isis *fpoc.NetworkInstance_Protocol_Isis) error {
	if err := l.oc.Validate(); err != nil {
		return err
	}
	loc := isis.GetLevel(l.oc.GetLevelNumber())
	if loc == nil {
		return isis.AppendLevel(&l.oc)
	}
	return ygot.MergeStructInto(loc, &l.oc)
}
