package qos_test

import (
	//"fmt"

	//"fmt"
	//"fmt"

	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestSchedQueue(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	defer teardownQos(t, dut)
	qos := d.GetOrCreateQos()
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Replace(t, q1)
	}

}

func TestSchedReplaceSched(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	//defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	//Replace at /qos/schedulerpolicy/scheduler(seq)
	schedqueues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = telemetry.Scheduler_Priority_STRICT

	var ind uint64
	ind = 0
	for _, schedqueue := range schedqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1
	}
	Config := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	Config.Replace(t, schedule)
	ConfigGot := Config.Get(t)
	if diff := cmp.Diff(*ConfigGot, *schedule); diff != "" {
		t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	}
	ConfigGotQos := dut.Config().Qos().Get(t)
	if diff := cmp.Diff(*ConfigGotQos, *qos); diff != "" {
		t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	}

	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := dut.Config().Qos().Interface(*schedinterface.InterfaceId)
	configIntf.Update(t, schedinterface)
	configGet := configIntf.Get(t)
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	ConfigGotQosRep := dut.Config().Qos().Get(t)
	if diff := cmp.Diff(*ConfigGotQosRep, *qos); diff != "" {
		t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	}
	schedqueuesrep := []string{"tc7", "tc6", "tc5", "tc4", "tc3"}
	schedulerpolrep := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedulerep := schedulerpolrep.GetOrCreateScheduler(1)
	schedulerep.Priority = telemetry.Scheduler_Priority_STRICT

	var indrep uint64
	indrep = 0
	for _, schedqueuerep := range schedqueuesrep {
		input := schedulerep.GetOrCreateInput(schedqueuerep)
		input.Id = ygot.String(schedqueuerep)
		input.Weight = ygot.Uint64(7 - indrep)
		input.Queue = ygot.String(schedqueuerep)
		indrep += 1
	}
	ConfigRep := dut.Config().Qos().SchedulerPolicy(*schedulerpolrep.Name).Scheduler(1)
	Config.Replace(t, schedulerep)
	ConfigGotRep := ConfigRep.Get(t)
	if diff := cmp.Diff(*ConfigGotRep, *schedulerep); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

}
func TestDeleteQos(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dut.Config().Qos().Delete(t)
}

func TestSchedSchedReplaceSchedPolDelQueue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	//defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	schedqueues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = telemetry.Scheduler_Priority_STRICT
	//schedule.Priority = telemetry.E_Scheduler_Priority(*ygot.Int64(1))

	var ind uint64
	ind = 0
	for _, schedqueue := range schedqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1
	}
	configsched := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name)
	configsched.Replace(t, schedulerpol)
	configGotsched := configsched.Get(t)
	if diff := cmp.Diff(*configGotsched, *schedulerpol); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()

	//schedinterface.Output.SchedulerPolicy.Name = ygot.String("eg_policy1111")
	//dut.Config().Qos().Interface(*schedinterface.InterfaceId).Output().SchedulerPolicy().Name().Update(t, "eg_policy1111")
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := dut.Config().Qos().Interface(*schedinterface.InterfaceId)
	configIntf.Update(t, schedinterface)
	configGet := configIntf.Get(t)
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	config := dut.Config().Qos().SchedulerPolicy("eg_policy1111").Scheduler(1).Input("tc5")
	t.Run(" Delete the queue from Input queue", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			config.Delete(t) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})

}

func TestSchedUpdate(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	schedqueues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = telemetry.Scheduler_Priority_STRICT
	//schedule.Priority = telemetry.E_Scheduler_Priority(*ygot.Int64(1))

	var ind uint64
	ind = 0
	for _, schedqueue := range schedqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1
	}
	configsched := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	configsched.Update(t, schedule)
	configGotsched := configsched.Get(t)
	if diff := cmp.Diff(*configGotsched, *schedule); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()

	//schedinterface.Output.SchedulerPolicy.Name = ygot.String("eg_policy1111")
	//dut.Config().Qos().Interface(*schedinterface.InterfaceId).Output().SchedulerPolicy().Name().Update(t, "eg_policy1111")
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := dut.Config().Qos().Interface(*schedinterface.InterfaceId)
	configIntf.Update(t, schedinterface)
	configGet := configIntf.Get(t)
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Interface fail: \n%v", diff)
	}

}

