// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package check_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/check"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/testing/fake/gnmi"
	fpb "github.com/openconfig/gnmi/testing/fake/proto"
	"github.com/openconfig/ygnmi/exampleoc/exampleocpath"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	childTwo          = exampleocpath.Root().Parent().Child().Two()
	childTwoStatePath = "/parent/child/state/two"
)

// fakeGNMI provides a client connected to a fake gNMI agent, as well as
// methods to stub data in it.
type fakeGNMI struct {
	Agent        *gnmi.Agent
	Client       *ygnmi.Client
	gen          *fpb.FixedGenerator
	childTwoPath *gpb.Path
}

func newFakeGNMI(ctx context.Context) (*fakeGNMI, error) {
	gChildTwo, _, err := ygnmi.ResolvePath(childTwo)
	if err != nil {
		return nil, fmt.Errorf("Resolving OC path: %w", err)
	}

	var responses []*gpb.SubscribeResponse
	gen := &fpb.FixedGenerator{
		Responses: responses,
	}
	config := &fpb.Config{
		Generator:   &fpb.Config_Fixed{Fixed: gen},
		EnableDelay: true, // Respect timestamps if present.
	}
	agent, err := gnmi.New(config, nil)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.DialContext(ctx, agent.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("DialContext(%s): %w", agent.Address(), err)
	}

	client, err := ygnmi.NewClient(gpb.NewGNMIClient(conn))
	if err != nil {
		return nil, err
	}

	return &fakeGNMI{
		Agent:        agent,
		Client:       client,
		gen:          gen,
		childTwoPath: gChildTwo,
	}, nil
}

func mustNewFakeGNMI(ctx context.Context, t *testing.T) (*fakeGNMI, *ygnmi.Client) {
	fake, err := newFakeGNMI(ctx)
	if err != nil {
		t.Fatalf("Creating fake gNMI: %v", err)
	}
	return fake, fake.Client
}

func (fg *fakeGNMI) Close() {
	fg.Agent.Close()
}

// update represents a (value, delay) pair that can turn into an update
// notification. A value of "" indicates no value.
type update struct {
	value string
	delay time.Duration
}

// stubChildTwo clears the fakeGNMI's stub and populates it with updates to
// parent/child/state/two based on the given updates.
func (fg *fakeGNMI) stubChildTwo(updates ...update) {
	fg.gen.Reset()
	for _, u := range updates {
		if u.value != "" {
			fg.gen.Responses = append(fg.gen.Responses, &gpb.SubscribeResponse{
				Response: &gpb.SubscribeResponse_Update{
					Update: &gpb.Notification{
						Timestamp: int64(u.delay),
						Update: []*gpb.Update{
							{
								Path: fg.childTwoPath,
								Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: u.value}},
							},
						},
					},
				},
			})
			fg.gen.Responses = append(fg.gen.Responses, &gpb.SubscribeResponse{
				Response: &gpb.SubscribeResponse_SyncResponse{
					SyncResponse: true,
				},
			})
		}
	}
}

// errContainsAll returns an error unless the string form of got contains every
// string in want.
func errContainsAll(got error, want []string) error {
	if got == nil {
		return fmt.Errorf("nil error, want error containing all of %v", want)
	}
	var errs []string
	msg := got.Error()
	for _, w := range want {
		if !strings.Contains(msg, w) {
			errs = append(errs, fmt.Sprintf("Missing substring %#v", w))
		}
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("Error [%v]: %v", got, errs[0])
	default:
		return fmt.Errorf("Error [%v]:\n  %v", got, strings.Join(errs, "\n  "))
	}
}

