package qos_test

import (
	//"fmt"

	"context"
	"fmt"

	//"strings"
	"testing"

	"strings"

	"github.com/google/go-cmp/cmp"
	//"github.com/openconfig/featureprofiles/tools/inputcisco/proto"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

type testArgs struct {
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	top        *ondatra.ATETopology
	interfaces *interfaces
	usecase    int
	prefix     *gribiPrefix
}

type interfaces struct {
	in  []string
	out []string
}

type gribiPrefix struct {
	scale int

	host string

	vrfName         string
	vipPrefixLength string

	vip1Ip string
	vip2Ip string

	vip1NhIndex  uint64
	vip1NhgIndex uint64

	vip2NhIndex  uint64
	vip2NhgIndex uint64

	vrfNhIndex  uint64
	vrfNhgIndex uint64
}

func TestQmRedPrSetReplaceQueue(t *testing.T) {
	//Configure red profiles
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}

	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
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
	gnmi.Replace(t, dut, configqos.Config(), qos)
	configGotqos := gnmi.GetConfig(t, dut, configqos.Config())
	if diff := cmp.Diff(*configGotqos, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	// Step2 scheduler policies and apply it to interface
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	var ind uint64
	ind = 0
	for i, schedqueue := range queues {
		schedule := schedulerpol.GetOrCreateScheduler(uint32(i))
		schedule.Priority = oc.Scheduler_Priority_STRICT
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Queue = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		ind += 1

	}
	ConfigSced := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
	gnmi.Replace(t, dut, ConfigSced.Config(), schedulerpol)
	ConfigGotSched := gnmi.GetConfig(t, dut, ConfigSced.Config())
	if diff := cmp.Diff(*ConfigGotSched, *schedulerpol); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")

	gnmi.Replace(t, dut, gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().SchedulerPolicy().Name().Config(), "eg_policy1111")
	ConfigGetIntf := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Interface("Bundle-Ether121").Config())
	if diff := cmp.Diff(*ConfigGetIntf, *schedinterface); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	priorqueus := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, priorque := range priorqueus {
		queueout := schedinterfaceout.GetOrCreateQueue(priorque)
		queueout.QueueManagementProfile = ygot.String(redprofilelist[i])
		configqm := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().Queue(priorque)
		gnmi.Replace(t, dut, configqm.Config(), queueout)
		configgotqm := gnmi.GetConfig(t, dut, configqm.Config())

		if diff := cmp.Diff(*configgotqm, *queueout); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
	cliHandle1 := dut.RawAPIs().CLI(t)
	defer cliHandle1.Close()
	resp1, err1 := cliHandle1.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp1)
	if err1 != nil {
		t.Error(err1)
	}

	classes1 := []string{}
	rd1 := []string{}
	for j := 1; j < 8; j++ {
		classes1 = append(classes1, fmt.Sprintf("class oc_queue_tc%d", j))
		rd1 = append(rd1, fmt.Sprintf("random-detect%8d bytes%8d bytes ", minthresholdlist[j-1], maxthresholdlist[j-1]))
	}
	for k, class1 := range classes1 {

		if strings.Contains(resp1, rd1[k]) == false || strings.Contains(resp1, class1) == false {
			t.Errorf("expected configs %v are  not there", rd1[k])

		} else {
			t.Logf("Substring present %v", rd1[k])
		}
		fmt.Println(strings.Contains(resp1, rd1[k]))

	}

	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name).Wred()
		gnmi.Replace(t, dut, configqm.Config(), wredqueumred)
		configGotQM := gnmi.GetConfig(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
	for i, priorque := range priorqueus {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(priorque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
		configqmwred := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().Queue(priorque)
		gnmi.Replace(t, dut, configqmwred.Config(), queueoutwred)
		configgotqmwred := gnmi.GetConfig(t, dut, configqmwred.Config())

		if diff := cmp.Diff(*configgotqmwred, *queueoutwred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

}

func TestQmRedWrrSetReplaceQueue(t *testing.T) {
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}

	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

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
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name).Wred()
		gnmi.Replace(t, dut, configqm.Config(), wredqueumred)
		configGotQM := gnmi.GetConfig(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}

	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	configprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	gnmi.Replace(t, dut, configprior.Config(), schedule)
	configGotprior := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotprior, *schedule); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	inputupd := schedule.GetOrCreateInput("tc5")
	inputupd.Id = ygot.String("tc5")
	inputupd.Weight = ygot.Uint64(5)
	inputupd.Queue = ygot.String("tc5")
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input("tc5").Config(), inputupd)

	nonpriorqueues := []string{"tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10
		configInputwrr := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		gnmi.Replace(t, dut, configInputwrr.Config(), inputwrr)
		configGotwrr := gnmi.GetConfig(t, dut, configInputwrr.Config())
		if diff := cmp.Diff(*configGotwrr, *inputwrr); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

	}
	confignonprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
	//confignonprior.Replace(t, schedulenonprior)
	configGotnonprior := gnmi.GetConfig(t, dut, confignonprior.Config())
	if diff := cmp.Diff(*configGotnonprior, *schedulenonprior); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
	gnmi.Replace(t, dut, configIntf.Config(), schedinterface)
	configGetIntf := gnmi.GetConfig(t, dut, configIntf.Config())
	if diff := cmp.Diff(*configGetIntf, *schedinterface); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
		configqmwred := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().Queue(wrrque)
		gnmi.Replace(t, dut, configqmwred.Config(), queueoutwred)
		configgotqmwred := gnmi.GetConfig(t, dut, configqmwred.Config())

		if diff := cmp.Diff(*configgotqmwred, *queueoutwred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
	configGet := gnmi.GetConfig(t, dut, configIntf.Config())
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Interface Get fail: \n%v", diff)
	}
	configGetScedPol := gnmi.GetConfig(t, dut, gnmi.OC().Qos().SchedulerPolicy("eg_policy1111").Config())
	if diff := cmp.Diff(*configGetScedPol, *schedulerpol); diff != "" {
		t.Errorf("Config of wrr red Get fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	defer cliHandle.Close()
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}

	}

	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
		configqm := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name).Red()
		gnmi.Replace(t, dut, configqm.Config(), redqueumred)
		configGotQM := gnmi.GetConfig(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *redqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}

	for i, wrrque := range wrrqueues {
		queueout := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueout.QueueManagementProfile = ygot.String(redprofilelist[i])
		configqm := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().Queue(wrrque)
		gnmi.Replace(t, dut, configqm.Config(), queueout)
		configgotqm := gnmi.GetConfig(t, dut, configqm.Config())

		if diff := cmp.Diff(*configgotqm, *queueout); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
	cliHandle1 := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp1, err1 := cliHandle1.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp1)
	if err1 != nil {
		t.Error(err1)
	}

	classes1 := []string{}
	rd1 := []string{}
	for j := 1; j < 8; j++ {
		classes1 = append(classes1, fmt.Sprintf("class oc_queue_tc%d", j))
		rd1 = append(rd1, fmt.Sprintf("random-detect%8d bytes%8d bytes ", minthresholdlist[j-1], maxthresholdlist[j-1]))
	}
	for k, class1 := range classes1 {

		if strings.Contains(resp1, rd1[k]) == false || strings.Contains(resp1, class1) == false {
			t.Errorf("expected configs %v are  not there", rd1[k])

		} else {
			t.Logf("Substring present %v", rd1[k])
		}
		fmt.Println(strings.Contains(resp1, rd1[k]))
	}

}

