package qos_test

import (
	"fmt"
	"strings"
	"testing"
    "github.com/google/go-cmp/cmp"
	"math/rand"
	"github.com/openconfig/testt"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func Test1QueueSchedulerPolicy(t *testing.T) {
	// These are only config tests.
	// Step 1 Initialize DUT onDatra
	// Step 2 Initialize baseconfig to be used.
	// Step 3 Create 1 single  Queue defined in base config , apply them.
	// Step 4 Create a single Policy-map and attach the queues in there.
	// Step 5 Attach the scheduler policy to an interface.
	// Step 6 Subscribe to the Queue of the interface .
	// Step 7 Delete just the 1 TC1 queue from Policy map config.
	// Step 8 Subscribe Again to check if the queue is configured or interface outputs (per queue)
	// Step 9  Defer called to delete all.
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
	for _, input := range baseConfigQueue {
		if strings.ToLower(input) != "tc1"  { // We want to update config when queue name matches to that.
		t.Run(fmt.Sprintf("Configuring queue under /qos/queues/queue using value %v", input), func(t *testing.T) {
			config := dut.Config().Qos().Queue(*input)
			t.Run("Update container", func(t *testing.T) {
				config.Update(t, baseConfigQueue)
			})
			/*
			Not working as of now via DDTS or OnDatra issue
			*/
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
		})
		/* No sunbsrciption needed as the sunbsrciption happens for interface level queues.*/
	}
}
	/// Don't delete just continue to configure scheduler.
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigSchedulerPolicy)
	})

	/// Add the interface config with scheduler.

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	baseConfigInterfaceOutput := baseConfigInterface.Output
	baseConfigInterfaceOutputQueue := setup.GetAnyValue(baseConfigInterfaceOutput.Queue)
	config := dut.Config().Qos().Interface(*baseConfigInterface)
	state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(*baseConfigInterfaceOutputQueue.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigInterface)
	})
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			//if cmp.Diff(*configGot,*baseConfigInterface) != "" {
			if *configGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v, want %v", configGot, *baseConfigInterface)
			}
		})
	}
	/*
	if !setup.SkipSubscribe() {
		t.Run("Subscribe container", func(t *testing.T) {
			stateGot := state.Get(t)
			if *stateGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("State /qos/interfaces/interface/config/interface-id: got %v, want %v", stateGot, )
			}
		})
	}
	*/
	// this might 
	t.Run("Update container", func(t *testing.T) {
		config.Delete(t, baseConfigInterface)
	})
	//verify if the Scheduler-policy got delted from interface or not.
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			if *configGot.InterfaceId == *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v", configGot)
			}
		})
	}
	// Now replace the whole config via replace 

	t.Run("Replace container", func(t *testing.T) {
		config.Replace(t, baseConfigInterface)//this needs to be changed.
	})
	// service policy remove frm interrace.
	// delete postponed  
	// or replace 
 }

func TestMultipleQueueSchedulerPolicy(t *testing.T) {
	// These are only config tests.
	// Step 1 Initialize DUT onDatra
	// Step 2 Initialize baseconfig to be used.
	// Step 3 For all (1-7) the Queues defined in base config , apply them.
	// Step 4 Create a single Policy-map and attach the queues in there.
	// Step 5 Attach the scheduler policy to an interface.
	// Step 6 Subscribe to the Queue of the interface .
	// Step 8  Defar called to delete all.


	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/queues/queue using value %v", input), func(t *testing.T) {
			baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
			*baseConfigQueue.Name = input

			config := dut.Config().Qos().Queue(*baseConfigQueue.Name)

			t.Run("Update container", func(t *testing.T) {
				config.Update(t, baseConfigQueue)
			})
			/*
			Not working as of now via DDTS or OnDatra issue
			*/
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
		})
		/* No sunbsrciption needed as the sunbsrciption happens for interface level queues.*/
	}
	/// Don't delete just continue to configure scheduler.
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigSchedulerPolicy)
	})

	/// Add the interface config with scheduler.

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	baseConfigInterfaceOutput := baseConfigInterface.Output
	baseConfigInterfaceOutputQueue := setup.GetAnyValue(baseConfigInterfaceOutput.Queue)
	config := dut.Config().Qos().Interface(*baseConfigInterface)
	state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(*baseConfigInterfaceOutputQueue.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigInterface)
	})
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			//if cmp.Diff(*configGot,*baseConfigInterface) != "" {
			if *configGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v, want %v", configGot, *baseConfigInterface)
			}
		})
	}
	if !setup.SkipSubscribe() {
		t.Run("Subscribe container", func(t *testing.T) {
			stateGot := state.Get(t)
			if *stateGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("State /qos/interfaces/interface/config/interface-id: got %v, want %v", stateGot, )
			}
		})
	}
	t.Run("Update container", func(t *testing.T) {
		config.Delete(t, baseConfigInterface)
	})
	//verify if the Scheduler-policy got deleted from interface or not.
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			//if cmp.Diff(*configGot,*baseConfigInterface) != "" {
			if *configGot.InterfaceId == *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v", configGot)
			}
		})
	}
	// Now replace the whole config via replace 
	// defer will trigger.
	// service policy remove frm interrace.
	// delete postponed  
	// or replace 
 }


