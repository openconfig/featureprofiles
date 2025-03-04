package large_set_consistency_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"strconv"

	"google3/third_party/golang/protobuf/v2/proto/proto"
	"google3/third_party/golang/ygot/util/util"
	"google3/third_party/golang/ygot/ygot/ygot"
	"google3/third_party/openconfig/featureprofiles/internal/attrs/attrs"
	"google3/third_party/openconfig/featureprofiles/internal/deviations/deviations"
	"google3/third_party/openconfig/featureprofiles/internal/fptest/fptest"
	gpb "google3/third_party/openconfig/gnmi/proto/gnmi/gnmi_go_proto"
	"google3/third_party/openconfig/ondatra/gnmi/gnmi"
	"google3/third_party/openconfig/ondatra/gnmi/oc/oc"
	"google3/third_party/openconfig/ondatra/ondatra"
	"google3/third_party/openconfig/ygnmi/ygnmi/ygnmi"
	"crypto/rand"
)

const (
	metadata1 = "1st_LARGE_CONFIGURATION"
	metadata2 = "2nd_LARGE_CONFIGURATION"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: 126,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// setEthernetFromBase merges the ethernet config from the interfaces in base config into
// the destination config.
func setEthernetFromBase(t testing.TB, config *oc.Root) {
	t.Helper()

	for iname, iface := range config.Interface {
		eb := config.GetInterface(iname).GetEthernet()
		ec := iface.GetOrCreateEthernet()
		if eb == nil || ec == nil {
			continue
		}
		if err := ygot.MergeStructInto(ec, eb); err != nil {
			t.Errorf("Cannot merge %s ethernet: %v", iname, err)
		}
	}
}

// buildGNMIUpdate builds a gnmi update for a given ygot path and value.
func buildGNMIUpdate(t *testing.T, yPath ygnmi.PathStruct, val any) *gpb.Update {
	t.Helper()
	path, _, errs := ygnmi.ResolvePath(yPath)
	if errs != nil {
		t.Fatalf("Could not resolve the ygot path; %v", errs)
	}
	js, err := ygot.Marshal7951(val, ygot.JSONIndent("  "), &ygot.RFC7951JSONConfig{AppendModuleName: true, PreferShadowPath: true})
	if err != nil {
		t.Fatalf("Could not encode value into JSON format: %v", err)
	}
	return &gpb.Update{
		Path: path,
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonIetfVal{
				JsonIetfVal: js,
			},
		},
	}
}

// extractMetadataAnnotation extracts the metadata protobuf message from a gNMI GetResponse.
func extractMetadataAnnotation(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice) (string, int64) {
	ns := getNotificationsUsingGNMIGet(t, gnmiClient, dut)
	var getRespTimeStamp int64
	if got := len(ns); got == 0 {
		t.Fatalf("number of notifications got %d, want > 0", got)
	}

	var annotation any
	for _, n := range ns {
		getRespTimeStamp = n.GetTimestamp()
		for _, u := range n.GetUpdate() {
			path, err := util.JoinPaths(new(gpb.Path), u.GetPath())
			if err != nil || len(path.GetElem()) > 0 {
				continue
			}

			var v map[string]any
			if err := json.Unmarshal(u.GetVal().GetJsonIetfVal(), &v); err == nil {
				if metav, ok := v["@"]; ok {
					if metaObj, ok := metav.(map[string]any); ok {
						if annotation, ok = metaObj["openconfig-metadata:protobuf-metadata"]; ok {
							break
						}
					}
				}
			}
		}
		if annotation != nil {
			break
		}
	}

	var ok bool
	var encoded string
	if annotation == nil {
		t.Fatal("received metadata object does not contain expected annotation")
	}
	if encoded, ok = annotation.(string); !ok {
		t.Fatalf("got %T type for annotation, expected string type", annotation)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("cannot decode base64 string, err: %v", err)
	}
	msg := &gpb.ModelData{}
	if err := proto.Unmarshal(decoded, msg); err != nil {
		t.Fatalf("cannot unmarshal received proto any msg, err: %v", err)
	}
	return msg.GetName(), getRespTimeStamp
}