func TestCheck(t *testing.T) {
	fakeGNMI, c := mustNewFakeGNMI(context.Background(), t)
	defer fakeGNMI.Close()
	query := childTwo.State()
	testCases := []struct {
		desc      string
		validator check.Validator
		value     string
		// if set, the validator should return a Failed with the correct value and
		// query, and this string should be present in the error message.
		errIncludes []string
	}{{
		desc:      "Equal/Correct",
		validator: check.Equal(query, "correct"),
		value:     "correct",
	}, {
		desc:        "Equal/Incorrect",
		validator:   check.Equal(query, "correct"),
		value:       "wrong",
		errIncludes: []string{childTwoStatePath, "correct", "wrong"},
	}, {
		desc:        "Equal/Missing",
		validator:   check.Equal(query, "correct"),
		errIncludes: []string{childTwoStatePath, "correct", "no value"},
	}, {
		desc:      "NotEqual/Correct",
		validator: check.NotEqual(query, "wrong"),
		value:     "correct",
	}, {
		desc:        "NotEqual/Incorrect",
		validator:   check.NotEqual(query, "wrong"),
		value:       "wrong",
		errIncludes: []string{childTwoStatePath, "wrong"},
	}, {
		desc:      "EqualOrNil/Correct",
		validator: check.EqualOrNil(query, "correct"),
		value:     "correct",
	}, {
		desc:      "EqualOrNil/Nil",
		validator: check.EqualOrNil(query, "correct"),
	}, {
		desc:        "EqualOrNil/Incorrect",
		validator:   check.EqualOrNil(query, "correct"),
		value:       "wrong",
		errIncludes: []string{childTwoStatePath, "correct", "wrong"},
	}, {
		desc:      "Present/Correct",
		value:     "correct",
		validator: check.Present[string](query),
	}, {
		desc:        "Present/Incorrect",
		validator:   check.Present[string](query),
		errIncludes: []string{childTwoStatePath, "any value", "no value"},
	}, {
		desc:      "NotPresent/Correct",
		validator: check.NotPresent[string](query),
	}, {
		desc:        "NotPresent/Incorrect",
		value:       "correct",
		validator:   check.NotPresent[string](query),
		errIncludes: []string{childTwoStatePath, "no value", "correct"},
	}}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			fakeGNMI.stubChildTwo(update{tc.value, 0})
			gotErr := tc.validator.Check(c)
			if len(tc.errIncludes) > 0 {
				if err := errContainsAll(gotErr, tc.errIncludes); err != nil {
					t.Error(err)
				}
			} else if gotErr != nil {
				t.Errorf("Unexpected error: %v", gotErr)
			}
		})
	}
}

func TestAwait(t *testing.T) {
	fakeGNMI, c := mustNewFakeGNMI(context.Background(), t)
	defer fakeGNMI.Close()
	query := childTwo.State()
	testCases := []struct {
		desc        string
		validator   check.Validator
		updates     []update
		errIncludes []string
	}{{
		desc:      "Immediately correct",
		validator: check.Equal(query, "correct"),
		updates:   []update{{"correct", 0}},
	}, {
		desc:      "Delayed correct",
		validator: check.Equal(query, "correct"),
		updates:   []update{{"wrong", 0}, {"correct", 0}},
	}, {
		desc:        "Too slow",
		validator:   check.Equal(query, "correct"),
		updates:     []update{{"wrong", 0}, {"wrong", 1}, {"wrong", 2}, {"correct", time.Hour}},
		errIncludes: []string{childTwoStatePath, "wrong", "correct", "deadline"},
	}, {
		desc:        "Too slow/multiple values",
		validator:   check.Equal(query, "correct"),
		updates:     []update{{"wrong1", 0}, {"wrong2", 0}, {"correct", time.Hour}},
		errIncludes: []string{childTwoStatePath, "wrong2", "correct", "deadline"},
	}, {
		desc:        "EOF before any value",
		validator:   check.Equal(query, "correct"),
		errIncludes: []string{childTwoStatePath, "EOF"},
	}}
	for _, tc := range testCases {
		t.Run(tc.desc+"/AwaitFor", func(t *testing.T) {
			fakeGNMI.stubChildTwo(tc.updates...)
			gotErr := tc.validator.AwaitFor(time.Millisecond*50, c)
			if len(tc.errIncludes) > 0 {
				if err := errContainsAll(gotErr, tc.errIncludes); err != nil {
					t.Error(err)
				}
			} else if gotErr != nil {
				t.Errorf("Unexpected error: %v", gotErr)
			}
		})
		t.Run(tc.desc+"/AwaitUntil", func(t *testing.T) {
			fakeGNMI.stubChildTwo(tc.updates...)
			gotErr := tc.validator.AwaitUntil(time.Now().Add(time.Millisecond*50), c)
			if len(tc.errIncludes) > 0 {
				if err := errContainsAll(gotErr, tc.errIncludes); err != nil {
					t.Error(err)
				}
			} else if gotErr != nil {
				t.Errorf("Unexpected error: %v", gotErr)
			}
		})
	}
}