func Test1DeleteQueueSchedulerPolicy(t *testing.T) {
	// These are only config tests.
	// Step 1 Initialize DUT onDatra
	// Step 2 Initialize baseconfig to be used.
	// Step 3 For all the Queues defined in base config , apply them.
	// Step 4 Create a single Policy-map and attach the queues in there.
	// Step 5 Attach the scheduler policy to an interface.
	// Step 6 Subscribe to the Queue of the interface .
	// Step 7 Delete just the 1 TC1 queue from Policy map config.
	// Step 8 Subscribe Again to check if the queue is configured or interface outputs (per queue)
	// Step 9  Defer called to delete all.
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/queues/queue using value %v", input), func(t *testing.T) {
			baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
			*baseConfigQueue.Name = input

			config := dut.Config().Qos().Queue(*baseConfigQueue.Name)

			t.Run("Update container", func(t *testing.T) {
				config.Update(t, baseConfigQueue)
			})
			/*
			Not working as of now via DDTS or OnDatra issue
			*/
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
		})
		/* No sunbsrciption needed as the sunbsrciption happens for interface level queues.*/
	}
	/// Don't delete just continue to configure scheduler.
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigSchedulerPolicy)
	})

	/// Add the interface config with scheduler.

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	baseConfigInterfaceOutput := baseConfigInterface.Output
	baseConfigInterfaceOutputQueue := setup.GetAnyValue(baseConfigInterfaceOutput.Queue)
	config := dut.Config().Qos().Interface(*baseConfigInterface)
	state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(*baseConfigInterfaceOutputQueue.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigInterface)
	})
	if !setup.SkipSubscribe() {
		t.Run("Subscribe container", func(t *testing.T) {
			stateGot := state.Get(t)
			if *stateGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("State /qos/interfaces/interface/config/interface-id: got %v, want %v", stateGot, )
			}
		})
	}
	// Delete 1 class from Policy

	for _, input := range testQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
			*baseConfigSchedulerPolicySchedulerInput.Queue = input

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
			
			if *baseConfigSchedulerPolicySchedulerInput.Id != "tc1" {
				break
			}
			t.Run(" Delete the queue from Input queue", func(t *testing.T) {
				config.Delete(t, baseConfigSchedulerPolicySchedulerInput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					//if cmp.Diff(*configGot, input) != "" {
					if *configGot.Queue != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue: got %v, want %v", configGot, input)
					}
				})
			}
		})
	}
	
	t.Run("Update container", func(t *testing.T) {
		config.Delete(t, baseConfigInterface)
	})
	
	//verify if the Scheduler-policy got deleted from interface or not.
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			//if cmp.Diff(*configGot,*baseConfigInterface) != "" {
			if *configGot.InterfaceId == *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v", configGot)
			}
		})
	}
	// Now replace the whole config via replace 
	// defer will trigger.
	// service policy remove frm interrace.
	// delete postponed  
	// or replace 
 }
 

