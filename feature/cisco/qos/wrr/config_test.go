package qos_test

import (
	"fmt"

	"testing"

	"github.com/google/go-cmp/cmp"
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

func TestSchedReplaceSched(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	//Replace at /qos/schedulerpolicy/scheduler(seq)
	schedqueues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	gnmi.Replace(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Config(), schedulerpol)
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT

	var ind uint64
	ind = 0
	for _, schedqueue := range schedqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1
	}
	Config := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	gnmi.Replace(t, dut, Config.Config(), schedule)
	ConfigGot := gnmi.GetConfig(t, dut, Config.Config())
	if diff := cmp.Diff(*ConfigGot, *schedule); diff != "" {
		t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	}
	ConfigGotQos := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
	if diff := cmp.Diff(*ConfigGotQos, *qos); diff != "" {
		t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	}

	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for _, wrrque := range wrrqueues {
		schedinterfaceout.GetOrCreateQueue(wrrque)
	}
	configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
	gnmi.Update(t, dut, configIntf.Config(), schedinterface)
	configGet := gnmi.GetConfig(t, dut, configIntf.Config())
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	ConfigGotQosRep := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
	if diff := cmp.Diff(*ConfigGotQosRep, *qos); diff != "" {
		t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	}
	schedqueuesrep := []string{"tc7", "tc6", "tc5", "tc4", "tc3"}
	schedulerpolrep := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedulerep := schedulerpolrep.GetOrCreateScheduler(1)
	schedulerep.Priority = oc.Scheduler_Priority_STRICT

	var indrep uint64
	indrep = 0
	for _, schedqueuerep := range schedqueuesrep {
		input := schedulerep.GetOrCreateInput(schedqueuerep)
		input.Id = ygot.String(schedqueuerep)
		input.Weight = ygot.Uint64(7 - indrep)
		input.Queue = ygot.String(schedqueuerep)
		indrep += 1
	}
	ConfigRep := gnmi.OC().Qos().SchedulerPolicy(*schedulerpolrep.Name).Scheduler(1)
	gnmi.Replace(t, dut, ConfigRep.Config(), schedulerep)
	ConfigRepGet := gnmi.GetConfig(t, dut, ConfigRep.Config())
	if diff := cmp.Diff(*ConfigRepGet, *schedulerep); diff != "" {
		t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	}

}

func TestSchedSchedReplaceSchedPolDelQueue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	schedqueues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
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
	configsched := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
	gnmi.Replace(t, dut, configsched.Config(), schedulerpol)
	configGotsched := gnmi.GetConfig(t, dut, configsched.Config())
	if diff := cmp.Diff(*configGotsched, *schedulerpol); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	for _, schedqueue := range schedqueues {
		schedinterfaceout.GetOrCreateQueue(schedqueue)
	}

	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()

	//schedinterface.Output.SchedulerPolicy.Name = ygot.String("eg_policy1111")
	//dut.Config().Qos().Interface(*schedinterface.InterfaceId).Output().SchedulerPolicy().Name().Update(t, "eg_policy1111")
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
	gnmi.Update(t, dut, configIntf.Config(), schedinterface)
	configGet := gnmi.GetConfig(t, dut, configIntf.Config())
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	config := gnmi.OC().Qos().SchedulerPolicy("eg_policy1111").Scheduler(1).Input("tc5")
	t.Run(" Delete the queue from Input queue", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, config.Config()) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})

}

func TestSchedUpdate(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	schedqueues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
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
	configsched := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
	gnmi.Update(t, dut, configsched.Config(), schedulerpol)
	configGotsched := gnmi.GetConfig(t, dut, configsched.Config())
	if diff := cmp.Diff(*configGotsched, *schedulerpol); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")

	schedinterfaceout := schedinterface.GetOrCreateOutput()
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for _, wrrque := range wrrqueues {
		schedinterfaceout.GetOrCreateQueue(wrrque)
	}
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()

	//schedinterface.Output.SchedulerPolicy.Name = ygot.String("eg_policy1111")
	//dut.Config().Qos().Interface(*schedinterface.InterfaceId).Output().SchedulerPolicy().Name().Update(t, "eg_policy1111")
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
	gnmi.Update(t, dut, configIntf.Config(), schedinterface)
	configGet := gnmi.GetConfig(t, dut, configIntf.Config())
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Interface fail: \n%v", diff)
	}

}

