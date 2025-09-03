package qos_test

import (
	//"fmt"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}
func TestQmWredQosSet(t *testing.T) {
	// testing creation of QM profiler at openconfig/qos

	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}

	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	t.Run("Step 1 updae /openconfig/qos and Get qos", func(t *testing.T) {
		qos := d.GetOrCreateQos()
		for i, wredprofile := range wredprofilelist {
			wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
			wredqueumred := wredqueum.GetOrCreateWred()
			wredqueumreduni := wredqueumred.GetOrCreateUniform()
			wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
			wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
			wredqueumreduni.EnableEcn = ygot.Bool(true)
			wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])
		}

		configqos := gnmi.OC().Qos()
		gnmi.Update(t, dut, configqos.Config(), qos)
		configGotqos := gnmi.Get(t, dut, configqos.Config())
		if diff := cmp.Diff(*configGotqos, *qos); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	})

}

func TestQmRedQosSet(t *testing.T) {
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}

	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}

	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
	}
	configqos := gnmi.OC().Qos()
	gnmi.Update(t, dut, configqos.Config(), qos)
	configGotqos := gnmi.Get(t, dut, configqos.Config())
	if diff := cmp.Diff(*configGotqos, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

}

func TestQueueManagementQosReplace(t *testing.T) {
	t.Run("Step 2 Replace /openconfig/qos and Get qos", func(t *testing.T) {

		dut := ondatra.DUT(t, "dut")
		d := &oc.Root{}
		defer teardownQos(t, dut)
		qosr := d.GetOrCreateQos()
		wredqueum1 := qosr.GetOrCreateQueueManagementProfile("wredprofile11")
		wredqueumredr := wredqueum1.GetOrCreateWred()
		wredqueumredunir := wredqueumredr.GetOrCreateUniform()
		wredqueumredunir.MinThreshold = ygot.Uint64(170000)
		wredqueumredunir.MaxThreshold = ygot.Uint64(180000)
		wredqueumredunir.EnableEcn = ygot.Bool(true)
		wredqueumredunir.MaxDropProbabilityPercent = ygot.Uint8(10)
		configqosreplace := gnmi.OC().Qos()
		gnmi.Replace(t, dut, configqosreplace.Config(), qosr)
		configGotqos := gnmi.Get(t, dut, configqosreplace.Config())
		if diff := cmp.Diff(*configGotqos, *qosr); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	})
}
func TestQMQmwredSet(t *testing.T) {
	//Replace/Get and  at /openconfig/qos/queue-management-profile

	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name)
		gnmi.Replace(t, dut, configqm.Config(), wredqueum)
		configGotQM := gnmi.Get(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
	configqos := gnmi.OC().Qos()
	configGotqos := gnmi.Get(t, dut, configqos.Config())
	if diff := cmp.Diff(*configGotqos, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	//update at queue-management-profile
	wredqueum5 := qos.GetOrCreateQueueManagementProfile("wredprofile5")
	wredqueumred5 := wredqueum5.GetOrCreateWred()
	wredqueumreduni5 := wredqueumred5.GetOrCreateUniform()
	wredqueumreduni5.MinThreshold = ygot.Uint64(614400)
	wredqueumreduni5.MaxThreshold = ygot.Uint64(390070272)
	wredqueumreduni5.EnableEcn = ygot.Bool(true)
	wredqueumreduni5.MaxDropProbabilityPercent = ygot.Uint8(10)
	configUpdate := gnmi.OC().Qos().QueueManagementProfile(*wredqueum5.Name)
	gnmi.Update(t, dut, configUpdate.Config(), wredqueum5)
	configGot := gnmi.Get(t, dut, configUpdate.Config())
	if diff := cmp.Diff(*configGot, *wredqueum5); diff != "" {
		t.Errorf("Config Schedule update failed: \n%v", diff)
	}

}

func TestQMQmredSet(t *testing.T) {
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}

	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}

	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
		configqm := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name)
		gnmi.Replace(t, dut, configqm.Config(), redqueum)
		configGotQM := gnmi.Get(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *redqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
	configqos := gnmi.OC().Qos()
	configGotqos := gnmi.Get(t, dut, configqos.Config())
	if diff := cmp.Diff(*configGotqos, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	redqueum1 := qos.GetOrCreateQueueManagementProfile("redprofile9")
	redqueumred1 := redqueum1.GetOrCreateRed()
	redqueumreduni1 := redqueumred1.GetOrCreateUniform()
	redqueumreduni1.MinThreshold = ygot.Uint64(120000)
	redqueumreduni1.MaxThreshold = ygot.Uint64(130000)
	configred := gnmi.OC().Qos().QueueManagementProfile(*redqueum1.Name)
	gnmi.Update(t, dut, configred.Config(), redqueum1)
	configGot := gnmi.Get(t, dut, configred.Config())
	if diff := cmp.Diff(*configGot, *redqueum1); diff != "" {
		t.Errorf("Config Schedule update failed: \n%v", diff)
	}
	redqueumreduni1.EnableEcn = ygot.Bool(true)
	configredecn := gnmi.OC().Qos().QueueManagementProfile(*redqueum1.Name)
	gnmi.Update(t, dut, configredecn.Config(), redqueum1)
	configGotEcn := gnmi.Get(t, dut, configredecn.Config())
	if diff := cmp.Diff(*configGotEcn, *redqueum1); diff != "" {
		t.Errorf("Config Schedule update failed: \n%v", diff)
	}

}

// func TestDeleteQos(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	dut.Config().Qos().Delete(t)

// }

func TestQMWredSetReplace(t *testing.T) {

	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for i, wredprofile := range wredprofilelist {

		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wred := make(map[uint32]*oc.Qos_QueueManagementProfile_Wred)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wred[1] = wredqueumred
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name)
		gnmi.Replace(t, dut, configqm.Config(), &oc.Qos_QueueManagementProfile{
			Name: &wredprofile,
			Wred: wredqueumred,
		})
		configGotQM := gnmi.Get(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
}

func TestQMWredUniReplace(t *testing.T) {

	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wred := make(map[uint32]*oc.Qos_QueueManagementProfile_Wred)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wred[1] = wredqueumred
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name)
		gnmi.Replace(t, dut, configqm.Config(), &oc.Qos_QueueManagementProfile{
			Name: &wredprofile,
			Wred: wredqueumred,
		})
		configGotQM := gnmi.Get(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
}

func TestQMWredSetUpdate(t *testing.T) {

	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wred := make(map[uint32]*oc.Qos_QueueManagementProfile_Wred)
		wred[1] = wredqueumred
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name)
		gnmi.Update(t, dut, configqm.Config(), &oc.Qos_QueueManagementProfile{
			Name: &wredprofile,
			Wred: wredqueumred,
		})
		configGotQM := gnmi.Get(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
}

func TestQMWredUniUpdate(t *testing.T) {

	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wred := make(map[uint32]*oc.Qos_QueueManagementProfile_Wred)
		wred[1] = wredqueumred
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name)
		gnmi.Update(t, dut, configqm.Config(), &oc.Qos_QueueManagementProfile{
			Name: &wredprofile,
			Wred: wredqueumred,
		})
		configGotQM := gnmi.Get(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
}

func TestQMRedReplace(t *testing.T) {

	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}

	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}

	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		red := make(map[uint32]*oc.Qos_QueueManagementProfile_Red)
		red[1] = redqueumred
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
		configqm := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name)
		gnmi.Replace(t, dut, configqm.Config(), &oc.Qos_QueueManagementProfile{
			Name: &redprofile,
			Red:  redqueumred,
		})
		configGotQM := gnmi.Get(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *redqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
}