func TestQmRedWrrSetReplaceOuput(t *testing.T) {
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}

	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	configureDUT(t, dut)

	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name).Wred()
		gnmi.Replace(t, dut, configqm.Config(), wredqueumred)
		configGotQM := gnmi.GetConfig(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}

	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	configprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	gnmi.Replace(t, dut, configprior.Config(), schedule)
	configGotprior := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotprior, *schedule); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	inputupd := schedule.GetOrCreateInput("tc5")
	inputupd.Id = ygot.String("tc5")
	inputupd.Weight = ygot.Uint64(5)
	inputupd.Queue = ygot.String("tc5")
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input("tc5").Config(), inputupd)

	nonpriorqueues := []string{"tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10
		configInputwrr := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		gnmi.Update(t, dut, configInputwrr.Config(), inputwrr)
		configGotwrr := gnmi.GetConfig(t, dut, configInputwrr.Config())
		if diff := cmp.Diff(*configGotwrr, *inputwrr); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

	}
	confignonprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
	gnmi.Replace(t, dut, confignonprior.Config(), schedulerpol)
	configGotnonprior := gnmi.GetConfig(t, dut, confignonprior.Config())
	if diff := cmp.Diff(*configGotnonprior, *schedulerpol); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

	//dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Update(t, schedulerpol)
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
	}
	ConfigOutput := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output()
	gnmi.Replace(t, dut, ConfigOutput.Config(), schedinterfaceout)
	ConfigOutputGot := gnmi.GetConfig(t, dut, ConfigOutput.Config())
	if diff := cmp.Diff(*ConfigOutputGot, *schedinterfaceout); diff != "" {
		t.Errorf("Config Input fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

	schedulerpolrep := qos.GetOrCreateSchedulerPolicy("eg_policy1112")
	var indrep uint64
	ind = 0
	for i, schedqueue := range queues {
		schedulerep := schedulerpolrep.GetOrCreateScheduler(uint32(i))
		schedulerep.Priority = oc.Scheduler_Priority_STRICT
		inputrep := schedulerep.GetOrCreateInput(schedqueue)
		inputrep.Id = ygot.String(schedqueue)
		inputrep.Queue = ygot.String(schedqueue)
		inputrep.Weight = ygot.Uint64(7 - indrep)
		indrep += 1

	}
	gnmi.Replace(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpolrep.Name).Config(), schedulerpolrep)
	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
		configqm := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name).Red()
		gnmi.Replace(t, dut, configqm.Config(), redqueumred)
		configGotQM := gnmi.GetConfig(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *redqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
	schedinterfaceoutrep := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpolrep := schedinterfaceoutrep.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpolrep.Name = ygot.String("eg_policy1112")
	for i, wrrque := range wrrqueues {
		queueoutred := schedinterfaceoutrep.GetOrCreateQueue(wrrque)
		queueoutred.QueueManagementProfile = ygot.String(redprofilelist[i])
	}
	ConfigOut := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output()
	gnmi.Replace(t, dut, ConfigOut.Config(), schedinterfaceoutrep)
	ConfigGotOut := gnmi.GetConfig(t, dut, ConfigOut.Config())
	if diff := cmp.Diff(*ConfigGotOut, *schedinterfaceoutrep); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

	cliHandle1 := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp1, err1 := cliHandle1.SendCommand(context.Background(), "show running-config policy-map eg_policy1112__intf__Bundle-Ether121 ")
	t.Logf(resp1)
	if err1 != nil {
		t.Error(err1)
	}

	classes1 := []string{}
	rd1 := []string{}
	for j := 1; j < 8; j++ {
		classes1 = append(classes1, fmt.Sprintf("class oc_queue_tc%d", j))
		rd1 = append(rd1, fmt.Sprintf("random-detect%8d bytes%8d bytes ", minthresholdlist[j-1], maxthresholdlist[j-1]))
	}
	for k, class1 := range classes1 {

		if strings.Contains(resp1, rd1[k]) == false || strings.Contains(resp1, class1) == false {
			t.Errorf("expected configs %v are  not there", rd1[k])

		} else {
			t.Logf("Substring present %v", rd1[k])
		}
		fmt.Println(strings.Contains(resp1, rd1[k]))
	}

}