func TestMultipeSchedUpdateInput(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
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
	configprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
	gnmi.Update(t, dut, configprior.Config(), schedulerpol)
	configGotprior := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotprior, *schedulerpol); diff != "" {
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
	}

	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Config(), schedulenonprior)
	// confignonprior.Update(t, schedulenonprior)

	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for _, wrrque := range wrrqueues {
		schedinterfaceout.GetOrCreateQueue(wrrque)
	}
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
	gnmi.Replace(t, dut, configIntf.Config(), schedinterface)
	configGet := gnmi.GetConfig(t, dut, configIntf.Config())
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Interface fail: \n%v", diff)
	}

}

func TestMultipeSchedReplaceSchuduler(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	//Replace with multiple sequence number at /qos/schedulerpol/schduler(seq)

	t.Run("Config/Replace and get  queues at schduler sequences", func(t *testing.T) {
		priorqueues := []string{"tc7", "tc6"}
		schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
		gnmi.Replace(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Config(), schedulerpol)
		schedule := schedulerpol.GetOrCreateScheduler(1)
		schedule.Priority = oc.Scheduler_Priority_STRICT
		var ind uint64
		ind = 0
		for _, schedqueue := range priorqueues {
			input := schedule.GetOrCreateInput(schedqueue)
			input.Id = ygot.String(schedqueue)
			input.Queue = ygot.String(schedqueue)
			input.Weight = ygot.Uint64(7 - ind)

			ind += 1

		}
		ConfigSchedule := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
		t.Logf("Configuring schduler with sequence one")
		gnmi.Replace(t, dut, ConfigSchedule.Config(), schedule)
		ConfigGetSchedule := gnmi.GetConfig(t, dut, ConfigSchedule.Config())
		if diff := cmp.Diff(*ConfigGetSchedule, *schedule); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

		nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
		schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
		schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
		var weight uint64
		weight = 0
		for _, wrrqueue := range nonpriorqueues {
			inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
			inputwrr.Id = ygot.String(wrrqueue)
			inputwrr.Weight = ygot.Uint64(60 - weight)
			inputwrr.Queue = ygot.String(wrrqueue)
			weight += 10

		}
		configScheNonPrior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
		t.Logf("Configuring schduler with sequence two")
		gnmi.Replace(t, dut, configScheNonPrior.Config(), schedulenonprior)
		configGotNonprior := gnmi.GetConfig(t, dut, configScheNonPrior.Config())
		if diff := cmp.Diff(*configGotNonprior, *schedulenonprior); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

	})
	t.Run("Apply Configs to interfaces", func(t *testing.T) {
		t.Logf("Replacing the existing Configs at sequnence level")

		schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
		schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
		schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")
		schedinterfaceout := schedinterface.GetOrCreateOutput()
		wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
		for _, wrrque := range wrrqueues {
			schedinterfaceout.GetOrCreateQueue(wrrque)

		}
		scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
		configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
		gnmi.Update(t, dut, configIntf.Config(), schedinterface)
		configGet := gnmi.GetConfig(t, dut, configIntf.Config())
		if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
			t.Errorf("Config Interface fail: \n%v", diff)
		}
		ConfigGotQos := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
		if diff := cmp.Diff(*ConfigGotQos, *qos); diff != "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}
	})
	t.Run("Config/Replace existing Configs at scheduler sequence", func(t *testing.T) {
		dut1 := ondatra.DUT(t, "dut")
		d1 := &oc.Root{}
		qosrep := d1.GetOrCreateQos()
		schedulerpolred := qosrep.GetOrCreateSchedulerPolicy("eg_policy1111")
		nonpriorqueuesrep := []string{"tc4", "tc3", "tc2"}
		schedulenonpriorrep := schedulerpolred.GetOrCreateScheduler(2)
		schedulenonpriorrep.Priority = oc.Scheduler_Priority_UNSET
		var weightrep uint64
		weightrep = 0
		for _, wrrqueuered := range nonpriorqueuesrep {
			inputwrrred := schedulenonpriorrep.GetOrCreateInput(wrrqueuered)
			inputwrrred.Id = ygot.String(wrrqueuered)
			inputwrrred.Weight = ygot.Uint64(60 - weightrep)
			inputwrrred.Queue = ygot.String(wrrqueuered)
			weightrep += 10

		}
		configScheNonPriorRep := gnmi.OC().Qos().SchedulerPolicy("eg_policy1111").Scheduler(2)
		gnmi.Replace(t, dut1, configScheNonPriorRep.Config(), schedulenonpriorrep)
		configGotNonpriorRep := gnmi.GetConfig(t, dut1, configScheNonPriorRep.Config())
		if diff := cmp.Diff(*configGotNonpriorRep, *schedulenonpriorrep); diff != "" {
			t.Errorf("Config Schedule fail after replace: \n%v", diff)
		}
		ConfigGotQosRep := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
		if diff := cmp.Diff(*ConfigGotQosRep, *qos); diff == "" {
			t.Errorf("Replace with config at scheduler not working as expected \n%v", diff)
		}
	})
}
func TestMultipeSchedDelSeqtwo(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	//Replace with multiple sequence number at /qos/schedulerpol/schduler(seq)

	t.Run("Config/Replace and get  queues at schduler sequences", func(t *testing.T) {
		priorqueues := []string{"tc7", "tc6"}
		schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
		schedule := schedulerpol.GetOrCreateScheduler(1)
		schedule.Priority = oc.Scheduler_Priority_STRICT
		var ind uint64
		ind = 0
		for _, schedqueue := range priorqueues {
			input := schedule.GetOrCreateInput(schedqueue)
			input.Id = ygot.String(schedqueue)
			input.Queue = ygot.String(schedqueue)
			input.Weight = ygot.Uint64(7 - ind)

			ind += 1

		}
		ConfigSchedule := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
		t.Logf("Configuring schduler with sequence one")
		gnmi.Replace(t, dut, ConfigSchedule.Config(), schedulerpol)
		ConfigGetSchedule := gnmi.GetConfig(t, dut, ConfigSchedule.Config())
		if diff := cmp.Diff(*ConfigGetSchedule, *schedulerpol); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

		nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
		schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
		schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
		var weight uint64
		weight = 0
		for _, wrrqueue := range nonpriorqueues {
			inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
			inputwrr.Id = ygot.String(wrrqueue)
			inputwrr.Weight = ygot.Uint64(60 - weight)
			inputwrr.Queue = ygot.String(wrrqueue)
			weight += 10

		}
		configScheNonPrior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
		t.Logf("Configuring schduler with sequence two")
		gnmi.Replace(t, dut, configScheNonPrior.Config(), schedulenonprior)
		configGotNonprior := gnmi.GetConfig(t, dut, configScheNonPrior.Config())
		if diff := cmp.Diff(*configGotNonprior, *schedulenonprior); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
		schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
		schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
		schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")
		schedinterfaceout := schedinterface.GetOrCreateOutput()
		wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
		for _, wrrque := range wrrqueues {
			schedinterfaceout.GetOrCreateQueue(wrrque)
		}
		scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
		configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
		gnmi.Update(t, dut, configIntf.Config(), schedinterface)
		configGet := gnmi.GetConfig(t, dut, configIntf.Config())
		if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
			t.Errorf("Config Interface fail: \n%v", diff)
		}
		ConfigGotQos := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
		if diff := cmp.Diff(*ConfigGotQos, *qos); diff != "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}
		t.Logf("Deleting Sequence number 2")
		gnmi.Delete(t, dut, configScheNonPrior.Config())
		ConfigGotQosAfterDel := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
		if diff := cmp.Diff(*ConfigGotQosAfterDel, *qos); diff == "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}
		gnmi.Update(t, dut, configScheNonPrior.Config(), schedulenonprior)
		if diff := cmp.Diff(*ConfigGotQos, *qos); diff != "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}

	})

}
func TestMultipeSchedDelSeqone(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
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
		input.Queue = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)

		ind += 1

	}
	ConfigSchedule := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
	t.Logf("Configuring schduler with sequence one")
	gnmi.Replace(t, dut, ConfigSchedule.Config(), schedulerpol)
	ConfigGetSchedule := gnmi.GetConfig(t, dut, ConfigSchedule.Config())
	if diff := cmp.Diff(*ConfigGetSchedule, *schedulerpol); diff != "" {
		t.Errorf("Config Input fail: \n%v", diff)
	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		inputwrr.Queue = ygot.String(wrrqueue)
		weight += 10

	}
	configScheNonPrior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
	t.Logf("Configuring schduler with sequence two")
	gnmi.Replace(t, dut, configScheNonPrior.Config(), schedulenonprior)
	configGotNonprior := gnmi.GetConfig(t, dut, configScheNonPrior.Config())
	if diff := cmp.Diff(*configGotNonprior, *schedulenonprior); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	ConfigGotQos := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
	if diff := cmp.Diff(*ConfigGotQos, *qos); diff != "" {
		t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	}
	t.Logf("Deleting Sequence number 1")
	gnmi.Delete(t, dut, ConfigSchedule.Config())
	ConfigGotQosAfterDel := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
	if diff := cmp.Diff(*ConfigGotQosAfterDel, *qos); diff == "" {
		t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for _, wrrque := range wrrqueues {
		schedinterfaceout.GetOrCreateQueue(wrrque)
	}
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
	//ExpectedErrMsg := "does not have traffic-class 7 set to priority 1"
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Update(t, dut, configIntf.Config(), schedinterface) //catch the error  as it is expected and absorb the panic.
	}); errMsg != nil {
		t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
	} else {
		t.Errorf("This update should have failed ")
	}

	// gnmi.Update(t, dut, ConfigSchedule.Config(), schedulerpol)
	// if diff := cmp.Diff(*ConfigGetSchedule, *schedule); diff != "" {
	// 	t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
	// }

}

