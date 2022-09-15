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

/*
Ondatra predates go generics, and the current telemetry model library handles
this via code generation, producing roughly 1600 copies of each type and
method for all the different possible node types in the ygot tree.

This file defines generic versions of the generated functions needed by this
package, using reflection to mimic proper generic typing.

For example, a generated node path whose value type is int will have
.Lookup(t) which returns a *QualifiedInt that represents either an int or
an indicator that no value is present. A similar path with a string value will
have its own .Lookup(t) that returns *QualifiedString, which is identical
to *QualifiedInt except for the type returned by the .Val(t) method. This
pattern repeats hundreds of times for different return types (including every
enum type defined by the yang model).

The callLookup[T](t, path) method defined in this file uses reflection to
find the Lookup method on path, call it, and extract the data from the
resulting *QualifiedT into a generic Value[T].

The call functions in this file will return errors (instead of panicking) if
their arguments don't have appropriately-typed methods.

We plan to implement a second package exposing a similar interface to tcheck
but handling ygnmi queries, which is the new standard slowly being rolled out;
it will be able to omit this library because it will operate on types that are
already generic.
*/

package tcheck

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

// qualified is an interface that all *QualifiedFoo types satisfy.
type qualified[T any] interface {
	IsPresent() bool
	Val(testing.TB) T
}

// watcher is an interface that all *FooWatcher types satisfy.
// The only method they have in common is .Watch(), which returns a different
// *SomeTypeWatcher for each one and so can't be expressed in go generics.
// This type exists solely to help typecheck this module itself.
type watcher[T any] interface{}

// value implements Value[T].
type value[T any] struct {
	val     T
	present bool
}

// Val returns the val and whether it is present.
func (v *value[T]) Val() (T, bool) {
	if v == nil {
		var t T
		return t, false
	}
	return v.val, v.present
}

// IsPresent returns whether the value is present.
func (v *value[T]) IsPresent() bool {
	if v == nil {
		return false
	}
	return v.present
}

// valuesOf converts a list of objects to reflect.Values. Any items that are
// already reflect.Values will be unchanged.
func valuesOf(args ...any) (out []reflect.Value) {
	for _, arg := range args {
		switch arg := arg.(type) {
		case reflect.Value:
			out = append(out, arg)
		default:
			out = append(out, reflect.ValueOf(arg))
		}
	}
	return out
}