func TestQmRedWrrSetReplaceInterface(t *testing.T) {
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	redprofilelist := []string{}
	for i := 1; i < 8; i++ {
		redprofilelist = append(redprofilelist, fmt.Sprintf("redprofile%d", i))
	}
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	configureDUT(t, dut)

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
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name).Wred().Uniform()
		gnmi.Update(t, dut, configqm.Config(), wredqueumreduni)
		configGotQM := gnmi.GetConfig(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueumreduni); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	configprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	gnmi.Replace(t, dut, configprior.Config(), schedule)
	configGotprior := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotprior, *schedule); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	inputupd := schedule.GetOrCreateInput("tc5")
	inputupd.Id = ygot.String("tc5")
	inputupd.Weight = ygot.Uint64(5)
	inputupd.Queue = ygot.String("tc5")
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input("tc5").Config(), inputupd)

	nonpriorqueues := []string{"tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10
		configInputwrr := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		gnmi.Update(t, dut, configInputwrr.Config(), inputwrr)
		configGotwrr := gnmi.GetConfig(t, dut, configInputwrr.Config())
		if diff := cmp.Diff(*configGotwrr, *inputwrr); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

	}
	confignonprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
	// confignonprior.Update(t, schedulenonprior)
	configGotnonprior := gnmi.GetConfig(t, dut, confignonprior.Config())
	if diff := cmp.Diff(*configGotnonprior, *schedulenonprior); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

	//dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Update(t, schedulerpol)
	interfaceList := []string{}
	for i := 121; i < 128; i++ {
		interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
	}
	for _, inter := range interfaceList {

		schedinterface := qos.GetOrCreateInterface(inter)
		schedinterface.InterfaceId = ygot.String(inter)
		schedinterfaceout := schedinterface.GetOrCreateOutput()
		scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
		wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
		for i, wrrque := range wrrqueues {
			queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
			queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
		}
		ConfigIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
		gnmi.Replace(t, dut, ConfigIntf.Config(), schedinterface)

		ConfigGotIntf := gnmi.GetConfig(t, dut, ConfigIntf.Config())
		if diff := cmp.Diff(*ConfigGotIntf, *schedinterface); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

	schedulerpolrep := qos.GetOrCreateSchedulerPolicy("eg_policy1112")
	var indrep uint64
	ind = 0
	for i, schedqueue := range queues {
		schedulerep := schedulerpolrep.GetOrCreateScheduler(uint32(i))
		schedulerep.Priority = oc.Scheduler_Priority_STRICT
		inputrep := schedulerep.GetOrCreateInput(schedqueue)
		inputrep.Id = ygot.String(schedqueue)
		inputrep.Queue = ygot.String(schedqueue)
		inputrep.Weight = ygot.Uint64(7 - indrep)
		indrep += 1

	}
	gnmi.Replace(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpolrep.Name).Config(), schedulerpolrep)
	for j, redprofile := range redprofilelist {
		redqueum := qos.GetOrCreateQueueManagementProfile(redprofile)
		redqueumred := redqueum.GetOrCreateRed()
		redqueumreduni := redqueumred.GetOrCreateUniform()
		redqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[j])
		redqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[j])
		redqueumreduni.EnableEcn = ygot.Bool(true)
		configqm := gnmi.OC().Qos().QueueManagementProfile(*redqueum.Name).Red()
		gnmi.Replace(t, dut, configqm.Config(), redqueumred)
		configGotQM := gnmi.GetConfig(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *redqueumred); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
	for _, inter := range interfaceList {

		schedinterfacerep := qos.GetOrCreateInterface(inter)
		schedinterfacerep.InterfaceId = ygot.String(inter)
		schedinterfaceoutrep := schedinterfacerep.GetOrCreateOutput()
		scheinterfaceschedpolrep := schedinterfaceoutrep.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpolrep.Name = ygot.String("eg_policy1112")
		wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
		for i, wrrque := range wrrqueues {
			queueoutwred := schedinterfaceoutrep.GetOrCreateQueue(wrrque)
			queueoutwred.QueueManagementProfile = ygot.String(redprofilelist[i])
		}
		ConfigIntf := gnmi.OC().Qos().Interface(*schedinterfacerep.InterfaceId)
		gnmi.Replace(t, dut, ConfigIntf.Config(), schedinterfacerep)

		ConfigGotIntf := gnmi.GetConfig(t, dut, ConfigIntf.Config())
		if diff := cmp.Diff(*ConfigGotIntf, *schedinterfacerep); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}

	cliHandle1 := dut.RawAPIs().CLI(t)
	defer cliHandle1.Close()
	resp1, err1 := cliHandle1.SendCommand(context.Background(), "show running-config policy-map eg_policy1112__intf__Bundle-Ether121 ")
	t.Logf(resp1)
	if err1 != nil {
		t.Error(err1)
	}

	classes1 := []string{}
	rd1 := []string{}
	for j := 1; j < 8; j++ {
		classes1 = append(classes1, fmt.Sprintf("class oc_queue_tc%d", j))
		rd1 = append(rd1, fmt.Sprintf("random-detect%8d bytes%8d bytes ", minthresholdlist[j-1], maxthresholdlist[j-1]))
	}
	for k, class1 := range classes1 {

		if strings.Contains(resp1, rd1[k]) == false || strings.Contains(resp1, class1) == false {
			t.Errorf("expected configs %v are  not there", rd1[k])

		} else {
			t.Logf("Substring present %v", rd1[k])
		}
		fmt.Println(strings.Contains(resp1, rd1[k]))
	}

}

