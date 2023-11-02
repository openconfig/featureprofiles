// Copyright 2019 Google Inc.
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

// Package gnmiutil implements helpers used for monioring oc leafs. The code is adapted fron Ondatra code.
package gnmiutil

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	log "github.com/golang/glog"
	closer "github.com/openconfig/gocloser"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/util"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

// DataPoint is a value of a gNMI path at a particular time.
type DataPoint struct {
	Path *gpb.Path
	// value of the data; nil means delete
	Value *gpb.TypedValue
	// time the value was updated on the device
	Timestamp time.Time
	// time the update was received by the test
	RecvTimestamp time.Time
	// Sync indicates whether the received datapoint was gNMI sync response.
	Sync bool
}

// Consumer is an interface that should be implemented by entities recieving steraming events.
type Consumer interface {
	Process(datapoints []*DataPoint)
}

type requestOpts struct {
	subMode         gpb.SubscriptionMode
	client          gpb.GNMIClient
	useGetForConfig bool
	md              metadata.MD
}

func (d *DataPoint) String() string {
	return fmt.Sprintf(`Value: %s
Timestamp: %v
RecvTimestamp: %v
Path: %s`, prototext.Format(d.Value), d.Timestamp, d.RecvTimestamp, prototext.Format(d.Path))
}

const (
	// DefaultClientKey is the key for the default client in the customData map.
	DefaultClientKey = "defaultclient"
)

// QualifiedTypeString renders a Qualified* telemetry value to a format that's
// easier to read when debugging.
func QualifiedTypeString(value interface{}, md *Metadata) string {
	// Get string for value
	var valStr string
	if v, ok := value.(ygot.GoStruct); ok && !reflect.ValueOf(v).IsNil() {
		// Display JSON for GoStructs.
		var err error
		valStr, err = ygot.EmitJSON(v, &ygot.EmitJSONConfig{
			Format: ygot.RFC7951,
			Indent: "  ",
			RFC7951Config: &ygot.RFC7951JSONConfig{
				AppendModuleName: true,
			},
		})
		if err != nil {
			valStr = fmt.Sprintf("Displaying normally as cannot marshal to JSON (%v):\n%v", err, v)
		}
		// Add a blank line for JSON output.
		valStr = "\n" + valStr
	} else {
		// TODO: Decide if value presence should be inferred by checking zero value
		if val := reflect.ValueOf(value); val.IsValid() && !val.IsZero() {
			valStr = fmt.Sprintf("%v (present)", value)
		} else {
			valStr = fmt.Sprintf("%v (not present)", value)
		}
	}

	// Get string for path
	pathStr, err := ygot.PathToString(md.Path)
	if err != nil {
		b, _ := prototext.Marshal(md.Path)
		pathStr = string(b)
	} else {
		pathStr = fmt.Sprintf("target: %v, path: %s", md.Path.GetTarget(), pathStr)
	}

	return fmt.Sprintf(`
Timestamp: %v
RecvTimestamp: %v
%s
Val: %s
%s
`, md.Timestamp, md.RecvTimestamp, pathStr, valStr, md.ComplianceErrors)
}

// TelemetryError stores the path, value, and error string from unsuccessfully
// unmarshalling a datapoint into a YANG schema.
type TelemetryError struct {
	Path  *gpb.Path
	Value *gpb.TypedValue
	Err   error
}

func (t *TelemetryError) String() string {
	if t == nil {
		return ""
	}
	return fmt.Sprintf("Unmarshal %v into %v: %s", t.Value, t.Path, t.Err.Error())
}

// ComplianceErrors contains the compliance errors encountered from an Unmarshal operation.
type ComplianceErrors struct {
	// PathErrors are compliance errors encountered due to an invalid schema path.
	PathErrors []*TelemetryError
	// TypeErrors are compliance errors encountered due to an invalid type.
	TypeErrors []*TelemetryError
	// ValidateErrors are compliance errors encountered while doing schema
	// validation on the unmarshalled data.
	ValidateErrors []error
}

// pathToString returns a string version of the input path for display during
// debugging.
func pathToString(path *gpb.Path) string {
	pathStr, err := ygot.PathToString(path)
	if err != nil {
		// Use Sprint instead of prototext.Format to avoid newlines.
		pathStr = fmt.Sprint(path)
	}
	return pathStr
}

// PathStructToString returns a string representing the path struct.
// Note: the output may contain an error message or invalid path;
// do not use this func outside of the generated code.
func PathStructToString(ps ygnmi.PathStruct) string {
	p, _, err := ygnmi.ResolvePath(ps)
	if err != nil {
		return fmt.Sprintf("unknown path: %v", err)
	}
	return pathToString(p)
}

