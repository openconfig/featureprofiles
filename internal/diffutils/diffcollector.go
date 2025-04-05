// Copyright 2024 Google LLC
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

// Package diffutils provides helper functions for performing diff operations on ygot structs.
package diffutils

import (
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
)

// DiffEntry represents a single path with different values
type DiffEntry struct {
	Path string
	Vx   *reflect.Value
	Vy   *reflect.Value
}

// DiffCollector is a reporter that collects all diff values
type DiffCollector struct {
	path  cmp.Path
	diffs []*DiffEntry
}

func (r *DiffCollector) PushStep(ps cmp.PathStep) {
	r.path = append(r.path, ps)
}

func (r *DiffCollector) Report(rs cmp.Result) {
	if !rs.Equal() {
		vx, vy := r.path.Last().Values()
		r.diffs = append(r.diffs, &DiffEntry{fmt.Sprintf("%#v", r.path), &vx, &vy})
	}
}

func (r *DiffCollector) PopStep() {
	r.path = r.path[:len(r.path)-1]
}

func (r *DiffCollector) String() string {
	str := ""
	for _, df := range r.diffs {
		str += df.String()
	}
	return str
}

// Diff returns all the diff entries
func (r *DiffCollector) Diff() []*DiffEntry {
	return r.diffs
}

func (e *DiffEntry) String() string {
	return fmt.Sprintf("%s:\n\t-: %+v\n\t+: %+v\n", e.Path, *e.Vx, *e.Vy)
}