func TestQmRedWrrSetUpdateQos(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()

	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
	}
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	ConfigQosGet := gnmi.GetConfig(t, dut, ConfigQos.Config())

	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

}

func TestQmRedWrrSetUpdateOutput(t *testing.T) {

	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

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
		configqm := gnmi.OC().Qos().QueueManagementProfile(*wredqueum.Name).Wred().Uniform()
		gnmi.Update(t, dut, configqm.Config(), wredqueumreduni)
		configGotQM := gnmi.GetConfig(t, dut, configqm.Config())
		if diff := cmp.Diff(*configGotQM, *wredqueumreduni); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	}
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	configprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	gnmi.Update(t, dut, configprior.Config(), schedule)
	configGotprior := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotprior, *schedule); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	inputupd := schedule.GetOrCreateInput("tc5")
	inputupd.Id = ygot.String("tc5")
	inputupd.Weight = ygot.Uint64(5)
	inputupd.Queue = ygot.String("tc5")
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input("tc5").Config(), inputupd)

	nonpriorqueues := []string{"tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10
		configInputwrr := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		gnmi.Update(t, dut, configInputwrr.Config(), inputwrr)
		configGotwrr := gnmi.GetConfig(t, dut, configInputwrr.Config())
		if diff := cmp.Diff(*configGotwrr, *inputwrr); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

	}
	confignonprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
	// confignonprior.Update(t, schedulenonprior)
	configGotnonprior := gnmi.GetConfig(t, dut, confignonprior.Config())
	if diff := cmp.Diff(*configGotnonprior, *schedulenonprior); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Config(), schedulerpol)
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
	}
	ConfigIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output()
	gnmi.Update(t, dut, ConfigIntf.Config(), schedinterfaceout)

	ConfigGotIntf := gnmi.GetConfig(t, dut, ConfigIntf.Config())
	if diff := cmp.Diff(*ConfigGotIntf, *schedinterfaceout); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	updatequeue := "tc7"
	queueoutupd := schedinterfaceout.GetOrCreateQueue(updatequeue)
	queueoutupd.QueueManagementProfile = ygot.String(wredprofilelist[6])
	UpdateConfig := gnmi.OC().Qos().Interface("Bundle-Ether121").Output().Queue(updatequeue)
	gnmi.Update(t, dut, UpdateConfig.Config(), queueoutupd)
	ConfigGetUpdateQueue := gnmi.GetConfig(t, dut, UpdateConfig.Config())

	if diff := cmp.Diff(*ConfigGetUpdateQueue, *queueoutupd); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

}