// isCovariant returns true if any of the following are true:
// 1. t1 == t2
// 2. t2 is an interface and t1 implements t2
// 3. t1 and t2 are functions whose inputs and outputs satisfy isCovariant
// This allows us to test e.g. that func(x *oc.QualifiedInt) *oc.IntWatcher
// can be wrapped to make a func(x Qualified[int]) watcher[int].
// Note that we do NOT descend into other compound types - e.g. a
// map[string]*oc.QualifiedInt will not match map[string]Qualified[int]
func isCovariant(t1, t2 reflect.Type) bool {
	if t1 == t2 {
		// exact same type
		return true
	}
	switch t2.Kind() {
	case reflect.Interface:
		// any type plus an interface that that type implements
		return t1.Implements(t2)
	case reflect.Func:
		if t1.Kind() != reflect.Func {
			return false
		}
		if t1.NumIn() != t2.NumIn() || t1.NumOut() != t2.NumOut() {
			return false
		}
		for i := 0; i < t1.NumIn(); i++ {
			if !isCovariant(t1.In(i), t2.In(i)) {
				return false
			}
		}
		for i := 0; i < t1.NumOut(); i++ {
			if !isCovariant(t1.Out(i), t2.Out(i)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// methodByName fetches a method by name from an arbitrary object, returning
// an error if there is no method by that name OR if the method doesn't pass
// isCovariant on the parameter type.
func methodByName[Sig any](obj any, name string) (reflect.Value, error) {
	fn := reflect.ValueOf(obj).MethodByName(name)
	if !fn.IsValid() {
		return fn, fmt.Errorf("reflection error: %T has no %s() method", obj, name)
	}
	sigType := reflect.TypeOf((*Sig)(nil)).Elem()
	if !isCovariant(fn.Type(), sigType) {
		return fn, fmt.Errorf("reflection error: %T has incompatible %s() method: want %v, got %v", obj, name, sigType, fn.Type())
	}
	return fn, nil
}

// extractValue converts a *QualifiedFoo to a Value[Foo].
// It does not do internal typechecking - if val is not some qualified type,
// this function will panic.
func extractValue[T any](t testing.TB, val reflect.Value) Value[T] {
	t.Helper()
	qv := val.Interface().(qualified[T])
	if !qv.IsPresent() {
		return &value[T]{present: false}
	}
	return &value[T]{
		val:     qv.Val(t),
		present: qv.IsPresent(),
	}
}

// callFn calls fn on (t, args...), capturing any t.Fatal calls and
// converting them into normal errors.
func callFn(t testing.TB, fn reflect.Value, args ...any) (result []reflect.Value, err error) {
	t.Helper()
	errStr := testt.CaptureFatal(t, func(t testing.TB) {
		result = fn.Call(valuesOf(append([]any{t}, args...)...))
	})
	if errStr != nil {
		return nil, fmt.Errorf(*errStr)
	}
	return result, nil
}

// callLookup(t, path) returns the result of path.Lookup(t) as a Value[T].
// The path must be a PathStruct[T] that also has a Lookup(testing.TB) method
// which returns something convertible to a qualified[T]; every path struct in
// Ondatra's generated telemetry models has such a method. We can't express
// this constraint through the PathStruct interface because of the way Go's
// type system handles function return types, so we check it manually instead.
func callLookup[T any](t testing.TB, path PathStruct[T]) (Value[T], error) {
	t.Helper()
	lookup, err := methodByName[func(testing.TB) qualified[T]](path, "Lookup")
	if err != nil {
		return nil, err
	}
	result, err := callFn(t, lookup)
	if err != nil {
		return nil, err
	}
	// Lookup must return a single value convertible to qualified[T] or else
	// the call to methodByName would have failed, so this is safe.
	v := extractValue[T](t, result[0])
	return v, nil
}

// callAwait returns the result of watcher.Await(t) as a Value[T].
// The watcher must have a .Await(testing.TB) (QT, bool) for some QT that
// implements qualified[T] (all Ondatra generated SomeTypeWatcher types do).
func callAwait[T any](t testing.TB, obj watcher[T]) (Value[T], bool, error) {
	t.Helper()
	await, err := methodByName[func(testing.TB) (qualified[T], bool)](obj, "Await")
	if err != nil {
		return nil, false, err
	}
	result, err := callFn(t, await)
	if err != nil {
		return nil, false, err
	}
	// Await must return a pair (qt, bool) where qt is convertible to
	// qualified[T] or else the call to methodByName would have failed, so this
	// is safe.
	v := extractValue[T](t, result[0])
	return v, result[1].Bool(), nil
}

// callWatch is path.Watch(t, timeout, predicate), except that
//  1. predicate takes a Value[T] instead of a *QualifiedT, and
//  2. the returned watcher will be an ErrorWatcher.
func callWatch[T any](t testing.TB, timeout time.Duration, path PathStruct[T], predicate func(Value[T]) bool) (watcher[T], error) {
	t.Helper()
	// Extract the Watch() method from whatever type we're using
	watch, err := methodByName[func(testing.TB, time.Duration, func(qualified[T]) bool) watcher[T]](path, "Watch")
	if err != nil {
		return nil, err
	}
	// Construct an appropriately typed function. Go doesn't support argument
	// covariance, so we can't use a func(Value[T]) bool as a func
	// (*QualifiedT) bool; instead we convert it into a func (*QualifiedT) bool
	// via MakeFunc.
	argpred := reflect.MakeFunc(watch.Type().In(2), func(args []reflect.Value) []reflect.Value {
		val := extractValue[T](t, args[0])
		return []reflect.Value{reflect.ValueOf(predicate(val))}
	})
	// Invoke the fetched Watch method on the properly wrapped args
	result, err := callFn(t, watch, timeout, argpred)
	if err != nil {
		return nil, err
	}
	// Wrap the returned TypeWatcher in the common interface
	return result[0].Interface().(watcher[T]), nil
}

// typelessQualified is a non-generic interface satisfied by *QualifiedFoo objects.
// We need this for the Present() check to work without type annotations.
type typelessQualified interface {
	IsPresent() bool
}

// extractValueAny calls .Val(t) on a typelessQualified and returns the result
// as a Value[any].
func extractValueAny(t testing.TB, rval reflect.Value) (Value[any], error) {
	qv, ok := rval.Interface().(typelessQualified)
	if !ok {
		return nil, fmt.Errorf("called extractValueAny on non-qualified type %T", rval.Interface())
	}
	if !qv.IsPresent() {
		return &value[any]{present: false}, nil
	}
	val, err := callFn(t, reflect.ValueOf(qv).MethodByName("Val"))
	if err != nil {
		return nil, err
	}
	return &value[any]{
		val:     val[0].Interface(),
		present: true,
	}, nil
}

// callLookupAny is callLookup without type information; the value will be a
// Value[any].
func callLookupAny(t testing.TB, path ygot.PathStruct) (Value[any], error) {
	t.Helper()
	lookup, err := methodByName[func(testing.TB) typelessQualified](path, "Lookup")
	if err != nil {
		return nil, err
	}
	result, err := callFn(t, lookup)
	if err != nil {
		return nil, err
	}
	return extractValueAny(t, result[0])
}

// callAwaitAny is callAwait without type information; the value will be a
// Value[any].
func callAwaitAny(t testing.TB, obj watcher[any]) (Value[any], bool, error) {
	await, err := methodByName[func(testing.TB) (typelessQualified, bool)](obj, "Await")
	if err != nil {
		return nil, false, err
	}
	result, err := callFn(t, await)
	if err != nil {
		return nil, false, err
	}
	v, err := extractValueAny(t, result[0])
	if err != nil {
		return nil, false, err
	}
	return v, result[1].Bool(), nil
}

// callWatchAny is callWatch without type information; the predicate takes a
// Value[any] and the returned watcher will be a watcher[any].
func callWatchAny(t testing.TB, timeout time.Duration, path ygot.PathStruct, predicate func(Value[any]) bool) (watcher[any], error) {
	t.Helper()
	// Extract the Watch() method from whatever type we're using
	watch, err := methodByName[func(testing.TB, time.Duration, func(typelessQualified) bool) watcher[any]](path, "Watch")
	if err != nil {
		return nil, err
	}
	// Construct an appropriately typed function. Go doesn't support argument
	// covariance, so we can't use a func(Value[T]) bool as a func
	// (*QualifiedT) bool; instead we convert it into a func (*QualifiedT) bool
	// via MakeFunc.
	argpred := reflect.MakeFunc(watch.Type().In(2), func(args []reflect.Value) []reflect.Value {
		val, err := extractValueAny(t, args[0])
		if err != nil {
			panic(err)
		}
		return []reflect.Value{reflect.ValueOf(predicate(val))}
	})
	// Invoke the fetched Watch method on the properly wrapped args
	result, err := callFn(t, watch, timeout, argpred)
	if err != nil {
		return nil, err
	}
	// Wrap the returned TypeWatcher in the common interface
	return result[0].Interface().(watcher[any]), nil
}