// Metadata contains to common fields and method for the generated Qualified structs.
type Metadata struct {
	Path             *gpb.Path         // Path is the sample's YANG path.
	Config           bool              // Config determines whether the query path was a config query (as opposed to a state query).
	Timestamp        time.Time         // Timestamp is the sample time.
	RecvTimestamp    time.Time         // Timestamp is the time the test received the sample.
	ComplianceErrors *ComplianceErrors // ComplianceErrors contains the compliance errors encountered from an Unmarshal operation.
}

// GetPath returns the YANG query path for this value.
func (q *Metadata) GetPath() *gpb.Path {
	return q.Path
}

// IsConfig returns whether the query path was a config query (as opposed to a state query).
func (q *Metadata) IsConfig() bool {
	return q.Config
}

// GetTimestamp returns the latest notification timestamp.
func (q *Metadata) GetTimestamp() time.Time {
	return q.Timestamp
}

// GetRecvTimestamp returns the latest timestamp when notification(s) were received.
func (q *Metadata) GetRecvTimestamp() time.Time {
	return q.RecvTimestamp
}

// GetComplianceErrors returns the schema compliance errors encountered while unmarshalling and validating the received data.
func (q *Metadata) GetComplianceErrors() *ComplianceErrors {
	return q.ComplianceErrors
}

// QualifiedValue is an interface for generated telemetry types.
type QualifiedValue interface {
	GetPath() *gpb.Path
	IsConfig() bool
	GetRecvTimestamp() time.Time
	GetTimestamp() time.Time
	GetComplianceErrors() *ComplianceErrors
}

// Watch starts a gNMI subscription for the provided duration. Specifying subPaths is optional, if unset will subscribe to the path at n.
// Note: For leaves the converter and predicate are evaluated once per DataPoint. For non-leaves, they are evaluated once per notification,
// after the first sync is received.
func Watch(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, n ygnmi.PathStruct, paths []*gpb.Path, isLeaf bool, consumer Consumer, mode gpb.SubscriptionList_Mode) (_ *Watcher, _ *gpb.Path, rerr error) {
	//cancel := func() {}
	//mode := gpb.SubscriptionList_ONCE
	/*collectEnd := time.Now().Add(duration)
	if time.Now().Before(collectEnd) {
		ctx, cancel = context.WithDeadline(ctx, collectEnd)
		// Only cancel the context in this function if there is an error;
		// otherwise it is up to the asynchronous go routine to cancel.
		defer closer.CloseVoidOnErr(&rerr, cancel)
		mode = gpb.SubscriptionList_STREAM
	}*/
	sub, path, err := subscribe(ctx, t, dut, n, paths, mode)
	if err != nil {
		return nil, path, fmt.Errorf("cannot subscribe to gNMI client: %w", err)
	}
	c := &Watcher{
		err:  make(chan error),
		path: path,
	}

	go func() {
		//defer cancel()
		defer close(c.err)
		err := receiveUntil(sub, mode, path, isLeaf, consumer)
		c.err <- err
	}()

	return c, path, nil
}

// receiveUntil receives gNMI notifications the subscription times out.
// Note: all DataPoint are saved using store function.
// they are evaluated once per notification, after the first sync is received.
func receiveUntil(sub gpb.GNMI_SubscribeClient, mode gpb.SubscriptionList_Mode, path *gpb.Path, isLeaf bool, consumer Consumer) error {
	var recvData []*DataPoint
	var hasSynced bool
	var sync bool
	var err error

	for {
		recvData, sync, err = receive(sub, recvData, true)
		if err != nil {
			return fmt.Errorf("error receiving gNMI response: %w", err)
		}
		if mode == gpb.SubscriptionList_ONCE && sync {
			datas := [][]*DataPoint{recvData}
			for _, data := range datas {
				consumer.Process(data)
			}
			return nil
		}
		firstSync := !hasSynced && (sync || isLeaf)
		hasSynced = hasSynced || sync || isLeaf
		// Skip conversion and predicate until first sync for non-leaves.
		if !hasSynced {
			continue
		}
		var datas [][]*DataPoint
		if isLeaf {
			for _, datum := range recvData {
				// Only add a sync datapoint on the first sync, if there are no other values.
				if (len(recvData) == 1 && firstSync) || !datum.Sync {
					datas = append(datas, []*DataPoint{datum})
				}
			}
		} else {
			datas = [][]*DataPoint{recvData}
		}
		for _, data := range datas {
			consumer.Process(data)
		}
		recvData = nil
	}
}