func TestQmRedWrrSetDeleteQueue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}

	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	//dut.Config().Qos().Interface("Bundle-Ether121").Output().Replace(t, schedinterfaceout)
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])

	}
	//dut.Config().Qos().Interface("Bundle-Ether121").Replace(t, schedinterface)
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	ConfigQosGet := gnmi.GetConfig(t, dut, ConfigQos.Config())

	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

	gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().Queue("tc7").Config())
	ConfigGetOutput := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().Config())
	if diff := cmp.Diff(*ConfigGetOutput, *schedinterfaceout); diff == "" {
		t.Errorf("Delete failed: \n%v", diff)
	}
	resp2, err2 := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp2)
	if err2 != nil {
		t.Error(err)
	}
	if strings.Contains(resp2, "random-detect 1043008 bytes 1343008 bytes probability percent 19 ") {
		t.Errorf("Delete of queue7 has not happened")
	}

	updatequeue := "tc7"
	queueoutupd := schedinterfaceout.GetOrCreateQueue(updatequeue)
	queueoutupd.QueueManagementProfile = ygot.String(wredprofilelist[6])
	UpdateConfig := gnmi.OC().Qos().Interface("Bundle-Ether121").Output().Queue(updatequeue)
	gnmi.Update(t, dut, UpdateConfig.Config(), queueoutupd)
	ConfigGetUpdateQueue := gnmi.GetConfig(t, dut, UpdateConfig.Config())

	if diff := cmp.Diff(*ConfigGetUpdateQueue, *queueoutupd); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle1 := dut.RawAPIs().CLI(t)
	defer cliHandle1.Close()
	resp1, err1 := cliHandle1.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp1)
	if err1 != nil {
		t.Error(err1)
	}
	classes1 := []string{}
	rd1 := []string{}
	for j := 1; j < 8; j++ {
		classes1 = append(classes1, fmt.Sprintf("class oc_queue_tc%d", j))
		rd1 = append(rd1, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d ", minthresholdlist[j-1], maxthresholdlist[j-1], dropprobablity[j-1]))
	}
	for k, class1 := range classes1 {

		if strings.Contains(resp1, rd1[k]) == false || strings.Contains(resp1, class1) == false {
			t.Errorf("expected configs %v are  not there", rd1[k])

		} else {
			t.Logf("Substring present %v", rd1[k])
		}
		fmt.Println(strings.Contains(resp1, rd1[k]))
	}

}