func TestMultipeSchedPolDelete(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
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
	configprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
	gnmi.Update(t, dut, configprior.Config(), schedulerpol)
	configGotprior := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotprior, *schedulerpol); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	schedinterface := qos.GetOrCreateInterface("Bundle-Ether121")
	schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
	schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")
	schedinterfaceout := schedinterface.GetOrCreateOutput()
	wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	for _, wrrque := range wrrqueues {
		schedinterfaceout.GetOrCreateQueue(wrrque)
	}
	scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
	scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
	configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
	gnmi.Update(t, dut, configIntf.Config(), schedinterface)
	configGet := gnmi.GetConfig(t, dut, configIntf.Config())
	if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
		t.Errorf("Config Interface fail: \n%v", diff)
	}
	t.Run("Delete the queue 5 ", func(t *testing.T) {
		gnmi.Delete(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input("tc5").Config())
	})
	configGotpriorafterDel := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotpriorafterDel, *schedulerpol); diff == "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	t.Run("Re add queue  5 ", func(t *testing.T) {
		inputafdel := schedulenonprior.GetOrCreateInput("tc5")
		inputafdel.Id = ygot.String("tc5")
		inputafdel.Queue = ygot.String("tc5")
		inputafdel.Weight = ygot.Uint64(61)
		gnmi.Replace(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input("tc5").Config(), inputafdel)

	})
	t.Run("Re add queue  5 ", func(t *testing.T) {
		gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input("tc4").Weight().Config(), 55)
	})

}