// Watcher represents an ongoing watch of telemetry values.
type Watcher struct {
	err  chan error
	path *gpb.Path
}

// Await waits for the watch to finish and returns a boolean indicating whether the predicate evaluated to true.
func (c *Watcher) Await(t testing.TB) bool {
	t.Helper()
	err := <-c.err
	isTimeout := false
	if err != nil {
		// if the err is gRPC timeout, then the predicate was never true
		st, ok := status.FromError(errors.Unwrap(err))
		if ok && st.Code() == codes.DeadlineExceeded {
			isTimeout = true
		} else if ok && st.Code() == codes.Canceled {
			// no need to return eror, since the called cancel the call
			return true
		} else {
			t.Error(err)
		}
	}
	return !isTimeout
}

func resolveBatch(ctx context.Context, customData map[string]interface{}) (*requestOpts, error) {
	opts, err := extractRequestOpts(customData)
	if err != nil {
		return nil, fmt.Errorf("error extracting request options from %v: %w", customData, err)
	}
	if opts.client != nil {
		return opts, nil
	}

	dc, ok := customData[DefaultClientKey]
	if !ok {
		return opts, fmt.Errorf("gnmi client getter not set on root object")
	}
	client, ok := dc.(func(context.Context) (gpb.GNMIClient, error))
	if !ok {
		return opts, fmt.Errorf("unexpected gnmi client getter type")
	}
	opts.client, err = client(ctx)
	if err != nil {
		return nil, err
	}
	return opts, nil
}

// ResolvePath resolves a path struct to a path and request options.
func ResolvePath(n ygnmi.PathStruct) (*gpb.Path, map[string]interface{}, error) {
	path, customData, errs := ygnmi.ResolvePath(n)
	if errs != nil {
		return nil, nil, fmt.Errorf("errors resolving path struct %v: %v", n, errs)
	}
	// All paths that don't start with "meta" must be OC paths.
	if len(path.GetElem()) == 0 || path.GetElem()[0].GetName() != "meta" {
		path.Origin = "openconfig"
	}
	return path, customData, nil
}

// resolve resolves a path struct to a path, device, and request options.
// The returned requestOpts contains the gnmi Client to use.
func resolve(ctx context.Context, n ygnmi.PathStruct) (*gpb.Path, *requestOpts, error) {
	path, customData, err := ResolvePath(n)
	if err != nil {
		return nil, nil, err
	}
	opts, err := resolveBatch(ctx, customData)
	return path, opts, err
}

// LatestTimestamp returns the latest timestamp of the input datapoints.
// If datapoints is empty, then the zero time is returned.
func LatestTimestamp(data []*DataPoint) time.Time {
	var latest time.Time
	for _, dp := range data {
		if ts := dp.Timestamp; ts.After(latest) {
			latest = ts
		}
	}
	return latest
}

// LatestRecvTimestamp returns the latest recv timestamp of the input datapoints.
// If datapoints is empty, then the zero time is returned.
func LatestRecvTimestamp(data []*DataPoint) time.Time {
	var latest time.Time
	for _, dp := range data {
		if ts := dp.RecvTimestamp; ts.After(latest) {
			latest = ts
		}
	}
	return latest
}

// BundleDatapoints splits the incoming datapoints into common-prefix groups.
//
// Each bundle is identified by a common prefix path of length prefixLen. A
// slice of sorted prefixes is returned so users can examine each group
// deterministically. If any path is longer than prefixLen, then it is stored
// in a special "/" bundle.
func BundleDatapoints(t testing.TB, datapoints []*DataPoint, prefixLen uint) (map[string][]*DataPoint, []string) {
	t.Helper()
	groups, prefixes, err := bundleDatapoints(datapoints, prefixLen)
	if err != nil {
		t.Fatal(err)
	}
	return groups, prefixes
}