func TestMultipeSchedUpdateInput(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	//defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = telemetry.Scheduler_Priority_STRICT
	var ind uint64
	//dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Priority().Update(t, telemetry.Scheduler_Priority_STRICT)
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1
		// configInput := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input(*input.Id)
		// configInput.Update(t, input)
		// InputGet := configInput.Get(t)
		// if diff := cmp.Diff(*InputGet, *input); diff != "" {
		// 	t.Errorf("Config Input fail: \n%v", diff)
		// }
	}
	configprior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	configprior.Update(t, schedule)
	configGotprior := configprior.Get(t)
	if diff := cmp.Diff(*configGotprior, *schedule); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	inputupd := schedule.GetOrCreateInput("tc5")
	inputupd.Id = ygot.String("tc5")
	inputupd.Weight = ygot.Uint64(5)
	inputupd.Queue = ygot.String("tc5")
	dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input("tc5").Update(t, inputupd)

	nonpriorqueues := []string{"tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = telemetry.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10
		configInputwrr := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		configInputwrr.Update(t, inputwrr)
		// configGotwrr := configInputwrr.Get(t)
		// if diff := cmp.Diff(*configGotwrr, *inputwrr); diff != "" {
		// 	t.Errorf("Config Input fail: \n%v", diff)
		// }

	}
	confignonprior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
	// confignonprior.Update(t, schedulenonprior)
	configGotnonprior := confignonprior.Get(t)
	if diff := cmp.Diff(*configGotnonprior, *schedulenonprior); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := dut.Config().Qos().Interface(*schedinterface.InterfaceId)
	configIntf.Update(t, schedinterface)
	configGet := configIntf.Get(t)
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Interface fail: \n%v", diff)
	}

}