func TestMultipeSchedReplaceSchedPol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
		gnmi.Replace(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}

	t.Run("Config/Replace and get  queues at schduler sequences", func(t *testing.T) {
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
			//configInputwrr := dut.Config().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
			//configInputwrr.Update(t, inputwrr)
		}
		ConfigSchedPol := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name)
		gnmi.Replace(t, dut, ConfigSchedPol.Config(), schedulerpol)
		ConfigSchedPolGet := gnmi.GetConfig(t, dut, ConfigSchedPol.Config())
		if diff := cmp.Diff(*ConfigSchedPolGet, *schedulerpol); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}

		ConfigGotQosRep := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
		if diff := cmp.Diff(*ConfigGotQosRep, *qos); diff != "" {
			t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
		}
	})
	t.Run("Replace the scheduler policy", func(t *testing.T) {
		dut1 := ondatra.DUT(t, "dut")
		d1 := &oc.Root{}
		qosrep := d1.GetOrCreateQos()
		schedulerpolred := qosrep.GetOrCreateSchedulerPolicy("eg_policy1111")
		nonpriorqueuesrep := []string{"tc4", "tc3", "tc2"}
		schedulenonpriorrep := schedulerpolred.GetOrCreateScheduler(2)
		schedulenonpriorrep.Priority = oc.Scheduler_Priority_UNSET
		var weightrep uint64
		weightrep = 0
		for _, wrrqueuered := range nonpriorqueuesrep {
			inputwrrred := schedulenonpriorrep.GetOrCreateInput(wrrqueuered)
			inputwrrred.Id = ygot.String(wrrqueuered)
			inputwrrred.Weight = ygot.Uint64(60 - weightrep)
			inputwrrred.Queue = ygot.String(wrrqueuered)
			weightrep += 10

		}
		configScheNonPriorRep := gnmi.OC().Qos().SchedulerPolicy("eg_policy1111")
		gnmi.Replace(t, dut1, configScheNonPriorRep.Config(), schedulerpolred)
		configGotNonpriorRep := gnmi.GetConfig(t, dut1, configScheNonPriorRep.Config())
		if diff := cmp.Diff(*configGotNonpriorRep, *schedulerpolred); diff != "" {
			t.Errorf("Config Schedule fail after replace: \n%v", diff)
		}
		t.Run("Apply Configs to interfaces", func(t *testing.T) {
			t.Logf("Replacing the existing Configs at sequnence level")

			schedinterface := qosrep.GetOrCreateInterface("Bundle-Ether121")
			schedinterface.InterfaceId = ygot.String("Bundle-Ether121")
			schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")
			schedinterfaceout := schedinterface.GetOrCreateOutput()
			wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
			for _, wrrque := range wrrqueues {
				schedinterfaceout.GetOrCreateQueue(wrrque)
			}
			scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
			scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
			configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				gnmi.Update(t, dut1, configIntf.Config(), schedinterface) //catch the error  as it is expected and absorb the panic.
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This update should have failed ")
			}

			ConfigGotQos := gnmi.GetConfig(t, dut, gnmi.OC().Qos().Config())
			if diff := cmp.Diff(*ConfigGotQos, *qos); diff == "" {
				t.Errorf("Config Schedule fail at scheduler sequnce: \n%v", diff)
			}
		})

	})

	//dut.Config().Qos().SchedulerPolicy("eg_policy1111").Scheduler(2).Delete(t)

}

