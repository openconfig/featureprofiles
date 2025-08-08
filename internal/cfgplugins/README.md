# cfgplugins package

The [contributing guide](https://github.com/openconfig/featureprofiles/blob/main/CONTRIBUTING.md)
specifies that cfgplugins should be used for DUT configuration generation.  

The goal of the featureprofiles cfgplugins is to provide a library of functions that generate
resuable configuration snippets. These in turn are used to compose configurations used in
featureprofiles tests.  

Each function in cfgplugins should define some struct to store attributes which is passed the
configuration generation function.   The code in the function should construct and return an ondatra
gnmi.BatchReplace object.  Here is an [example](https://github.com/openconfig/featureprofiles/blob/bc105a443b44d862c70e91112708b5a339c71ae5/internal/cfgplugins/sflow.go#L52))

Test code (`_test.go` files) should call as many cfgplugins as needed to compose the desired
configuration.  After assembling the configuration needed for a particular test or subtest,
the test should call batch.Set() to replace the portion of the configuration on the DUT described
by in the batch object.  Here is a simple
[example](https://github.com/openconfig/featureprofiles/blob/main/feature/sflow/otg_tests/sflow_base_test/sflow_base_test.go#L212-L215).

The idea behind placing the .Set call in the test code is this is what is modifying the DUT and
is what is being tested.  In other words, we are not testing the configuration generation helper,
but rather we are testing if the DUT will accept the configuration.