func TestQmRedWrrSetUpdateWredProfile(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
	}
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	ConfigQosGet := gnmi.GetConfig(t, dut, ConfigQos.Config())

	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}
	wredqueum5 := qos.GetOrCreateQueueManagementProfile("wredprofile5")
	wredqueumred5 := wredqueum5.GetOrCreateWred()
	wredqueumreduni5 := wredqueumred5.GetOrCreateUniform()
	wredqueumreduni5.MinThreshold = ygot.Uint64(614400)
	wredqueumreduni5.MaxThreshold = ygot.Uint64(390070272)
	wredqueumreduni5.EnableEcn = ygot.Bool(true)
	wredqueumreduni5.MaxDropProbabilityPercent = ygot.Uint8(10)
	configUpdate := gnmi.OC().Qos().QueueManagementProfile(*wredqueum5.Name)
	gnmi.Update(t, dut, configUpdate.Config(), wredqueum5)
	ConfigAfterUpdate := gnmi.GetConfig(t, dut, configUpdate.Config())
	if diff := cmp.Diff(*ConfigAfterUpdate, *wredqueum5); diff != "" {
		t.Errorf("Update failed: \n%v", diff)
	}
	resp2, err2 := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp2)
	if err2 != nil {
		t.Error(err)
	}
	if !strings.Contains(resp2, "random-detect 614400 bytes 390070272 bytes probability percent 10") {
		t.Errorf("Update  has not happened to main policy")
	}

}

func TestQmRedWrrSetUpdateWrr(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}

	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
		//dut.Config().Qos().Interface("Bundle-Ether121").Output().Queue(wrrque).Replace(t, queueoutwred)
	}
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	ConfigQosGet := gnmi.GetConfig(t, dut, ConfigQos.Config())

	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()

	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

	updtwrr := schedulenonprior.GetOrCreateInput("tc5")
	updtwrr.Id = ygot.String("tc5")
	updtwrr.Queue = ygot.String("tc5")
	updtwrr.Weight = ygot.Uint64(55)
	ConfigUpdWrr := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*updtwrr.Id)
	gnmi.Update(t, dut, ConfigUpdWrr.Config(), updtwrr)

	resp1, err1 := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp1)
	if err1 != nil {
		t.Error(err1)
	}
	if !strings.Contains(resp1, "bandwidth remaining ratio 55") {
		t.Errorf("update of bandwidth has not happened")
	}

}

func TestQmRedDelSchedIntf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	// ConfigQosGet := ConfigQos.Get(t)

	// if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
	// 	t.Errorf("Config Schedule fail: \n%v", diff)
	// }
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
		gnmi.Replace(t, dut, gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().Queue(wrrque).Config(), queueoutwred)
	}

	gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().Config())
	ConfigPolicyIntf := gnmi.OC().Qos().Interface("Bundle-Ether121").Output()
	t.Run("Delete the wredprofile attached to interface", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.GetConfig(t, dut, ConfigPolicyIntf.Config()) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})

	// //Add back the configs
	ConfigOutput := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output()
	gnmi.Update(t, dut, ConfigOutput.Config(), schedinterfaceout)
	ConfigOutputGet := gnmi.GetConfig(t, dut, ConfigOutput.Config())
	if diff := cmp.Diff(*ConfigOutputGet, *schedinterfaceout); diff != "" {
		t.Errorf("Config delete output fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

}

// Expected failure testcases Begins
func TestDelWredAttchdIntf(t *testing.T) {

	//This tests will try to Delete the wred profile already attached to interface
	//Expected to Fail and will have to capture the Error
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	// ConfigQosGet := ConfigQos.Get(t)

	// if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
	// 	t.Errorf("Config Schedule fail: \n%v", diff)
	// }
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
		gnmi.Update(t, dut, gnmi.OC().Qos().Interface("Bundle-Ether121").Output().Queue(wrrque).Config(), queueoutwred)
	}

	ConfigWredDel := gnmi.OC().Qos().QueueManagementProfile(wredprofilelist[1])
	t.Run("Delete the wredprofile attached to interface", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, ConfigWredDel.Config()) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})

}

