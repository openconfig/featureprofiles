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

// Package assert is deprecated and scoped only to be used with
// feature/experimental/isis/ate_tests/*.  Do not use elsewhere.
package assert

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/ygot/ygot"
)

// YgotPathLabel converts a ygot PathStruct to a string, or to an error indicator
// if that fails.
func YgotPathLabel(p ygot.PathStruct) string {
	gnmiPth, _, errs := ygot.ResolvePath(p)
	if len(errs) > 0 {
		return fmt.Sprintf("<unable to stringify path: %v>", errs)
	}
	return confirm.PathLabel(gnmiPth)
}

// argsT returns [t] as a list of reflect.Values, so that it can be passed to reflect.Value.Call()
func argsT(t testing.TB) []reflect.Value {
	return []reflect.Value{reflect.ValueOf(t)}
}

// QualifiedValue is a supertype of all Qualified* types from ygot-generated OpenConfig
type QualifiedValue interface {
	IsPresent() bool
}

// callLookup uses reflection to call pth.callLookup(t) and return the result. It will return an
// error if pth doesn't have a callLookup(t) method or if pth.callLookup(t) returns anything other
// than a pointer to some qualified type object.
func callLookup(t testing.TB, pth ygot.PathStruct) (QualifiedValue, error) {
	lookup := reflect.ValueOf(pth).MethodByName("Lookup")
	if lookup.IsNil() {
		return nil, fmt.Errorf("pathstruct %v has no Lookup() method", YgotPathLabel(pth))
	}
	result := lookup.Call(argsT(t))
	if len(result) != 1 {
		return nil, fmt.Errorf("pathstruct %v has wrong Lookup() signature: got %v return values, want 1", YgotPathLabel(pth), len(result))
	}
	if val, ok := result[0].Interface().(QualifiedValue); ok {
		return val, nil
	}
	return nil, fmt.Errorf("pathstruct %v has wrong Lookup() signature: got %v, want a QualifiedValue", YgotPathLabel(pth), result[0])
}

// getVal extracts the value from a QualifiedValue, returning an error if the input isn't present.
// It uses reflection, and will also return an error if the input doesn't have a .Val(t) method.
func getVal(t testing.TB, qVal QualifiedValue) (interface{}, error) {
	if !qVal.IsPresent() {
		return nil, fmt.Errorf("qualified value %v is not present (check this before calling Unpack)", qVal)
	}
	valFn := reflect.ValueOf(qVal).MethodByName("Val")
	if valFn.IsNil() {
		return nil, fmt.Errorf("qualified value %v has no Val(t) method", qVal)
	}
	result := valFn.Call(argsT(t))
	if len(result) != 1 {
		return nil, fmt.Errorf("qualified value %v has wrong Val(t) signature: got %v return values, want 1", qVal, len(result))
	}
	return result[0].Interface(), nil
}

// predicateMaybe calls t.Errorf if the value at the given path doesn't satisfy the given
// predicate. It will also Errorf if the value is missing, unless nilOk is true.
func predicateMaybe(t testing.TB, pth ygot.PathStruct, wantLabel string, predicate func(interface{}) bool, nilOk bool) {
	t.Helper()
	result, err := callLookup(t, pth)
	if err != nil {
		t.Errorf("Failed to introspect pathstruct: %v", err)
		return
	}
	if !result.IsPresent() {
		if !nilOk {
			t.Errorf("%v: no value, want %v.", YgotPathLabel(pth), wantLabel)
		}
		return
	}
	got, err := getVal(t, result)
	if err != nil {
		t.Errorf("%v: cannot extract value from %v (want %v).", YgotPathLabel(pth), result, wantLabel)
	}
	if !predicate(got) {
		t.Errorf("%v: got %[2]T %[2]v, want %v.", YgotPathLabel(pth), got, wantLabel)
	}

}

// Predicate calls t.Errorf if the value at the given path is missing or doesn't satisfy the
// given predicate.
func Predicate(t testing.TB, pth ygot.PathStruct, wantLabel string, predicate func(interface{}) bool) {
	t.Helper()
	predicateMaybe(t, pth, wantLabel, predicate, false)
}

// Value calls t.Errorf the value at the given path is missing or doesn't equal want.
func Value(t testing.TB, pth ygot.PathStruct, want interface{}) {
	t.Helper()
	predicateMaybe(t, pth, fmt.Sprintf("%[1]T %[1]v", want), func(got interface{}) bool {
		return reflect.DeepEqual(got, want)
	}, false)
}

// ValueOrNil calls Errorf if the value at the given path doesn't equal want; it will NOT
// log an error if the value is unset.
func ValueOrNil(t testing.TB, pth ygot.PathStruct, want interface{}) {
	t.Helper()
	predicateMaybe(t, pth, fmt.Sprintf("%[1]T %[1]v", want), func(got interface{}) bool {
		return reflect.DeepEqual(got, want)
	}, true)
}

// NonZero calls Errorf if the value at the given path is missing, is numerically 0, or is not
// a number.
func NonZero(t testing.TB, pth ygot.PathStruct) {
	t.Helper()
	Predicate(t, pth, "!=0", func(got interface{}) bool {
		switch v := reflect.ValueOf(got); v.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return v.Uint() != 0
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return v.Int() != 0
		case reflect.Float32, reflect.Float64:
			return v.Float() != 0
		default:
			return false
		}
	})
}

// Present calls Errorf if the value at a given path is missing.
func Present(t testing.TB, pth ygot.PathStruct) {
	t.Helper()
	Predicate(t, pth, "any value", func(interface{}) bool { return true })
}