func bundleDatapoints(datapoints []*DataPoint, prefixLen uint) (map[string][]*DataPoint, []string, error) {
	groups := map[string][]*DataPoint{}

	for _, dp := range datapoints {
		if dp.Sync { // Sync datapoints don't have a path, so ignore them.
			continue
		}
		elems := dp.Path.GetElem()
		if uint(len(elems)) < prefixLen {
			groups["/"] = append(groups["/"], dp)
			continue
		}
		prefixPath, err := ygot.PathToString(&gpb.Path{Elem: elems[:prefixLen]})
		if err != nil {
			return nil, nil, err
		}
		groups[prefixPath] = append(groups[prefixPath], dp)
	}

	var prefixes []string
	for prefix := range groups {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	return groups, prefixes, nil
}

// getSubscriber is an implementation of gpb.GNMI_SubscribeClient that uses gpb.Get.
// Send() does the Get call and Recv returns the Get response.
type getSubscriber struct {
	gpb.GNMI_SubscribeClient
	client gpb.GNMIClient
	ctx    context.Context
	notifs []*gpb.Notification
}

func (gs *getSubscriber) Send(req *gpb.SubscribeRequest) error {
	getReq := &gpb.GetRequest{
		Prefix:   req.GetSubscribe().GetPrefix(),
		Encoding: gpb.Encoding_JSON_IETF,
		Type:     gpb.GetRequest_CONFIG,
	}
	for _, sub := range req.GetSubscribe().GetSubscription() {
		getReq.Path = append(getReq.Path, sub.GetPath())
	}
	log.V(1).Info(prototext.Format(getReq))
	resp, err := gs.client.Get(gs.ctx, getReq)
	if err != nil {
		return err
	}
	gs.notifs = resp.GetNotification()
	return nil
}

func (gs *getSubscriber) Recv() (*gpb.SubscribeResponse, error) {
	if len(gs.notifs) == 0 {
		return nil, io.EOF
	}
	resp := &gpb.SubscribeResponse{
		Response: &gpb.SubscribeResponse_Update{
			Update: gs.notifs[0],
		},
	}
	gs.notifs = gs.notifs[1:]
	return resp, nil
}

func (gs *getSubscriber) CloseSend() error {
	return nil
}

// subscribe create a gNMI SubscribeClient. Specifying subPaths is optional, if unset will subscribe to the path at n.
func subscribe(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, n ygnmi.PathStruct, subPaths []*gpb.Path, mode gpb.SubscriptionList_Mode) (_ gpb.GNMI_SubscribeClient, _ *gpb.Path, rerr error) {
	path, opts, err := resolve(ctx, n)
	if err != nil {
		return nil, path, err
	}
	if len(subPaths) == 0 {
		subPaths = []*gpb.Path{path}
	}
	ctx = metadata.NewOutgoingContext(ctx, opts.md)

	usesGet := opts.useGetForConfig && mode == gpb.SubscriptionList_ONCE
	var sub gpb.GNMI_SubscribeClient
	//create a gnmi connection oper watch to support multi-threading
	opts.client = dut.RawAPIs().GNMI(t)
	if usesGet {
		sub = &getSubscriber{
			client: opts.client,
			ctx:    ctx,
		}
	} else {
		sub, err = opts.client.Subscribe(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("gNMI failed to Subscribe: %w", err)
		}
	}
	defer closer.Close(&rerr, sub.CloseSend, "error closing gNMI send stream")

	var subs []*gpb.Subscription
	for _, path := range subPaths {
		subs = append(subs, &gpb.Subscription{
			Path: &gpb.Path{
				Elem:   path.GetElem(),
				Origin: path.GetOrigin(),
			},
			Mode: opts.subMode,
		})
	}

	sr := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Prefix:       &gpb.Path{Target: path.GetTarget()},
				Subscription: subs,
				Mode:         mode,
				Encoding:     gpb.Encoding_PROTO,
			},
		},
	}
	if !usesGet {
		log.V(1).Info(prototext.Format(sr))
	}
	if err := sub.Send(sr); err != nil {
		return nil, nil, fmt.Errorf("gNMI failed to Send(%+v): %w", sr, err)
	}
	// Use the target only for the subscription but exclude from the datapoint construction.
	path.Target = ""
	return sub, path, nil
}