func TestRepWredAttchdIntf(t *testing.T) {
	//This test will try to replace the wred profile attached and this will fail
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}

	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	ConfigQosGet := gnmi.GetConfig(t, dut, ConfigQos.Config())

	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
		gnmi.Replace(t, dut, gnmi.OC().Qos().Interface("Bundle-Ether121").Output().Queue(wrrque).Config(), queueoutwred)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

	wredqueum5 := qos.GetOrCreateQueueManagementProfile("wredprofile5")
	wredqueumred5 := wredqueum5.GetOrCreateWred()
	wredqueumreduni5 := wredqueumred5.GetOrCreateUniform()
	wredqueumreduni5.MinThreshold = ygot.Uint64(614400)
	wredqueumreduni5.MaxThreshold = ygot.Uint64(390070272)
	wredqueumreduni5.EnableEcn = ygot.Bool(true)
	wredqueumreduni5.MaxDropProbabilityPercent = ygot.Uint8(17)
	configUpdate := gnmi.OC().Qos().QueueManagementProfile(*wredqueum5.Name)
	gnmi.Replace(t, dut, configUpdate.Config(), wredqueum5)
	ConfigGotUpdate := gnmi.GetConfig(t, dut, configUpdate.Config())
	if diff := cmp.Diff(*ConfigGotUpdate, *wredqueum5); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle1 := dut.RawAPIs().CLI(t)
	defer cliHandle1.Close()
	resp1, err1 := cliHandle1.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp1)
	if err1 != nil {
		t.Error(err1)
	}
	if !strings.Contains(resp1, "random-detect 614400 bytes 390070272 bytes probability percent 17") {
		t.Errorf("Replace of RED not updated to main policy")
	}

}

func TestRepSchedQueueAttchdIntf(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)

	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
		gnmi.Replace(t, dut, gnmi.OC().Qos().Interface("Bundle-Ether121").Output().Queue(wrrque).Config(), queueoutwred)
	}

	ConfigQosGet := gnmi.GetConfig(t, dut, ConfigQos.Config())
	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}

	}

	updtwrr := schedulenonprior.GetOrCreateInput("tc1")
	updtwrr.Id = ygot.String("tc1")
	updtwrr.Queue = ygot.String("tc1")
	updtwrr.Weight = ygot.Uint64(15)
	ConfigUpdWrr := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*updtwrr.Id)

	t.Run(" Replace input queue attached to interface", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Replace(t, dut, ConfigUpdWrr.Config(), updtwrr) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})

}

func TestDelSchedQueueAttchdIntf(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
	}
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	ConfigQosGet := gnmi.GetConfig(t, dut, ConfigQos.Config())

	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

	ConfigUpdWrr := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input("tc1")

	t.Run(" Delete the queue from Input queue", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, ConfigUpdWrr.Config()) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})

	//Add Back

}

func TestRepSchedSeqAttchdIntf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
	}
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	ConfigQosGet := gnmi.GetConfig(t, dut, ConfigQos.Config())

	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

	cliHandle := dut.RawAPIs().CLI(t)
	defer cliHandle.Close()
	resp, err := cliHandle.SendCommand(context.Background(), "show running-config policy-map eg_policy1111__intf__Bundle-Ether121 ")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	classes := []string{}
	rd := []string{}
	for i := 1; i < 8; i++ {
		classes = append(classes, fmt.Sprintf("class oc_queue_tc%d", i))
		rd = append(rd, fmt.Sprintf("random-detect%8d bytes%8d bytes probability percent %d", minthresholdlist[i-1], maxthresholdlist[i-1], dropprobablity[i-1]))
	}
	for i, class := range classes {

		if strings.Contains(resp, class) == false || strings.Contains(resp, rd[i]) == false {
			t.Errorf("expected configs %s are  not there", rd[i])

		} else {
			t.Logf("Substring present")
		}
	}

	nonpriorrepqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonreprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonreprior.Priority = oc.Scheduler_Priority_STRICT

	for _, wrrqueue := range nonpriorrepqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(7 - ind)
		ind += 1
	}
	ConfigRepSeq := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)

	t.Run("Replace the scheduler attached with WRR", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Replace(t, dut, ConfigRepSeq.Config(), schedulenonreprior) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})

}

func TestDelSchedSeqAttchdIntf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), qos)
	interfaceList := []string{}
	for i := 121; i < 128; i++ {
		interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
	}
	for _, inter := range interfaceList {

		schedinterface := qos.GetOrCreateInterface(inter)
		schedinterface.InterfaceId = ygot.String(inter)
		schedinterfaceout := schedinterface.GetOrCreateOutput()
		scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
		wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
		for i, wrrque := range wrrqueues {
			queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
			queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
		}
		ConfigIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
		gnmi.Replace(t, dut, ConfigIntf.Config(), schedinterface)

		ConfigGotIntf := gnmi.GetConfig(t, dut, ConfigIntf.Config())
		if diff := cmp.Diff(*ConfigGotIntf, *schedinterface); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}
	ConfigSeq := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input("tc1")

	t.Run(" Delete the sequence attached with wrr", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, ConfigSeq.Config()) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})

}