func TestMultipeSchedReplaceSchuduler(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	//Replace with multiple sequence number at /qos/schedulerpol/schduler(seq)

	t.Run("Config/Replace and get  queues at schduler sequences", func(t *testing.T) {
		priorqueues := []string{"tc7", "tc6"}
		schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
		schedule := schedulerpol.GetOrCreateScheduler(1)
		schedule.Priority = telemetry.Scheduler_Priority_STRICT
		var ind uint64
		ind = 0
		for _, schedqueue := range priorqueues {
			input := schedule.GetOrCreateInput(schedqueue)
			input.Id = ygot.String(schedqueue)
			input.Queue = ygot.String(schedqueue)
			input.Weight = ygot.Uint64(7 - ind)

			ind += 1

		}
		ConfigSchedule := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
		t.Logf("Configuring schduler with sequence one")
		ConfigSchedule.Replace(t, schedule)
		ConfigGetSchedule := ConfigSchedule.Get(t)
		if diff := cmp.Diff(*ConfigGetSchedule, *schedule); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

		nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
		schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
		schedulenonprior.Priority = telemetry.Scheduler_Priority_UNSET
		var weight uint64
		weight = 0
		for _, wrrqueue := range nonpriorqueues {
			inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
			inputwrr.Id = ygot.String(wrrqueue)
			inputwrr.Weight = ygot.Uint64(60 - weight)
			inputwrr.Queue = ygot.String(wrrqueue)
			weight += 10

		}
		configScheNonPrior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
		t.Logf("Configuring schduler with sequence two")
		configScheNonPrior.Replace(t, schedulenonprior)
		configGotNonprior := configScheNonPrior.Get(t)
		if diff := cmp.Diff(*configGotNonprior, *schedulenonprior); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	})
	t.Run("Apply Configs to interfaces", func(t *testing.T) {
		t.Logf("Replacing the existing Configs at sequnence level")

		schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
		schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
		schedinterfaceout := schedinterface.GetOrCreateOutput()
		scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
		configIntf := dut.Config().Qos().Interface(*schedinterface.InterfaceId)
		configIntf.Update(t, schedinterface)
		configGet := configIntf.Get(t)
		if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
			t.Errorf("Config Interface fail: \n%v", diff)
		}
		ConfigGotQos := dut.Config().Qos().Get(t)
		if diff := cmp.Diff(*ConfigGotQos, *qos); diff != "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}
	})
	t.Run("Config/Replace existing Configs at scheduler sequence", func(t *testing.T) {
		dut1 := ondatra.DUT(t, "dut")
		d1 := &telemetry.Device{}
		qosrep := d1.GetOrCreateQos()
		schedulerpolred := qosrep.GetOrCreateSchedulerPolicy("eg_policy1111")
		nonpriorqueuesrep := []string{"tc4", "tc3", "tc2"}
		schedulenonpriorrep := schedulerpolred.GetOrCreateScheduler(2)
		schedulenonpriorrep.Priority = telemetry.Scheduler_Priority_UNSET
		var weightrep uint64
		weightrep = 0
		for _, wrrqueuered := range nonpriorqueuesrep {
			inputwrrred := schedulenonpriorrep.GetOrCreateInput(wrrqueuered)
			inputwrrred.Id = ygot.String(wrrqueuered)
			inputwrrred.Weight = ygot.Uint64(60 - weightrep)
			inputwrrred.Queue = ygot.String(wrrqueuered)
			weightrep += 10

		}
		configScheNonPriorRep := dut1.Config().Qos().SchedulerPolicy("eg_policy1111").Scheduler(2)
		configScheNonPriorRep.Replace(t, schedulenonpriorrep)
		configGotNonpriorRep := configScheNonPriorRep.Get(t)
		if diff := cmp.Diff(*configGotNonpriorRep, *schedulenonpriorrep); diff != "" {
			t.Errorf("Config Schedule fail after replace: \n%v", diff)
		}
		ConfigGotQosRep := dut.Config().Qos().Get(t)
		if diff := cmp.Diff(*ConfigGotQosRep, *qos); diff == "" {
			t.Errorf("Replace with config at scheduler not working as expected \n%v", diff)
		}
	})
}
func TestMultipeSchedDelSeqtwo(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	//defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	//Replace with multiple sequence number at /qos/schedulerpol/schduler(seq)

	t.Run("Config/Replace and get  queues at schduler sequences", func(t *testing.T) {
		priorqueues := []string{"tc7", "tc6"}
		schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
		schedule := schedulerpol.GetOrCreateScheduler(1)
		schedule.Priority = telemetry.Scheduler_Priority_STRICT
		var ind uint64
		ind = 0
		for _, schedqueue := range priorqueues {
			input := schedule.GetOrCreateInput(schedqueue)
			input.Id = ygot.String(schedqueue)
			input.Queue = ygot.String(schedqueue)
			input.Weight = ygot.Uint64(7 - ind)

			ind += 1

		}
		ConfigSchedule := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
		t.Logf("Configuring schduler with sequence one")
		ConfigSchedule.Replace(t, schedule)
		ConfigGetSchedule := ConfigSchedule.Get(t)
		if diff := cmp.Diff(*ConfigGetSchedule, *schedule); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

		nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
		schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
		schedulenonprior.Priority = telemetry.Scheduler_Priority_UNSET
		var weight uint64
		weight = 0
		for _, wrrqueue := range nonpriorqueues {
			inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
			inputwrr.Id = ygot.String(wrrqueue)
			inputwrr.Weight = ygot.Uint64(60 - weight)
			inputwrr.Queue = ygot.String(wrrqueue)
			weight += 10

		}
		configScheNonPrior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
		t.Logf("Configuring schduler with sequence two")
		configScheNonPrior.Replace(t, schedulenonprior)
		configGotNonprior := configScheNonPrior.Get(t)
		if diff := cmp.Diff(*configGotNonprior, *schedulenonprior); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
		schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
		schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
		schedinterfaceout := schedinterface.GetOrCreateOutput()
		scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
		configIntf := dut.Config().Qos().Interface(*schedinterface.InterfaceId)
		configIntf.Update(t, schedinterface)
		configGet := configIntf.Get(t)
		if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
			t.Errorf("Config Interface fail: \n%v", diff)
		}
		ConfigGotQos := dut.Config().Qos().Get(t)
		if diff := cmp.Diff(*ConfigGotQos, *qos); diff != "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}
		t.Logf("Deleting Sequence number 2")
		configScheNonPrior.Delete(t)
		ConfigGotQosAfterDel := dut.Config().Qos().Get(t)
		if diff := cmp.Diff(*ConfigGotQosAfterDel, *qos); diff == "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}
		configScheNonPrior.Update(t, schedulenonprior)
		if diff := cmp.Diff(*ConfigGotQos, *qos); diff != "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}

	})

}

func TestMultipeSchedPolDelete(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	//defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = telemetry.Scheduler_Priority_STRICT
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
	schedulenonprior.Priority = telemetry.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	configprior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name)
	configprior.Update(t, schedulerpol)
	configGotprior := configprior.Get(t)
	if diff := cmp.Diff(*configGotprior, *schedulerpol); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := dut.Config().Qos().Interface(*schedinterface.InterfaceId)
	configIntf.Update(t, schedinterface)
	configGet := configIntf.Get(t)
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Interface fail: \n%v", diff)
	}
	t.Run("Delete the queue 5 ", func(t *testing.T) {
		dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input("tc5").Delete(t)
	})
	configGotpriorafterDel := configprior.Get(t)
	if diff := cmp.Diff(*configGotpriorafterDel, *schedulerpol); diff == "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	t.Run("Re add queue  5 ", func(t *testing.T) {
		inputafdel := schedulenonprior.GetOrCreateInput("tc5")
		inputafdel.Id = ygot.String("tc5")
		inputafdel.Queue = ygot.String("tc5")
		inputafdel.Weight = ygot.Uint64(61)
		dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input("tc5").Replace(t, inputafdel)

	})
	t.Run("Re add queue  5 ", func(t *testing.T) {
		dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input("tc4").Weight().Update(t, 55)
	})

}