func TestMultipleDeleteQueueSchedulerPolicy(t *testing.T) {
	// These are only config tests.
	// Step 1 Initialize DUT onDatra
	// Step 2 Initialize baseconfig to be used.
	// Step 3 For all the Queues defined in base config , apply them.
	// Step 4 Create a single Policy-map and attach the queues in there.
	// Step 5 Attach the scheduler policy to an interface.
	// Step 6 Subscribe to the Queue of the interface .
	// Step 7 Delete TC1-TC6 in order  queue from Policy map config.
	// Step 8 Subscribe Again to check if the queue is configured or interface outputs (per queue)
	// Step 9  Defar called to delete all.
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/queues/queue using value %v", input), func(t *testing.T) {
			baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
			*baseConfigQueue.Name = input

			config := dut.Config().Qos().Queue(*baseConfigQueue.Name)

			t.Run("Update container", func(t *testing.T) {
				config.Update(t, baseConfigQueue)
			})
			/*
			Not working as of now via DDTS or OnDatra issue
			*/
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
		})
		/* No sunbsrciption needed as the sunbsrciption happens for interface level queues.*/
	}
	/// Don't delete just continue to configure scheduler.
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigSchedulerPolicy)
	})

	/// Add the interface config with scheduler.

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	baseConfigInterfaceOutput := baseConfigInterface.Output
	baseConfigInterfaceOutputQueue := setup.GetAnyValue(baseConfigInterfaceOutput.Queue)
	config := dut.Config().Qos().Interface(*baseConfigInterface)
	state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(*baseConfigInterfaceOutputQueue.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigInterface)
	})
	if !setup.SkipSubscribe() {
		t.Run("Subscribe container", func(t *testing.T) {
			stateGot := state.Get(t)
			if *stateGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("State /qos/interfaces/interface/config/interface-id: got %v, want %v", stateGot, )
			}
		})
	}
	// Delete All queues from Policy
	for _, input := range testQueueInput {
		t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
			baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
			*baseConfigSchedulerPolicySchedulerInput.Queue = input

			config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
			
			if *baseConfigSchedulerPolicySchedulerInput.Id != "tc7" {
				break
			}
			t.Run(" Delete the queue from Input queue", func(t *testing.T) {
				config.Delete(t, baseConfigSchedulerPolicySchedulerInput)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					//if cmp.Diff(*configGot, input) != "" {
					if *configGot.Queue != input {
						t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue: got %v, want %v", configGot, input)
					}
				})
			}
		})
	}
	// final check to see the count of 
	t.Run("Get container", func(t *testing.T) {
		config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence)
		configGot := config.Get(t)
		if diff := cmp.Diff(*configGot, *baseConfigSchedulerPolicySchedulerInput); diff != "" {
			t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue: got %v, want %v", configGot, input)
		}
	})
	
	t.Run("Update container", func(t *testing.T) {
		config.Delete(t, baseConfigInterface)
	})
	
	//verify if the Scheduler-policy got deleted from interface or not.
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			//if cmp.Diff(*configGot,*baseConfigInterface) != "" {
			if *configGot.InterfaceId == *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v", configGot)
			}
		})
	}
	// Now replace the whole config via replace 
	// defer will trigger.
	// service policy remove frm interrace.
	// delete postponed  
	// or replace 
 }

