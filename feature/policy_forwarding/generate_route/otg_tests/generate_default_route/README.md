# RT-10.1: Default Route Generation based on 10.0.0.0/8 Presence

## Testcase summary

# Testbed type 

* [`featureprofiles/topologies/atedut_2.testbed`](https://github.com/openconfig/featureprofiles/blob/main/topologies/atedut_2.testbed)

## Topology

*  Connect ATE Port-1 to DUT Port-1

*  Preconditions:
    * A BGP speaker DUT is configured.

    * BGP peering is established and stable between DUT and at least one neighbor ATE-1.

    * A policy to generate default route 0.0.0.0/0 upon receiving a bgp route 10.0.0.0/8
    
    * Initial routing tables are verified to be free of any 10.0.0.0/8 or 0.0.0.0/0 routes.
    
      
 

# Procedure

# Scenario 1: 10.0.0.0/8 route is present		

1.1	On ATE-1, advertise a bgp route 10.0.0.0/8 towards DUT.	

Expected result: The 10.0.0.0/8 route is visible in DUT's routing table.

1.2	Wait for BGP to converge.	

Expected result: The BGP session remains in the Established state.

1.3	On DUT, inspect the routing table.	

Expected result: A default route (0.0.0.0/0) is now present in DUT.

1.4	On ATE-1, stop advertising the route 10.0.0.0/8.	

Expected result: The 10.0.0.0/8 route is removed from DUT's routing table.

1.5	Wait for BGP to converge.	

Expected result: The BGP session remains in the Established state.

1.6	On DUT, inspect the routing table.	

Expected result: The default route (0.0.0.0/0) is withdrawn and is no longer present in DUT.

# Scenario 2: 10.0.0.0/8 route is NOT present		

2.1	Ensure no 10.0.0.0/8 route is present in DUT's routing table.	

Expected result: No 10.0.0.0/8 route is present in DUT's routing table.

2.2	On DUT, inspect the routing table.	

Expected result: The default route (0.0.0.0/0) is withdrawn and is no longer present in DUT.


# Conclusion:

Pass: The test case passes if all expected results are achieved.

Fail: The test case fails if any of the following occur:

A default route is generated and advertised when the 10.0.0.0/8 route is not present.
A default route is not generated and advertised when the 10.0.0.0/8 route is present.
The BGP session drops or gets stuck in an intermediate state.

## Canonical OC 

```
            {
               "config": {
                  "name": 
               },
               "name": ,
               "statements": {
                  "statement": [
                     {
                        "actions": {
                           "bgp-actions": {
                              "config": {
                                 "set-local-pref": ,
                                 "set-route-origin": 
                              }
                           },
                           "config": {
                              "policy-result": 
                           }
                        },
                        "conditions": {
                           "match-prefix-set": {
                              "config": {
                                 "match-set-options": ,
                                 "prefix-set": 
                              }
                           }
                        },
            },

```

## OpenConfig Path and RPC Coverage
```yaml
rpcs:
  gnmi:
    gNMI.Get:
    gNMI.Set:
    gNMI.Subscribe:
```
