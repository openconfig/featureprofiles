package qos_test

import (
	//"fmt"

	//"fmt"
	//"fmt"

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
	defer teardownQos(t, dut)
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
	schedqueuesrep := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2"}
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

}
func TestDeleteQos(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dut.Config().Qos().Delete(t)
}

func TestSchedSchedReplaceSchedPolDelQueue(t *testing.T) {
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
	configsched.Replace(t, schedule)
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
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1
		//dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input(*input.Id).Update(t, input)
		dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Priority().Update(t, schedule.Priority)
		dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Sequence().Update(t, *schedule.Sequence)
		configInput := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input(*input.Id)
		configInput.Update(t, input)
		InputGet := configInput.Get(t)
		if diff := cmp.Diff(*InputGet, *input); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}
	}
	configprior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	//configprior.Update(t, schedule)
	configGotprior := configprior.Get(t)
	if diff := cmp.Diff(*configGotprior, *schedule); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
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
		weight += 10
		configInputwrr := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		configInputwrr.Update(t, inputwrr)
		configGotwrr := configInputwrr.Get(t)
		if diff := cmp.Diff(*configGotwrr, *inputwrr); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

	}
	confignonprior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
	// confignonprior.Update(t, schedulenonprior)
	configGotnonprior := confignonprior.Get(t)
	if diff := cmp.Diff(*configGotnonprior, *schedulenonprior); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	// schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	// schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	// schedinterfaceout := schedinterface.GetOrCreateOutput()
	// scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	// scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	// configIntf := dut.Config().Qos().Interface(*schedinterface.InterfaceId)
	// configIntf.Update(t, schedinterface)
	// configGet := configIntf.Get(t)
	// if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
	// 	t.Errorf("Config Interface fail: \n%v", diff)
	// }

}

func TestMultipeSchedUpdateSchuduler(t *testing.T) {
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

	t.Run("Config and get  queues at schduler sequences", func(t *testing.T) {
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

		nonpriorqueues := []string{"tc4", "tc3", "tc2", "tc1"}
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
			// configInputwrr := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
			// configInputwrr.Update(t, inputwrr)
		}
		configScheNonPrior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
		t.Logf("Configuring schduler with sequence two")
		configScheNonPrior.Replace(t, schedulenonprior)
		configGotprior := configScheNonPrior.Get(t)
		if diff := cmp.Diff(*configGotprior, *schedulenonprior); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	})
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

func TestMultipeSchedDeleteInput(t *testing.T) {
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
		dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input(*input.Id).Update(t, input)
		// configInput := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1).Input(*input.Id)
		// configInput.Update(t, input)
		// InputGet := configInput.Get(t)
		// if diff := cmp.Diff(*InputGet, *input); diff != "" {
		// 	t.Errorf("Config Input fail: \n%v", diff)
		// }
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
		configInputwrr := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		configInputwrr.Update(t, inputwrr)
	}
	// configprior := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name)
	// //configprior.Update(t, schedulerpol)
	// configGotprior := configprior.Get(t)
	// if diff := cmp.Diff(*configGotprior, *schedulerpol); diff != "" {
	// 	t.Errorf("Config Schedule fail: \n%v", diff)
	// }
	t.Run("Delete the queue 5 ", func(t *testing.T) {
		dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input("tc5").Delete(t)
	})
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

func TestMultipeSchedDeleteSequence(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		dut.Config().Qos().Queue(*q1.Name).Update(t, q1)
	}

	// t.Run("Delete Sequnce number 2 ", func(t *testing.T) {
	// 	dut.Config().Qos().SchedulerPolicy("eg_policy1111").Scheduler(2).Delete(t)
	// })
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
		//configInputwrr := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		//configInputwrr.Update(t, inputwrr)
	}
	ConfigSchedPol := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name)
	ConfigSchedPol.Replace(t, schedulerpol)
	ConfigSchedPolGet := ConfigSchedPol.Get(t)
	if diff := cmp.Diff(*ConfigSchedPolGet, *schedulerpol); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	//dut.Config().Qos().SchedulerPolicy("eg_policy1111").Scheduler(2).Delete(t)

}