func TestRandomDeleteQueueSchedulerPolicy(t *testing.T) {
	// These are only config tests.
	// Step 1 Initialize DUT onDatra
	// Step 2 Initialize baseconfig to be used.
	// Step 3 For all the Queues defined in base config , apply them.
	// Step 4 Create a single Policy-map and attach the queues in there.
	// Step 5 Attach the scheduler policy to an interface.
	// Step 6 Subscribe to the Queue of the interface .
	// Step 7 Delete TC1-TC6 in non-conforming order and it should fail.
	// Step 8 Subscribe Again to check if the queue is configured or interface outputs (per queue)
	// Step 9  Defar called to delete all.
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/queues/queue using value %v", input), func(t *testing.T) {
			baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
			*baseConfigQueue.Name = input

			config := dut.Config().Qos().Queue(*baseConfigQueue.Name)

			t.Run("Update container", func(t *testing.T) {
				config.Update(t, baseConfigQueue)
			})
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
		})
		/* No sunbsrciption needed as the sunbsrciption happens for interface level queues.*/
	}
	/// Don't delete just continue to configure scheduler.
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigSchedulerPolicy)
	})

	/// Add the interface config with scheduler.

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	baseConfigInterfaceOutput := baseConfigInterface.Output
	baseConfigInterfaceOutputQueue := setup.GetAnyValue(baseConfigInterfaceOutput.Queue)
	config := dut.Config().Qos().Interface(*baseConfigInterface)
	state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(*baseConfigInterfaceOutputQueue.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigInterface)
	})
	if !setup.SkipSubscribe() {
		t.Run("Subscribe container", func(t *testing.T) {
			stateGot := state.Get(t)
			if *stateGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("State /qos/interfaces/interface/config/interface-id: got %v, ", stateGot )
			}
		})
	}
	// Delete All queues from Policy
	queueList := []string{"cs1","cs2","cs3","cs4","cs5","cs6"}
	n := rand.Int() % len(queueList)
	queueToBeDeleted := queueList[n]
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
	config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*queueToBeDeleted)
	
	// catch the fatal error.
	if errMsg := testt.CaptureFatal(t, func(t *testing.T){
		config.Delete(t) //catch the error  as it is expected and absorb the panic.
	}); errMsg != nil {
		t.LogF("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
	} else {
		t.Errorf("This update should have failed  , ")
	}
	
	
	// final check to see the count of 
	t.Run("Get container", func(t *testing.T) {
		config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence)
		configGot := config.Get(t)
		if diff := cmp.Diff(*configGot, baseConfigSchedulerPolicySchedulerInput); diff != "" {
			t.Errorf("Config /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue: got %v, want %v", configGot, input)
		}
	})
	
	t.Run("Update container", func(t *testing.T) {
		config.Delete(t, baseConfigInterface)
	})
	
	//verify if the Scheduler-policy got deleted from interface or not.
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			//if cmp.Diff(*configGot,*baseConfigInterface) != "" {
			if *configGot.InterfaceId == *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v", configGot)
			}
		})
	}
 }
func TestUpdateWrongchedulerPolicy(t *testing.T) {
	// These are only config tests.
	// Step 1 Initialize DUT onDatra
	// Step 2 Initialize baseconfig to be used.
	// Step 3 For all the Queues defined in base config , apply them.
	// Step 4 Create a single Policy-map with invalid weights and attach the queues in there.<> This should fail.
	// Step 5 Defer called to delete all.
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/queues/queue using value %v", input), func(t *testing.T) {
			baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
			*baseConfigQueue.Name = input

			config := dut.Config().Qos().Queue(*baseConfigQueue.Name)

			t.Run("Update container", func(t *testing.T) {
				config.Update(t, baseConfigQueue)
			})
			/*
			Not working as of now via DDTS or OnDatra issue
			*/
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
		})
		/* No sunbsrciption needed as the sunbsrciption happens for interface level queues.*/
	}
	/// Don't delete just continue to configure scheduler.
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name)

	// catch the fatal error.
	if errMsg := testt.CaptureFatal(t, func(t *testing.T){
		config.Update(t, baseConfigSchedulerPolicy) //catch the error  as it is expected and absorb the panic.
	}); errMsg != nil {
			t.LogF("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
	} else {
			t.Errorf("This update should have failed  , ")
		}
		
	/// Add the interface config with scheduler.

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	baseConfigInterfaceOutput := baseConfigInterface.Output
	baseConfigInterfaceOutputQueue := setup.GetAnyValue(baseConfigInterfaceOutput.Queue)
	config := dut.Config().Qos().Interface(*baseConfigInterface)
	state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(*baseConfigInterfaceOutputQueue.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigInterface)
	})
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			//if cmp.Diff(*configGot,*baseConfigInterface) != "" {
			if *configGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v, want %v", configGot, *baseConfigInterface)
			}
		})
	}
	if !setup.SkipSubscribe() {
		t.Run("Subscribe container", func(t *testing.T) {
			stateGot := state.Get(t)
			if *stateGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("State /qos/interfaces/interface/config/interface-id: got %v, want %v", stateGot, )
			}
		})
	}
	t.Run("Update container", func(t *testing.T) {
		config.Delete(t, baseConfigInterface)
	})
	//verify if the Scheduler-policy got deleted from interface or not.
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			//if cmp.Diff(*configGot,*baseConfigInterface) != "" {
			if *configGot.InterfaceId == *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v", configGot)
			}
		})
	}
 }
 