func TestQMRedSetUpdate(t *testing.T) {
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}

	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 100000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 130000+uint64(i*6144))
	}

	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		red := make(map[uint32]*oc.Qos_QueueManagementProfile_Red)
		red[1] = redqueumred
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
		configqm := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name)
		gnmi.Update(t, dut, configqm.Config(), &oc.Qos_QueueManagementProfile{
			Name: &redprofile,
			Red:  redqueumred,
		})
		configGotQM := gnmi.Get(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *redqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
}

func TestQmUpdateEcn(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	wredprofile := "wredprofile11"
	wredqueum := qos.GetOrCreateQueueManagementProfile("wredprofile11")
	wredqueumred := wredqueum.GetOrCreateWred()
	wredqueumreduni := wredqueumred.GetOrCreateUniform()
	wred := make(map[uint32]*oc.Qos_QueueManagementProfile_Wred)
	wred[1] = wredqueumred
	wredqueumreduni.MinThreshold = ygot.Uint64(150000)
	wredqueumreduni.MaxThreshold = ygot.Uint64(160000)
	wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(10)
	redprofile := "redprofile8"
	redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
	redqueumred := redqueum.GetOrCreateRed()
	redqueumreduni := redqueumred.GetOrCreateUniform()
	red := make(map[uint32]*oc.Qos_QueueManagementProfile_Red)
	red[1] = redqueumred
	redqueumreduni.MinThreshold = ygot.Uint64(120000)
	redqueumreduni.MaxThreshold = ygot.Uint64(130000)
	config1 := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name)
	gnmi.Update(t, dut, config1.Config(), &oc.Qos_QueueManagementProfile{
		Name: &wredprofile,
		Wred: wredqueumred,
	})
	config2 := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name)
	gnmi.Update(t, dut, config2.Config(), &oc.Qos_QueueManagementProfile{
		Name: &redprofile,
		Red:  redqueumred,
	})
	configsetwredecn := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name).Wred().Uniform().EnableEcn()
	gnmi.Update(t, dut, configsetwredecn.Config(), true)
	ConfigGetEcn := gnmi.Get(t, dut, gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name).Config())
	if diff := cmp.Diff(*ConfigGetEcn, *redqueumred); diff == "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
}

