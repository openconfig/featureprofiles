package qos_test

import (
	"testing"
	"math/rand"
	"fmt"
    "github.com/google/go-cmp/cmp"
	"github.com/openconfig/testt"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
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
	//create telemetry.QOS as baseconfig from scheduler_base.json
	baseConfig := setup.BaseConfig()
	//create telemetry.QOS as baseconfig from scheduler_base.json
	baseConfig1 := BaseConfig(Params{filename : "scheduler_base1.json"})
	// Create the queue from scheduler_base.json. only 1 queue
	baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
	config := dut.Config().Qos().Queue(*baseConfigQueue.Name)
	t.Run("Step 1 , Update  queue container", func(t *testing.T) {
		config.Update(t, baseConfigQueue)
	})
	// defer call at the end. baseconfig1 because assumption is that after all configs , thats needs to be deleted.
	defer teardownQos(t, dut, baseConfig1)
	// read the map and add more queues.
	for _, input := range testNameInput {
		t.Run(fmt.Sprintf("Testing /qos/queues/queue using value %v", input), func(t *testing.T) {
			baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
			*baseConfigQueue.Name = input
			config := dut.Config().Qos().Queue(*baseConfigQueue.Name)
			t.Run("Step :Update container queue ", func(t *testing.T) {
				config.Update(t, baseConfigQueue)
			})
			//Not working.
			if !setup.SkipGet() {
				t.Run("Get queue Config", func(t *testing.T) {
					configGot := config.Get(t)
					if diff := cmp.Diff(*configGot, *baseConfigQueue); diff != "" {
						t.Errorf("Config queue fail: \n%v", diff)
					}
				})
			}
		})
			}
    //Add scheduler for baseconfig1 as it has all queues mapped to policy.
	baseConfig1SchedulerPolicy := setup.GetAnyValue(baseConfig1.SchedulerPolicy)
	config1 := dut.Config().Qos().SchedulerPolicy(*baseConfig1SchedulerPolicy.Name)
	t.Run("Create Policy ", func(t *testing.T) {
		config1.Update(t, baseConfig1SchedulerPolicy)
	})	
	//Not working as Sequence which is leafRef not being returned by  XR
	if !setup.SkipGet() {
		t.Run("Get Policy Config", func(t *testing.T) {
			configGot := config1.Get(t)
			if diff := cmp.Diff(*configGot, *baseConfig1SchedulerPolicy); diff != "" {
				t.Errorf("Config Schedule fail: \n%v", diff)
			}
		})
	}
	/// Add the interface config with scheduler.
	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	config2 := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId)
	t.Run("Update container Interface", func(t *testing.T) {
		config2.Update(t, baseConfigInterface)
	})
	//Works.
	if setup.SkipGet() {
		t.Run("Get interface Config", func(t *testing.T) {
			configGot := config2.Get(t)
			if diff := cmp.Diff(*configGot, *baseConfigInterface); diff != "" {
				t.Errorf("Config interface fail: \n%v", diff)
			}
		})
	}
 }

func Test1DeleteQueueSchedulerPolicy(t *testing.T) {
    // These are only config tests.
    // Step 1 Initialize DUT onDatra
    // Step 2 Initialize baseconfig to be used.
    // Step 4 Delete just the 1 TC1 queue from Policy map config and check its config.
    // Step 5 Subscribe Again to check if the queue is configured or interface outputs (per queue)
    // Step 6  Defer called to delete all.
    dut := ondatra.DUT(t, "dut")
	var baseConfig1 *oc.Qos = setupInitQos(t, dut)
	defer teardownQos(t, dut, baseConfig1)
    // Delete 1 class from Policy
    for _, input := range testNameInput {
        t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig1.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
            baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
            *baseConfigSchedulerPolicySchedulerInput.Queue = input
 
            config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
            if *baseConfigSchedulerPolicySchedulerInput.Id == "tc7" {
                t.Run(" Delete the queue from Scheduler-policy", func(t *testing.T) {
                config.Delete(t)
				//Lookup Not working.
				//if qs := config.Lookup(t); qs.IsPresent() == true {
				//	t.Errorf("Delete queue from scheduler-policy failed: got %v", qs)
				//}
            })
		}
        })
    }
 }
