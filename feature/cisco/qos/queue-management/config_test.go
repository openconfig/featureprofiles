package qos_test

import (
	//"fmt"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}
func TestQmWredQosSet(t *testing.T) {
	// testing creation of QM profiler at openconfig/qos
	type qmprofiles struct {
		redprofileName  []string
		wredprfileName  []string
		MinThreshold    []uint64
		MaxThreshold    []uint64
		DropProbability []uint8
		EnableEcn       bool
	}
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
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
	d := &telemetry.Device{}
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

		configqos := dut.Config().Qos()
		configqos.Update(t, qos)
		configGotqos := configqos.Get(t)
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
	d := &telemetry.Device{}
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
	configqos := dut.Config().Qos()
	configqos.Update(t, qos)
	configGotqos := configqos.Get(t)
	if diff := cmp.Diff(*configGotqos, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

}

func TestQueueManagementQosReplace(t *testing.T) {
	t.Run("Step 2 Replace /openconfig/qos and Get qos", func(t *testing.T) {

		dut := ondatra.DUT(t, "dut")
		d := &telemetry.Device{}
		defer teardownQos(t, dut)
		qosr := d.GetOrCreateQos()
		wredqueum1 := qosr.GetOrCreateQueueManagementProfile("wredprofile11")
		wredqueumredr := wredqueum1.GetOrCreateWred()
		wredqueumredunir := wredqueumredr.GetOrCreateUniform()
		wredqueumredunir.MinThreshold = ygot.Uint64(170000)
		wredqueumredunir.MaxThreshold = ygot.Uint64(180000)
		wredqueumredunir.EnableEcn = ygot.Bool(true)
		wredqueumredunir.MaxDropProbabilityPercent = ygot.Uint8(10)
		configqosreplace := dut.Config().Qos()
		configqosreplace.Replace(t, qosr)
		configGotqos := configqosreplace.Get(t)
		if diff := cmp.Diff(*configGotqos, *qosr); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	})
}
func TestQMQmwredSet(t *testing.T) {
	//Replace/Get and  at /openconfig/qos/queue-management-profile
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
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
	d := &telemetry.Device{}
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
		configqm := dut.Config().Qos().QueueManagementProfile(*wredqueum.Name)
		configqm.Replace(t, wredqueum)
		configGotQM := configqm.Get(t)
		if diff := cmp.Diff(*configGotQM, *wredqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
	configqos := dut.Config().Qos()
	configGotqos := configqos.Get(t)
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
	configUpdate := dut.Config().Qos().QueueManagementProfile(*wredqueum5.Name)
	configUpdate.Update(t, wredqueum5)
	configGot := configUpdate.Get(t)
	if diff := cmp.Diff(*configGot, *wredqueum5); diff != "" {
		t.Errorf("Config Schedule update failed: \n%v", diff)
	}

}

func TestQMQmredSet(t *testing.T) {
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
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
	d := &telemetry.Device{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
		configqm := dut.Config().Qos().QueueManagementProfile(*redqueum.Name)
		configqm.Replace(t, redqueum)
		configGotQM := configqm.Get(t)
		if diff := cmp.Diff(*configGotQM, *redqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
	configqos := dut.Config().Qos()
	configGotqos := configqos.Get(t)
	if diff := cmp.Diff(*configGotqos, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	redqueum1 := qos.GetOrCreateQueueManagementProfile("redprofile9")
	redqueumred1 := redqueum1.GetOrCreateRed()
	redqueumreduni1 := redqueumred1.GetOrCreateUniform()
	redqueumreduni1.MinThreshold = ygot.Uint64(120000)
	redqueumreduni1.MaxThreshold = ygot.Uint64(130000)
	configred := dut.Config().Qos().QueueManagementProfile(*redqueum1.Name)
	configred.Update(t, redqueum1)
	configGot := configred.Get(t)
	if diff := cmp.Diff(*configGot, *redqueum1); diff != "" {
		t.Errorf("Config Schedule update failed: \n%v", diff)
	}
	redqueumreduni1.EnableEcn = ygot.Bool(true)
	configredecn := dut.Config().Qos().QueueManagementProfile(*redqueum1.Name)
	configredecn.Update(t, redqueum1)
	configGotEcn := configredecn.Get(t)
	if diff := cmp.Diff(*configGotEcn, *redqueum1); diff != "" {
		t.Errorf("Config Schedule update failed: \n%v", diff)
	}

}

// func TestDeleteQos(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	dut.Config().Qos().Delete(t)

// }

func TestQMWredSetReplace(t *testing.T) {
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
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
	d := &telemetry.Device{}
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
		configqm := dut.Config().Qos().QueueManagementProfile(*wredqueum.Name).Wred()
		configqm.Replace(t, wredqueumred)
		configGotQM := configqm.Get(t)
		if diff := cmp.Diff(*configGotQM, *wredqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}

}

func TestQMWredUniReplace(t *testing.T) {

	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
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
	d := &telemetry.Device{}
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
		configqm := dut.Config().Qos().QueueManagementProfile(*wredqueum.Name).Wred().Uniform()
		configqm.Replace(t, wredqueumreduni)
		configGotQM := configqm.Get(t)
		if diff := cmp.Diff(*configGotQM, *wredqueumreduni); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}

}

func TestQMWredSetUpdate(t *testing.T) {
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
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
	d := &telemetry.Device{}
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
		configqm := dut.Config().Qos().QueueManagementProfile(*wredqueum.Name).Wred()
		configqm.Update(t, wredqueumred)
		configGotQM := configqm.Get(t)
		if diff := cmp.Diff(*configGotQM, *wredqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}

}

func TestQMWredUniUpdate(t *testing.T) {
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
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
	d := &telemetry.Device{}
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
		configqm := dut.Config().Qos().QueueManagementProfile(*wredqueum.Name).Wred().Uniform()
		configqm.Update(t, wredqueumreduni)
		configGotQM := configqm.Get(t)
		if diff := cmp.Diff(*configGotQM, *wredqueumreduni); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
}

func TestQMRedReplace(t *testing.T) {

	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
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
	d := &telemetry.Device{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
		configqm := dut.Config().Qos().QueueManagementProfile(*redqueum.Name).Red()
		configqm.Replace(t, redqueumred)
		configGotQM := configqm.Get(t)
		if diff := cmp.Diff(*configGotQM, *redqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
}

func TestQMRedSetUpdate(t *testing.T) {
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
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
	d := &telemetry.Device{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
		configqm := dut.Config().Qos().QueueManagementProfile(*redqueum.Name).Red()
		configqm.Update(t, redqueumred)
		configGotQM := configqm.Get(t)
		if diff := cmp.Diff(*configGotQM, *redqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
}

func TestQmUpdateEcn(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	wredqueum := qos.GetOrCreateQueueManagementProfile("wredprofile11")
	wredqueumred := wredqueum.GetOrCreateWred()
	wredqueumreduni := wredqueumred.GetOrCreateUniform()
	wredqueumreduni.MinThreshold = ygot.Uint64(150000)
	wredqueumreduni.MaxThreshold = ygot.Uint64(160000)
	wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(10)
	redqueum := qos.GetOrCreateQueueManagementProfile("redprofile8")
	redqueumred := redqueum.GetOrCreateRed()
	redqueumreduni := redqueumred.GetOrCreateUniform()
	redqueumreduni.MinThreshold = ygot.Uint64(120000)
	redqueumreduni.MaxThreshold = ygot.Uint64(130000)
	config1 := dut.Config().Qos().QueueManagementProfile(*redqueum.Name).Wred().Uniform()
	config1.Update(t, wredqueumreduni)
	config2 := dut.Config().Qos().QueueManagementProfile(*wredqueum.Name).Red().Uniform()
	config2.Update(t, redqueumreduni)
	configsetwredecn := dut.Config().Qos().QueueManagementProfile(*redqueum.Name).Wred().Uniform().EnableEcn()
	configsetwredecn.Update(t, true)
	ConfigGetEcn := dut.Config().Qos().QueueManagementProfile(*redqueum.Name).Get(t)
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
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	wredqueum := qos.GetOrCreateQueueManagementProfile("wredprofile11")
	wredqueumred := wredqueum.GetOrCreateWred()
	wredqueumreduni := wredqueumred.GetOrCreateUniform()
	wredqueumreduni.MinThreshold = ygot.Uint64(150000)
	wredqueumreduni.MaxThreshold = ygot.Uint64(160000)
	wredqueumreduni.EnableEcn = ygot.Bool(true)
	wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(10)
	redqueum := qos.GetOrCreateQueueManagementProfile("redprofile8")
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
	config1 := dut.Config().Qos().QueueManagementProfile(*redqueum.Name)
	config2 := dut.Config().Qos().QueueManagementProfile(*wredqueum.Name)
	config3 := dut.Config().Qos().QueueManagementProfile(*redqueum1.Name)
	t.Run("Step 1 , Update  queue-management-container", func(t *testing.T) {
		config1.Update(t, redqueum)
		config2.Update(t, wredqueum)
		config3.Update(t, redqueum1)
	})
	t.Run("Step 2 , Get one of queue-management-container before delete", func(t *testing.T) {
		configGot1 := config1.Get(t)
		if diff := cmp.Diff(*configGot1, *redqueum); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	})
	t.Run("Step 3 , Delete one of queue-management-container and verify with Get", func(t *testing.T) {
		config1.Delete(t)
		ConfigGotqos := dut.Config().Qos().Get(t)

		if diff := cmp.Diff(*ConfigGotqos, *qos); diff == "" {
			t.Errorf("Failed to Delete %v", *redqueum.Name)
		}
	})
	t.Run("Step 4 , add back qm profile back and verify with Get", func(t *testing.T) {
		configuni := dut.Config().Qos().QueueManagementProfile(*redqueum.Name).Red().Uniform()
		configuni.Update(t, redqueumreduni)
		configGotUni := configuni.Get(t)
		if diff := cmp.Diff(*configGotUni, *redqueumreduni); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
		ConfigGotqos := dut.Config().Qos().Get(t)
		if diff := cmp.Diff(*ConfigGotqos, *qos); diff != "" {
			t.Errorf("Qm profile %v not  added back", *redqueum.Name)
		}
	})
	//defer teardownQos(t, dut)
	//}
	//dut.Config().Qos().QueueManagementProfile(*queum.Name).Replace(t, queum)
	//create telemetry.QOS as baseconfig from scheduler_base.json

}