func TestDelSchedPolAttchdIntf(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
	}
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	ConfigQosGet := gnmi.GetConfig(t, dut, ConfigQos.Config())
	if diff := cmp.Diff(*ConfigQosGet, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	ConfigSchedPol := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
	t.Run("Delete  the Sequence  attached to the interface", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, ConfigSchedPol.Config()) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})

}

func TestRepSchedPolAttchdIntf(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	d := &oc.Root{}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1

	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	minthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		minthresholdlist = append(minthresholdlist, 1000000+uint64(i*6144))
	}
	maxthresholdlist := []uint64{}
	for i := 1; i < 8; i++ {
		maxthresholdlist = append(maxthresholdlist, 1300000+uint64(i*6144))
	}
	wredprofilelist := []string{}
	for i := 1; i < 8; i++ {
		wredprofilelist = append(wredprofilelist, fmt.Sprintf("wredprofile%d", i))
	}
	dropprobablity := []uint8{}
	for i := 1; i < 8; i++ {
		dropprobablity = append(dropprobablity, 10+uint8(i+2))
	}
	for i, wredprofile := range wredprofilelist {
		wredqueum := qos.GetOrCreateQueueManagementProfile(wredprofile)
		wredqueumred := wredqueum.GetOrCreateWred()
		wredqueumreduni := wredqueumred.GetOrCreateUniform()
		wredqueumreduni.MinThreshold = ygot.Uint64(minthresholdlist[i])
		wredqueumreduni.MaxThreshold = ygot.Uint64(maxthresholdlist[i])
		wredqueumreduni.EnableEcn = ygot.Bool(true)
		wredqueumreduni.MaxDropProbabilityPercent = ygot.Uint8(dropprobablity[i])

	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for i, wrrque := range wrrqueues {
		queueoutwred := schedinterfaceout.GetOrCreateQueue(wrrque)
		queueoutwred.QueueManagementProfile = ygot.String(wredprofilelist[i])
	}
	ConfigQos := gnmi.OC().Qos()
	gnmi.Update(t, dut, ConfigQos.Config(), qos)
	// ConfigQosGet := ConfigQos.Get(t)
	// if diff := cmp.Diff(*ConfigQosGet, *ConfigQos); diff != "" {
	// 	t.Errorf("Config Schedule fail: \n%v", diff)
	// }

	repqueues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}

	repschedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1112")
	var repind uint64
	ind = 0
	for i, schedqueue := range repqueues {
		schedule := repschedulerpol.GetOrCreateScheduler(uint32(i))
		schedule.Priority = oc.Scheduler_Priority_STRICT
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Queue = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - repind)
		repind += 1

	}
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*repschedulerpol.Name).Config(), repschedulerpol)
	// repscheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	// repscheinterfaceschedpol.Name = ygot.String("eg_policy1112")
	ConfigReplSchedInt := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId).Output().SchedulerPolicy().Name()
	gnmi.Replace(t, dut, ConfigReplSchedInt.Config(), "eg_policy1112")
	ConfigGotQosPol := gnmi.GetConfig(t, dut, gnmi.OC().Qos().SchedulerPolicy("eg_policy1112").Config())

	if diff := cmp.Diff(*ConfigGotQosPol, *repschedulerpol); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	cliHandle1 := dut.RawAPIs().CLI(t)
	defer cliHandle1.Close()
	resp1, err1 := cliHandle1.SendCommand(context.Background(), "show running-config policy-map eg_policy1112__intf__Bundle-Ether121 ")
	t.Logf(resp1)
	if err1 != nil {
		t.Error(err1)
	}
	classes1 := []string{}
	priority := []string{}
	for j := 1; j < 8; j++ {
		classes1 = append(classes1, fmt.Sprintf("class oc_queue_tc%d", j))
		priority = append(priority, fmt.Sprintf("priority level %d ", 7-(j-1)))
	}
	for k, class1 := range classes1 {

		if strings.Contains(resp1, priority[k]) == false || strings.Contains(resp1, class1) == false {
			t.Errorf("expected configs %v are  not there", priority[k])

		} else {
			t.Logf("Substring present %v", priority[k])
		}

	}

}