func TestMultipeSchedReplaceSchedPol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	t.Run("Config/Replace and get  queues at schduler sequences", func(t *testing.T) {
		priorqueues := []string{"tc7", "tc6"}
		schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
		schedule := schedulerpol.GetOrCreateScheduler(1)
		schedule.Priority = telemetry.Scheduler_Priority_STRICT
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
		schedulenonprior.Priority = telemetry.Scheduler_Priority_UNSET
		var weight uint64
		weight = 0
		for _, wrrqueue := range nonpriorqueues {
			inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
			inputwrr.Id = ygot.String(wrrqueue)
			inputwrr.Queue = ygot.String(wrrqueue)
			inputwrr.Weight = ygot.Uint64(60 - weight)
			weight += 10
			//configInputwrr := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
			//configInputwrr.Update(t, inputwrr)
		}
		ConfigSchedPol := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name)
		ConfigSchedPol.Replace(t, schedulerpol)
		ConfigSchedPolGet := ConfigSchedPol.Get(t)
		if diff := cmp.Diff(*ConfigSchedPolGet, *schedulerpol); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

		ConfigGotQosRep := dut.Config().Qos().Get(t)
		if diff := cmp.Diff(*ConfigGotQosRep, *qos); diff != "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}
	})
	t.Run("Replace the scheduler policy", func(t *testing.T) {
		dut1 := ondatra.DUT(t, "dut")
		d1 := &telemetry.Device{}
		qosrep := d1.GetOrCreateQos()
		schedulerpolred := qosrep.GetOrCreateSchedulerPolicy("eg_policy1111")
		nonpriorqueuesrep := []string{"tc4", "tc3", "tc2"}
		schedulenonpriorrep := schedulerpolred.GetOrCreateScheduler(2)
		schedulenonpriorrep.Priority = telemetry.Scheduler_Priority_UNSET
		var weightrep uint64
		weightrep = 0
		for _, wrrqueuered := range nonpriorqueuesrep {
			inputwrrred := schedulenonpriorrep.GetOrCreateInput(wrrqueuered)
			inputwrrred.Id = ygot.String(wrrqueuered)
			inputwrrred.Weight = ygot.Uint64(60 - weightrep)
			inputwrrred.Queue = ygot.String(wrrqueuered)
			weightrep += 10

		}
		configScheNonPriorRep := dut1.Config().Qos().SchedulerPolicy("eg_policy1111")
		configScheNonPriorRep.Replace(t, schedulerpolred)
		configGotNonpriorRep := configScheNonPriorRep.Get(t)
		if diff := cmp.Diff(*configGotNonpriorRep, *schedulerpolred); diff != "" {
			t.Errorf("Config Schedule fail after replace: \n%v", diff)
		}
		t.Run("Apply Configs to interfaces", func(t *testing.T) {
			t.Logf("Replacing the existing Configs at sequnence level")

			schedinterface := qosrep.GetOrCreateInterface("Bundle-Ether121")
			schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
			schedinterfaceout := schedinterface.GetOrCreateOutput()
			scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
			scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
			configIntf := dut1.Config().Qos().Interface(*schedinterface.InterfaceId)
			ExpectedErrMsg := "does not have traffic-class 7 set to priority 1"
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				configIntf.Update(t, schedinterface) //catch the error  as it is expected and absorb the panic.
			}); errMsg != nil && strings.Contains(*errMsg, ExpectedErrMsg) {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This update should have failed ")
			}

			ConfigGotQos := dut.Config().Qos().Get(t)
			if diff := cmp.Diff(*ConfigGotQos, *qos); diff == "" {
				t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
			}
		})

	})

	//dut.Config().Qos().SchedulerPolicy("eg_policy1111").Scheduler(2).Delete(t)

}

func TestMultipeSchedDeleteShedPol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	//defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = telemetry.Scheduler_Priority_STRICT
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
	schedulenonprior.Priority = telemetry.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10

	}
	configprior := dut.Config().Qos()
	configprior.Update(t, qos)
	configGotprior := configprior.Get(t)
	if diff := cmp.Diff(*configGotprior, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Delete(t)
	configGotpriorAfterDel := configprior.Get(t)
	if diff := cmp.Diff(*configGotpriorAfterDel, *qos); diff == "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Update(t, schedulerpol)
	if diff := cmp.Diff(*configGotprior, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}

}