func TestMultipeSchedDeleteShedPol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	defer teardownQos(t, dut)
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}

	for i, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queues) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))
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
	configprior := gnmi.OC().Qos()
	gnmi.Update(t, dut, configprior.Config(), qos)
	configGotprior := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotprior, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	gnmi.Delete(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Config())
	configGotpriorAfterDel := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotpriorAfterDel, *qos); diff == "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Config(), schedulerpol)
	if diff := cmp.Diff(*configGotprior, *qos); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	interfaceList := []string{}
	for i := 121; i < 128; i++ {
		interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
	}

	for _, bundleint := range interfaceList {
		schedinterface := qos.GetOrCreateInterface(bundleint)
		schedinterface.InterfaceId = ygot.String(bundleint)
		schedinterface.GetOrCreateInterfaceRef().Interface = ygot.String(bundleint)
		schedinterfaceout := schedinterface.GetOrCreateOutput()
		wrrqueues := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
		for _, wrrque := range wrrqueues {
			schedinterfaceout.GetOrCreateQueue(wrrque)
		}
		scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpol.Name = ygot.String("eg_policy1111")
		configIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
		gnmi.Update(t, dut, configIntf.Config(), schedinterface)
		configGet := gnmi.GetConfig(t, dut, configIntf.Config())
		if diff := cmp.Diff(*configGet, *schedinterface); diff != "" {
			t.Errorf("Config Interface fail: \n%v", diff)
		}
	}

}