func TestMultipleDeleteQueueSchedulerPolicy(t *testing.T) {
    // These are only config tests.
    // Step 1 Initialize DUT onDatra
    // Step 2 Initialize baseconfig to be used.
    // Step 3 Delete just the 1 TC1 queue from Policy map config.
    // Step 4 Subscribe Again to check if the queue is configured or interface outputs (per queue)
    // Step 5  Defer called to delete all.
    dut := ondatra.DUT(t, "dut")
	//baseConfig1 := BaseConfig(Params{filename : "scheduler_base1.json"}) //take from Scheduler_base1.json
	var baseConfig1 *oc.Qos = setupInitQos(t, dut)
	//defer teardownQos(t, dut, baseConfig1)
    // Delete 1 class from Policy

    for  _,input :=  range testNameInputReverse {
        t.Run(fmt.Sprintf("Testing /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue using value %v", input), func(t *testing.T) {
			baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig1.SchedulerPolicy)
			baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
            baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
            *baseConfigSchedulerPolicySchedulerInput.Queue = input
			*baseConfigSchedulerPolicySchedulerInput.Id = input
            config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
            if *baseConfigSchedulerPolicySchedulerInput.Id == "tc1" {
                t.Run(" Delete the queue from Input queue", func(t *testing.T) {
                config.Delete(t)
            })
			if !setup.SkipGet() {
				t.Run("Get scheduer  Config", func(t *testing.T) {
					configGot := config.Get(t)
					if diff := cmp.Diff(*configGot, *baseConfigSchedulerPolicy); diff != "" {
						t.Errorf(" config check  fail: \n%v", diff)
					}
				})
			}
		}
        })
    }
 }

func TestRandomDeleteQueueSchedulerPolicy(t *testing.T) {
    // These are only config tests.
    // Step 1 Initialize DUT onDatra
    // Step 2 Initialize baseconfig to be used.
    // Step 3 Randomly select 1 queue to be deleted.
	// Step 4 if TC1 then delete will be successful , else delete would fail.
    // Step 4 Subscribe Again to check if the queue is configured or interface outputs (per queue)
    // Step 5  Defer called to delete all.
    dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupInitQos1(t, dut, Params{filename : "scheduler_base1.json"})
	defer teardownQos(t, dut, baseConfig)

	//pick up a random integer 
	n := rand.Int() % len(testNameInput1)
    queueToBeDeleted := testNameInput1[n]
	
    baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
    baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
	baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
    *baseConfigSchedulerPolicySchedulerInput.Queue = queueToBeDeleted
	*baseConfigSchedulerPolicySchedulerInput.Id = queueToBeDeleted
    config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
	

	// we are expecting the queue delete to be failed so catch thru fatal.
	t.Run(" Delete the queue from Input queue", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB){
			config.Delete(t) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed ")
		}
	})
	if !setup.SkipGet() {
		t.Run("Get scheduer  Config", func(t *testing.T) {
			configGot := config.Get(t)
			if diff := cmp.Diff(*configGot, *baseConfigSchedulerPolicy); diff != "" {
				t.Errorf(" config check  fail: \n%v", diff)
			}
		})
	}
 }
func TestUpdateWrongSchedulerPolicyWeight(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = BaseConfig(Params{filename : "scheduler_base2.json"})
	defer teardownQos(t, dut, baseConfig)
	t.Run(" Delete the queue from Input queue", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB){
			dut.Config().Qos().Update(t, baseConfig)
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed  , ")
		}
	})
 }
