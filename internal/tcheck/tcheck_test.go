/*
Copyright 2022 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tcheck

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/ygot/ygot"
)

// The following types mock various types from generated ondatra telemetry.

// MockQualified behaves like a generated QualifiedSomething type.
type MockQualified[T any] struct {
	val     T
	present bool
}

// Val returns the value of the sample, erroring out if not present.
func (q *MockQualified[T]) Val(t testing.TB) T {
	t.Helper()
	if !q.IsPresent() {
		t.Fatal("No value present")
	}
	return q.val
}

// IsPresent returns true if the qualified struct contains a value.
func (q *MockQualified[T]) IsPresent() bool {
	return q != nil && q.present
}

var _ qualified[string] = (*MockQualified[string])(nil)

// NoMethods mocks a NodePath but is missing the methods that reflection.go
// checks for.
type NoMethods[T any] struct {
	*ygot.NodePath
}

// Get exists only for type inference
func (path *NoMethods[T]) Get(t testing.TB) (x T) {
	return x
}

// BadMethods mocks a NodePath but has the wrong signature on the methods
// we fetch via reflection.
type BadMethods[T any] struct {
	*NoMethods[T]
}

// Lookup has the wrong signature
func (path *BadMethods[T]) Lookup(t testing.TB, x int) string {
	return "This should fail."
}

// Watch has the wrong signature on its predicate
func (path *BadMethods[T]) Watch(t testing.TB, timeout time.Duration, predicate func(*MockQualified[T]) bool) bool {
	return false
}

// MockPathStruct mocks a ygot-generated NodePath.
type MockPathStruct[T any] struct {
	*NoMethods[T]
	value *MockQualified[T]
	// if set, the path will call Fatalf() on Lookup/Watch
	failure bool
	// if set, Lookup() will always return an empty Value, and Watch
	// will act like the value is empty until delay has elapsed.
	delay time.Duration
}

// Lookup mimics the Ondatra .Lookup().
func (path *MockPathStruct[T]) Lookup(t testing.TB) *MockQualified[T] {
	if path.failure {
		t.Fatalf("deliberate fatal in Lookup")
	}
	if path.delay > 0 {
		return &MockQualified[T]{}
	}
	return path.value
}

// Watch mimics the Ondatra .Watch().
func (path *MockPathStruct[T]) Watch(t testing.TB, timeout time.Duration, predicate func(*MockQualified[T]) bool) *ConcreteWatcher[T] {
	if path.failure {
		t.Fatalf("deliberate fatal in Watch")
	}
	value := path.value
	if path.delay > timeout {
		value = &MockQualified[T]{}
	}
	return &ConcreteWatcher[T]{
		value: value,
		ok:    predicate(value),
	}
}

// WithValue sets the return value of this path to Value[T]{v}
func (path *MockPathStruct[T]) WithValue(v T) *MockPathStruct[T] {
	path.value = &MockQualified[T]{
		val:     v,
		present: true,
	}
	return path
}

// WithFailing causes future Lookup()/Watch() calls to call t.Fatalf().
func (path *MockPathStruct[T]) WithFailing(v bool) *MockPathStruct[T] {
	path.failure = v
	return path
}

// WithDelay causes this path to return no value on Lookup() or on a Watch()
// for less than the specified delay.
func (path *MockPathStruct[T]) WithDelay(delay time.Duration) *MockPathStruct[T] {
	path.delay = delay
	return path
}

var _ PathStruct[int] = (*MockPathStruct[int])(nil)

// ConcreteWatcher mimics a ygot <Type>Watcher (but never actually delays).
type ConcreteWatcher[T any] struct {
	value *MockQualified[T]
	ok    bool
}

func (w *ConcreteWatcher[T]) String() string {
	return fmt.Sprintf("ConcreteWatcher(%v, %v)", w.value, w.ok)
}

func (w *ConcreteWatcher[T]) Await(t testing.TB) (*MockQualified[T], bool) {
	if w.ok {
		return w.value, true
	}
	return w.value, false
}

// NewPath generates a fake path struct with no value
func NewPath[T any](path string) *MockPathStruct[T] {
	return &MockPathStruct[T]{
		NoMethods: NoMethodsPath[T](path),
	}
}

// NoMethodsPath generates a PathStruct[T] that's missing member functions.
func NoMethodsPath[T any](path string) *NoMethods[T] {
	return &NoMethods[T]{
		NodePath: ygot.NewNodePath([]string{path}, map[string]interface{}{}, ygot.NewDeviceRootBase("root")),
	}
}

// BadMethodsPath generates a PathStruct[T] with malformed member functions.
func BadMethodsPath[T any](path string) *BadMethods[T] {
	return &BadMethods[T]{
		NoMethodsPath[T](path),
	}
}

func HostName() *MockPathStruct[string] {
	return NewPath[string]("system/hostname")
}

func TestCheck(t *testing.T) {
	testCases := []struct {
		desc      string
		validator Validator
		wantErr   []string
	}{{
		desc:      "Equal/Correct",
		validator: Equal(HostName().WithValue("thehost"), "thehost"),
	}, {
		desc:      "Equal/Incorrect",
		wantErr:   []string{"/system/hostname", "wronghost", "thehost"},
		validator: Equal(HostName().WithValue("wronghost"), "thehost"),
	}, {
		desc:      "Equal/Missing",
		wantErr:   []string{"/system/hostname", "thehost"},
		validator: Equal(HostName(), "thehost"),
	}, {
		desc:      "NotEqual/Correct",
		validator: NotEqual(HostName().WithValue("thehost"), "notthehost"),
	}, {
		desc:      "NotEqual/Incorrect",
		wantErr:   []string{"/system/hostname", "thehost"},
		validator: NotEqual(HostName().WithValue("thehost"), "thehost"),
	}, {
		desc:      "EqualOrNil/Correct",
		validator: EqualOrNil(HostName().WithValue("thehost"), "thehost"),
	}, {
		desc:      "EqualOrNil/Nil",
		validator: EqualOrNil(HostName(), "thehost"),
	}, {
		desc:      "EqualOrNil/Incorrect",
		wantErr:   []string{"/system/hostname", "notthehost"},
		validator: EqualOrNil(HostName().WithValue("notthehost"), "thehost"),
	}, {
		desc:      "Present/Correct",
		validator: Present(HostName().WithValue("thehost")),
	}, {
		desc:      "Present/Incorrect",
		wantErr:   []string{"/system/hostname"},
		validator: Present(HostName()),
	}, {
		desc:      "NotPresent/Correct",
		validator: NotPresent(HostName()),
	}, {
		desc:      "NotPresent/Incorrect",
		wantErr:   []string{"/system/hostname"},
		validator: NotPresent(HostName().WithValue("thehost")),
	}, {
		desc:      "PathStruct without methods",
		wantErr:   []string{"reflection error"},
		validator: Equal(NoMethodsPath[string]("system/hostname"), "thehost"),
	}, {
		desc:      "PathStruct with bad methods",
		wantErr:   []string{"reflection error"},
		validator: Equal(BadMethodsPath[string]("system/hostname"), "thehost"),
	}, {
		desc:      "Fatal Lookup",
		wantErr:   []string{"deliberate fatal in Lookup"},
		validator: Equal(HostName().WithFailing(true), "thehost"),
	}}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotErr := tc.validator.Check(t)
			if gotErr != nil {
				if len(tc.wantErr) == 0 {
					t.Errorf("Unexpected error: %v", gotErr)
				} else {
					for _, want := range tc.wantErr {
						if !strings.Contains(gotErr.Error(), want) {
							t.Errorf("error %v is missing substring %#v", gotErr, want)
						}
					}
				}
			} else if len(tc.wantErr) > 0 {
				t.Errorf("Got no error, want an error containing:\n  %v", tc.wantErr)
			}
		})
	}
}

func TestAwait(t *testing.T) {
	testCases := []struct {
		desc      string
		validator Validator
		wantErr   []string
	}{{
		desc:      "Correct Value",
		validator: Equal(HostName().WithValue("thehost"), "thehost"),
	}, {
		desc:      "Wrong Value",
		wantErr:   []string{"/system/hostname", "wronghost", "thehost"},
		validator: Equal(HostName().WithValue("wronghost"), "thehost"),
	}, {
		desc:      "Delayed Value/Correct",
		validator: Equal(HostName().WithValue("thehost").WithDelay(time.Nanosecond), "thehost"),
	}, {
		desc:      "Delayed Value/Too Slow",
		wantErr:   []string{"/system/hostname", "thehost"},
		validator: Equal(HostName().WithValue("thehost").WithDelay(time.Second*10), "thehost"),
	}, {
		desc:      "Present/Correct",
		validator: Present(HostName().WithValue("thehost")),
	}, {
		desc:      "Present/Incorrect",
		wantErr:   []string{"/system/hostname"},
		validator: Present(HostName()),
	}, {
		desc:      "PathStruct without methods",
		wantErr:   []string{"reflection error"},
		validator: Equal(NoMethodsPath[string]("system/hostname"), "thehost"),
	}, {
		desc:      "PathStruct with bad methods",
		wantErr:   []string{"reflection error"},
		validator: Equal(BadMethodsPath[string]("system/hostname"), "thehost"),
	}, {
		desc:      "Fatal Watch",
		wantErr:   []string{"deliberate fatal in Watch"},
		validator: Equal(HostName().WithFailing(true), "thehost"),
	}}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			for _, gotErr := range []error{
				tc.validator.Await(t, time.Millisecond),
				tc.validator.AwaitUntil(t, time.Now().Add(time.Millisecond)),
			} {
				if gotErr != nil {
					if len(tc.wantErr) == 0 {
						t.Errorf("Unexpected error: %v", gotErr)
					} else {
						for _, want := range tc.wantErr {
							if !strings.Contains(gotErr.Error(), want) {
								t.Errorf("error %v is missing substring %#v", gotErr, want)
							}
						}
					}
				} else if len(tc.wantErr) > 0 {
					t.Errorf("Got no error, want an error containing:\n  %v", tc.wantErr)
				}
			}
		})
	}
}

func TestValidation(t *testing.T) {
	path := NewPath[int]("x/y/z").WithValue(1)
	vd := Equal(path, 1)
	if got, want := vd.Path(), "/x/y/z"; got != want {
		t.Errorf("Equal(x/y/z, 1).Path(): got %#v, want %#v", got, want)
	}
	if got, want := vd.RelPath(NewPath[string]("x")), "y/z"; got != want {
		t.Errorf("Equal(x/y/z, 1).RelPath(x): got %#v, want %#v", got, want)
	}
	if got, want := vd.RelPath(NewPath[string]("x/y/z/a/b")), "../.."; got != want {
		t.Errorf("Equal(x/y/z, 1).RelPath(x/y/z/a/b): got %#v, want %#v", got, want)
	}
}

func TestPresentValidation(t *testing.T) {
	path := NewPath[int]("x/y/z")
	vd := Present(path)
	if got, want := vd.Path(), "/x/y/z"; got != want {
		t.Errorf("Present(x/y/z).Path(): got %#v, want %#v", got, want)
	}
	if got, want := vd.RelPath(NewPath[string]("x")), "y/z"; got != want {
		t.Errorf("Present(x/y/z).RelPath(x): got %#v, want %#v", got, want)
	}
	if got, want := vd.RelPath(NewPath[string]("x/y/z/a/b")), "../.."; got != want {
		t.Errorf("Present(x/y/z).RelPath(x/y/z/a/b): got %#v, want %#v", got, want)
	}
}