func TestContext(t *testing.T) {
	ctx := context.Background()
	fakeGNMI, c := mustNewFakeGNMI(ctx, t)
	defer fakeGNMI.Close()
	query := childTwo.State()
	vd := check.Equal(query, "correct")
	fakeGNMI.stubChildTwo(update{"correct", 0})
	t.Run("Canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		err := vd.Await(ctx, c)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
	t.Run("Past context", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Hour))
		defer cancel()
		err := vd.Await(ctx, c)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
	t.Run("AwaitFor/past", func(t *testing.T) {
		err := vd.AwaitFor(-time.Second, c)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
	t.Run("AwaitUntil/past", func(t *testing.T) {
		err := vd.AwaitUntil(time.Now().Add(time.Hour*-10), c)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}

func TestNetworkErrors(t *testing.T) {
	fakeGNMI, c := mustNewFakeGNMI(context.Background(), t)
	fakeGNMI.Close()
	vd := check.Equal(childTwo.State(), "foo")
	if err := vd.Check(c); err != nil {
		if err := errContainsAll(err, []string{childTwoStatePath, "rpc error"}); err != nil {
			t.Error(err)
		}
	} else {
		t.Errorf("Expected error from validation against closed gNMI client.")
	}
}

func TestFormatValue(t *testing.T) {
	x := &ygnmi.Value[string]{}
	x.SetVal("x")
	for _, tc := range []struct {
		val  *ygnmi.Value[string]
		want string
	}{{
		val:  x,
		want: `"x"`,
	}, {
		val:  &ygnmi.Value[string]{},
		want: "no value",
	}, {
		val:  nil,
		want: "nil",
	}} {
		if got, want := check.FormatValue(tc.val), tc.want; got != want {
			t.Errorf("FormatValue(%v): got %#v, want %#v", tc.val, got, want)
		}
	}
}

func TestFormatPath(t *testing.T) {
	root := exampleocpath.Root()
	if got, want := check.FormatPath(root.Parent()), "/parent"; got != want {
		t.Errorf("FormatPath(): got %#v, want %#v", got, want)
	}
	base := root.Parent().Child()
	for _, tc := range []struct {
		path       ygnmi.PathStruct
		name, want string
	}{{
		path: base.One(),
		name: "/parent/child/*/one",
		want: "*/one",
	}, {
		path: base.One().State().PathStruct(),
		name: "/parent/child/state/one",
		want: "state/one",
	}, {
		path: base.One().Config().PathStruct(),
		name: "/parent/child/config/one",
		want: "config/one",
	}, {
		path: root.A().B().C(),
		name: "/a/b/c",
		want: "../../a/b/c",
	}, {
		path: nil,
		name: "nil path",
		want: "<nil path>",
	},
	} {
		if got := check.FormatRelativePath(base, tc.path); got != tc.want {
			t.Errorf("FormatRelativePath(parent/child, %v): got %#v, want %#v", tc.name, got, tc.want)
		}
	}

	vd := check.Equal(root.Parent().Child().Two().State(), "ignored")
	if got, want := vd.Path(), "/parent/child/state/two"; got != want {
		t.Errorf("vd.Path(): got %#v, want %#v", got, want)
	}
	if got, want := vd.RelPath(root.Parent().Child()), "state/two"; got != want {
		t.Errorf("vd.RelPath(): got %#v, want %#v", got, want)
	}
	if got, want := vd.RelPath(root.Parent().Child().Four()), "../../state/two"; got != want {
		t.Errorf("Equal(x/y/z, 1).RelPath(x/y/z/a/b): got %#v, want %#v", got, want)
	}
}
