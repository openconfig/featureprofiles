// Copyright 2021 Google Inc.
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

// Package confirm provides experimental assertion helpers.
package confirm

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygot/ytypes"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// getSchema looks up a struct's schema by reflected name
func getSchema(s ygot.ValidatedGoStruct) (*yang.Entry, error) {
	typ := reflect.TypeOf(s)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	typeName := typ.Name()
	schema, ok := oc.SchemaTree[typeName]
	if !ok {
		return nil, fmt.Errorf("no schema for type %v", typeName)
	}
	return schema, nil
}

// PathLabel converts a gnmi Path to a string, or to an error indicator if that
// fails.
func PathLabel(pth *gnmipb.Path) string {
	pstr, err := ygot.PathToString(pth)
	if err != nil {
		return fmt.Sprintf("<unstringable path: %v>", err)
	}
	return pstr
}

// getSingleValue is ytypes.GetNode, except it returns the Data of the single
// node found, or an error if the number of matching nodes is not exactly 1.
func getSingleValue(schema *yang.Entry, root ygot.ValidatedGoStruct, pth *gnmipb.Path) (interface{}, error) {
	vals, err := ytypes.GetNode(schema, root, pth)
	if err != nil {
		return nil, err
	}
	if len(vals) != 1 {
		return nil, fmt.Errorf("expected exactly one value, found %v", len(vals))
	}
	return vals[0].Data, nil
}

// Change represents a difference in value at the gNMI path.
type Change struct {
	Path    *gnmipb.Path
	Want    interface{}
	Missing bool
	Got     interface{}
}

// Readable is the same as fmt.Sprintf("%v", v), except that pointers to basic types will be
// formatted as e.g. "&42" instead of "0xc0000b6020", so that error messages are not useless.
func Readable(v interface{}) string {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		return fmt.Sprintf("&%v", val.Elem().Interface())
	}
	return fmt.Sprintf("%v", v)
}

// ExtractChanges turns a Notification into a collection of Change objects.
func ExtractChanges(diff *gnmipb.Notification, want, got ygot.ValidatedGoStruct) ([]*Change, error) {
	schema, err := getSchema(want)
	if err != nil {
		return nil, fmt.Errorf("schema lookup failure: %v", err)
	}
	var changes []*Change
	for _, pth := range diff.GetDelete() {
		wantVal, err := getSingleValue(schema, want, pth)
		if err != nil {
			return nil, fmt.Errorf("faild to parse expected value at path %v: %v", pth, err)
		}
		changes = append(changes, &Change{pth, wantVal, true, nil})
	}
	for _, upd := range diff.GetUpdate() {
		pth := upd.GetPath()
		gotVal, err := getSingleValue(schema, got, pth)
		if err != nil {
			return nil, fmt.Errorf("faild to parse received value at path %v: %v", pth, err)
		}
		wantVal, err := getSingleValue(schema, want, pth)
		if err != nil {
			return nil, fmt.Errorf("faild to parse expected value at path %v: %v", pth, err)
		}
		changes = append(changes, &Change{pth, wantVal, false, gotVal})
	}
	return changes, nil
}

// State checks that every set value in want is present in got. Extra fields in got will be ignored
// (typically the state contains many more keys than just the ones we're setting).
//
// DEPRECATED: experimental function
func State(t testing.TB, want, got ygot.ValidatedGoStruct) {
	t.Helper()
	diff, err := ygot.Diff(want, got, &ygot.IgnoreAdditions{})
	if err != nil {
		t.Errorf("ygot.Diff failure: %v", err)
		return
	}
	changes, err := ExtractChanges(diff, want, got)
	if err != nil {
		t.Errorf("Failed to compare states: %v", err)
		return
	}
	for _, change := range changes {
		t.Errorf("%v: got %v, want %v", PathLabel(change.Path), Readable(change.Got), Readable(change.Want))
	}
}
