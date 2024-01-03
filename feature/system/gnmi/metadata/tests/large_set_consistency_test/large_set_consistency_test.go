package large_set_consistency_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

// getDeviceConfig gets a full config from a device but refurbishes it enough so it can be
// pushed out again
func getDeviceConfig(t testing.TB, dev gnmi.DeviceOrOpts) *oc.Root {
	config := gnmi.Get[*oc.Root](t, dev, gnmi.OC().Config())
	fptest.WriteQuery(t, "Untouched", gnmi.OC().Config(), config)

	for cname, component := range config.Component {
		// Keep the port components in order to preserve the breakout-mode config.
		if component.GetPort() == nil {
			delete(config.Component, cname)
			continue
		}
		// Need to prune subcomponents that may have a leafref to a component that was
		// pruned.
		component.Subcomponent = nil
	}

	for iname, iface := range config.Interface {
		if iface.GetEthernet() == nil {
			continue
		}
		// Ethernet config may not contain meaningful values if it wasn't explicitly
		// configured, so use its current state for the config, but prune non-config leaves.
		intf := gnmi.Get(t, dev, gnmi.OC().Interface(iname).State())
		breakout := config.GetComponent(intf.GetHardwarePort()).GetPort().GetBreakoutMode()
		e := intf.GetEthernet()
		// Set port speed to unknown for non breakout interfaces
		if breakout.GetGroup(1) == nil && e != nil {
			e.SetPortSpeed(oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN)
		}
		ygot.PruneConfigFalse(oc.SchemaTree["Interface_Ethernet"], e)
		if e.PortSpeed != 0 && e.PortSpeed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
			iface.Ethernet = e
		}
	}

	if config.Lldp != nil {
		config.Lldp.ChassisId = nil
		config.Lldp.ChassisIdType = oc.Lldp_ChassisIdType_UNSET
	}

	config.Qos = nil

	for _, ni := range config.NetworkInstance {
		ni.Fdb = nil
	}

	fptest.WriteQuery(t, "Touched", gnmi.OC().Config(), config)
	return config
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
func extractMetadataAnnotation(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice) string {
	ns := getNotificationsUsingGNMIGet(t, gnmiClient, dut)
	if got := len(ns); got == 0 {
		t.Fatalf("number of notifications got %d, want > 0", got)
	}

	var annotation any
	for _, n := range ns {
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
	return msg.GetName()
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

// checkMetadata checks protobuf-metadata
func checkMetadata(t *testing.T, gnmiClient gpb.GNMIClient, dut *ondatra.DUTDevice, done *atomic.Bool) {
	t.Helper()

	got := extractMetadataAnnotation(t, gnmiClient, dut)

	want := metadata1
	if done.Load() {
		want = metadata2
	}
	if got != want {
		t.Errorf("extractMetadataAnnotation: got %v, want %v", got, want)
	}
}

func TestLargeSetConsistency(t *testing.T) {
	done := &atomic.Bool{}
	done.Store(false)
	dut := ondatra.DUT(t, "dut")

	// configuring basic interface and network instance as some devices only populate OC after configuration
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	// Configuring basic interface and network instance as some devices only populate OC after configuration.
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Type().Config(),
		oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	baselineConfig := getDeviceConfig(t, dut)
	setEthernetFromBase(t, baselineConfig)
	gnmiClient := dut.RawAPIs().GNMI(t)

	// send 1st update request in one goroutine
	gpbSetRequest := buildGNMISetRequest(t, metadata1, baselineConfig)
	t.Log("gnmiClient Set 1st large config")
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmi.Set unexpected error: %v", err)
	}
	checkMetadata(t, gnmiClient, dut, done)

	var wg sync.WaitGroup
	ch := make(chan struct{}, 1)

	// sending 2nd update request in one goroutine
	gpbSetRequest = buildGNMISetRequest(t, metadata2, baselineConfig)
	wg.Add(1)
	go func() {
		defer wg.Done()
		t.Log("gnmiClient Set 2nd large config")
		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Errorf("gnmi.Set unexpected error: %v", err)
			return
		}
		close(ch)
		done.Store(true)
	}()

	// sending 4 Get requests concurrently every 5 seconds.
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
					checkMetadata(t, gnmiClient, dut, done)
					time.Sleep(5 * time.Second)
				}
			}
		}(i)
	}

	wg.Wait()

	checkMetadata(t, gnmiClient, dut, done)
}