// receive processes a single response from the subscription stream. If an "update" response is
// received, those points are appended to the given data and the result of that concatenation is
// the first return value, and the second return value is false. If a "sync" response is received,
// the data is returned as-is and the second return value is true. If Delete paths are present in
// the update, they are appended to the given data before the Update values. If deletesExpected
// is false, however, any deletes received will cause an error.
func receive(sub gpb.GNMI_SubscribeClient, data []*DataPoint, deletesExpected bool) ([]*DataPoint, bool, error) {
	res, err := sub.Recv()
	if err != nil {
		return data, false, err
	}
	recvTS := time.Now()

	switch v := res.Response.(type) {
	case *gpb.SubscribeResponse_Update:
		n := v.Update
		if !deletesExpected && len(n.Delete) != 0 {
			return data, false, fmt.Errorf("unexpected delete updates: %v", n.Delete)
		}
		ts := time.Unix(0, n.GetTimestamp())
		newDataPoint := func(p *gpb.Path, val *gpb.TypedValue) (*DataPoint, error) {
			j, err := util.JoinPaths(n.GetPrefix(), p)
			if err != nil {
				return nil, err
			}
			// Record the deprecated Element field for clearer compliance error messages.
			//if elements := append(append([]string{}, n.GetPrefix().Element...), p.Element...); len(elements) > 0 {
			//	j.Element = elements
			//}
			// Use the target only for the subscription but exclude from the datapoint construction.
			j.Target = ""
			return &DataPoint{Path: j, Value: val, Timestamp: ts, RecvTimestamp: recvTS}, nil
		}

		// Append delete data before the update values -- per gNMI spec, they
		// should always be processed first if both update types exist in the
		// same notification.
		for _, p := range n.Delete {
			log.V(2).Infof("Received gNMI Delete at path: %s", prototext.Format(p))
			dp, err := newDataPoint(p, nil)
			if err != nil {
				return data, false, err
			}
			log.V(2).Infof("Constructed datapoint for delete: %s", dp)
			data = append(data, dp)
		}
		for _, u := range n.GetUpdate() {
			if u.Path == nil {
				return data, false, fmt.Errorf("invalid nil path in update: %v", u)
			}
			if u.Val == nil {
				return data, false, fmt.Errorf("invalid nil Val in update: %v", u)
			}
			log.V(2).Infof("Received gNMI Update value %s at path: %s", prototext.Format(u.Val), prototext.Format(u.Path))
			dp, err := newDataPoint(u.Path, u.Val)
			if err != nil {
				return data, false, err
			}
			log.V(2).Infof("Constructed datapoint for update: %s", dp)
			data = append(data, dp)
		}
		return data, false, nil
	case *gpb.SubscribeResponse_SyncResponse:
		log.V(2).Infof("Received gNMI SyncResponse.")
		data = append(data, &DataPoint{
			RecvTimestamp: recvTS,
			Sync:          true,
		})
		return data, true, nil
	default:
		return data, false, fmt.Errorf("unexpected response: %v (%T)", v, v)
	}
}

const (
	metadataKeyPrefix   = "metadata-"
	subscriptionModeKey = "subscriptionMode"
	clientKey           = "client"
	useGetForConfigKey  = "useGetForConfig"
)

// extractRequestOpts translates the root path's custom data to request options.
func extractRequestOpts(customData map[string]interface{}) (*requestOpts, error) {
	opts := new(requestOpts)
	if v, ok := customData[subscriptionModeKey]; ok {
		m, ok := v.(gpb.SubscriptionMode)
		if !ok {
			return nil, fmt.Errorf("customData key %q but value is not SubscriptionMode type (%T, %v)", subscriptionModeKey, v, v)
		}
		opts.subMode = m
	}
	if v, ok := customData[clientKey]; ok {
		if v == nil {
			return nil, fmt.Errorf("customData key %q but value is nil", clientKey)
		}
		m, ok := v.(gpb.GNMIClient)
		if !ok {
			return nil, fmt.Errorf("customData key %q but value is not GNMIClient type (%T, %v)", clientKey, v, v)
		}
		opts.client = m
	}
	if v, ok := customData[useGetForConfigKey]; ok {
		m, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("customData key %q but value is not bool type (%T, %v)", useGetForConfigKey, v, v)
		}
		opts.useGetForConfig = m
	}
	md := make(map[string]string)
	for k, v := range customData {
		if !strings.HasPrefix(k, metadataKeyPrefix) {
			continue
		}
		sv, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("customData metadata key %q but value is not string type (%T, %v)", k, v, v)
		}
		md[strings.TrimPrefix(k, metadataKeyPrefix)] = sv
	}
	opts.md = metadata.New(md)
	return opts, nil
}

// EmitJSONFromGoStruct takes an input GoStruct and serializes it to a JSON string
// this is an abstraction for test conciseness, based on ygot.EmitJSON function
func EmitJSONFromGoStruct(t *testing.T, gostruct ygot.GoStruct) string {
	t.Helper()
	if gostruct == nil || util.IsValueNil(gostruct) {
		return ""
	}
	json, err := ygot.EmitJSON(gostruct, &ygot.EmitJSONConfig{
		Format: ygot.RFC7951,
		Indent: "  ",
		RFC7951Config: &ygot.RFC7951JSONConfig{
			AppendModuleName: true,
		},
	})

	if err != nil {
		t.Logf("Failed to emit JSON from default config struct: %v", err)
		return ""
	}

	return json
}