func TestCreateAndUpdateSchedulerPolicy(t *testing.T) {
	// These are only config tests.
	// Step 1 Initialize DUT onDatra
	// Step 2 Initialize baseconfig to be used.
	// Step 3 For all the Queues defined in base config , apply them.
	// Step 4 Create a single Policy-map and attach the queues in there.
	// Step 5 Attach the scheduler policy to an interface.
	// Step 6 Do modify of the Scheduler-policy with invalid weights and it should fail.
	// Step 7 Defer called to delete all.
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQos(t, dut)
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/queues/queue using value %v", input), func(t *testing.T) {
			baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
			*baseConfigQueue.Name = input

			config := dut.Config().Qos().Queue(*baseConfigQueue.Name)

			t.Run("Update container", func(t *testing.T) {
				config.Update(t, baseConfigQueue)
			})
			/*
			Not working as of now via DDTS or OnDatra issue
			*/
			if !setup.SkipGet() {
				t.Run("Get container", func(t *testing.T) {
					configGot := config.Get(t)
					if *configGot.Name != input {
						t.Errorf("Config /qos/queues/queue/config/name: got %v, want %v", configGot, input)
					}
				})
			}
		})
		/* No sunbsrciption needed as the sunbsrciption happens for interface level queues.*/
	}
	/// Don't delete just continue to configure scheduler.
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigSchedulerPolicy)
	})

	/// Add the interface config with scheduler.

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	baseConfigInterfaceOutput := baseConfigInterface.Output
	baseConfigInterfaceOutputQueue := setup.GetAnyValue(baseConfigInterfaceOutput.Queue)
	config := dut.Config().Qos().Interface(*baseConfigInterface)
	state := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(*baseConfigInterfaceOutputQueue.Name)
	t.Run("Update container", func(t *testing.T) {
		config.Update(t, baseConfigInterface)
	})
	if !setup.SkipGet() {
		t.Run("Get container", func(t *testing.T) {
			configGot := config.Get(t)
			//if cmp.Diff(*configGot,*baseConfigInterface) != "" {
			if *configGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v, want %v", configGot, *baseConfigInterface)
			}
		})
	}
	if !setup.SkipSubscribe() {
		t.Run("Subscribe container", func(t *testing.T) {
			stateGot := state.Get(t)
			if *stateGot.InterfaceId != *baseConfigInterface.InterfaceId {
				t.Errorf("State /qos/interfaces/interface/config/interface-id: got %v, want %v", stateGot, )
			}
		})
	}
	//build datastructre to update.
	type schedulerWeight struct {

	}
	if errMsg := testt.CaptureFatal(t, func(t *testing.T) {
		config.Update(t, baseConfigSchedulerPolicy) //catch the error  as it is expected and absorb the panic.
	}); errMsg != nil {
			t.LogF("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
	} else {
			t.Errorf("This update should have failed  , ")
		}

 }
func TestCreateAndUpdateSamePrioritySchedulerPolicy(t *testing.T) {
	// These are only config tests.
	// Step 1 Initialize DUT onDatra
	// Step 2 Initialize baseconfig to be used.
	// Step 3 For all the Queues defined in base config , apply them.
	// Step 4 Create a single Policy-map and attach the queues in there.
	// Step 5 Attach the scheduler policy to an interface.
	// Step 6 No do modify of the Scheduler-policy with 2 or All TC's with Same weight  and it should fail.
	// Step 7  Defar called to delete all.
 }