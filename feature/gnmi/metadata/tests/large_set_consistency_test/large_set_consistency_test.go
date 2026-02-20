package large_set_consistency_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/util"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/proto"
)

const (
	shortStringMetadata1 = "1st_LARGE_CONFIGURATION"
	shortStringMetadata2 = "2nd_LARGE_CONFIGURATION"
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

// filterBaselineConfig filters the baseline config to remove unwanted fields.
func filterBaselineConfig(baselineConfig *oc.Root) {
	for _, ni := range baselineConfig.NetworkInstance {
		for _, p := range ni.Protocol {
			if p.Bgp != nil {
				if p.Identifier != oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP {
					p.Bgp = nil
				} else if p.Bgp.Global != nil {
					p.Bgp.Global.UseMultiplePaths = nil
				}
			}
			if p.Identifier != oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS {
				p.Isis = nil
			}
		}
		if ni.Type != oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE {
			ni.Mpls = nil
		}
		ni.SegmentRouting = nil
	}
	if baselineConfig.System != nil && baselineConfig.System.Ntp != nil {
		for _, s := range baselineConfig.System.Ntp.Server {
			s.Port = nil
		}
	}
	for _, i := range baselineConfig.Interface {
		if i.Type != oc.IETFInterfaces_InterfaceType_ieee8023adLag {
			i.Aggregation = nil
		}
		if i.Type != oc.IETFInterfaces_InterfaceType_l3ipvlan {
			i.RoutedVlan = nil
		}
		if i.Type != oc.IETFInterfaces_InterfaceType_ethernetCsmacd && i.Type != oc.IETFInterfaces_InterfaceType_ieee8023adLag {
			i.Ethernet = nil
		}
	}
	if baselineConfig.System != nil {
		baselineConfig.System.Utilization = nil
	}
	if baselineConfig.Acl != nil {
		for k, as := range baselineConfig.Acl.AclSet {
			if as.GetName() == "default-control-plane-acl" {
				delete(baselineConfig.Acl.AclSet, k)
			}
		}
	}
	baselineConfig.Sampling = nil
	baselineConfig.RoutingPolicy = nil
	baselineConfig.Qos = nil
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
func buildGNMISetRequest(t *testing.T, metadataText string, baselineConfig *oc.Root, size int) *gpb.SetRequest {
	var trimSize float64

	// For 100KB and 1M cases trim the data according to proto and base64encoding overheads
	if size >= 100000 {
		randomBytes := make([]byte, size)
		_, err := io.ReadFull(rand.Reader, randomBytes)
		t.Logf("Length of randomBytes: %d\n", len(randomBytes))
		if err != nil {
			t.Fatalf("failed to generate random bytes: %v", err)
		}
		// Encode the bytes to a base64 string.
		// account for 4 byte proto message overhead and 25% for subsequent base64encoding overhead
		trimSize = 0.75*float64(size) - 4
		largeMetadata := base64.StdEncoding.EncodeToString(randomBytes)
		metadataText = largeMetadata[:int(trimSize)]
	}
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

func checkshortStringMetadata1(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice, done *atomic.Int64) {
	t.Helper()
	got, getRespTimeStamp := extractMetadataAnnotation(t, gnmiClient, dut)
	want := shortStringMetadata1
	if got != want && getRespTimeStamp < done.Load() {
		t.Errorf("extractMetadataAnnotation: got %v, want %v", got, want)
	}
}

func checkshortStringMetadata2(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice) {
	t.Helper()
	got, getRespTimeStamp := extractMetadataAnnotation(t, gnmiClient, dut)
	want := shortStringMetadata2
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

func testLargeMetadata(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice, baselineConfig *oc.Root, size int, done *atomic.Int64) {
	randomBytes := make([]byte, size)
	_, err := io.ReadFull(rand.Reader, randomBytes)
	if err != nil {
		t.Fatalf("failed to generate random bytes: %v", err)
	}
	// Encode the bytes to a base64 string.
	largeMetadata := base64.StdEncoding.EncodeToString(randomBytes)
	largeMetadata = largeMetadata[:size]
	// send large metadata update request in one goroutine
	gpbSetRequest := buildGNMISetRequest(t, largeMetadata, baselineConfig, size)
	t.Log("gnmiClient Set large metadataconfig request")
	_, err = gnmiClient.Set(context.Background(), gpbSetRequest)
	if err != nil {
		t.Fatalf("gnmi.Set unexpected error , got: %v", err)
	}
	checkLargeMetadata(t, gnmiClient, dut, largeMetadata, done)
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
	filterBaselineConfig(baselineConfig)
	setEthernetFromBase(t, baselineConfig)
	gnmiClient := dut.RawAPIs().GNMI(t)

	// send 1st update request in one goroutine
	sizeMetadata1 := len(shortStringMetadata1)
	gpbSetRequest := buildGNMISetRequest(t, shortStringMetadata1, baselineConfig, sizeMetadata1)
	t.Log("gnmiClient Set 1st large config")
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmi.Set unexpected error: %v", err)
	}
	t.Run("check shortStringMetadata1", func(t *testing.T) {
		checkshortStringMetadata1(t, gnmiClient, dut, done)
	})

	// Add delay to allow the first SET request value to propagate to the datastore
	time.Sleep(5 * time.Second)

	var wg sync.WaitGroup
	ch := make(chan struct{}, 1)

	// sending 2nd update request in one goroutine
	sizeMetadata2 := len(shortStringMetadata2)
	gpbSetRequest = buildGNMISetRequest(t, shortStringMetadata2, baselineConfig, sizeMetadata2)
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
					checkshortStringMetadata1(t, gnmiClient, dut, done)
				}
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(5 * time.Second)
	t.Run("check shortStringMetadata2", func(t *testing.T) {
		checkshortStringMetadata2(t, gnmiClient, dut)
	})
}

func TestLargeMetadataConfigPush(t *testing.T) {
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
	filterBaselineConfig(baselineConfig)
	setEthernetFromBase(t, baselineConfig)
	gnmiClient := dut.RawAPIs().GNMI(t)

	// Large metadata Test cases.
	type testCase struct {
		name string
		size int
	}
	testCases := []testCase{
		{
			name: "Metadata with Size 100KB",
			size: 100 * 1024,
		},
		{
			name: "Metadata with Size 1MB",
			size: 1 * 1000 * 1024,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.name)
			testLargeMetadata(t, gnmiClient, dut, baselineConfig, tc.size, done)
		})
	}
}