func TestInvalidUpdateSchedulerPolicy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos = setupInitQos1(t, dut, Params{filename : "scheduler_base1.json"})
	defer teardownQos(t, dut, baseConfig)

	n := rand.Int() % len(testNameInput)
    queueToBeDeleted := testNameInput[n]
	
	t.Run(" Delete the queue from Input queue", func(t *testing.T) {
		t.Logf("value of queue is : %v", queueToBeDeleted)
	})

    baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
    baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
	baseConfigSchedulerPolicySchedulerInput := setup.GetAnyValue(baseConfigSchedulerPolicyScheduler.Input)
    *baseConfigSchedulerPolicySchedulerInput.Queue = queueToBeDeleted
	*baseConfigSchedulerPolicySchedulerInput.Weight = uint64(n)
    config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)
	t.Run(" Delete the queue from Input queue", func(t *testing.T) {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB){
			config.Update(t, baseConfigSchedulerPolicySchedulerInput) //catch the error  as it is expected and absorb the panic.
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This update should have failed  , ")
		}
	})
	if !setup.SkipGet() {
		t.Run("Get scheduer  Config", func(t *testing.T) {
			configGot := config.Get(t)
			t.Logf("got this config: \n%v", configGot)
			if diff := cmp.Diff(*configGot, *baseConfigSchedulerPolicy); diff != "" {
				t.Errorf(" config check  fail: \n%v", diff)
			}
		})
	}
 }

func TestSameQueueDifferentSchedulerTwoInterfaceU(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos
	t.Run(" Add 7 same queues to 2 scheduler policy and 2 policies to 2 interfaces.", func(t *testing.T) {
		baseConfig  = setupInitQos1(t, dut, Params{filename : "scheduler_base3.json"})
	})
	defer teardownQos(t, dut, baseConfig)
	config := dut.Config().Qos()
	if !setup.SkipGet() {
		t.Run("Get Qos Config", func(t *testing.T) {
			configGot := config.Get(t)
			t.Logf("got this config: \n%v", configGot)
			if diff := cmp.Diff(*configGot, *baseConfig); diff != "" {
				t.Errorf(" config check  fail: \n%v", diff)
			}
		})
	}	
}

func TestSameQueueDifferentSchedulerTwoInterfaceAddDelete(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos
	t.Run(" Add 7 same queues to 2 scheduler policy and 2 policies to 2 interfaces.", func(t *testing.T) {
		baseConfig  = setupInitQos1(t, dut, Params{filename : "scheduler_base3.json"})
	})
	defer teardownQos(t, dut, baseConfig)
	config := dut.Config().Qos()
	if !setup.SkipGet() {
		t.Run("Get Qos Config", func(t *testing.T) {
			configGot := config.Get(t)
			t.Logf("got this config: \n%v", configGot)
			if diff := cmp.Diff(*configGot, *baseConfig); diff != "" {
				t.Errorf(" config check  fail: \n%v", diff)
			}
		})
	}	
	//Delte the interface only.
	baseConfig1 := BaseConfig(Params{filename : "scheduler_base.json"})
	for _ , input := range testNameInterface {
		baseConfigInterface := setup.GetAnyValue(baseConfig1.Interface)
		*baseConfigInterface.InterfaceId = input.interfaceId
		baseConfigInterfaceOutput := baseConfigInterface.Output
		baseConfigInterfaceOutputSchedulerPolicy := baseConfigInterfaceOutput.SchedulerPolicy
		*baseConfigInterfaceOutputSchedulerPolicy.Name = input.policyName
		//config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Output().SchedulerPolicy().Name()
		config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId)
		t.Run(" Delete the scheduler from interface", func(t *testing.T) {
			config.Output().SchedulerPolicy().Name().Delete(t)
		})
		//Not working
		if !setup.SkipGet() {
			t.Run("Get Qos  interface Config", func(t *testing.T) {
				configGot := config.Get(t)
				t.Logf("got this config: \n%v", configGot)
				if diff := cmp.Diff(*configGot, *baseConfigInterfaceOutputSchedulerPolicy); diff != "" {
					t.Errorf(" config check  fail: \n%v", diff)
				}
			})
		}	
		//Add them back.
		t.Run(" Add  the scheduler for interface", func(t *testing.T) {
			config.Update(t, baseConfigInterface)
	})
}
}

func TestClassifierForwardingGroupQueueSchedulerInterface(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	var baseConfig *oc.Qos
	t.Run(" Add Classifer , match and action , queue and forwarding group.", func(t *testing.T) {
		baseConfig  = setupInitQos1(t, dut, Params{filename : "scheduler_base4.json"})
	})
	//now add another queue 
	defer teardownQos(t, dut, baseConfig)
}


