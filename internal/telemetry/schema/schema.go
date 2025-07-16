// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package schema provides utilities for interacting with gNMI Notifications
// and the OpenConfig schema for those Notifications.
//
// The package offers a function to normalize gNMI notifications into
// slices of Paths. Each Path contains a fully specified gNMI path,
// and will contain a single updated value, instead of the compressed
// representation in Notifications, where there are multiple updates
// per Notification, and the path is split between the prefix and the Updates.
//
// To use this library, get a slice of Notifications, convert them to Paths,
// then call the various helper libraries on each Path to validate the data.
package schema

import (
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// Point stores data about a single gNMI/OpenConfig data point.
// This is the equivalent of a gNMI Update, though the path is
// fully normalized, i.e. Point.Path should contain the elements from both
// the gnmi.Notification prefix and the Update.Path concatenated together.
type Point struct {
	// Path is the complete gNMI path for the data point.
	Path *gnmipb.Path
	// Val is the value of the data point, called "Val" for consistency with
	// the gNMI protobuf, where "Value" is a legacy type.
	Val *gnmipb.TypedValue
}

// NotificationToPoints extracts all of the gnmi.Updates from the Notification,
// concatenates the prefix and Update path, then creates a new Path
// for each new update.
//
// Note that we only accept TypedValue values.
func NotificationToPoints(n *gnmipb.Notification) []Point {
	if n == nil {
		return []Point{}
	}
	points := make([]Point, 0, len(n.GetUpdate()))
	for _, u := range n.GetUpdate() {
		fullPath := &gnmipb.Path{}
		fullPath.Elem = append(fullPath.GetElem(), n.GetPrefix().GetElem()...)
		fullPath.Elem = append(fullPath.GetElem(), u.GetPath().GetElem()...)
		points = append(points, Point{
			Path: fullPath,
			Val:  u.GetVal(),
		})
	}
	return points
}
