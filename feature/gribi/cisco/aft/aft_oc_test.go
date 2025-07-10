package aft

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
)

type testArgs struct {
	ctx    context.Context
	dut    *ondatra.DUTDevice
	client *gribi.Client
}

const (
	// Destination prefix for DUT to ATE traffic.
	dstPfx   = "198.51.100.0"
	vrfB     = "VRF-B"
	nh1ID    = 1
	nh2ID    = 2
	nh100ID  = 100
	nh101ID  = 101
	nhg1ID   = 1
	nhg2ID   = 2
	nhg100ID = 100
	nhg101ID = 101
	nhip     = "192.0.2.254"
	mask     = "32"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT Port 3",
		IPv4:    "192.0.2.9",
		IPv4Len: 30,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		Desc:    "ATE Port 2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		Desc:    "ATE Port 3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: 30,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBackupActive(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
	configureNetworkInstance(t, dut)

	client := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	defer client.Close(t)
	defer client.FlushAll(t)
	if err := client.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	client.BecomeLeader(t)
	client.FlushAll(t)

	tcArgs := &testArgs{
		ctx:    ctx,
		dut:    dut,
		client: &client,
	}

	tcArgs.configureBackupNextHopGroup(t)

	applyImpairmentFn := func() {
		dutP2 := dut.Port(t, "port2")
		gnmi.Replace(t, dut, gnmi.OC().Interface(dutP2.Name()).Enabled().Config(), false)
		gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)
	}

	removeImpairmentFn := func() {
		dutP2 := dut.Port(t, "port2")
		gnmi.Replace(t, dut, gnmi.OC().Interface(dutP2.Name()).Enabled().Config(), true)
		gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	}

	nhg := getNHGByID(t, dut, nhg101ID)
	if nhg == nil {
		t.Fatalf("Could not get next-hop-group with id: %v", nhg101ID)
	}

	gnmic, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t))
	if err != nil {
		t.Fatalf("Error creating ygnmi client: %v", err)
	}

	watcherValues := []bool{}
	watcherContext, wathcerCancelFn := context.WithCancel(context.Background())
	defer wathcerCancelFn()

	watcher := ygnmi.WatchAll[bool](watcherContext, gnmic, getBackupActiveQuery(t, *nhg.Id),
		func(v *ygnmi.Value[bool]) error {
			val, pres := v.Val()
			if !pres {
				t.Fatalf("Value not present: %v", err)
			}
			watcherValues = append(watcherValues, val)
			return ygnmi.Continue
		})

	if want, got := false, getBackupActive(t, gnmic, *nhg.Id); want != got {
		t.Fatalf("Unexpected value for backup-active: want %v, got %v", want, got)
	}

	applyImpairmentFn()

	time.Sleep(5 * time.Second)

	if want, got := true, getBackupActive(t, gnmic, *nhg.Id); want != got {
		t.Fatalf("Unexpected value for backup-active: want %v, got %v", want, got)
	}

	removeImpairmentFn()

	time.Sleep(5 * time.Second)
	if want, got := false, getBackupActive(t, gnmic, *nhg.Id); want != got {
		t.Fatalf("Unexpected value for backup-active: want %v, got %v", want, got)
	}

	t.Run("EDT", func(t *testing.T) {
		wathcerCancelFn()
		watcher.Await()
		if want, got := []bool{false, true, false}, watcherValues; cmp.Diff(want, got) != "" {
			t.Fatalf("Unexpected value for backup-active: want %v, got %v", want, got)
		}
	})
}

func getBackupActiveQuery(t testing.TB, nhgIndex uint64) ygnmi.WildcardQuery[bool] {
	elems := []string{
		"/network-instances",
		"network-instance[name=DEFAULT]",
		"afts",
		"next-hop-groups",
		fmt.Sprintf("next-hop-group[id=%v]", nhgIndex),
		"state",
		"backup-active",
	}

	q, err := schemaless.NewWildcard[bool](strings.Join(elems, "/"), "openconfig")
	if err != nil {
		t.Fatalf("Error creating querry for /backup-active: %v", err)
	}
	return q
}

func getBackupActive(t testing.TB, gnmic *ygnmi.Client, nhgIndex uint64) bool {
	vals, err := ygnmi.GetAll(context.Background(), gnmic, getBackupActiveQuery(t, nhgIndex))
	if err != nil {
		t.Fatalf("Error subscribing to /backup-active: %v", err)
	}

	if len(vals) == 0 {
		t.Fatalf("Did not receive a response for /backup-active")
	}

	return vals[0]
}

func getNHGByID(t *testing.T, dut *ondatra.DUTDevice, id uint64) *oc.NetworkInstance_Afts_NextHopGroup {
	aftNHGs := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().NextHopGroupAny().State())
	for _, nhg := range aftNHGs {
		if nhg.GetProgrammedId() == id {
			return nhg
		}
	}
	return nil
}

// configureDUT configures port1-3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))
}

func (a *testArgs) configureBackupNextHopGroup(t *testing.T) {
	t.Logf("Adding NH %d with atePort2 via gRIBI", nh1ID)
	nh1, op1 := gribi.NHEntry(nh1ID, atePort2.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	t.Logf("Adding NH %d with atePort3 and NHGs %d, %d via gRIBI", nh2ID, nhg1ID, nhg2ID)
	nh2, op2 := gribi.NHEntry(nh2ID, atePort3.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg1, op3 := gribi.NHGEntry(nhg1ID, map[uint64]uint64{nh1ID: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg2, op4 := gribi.NHGEntry(nhg2ID, map[uint64]uint64{nh2ID: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddEntries(t, []fluent.GRIBIEntry{nh1, nh2, nhg1, nhg2}, []*client.OpResult{op1, op2, op3, op4})
	t.Logf("Adding an IPv4Entry for %s via gRIBI", nhip)
	a.client.AddIPv4(t, nhip+"/"+mask, nhg1ID, deviations.DefaultNetworkInstance(a.dut), deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	t.Logf("Adding NH %d in VRF-B via gRIBI", nh100ID)
	nh100, op5 := gribi.NHEntry(nh100ID, "VRFOnly", deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfB})
	if deviations.BackupNHGRequiresVrfWithDecap(a.dut) {
		nh100, op5 = gribi.NHEntry(nh100ID, "Decap", deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfB})
	}
	t.Logf("Adding NH %d and NHGs %d, %d via gRIBI", nh101ID, nhg100ID, nhg101ID)
	nh101, op6 := gribi.NHEntry(nh101ID, nhip, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg100, op7 := gribi.NHGEntry(nhg100ID, map[uint64]uint64{nh100ID: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg101, op8 := gribi.NHGEntry(nhg101ID, map[uint64]uint64{nh101ID: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg100ID})
	a.client.AddEntries(t, []fluent.GRIBIEntry{nh100, nh101, nhg100, nhg101}, []*client.OpResult{op5, op6, op7, op8})
	t.Logf("Adding IPv4Entries for %s for DEFAULT and VRF-B via gRIBI", dstPfx)
	a.client.AddIPv4(t, dstPfx+"/"+mask, nhg101ID, deviations.DefaultNetworkInstance(a.dut), deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddIPv4(t, dstPfx+"/"+mask, nhg2ID, vrfB, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
}

func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	c := &oc.Root{}
	ni := c.GetOrCreateNetworkInstance(vrfB)
	ni.Description = ygot.String("Non Default routing instance VRF-B created for testing")
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfB).Config(), ni)
}