// buildGNMISetRequest builds gnmi set request with protobuf-metadata
func buildGNMISetRequest(t *testing.T, metadataText string, baselineConfig *oc.Root) *gpb.SetRequest {
	msg := &gpb.ModelData{Name: metadataText}
	b, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("cannot marshal proto msg - error: %v", err)
	}
	metadataEncoded := base64.StdEncoding.EncodeToString(b)
	j := map[string]any{
		"@": map[string]any{
			"openconfig-metadata:protobuf-metadata": metadataEncoded,
		},
	}
	v, err := json.Marshal(j)
	if err != nil {
		t.Fatalf("marshal config failed with unexpected error: %v", err)
	}

	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Elem: []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: v},
			},
		}},
	}

	accompaniedPath := gnmi.OC().Config().PathStruct()
	gpbSetRequest.Update = append(gpbSetRequest.Update, buildGNMIUpdate(t, accompaniedPath, baselineConfig))
	return gpbSetRequest
}

// returns notifications using gnmi get
func getNotificationsUsingGNMIGet(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice) []*gpb.Notification {
	getResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{},
		}},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: gpb.Encoding_JSON_IETF,
	})
	if err != nil {
		t.Fatalf("Cannot fetch protobuf-metadata annotation from the DUT: %v", err)
	}

	return getResponse.GetNotification()
}

func checkMetadata1(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice, done *atomic.Int64) {
	t.Helper()
	got, getRespTimeStamp := extractMetadataAnnotation(t, gnmiClient, dut)
	want := metadata1
	if got != want && getRespTimeStamp < done.Load() {
		t.Errorf("extractMetadataAnnotation: got %v, want %v", got, want)
	}
}

func checkMetadata2(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice) {
	t.Helper()
	got, getRespTimeStamp := extractMetadataAnnotation(t, gnmiClient, dut)
	want := metadata2
	t.Logf("getResp: %v ", getRespTimeStamp)
	if got != want {
		t.Errorf("extractMetadataAnnotation: got %v, want %v", got, want)
	}
}

func checkLargeMetadata(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice, largeMetadata string, done *atomic.Int64) {
	t.Helper()
	got, getRespTimeStamp := extractMetadataAnnotation(t, gnmiClient, dut)
	want := largeMetadata
	if got != want && getRespTimeStamp < done.Load() {
		t.Errorf("extractMetadataAnnotation: got %v, want %v", got, want)
	}
}

func TestLargeSetConsistency(t *testing.T) {
	done := &atomic.Int64{}
	dut := ondatra.DUT(t, "dut")

	// configuring basic interface and network instance as some devices only populate OC after configuration
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Configuring basic interface and network instance as some devices only populate OC after configuration.
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Type().Config(),
		oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	baselineConfig := fptest.GetDeviceConfig(t, dut)
	setEthernetFromBase(t, baselineConfig)
	gnmiClient := dut.RawAPIs().GNMI(t)

	// send 1st update request in one goroutine
	gpbSetRequest := buildGNMISetRequest(t, metadata1, baselineConfig)
	t.Log("gnmiClient Set 1st large config")
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmi.Set unexpected error: %v", err)
	}
	checkMetadata1(t, gnmiClient, dut, done)

	var wg sync.WaitGroup
	ch := make(chan struct{}, 1)

	// sending 2nd update request in one goroutine
	gpbSetRequest = buildGNMISetRequest(t, metadata2, baselineConfig)
	wg.Add(1)
	go func() {
		defer wg.Done()
		t.Log("gnmiClient Set 2nd large config")
		setResp, err := gnmiClient.Set(context.Background(), gpbSetRequest)
		if err != nil {
			t.Errorf("gnmi.Set unexpected error: %v", err)
			return
		}
		close(ch)
		done.Store(setResp.GetTimestamp())
	}()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-ch:
					t.Logf("[%d - exiting]", i)
					return
				default:
					t.Logf("[%d - running] checking config protobuf-metadata", i)
					time.Sleep(5 * time.Millisecond)
					checkMetadata1(t, gnmiClient, dut, done)
				}
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(5 * time.Second)
	checkMetadata2(t, gnmiClient, dut)
	b := make([]byte, 100*1024)
	largeMetadataInt, err := rand.Read(b)
	if err != nil && err != io.EOF {
		t.Fatalf("Error reading bytes: %v", err)
	}
	largeMetadata := strconv.Itoa(largeMetadataInt)
	// send 3rd large metadata update request in one goroutine
	gpbSetRequest = buildGNMISetRequest(t, largeMetadata, baselineConfig)
	t.Log("gnmiClient Set large metadataconfig request")
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmi.Set unexpected error: %v", err)
	}
	checkLargeMetadata(t, gnmiClient, dut, largeMetadata, done)
}