func TestQueueManagementDeleteAdd(t *testing.T) {

	// Step 4 Create a single Policy-map and attach the queues in there.
	// Step 5 Attach the scheduler policy to an interface.
	// Step 6 Subscribe to the Queue of the interface .
	// Step 8  Defar called to delete all.
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	wredqueum := qos.GetOrCreateQueueManagementProfile("wredprofile11")
	wredqueumred := wredqueum.GetOrCreateWred()
	wredqueumreduni := wredqueumred.GetOrCreateUniform()
	wredqueumreduni.MinThreshold = ygot.Uint64(150000)
	wredqueumreduni.MaxThreshold = ygot.Uint64(160000)
	wredqueumreduni.EnableEcn = ygot.Bool(true)
	wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(10)
	redprofile := "redprofile8"
	redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
	redqueumred := redqueum.GetOrCreateRed()
	redqueumreduni := redqueumred.GetOrCreateUniform()
	redqueumreduni.MinThreshold = ygot.Uint64(120000)
	redqueumreduni.MaxThreshold = ygot.Uint64(130000)
	redqueumreduni.EnableEcn = ygot.Bool(true)
	redqueum1 := qos.GetOrCreateQueueManagementProfile("redprofile9")
	redqueumred1 := redqueum1.GetOrCreateRed()
	redqueumreduni1 := redqueumred1.GetOrCreateUniform()
	redqueumreduni1.MinThreshold = ygot.Uint64(120000)
	redqueumreduni1.MaxThreshold = ygot.Uint64(130000)
	redqueumreduni1.EnableEcn = ygot.Bool(true)
	//dut.Config().Qos().QueueManagementProfile(*wredqueum.Name).Wred().Replace(t, wredqueumred)
	config1 := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name)
	config2 := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name)
	config3 := gnmi.OC().Qos().QueueManagementProfile(*redqueum1.Name)
	t.Run("Step 1 , Update  queue-management-container", func(t *testing.T) {
		gnmi.Update(t, dut, config1.Config(), redqueum)
		gnmi.Update(t, dut, config2.Config(), wredqueum)
		gnmi.Update(t, dut, config3.Config(), redqueum1)
	})
	t.Run("Step 2 , Get one of queue-management-container before delete", func(t *testing.T) {
		configGot1 := gnmi.Get(t, dut, config1.Config())
		if diff := cmp.Diff(*configGot1, *redqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	})
	t.Run("Step 3 , Delete one of queue-management-container and verify with Get", func(t *testing.T) {
		gnmi.Delete(t, dut, config1.Config())
		ConfigGotqos := gnmi.Get(t, dut, gnmi.OC().Qos().Config())

		if diff := cmp.Diff(*ConfigGotqos, *qos); diff == "" {
			t.Errorf("Failed to Delete %v", *redqueum.Name)
		}
	})
	t.Run("Step 4 , add back qm profile back and verify with Get", func(t *testing.T) {
		configuni := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name)
		gnmi.Update(t, dut, configuni.Config(), &oc.Qos_QueueManagementProfile{
			Name: &redprofile,
			Red:  redqueumred,
		})
		configGotUni := gnmi.Get(t, dut, configuni.Config())
		if diff := cmp.Diff(*configGotUni, *redqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
		ConfigGotqos := gnmi.Get(t, dut, gnmi.OC().Qos().Config())
		if diff := cmp.Diff(*ConfigGotqos, *qos); diff != "" {
			t.Errorf("Qm profile %v not  added back", *redqueum.Name)
		}
	})
	//defer teardownQos(t, dut)
	//}
	//dut.Config().Qos().QueueManagementProfile(*queum.Name).Replace(t, queum)
	//create telemetry.QOS as baseconfig from scheduler_base.json

}
