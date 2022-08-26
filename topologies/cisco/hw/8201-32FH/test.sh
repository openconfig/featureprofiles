go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "experimental/backup_nhg","TE-11.1","Backup NHG: Single NH","feature/experimental/backup_nhg/ate_tests/backup_nhg_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/experimental/backup_nhg/ate_tests/backup_nhg_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-11.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "experimental/gribi","TE-3.6","ACK in the Presence of Other Routes","feature/experimental/gribi/ate_tests/ack_in_presence_other_routes" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/experimental/gribi/ate_tests/ack_in_presence_other_routes   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-3.6.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "experimental/isis","RT-2.2","IS-IS LSP Updates","feature/experimental/isis/ate_tests/lsp_updates_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/experimental/isis/ate_tests/lsp_updates_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/RT-2.2.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "experimental/isis","RT-2.1","Base IS-IS Process and Adjacencies","feature/experimental/isis/ate_tests/base_adjacencies_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/experimental/isis/ate_tests/base_adjacencies_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/RT-2.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "interface/staticarp","TE-1.1","Static ARP","feature/interface/staticarp/ate_tests/static_arp_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/interface/staticarp/ate_tests/static_arp_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-1.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "interface/aggregate","RT-5.3","Aggregate Balancing","feature/interface/aggregate/ate_tests/balancing_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/interface/aggregate/ate_tests/balancing_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/RT-5.3.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "interface/aggregate","RT-5.2","Aggregate Interfaces","feature/interface/aggregate/ate_tests/aggregate_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/interface/aggregate/ate_tests/aggregate_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/RT-5.2.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "interface/singleton","RT-5.1","Singleton Interface","feature/interface/singleton/ate_tests/singleton_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/interface/singleton/ate_tests/singleton_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/RT-5.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gnmi","gNMI-1.10","Telemetry: Basic Check","feature/gnmi/ate_tests/telemetry_basic_check_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gnmi/ate_tests/telemetry_basic_check_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/gNMI-1.10.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gnmi","gNMI-1.11","Telemetry: Interface Packet Counters","feature/gnmi/ate_tests/telemetry_interface_packet_counters_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gnmi/ate_tests/telemetry_interface_packet_counters_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/gNMI-1.11.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "platform","gNMI-1.4","Telemetry: Inventory","feature/platform/ate_tests/telemetry_inventory_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/platform/ate_tests/telemetry_inventory_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/gNMI-1.4.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "system/gnmi/cliorigin","gNMI-1.1","cli Origin","feature/system/gnmi/cliorigin/ate_tests/cli_origin_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/system/gnmi/cliorigin/ate_tests/cli_origin_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/gNMI-1.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "bgp/addpath","RT-1.3","BGP Route Propagation","feature/bgp/addpath/ate_tests/route_propagation_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/bgp/addpath/ate_tests/route_propagation_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/RT-1.3.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "bgp/gracefulrestart","RT-1.4","BGP Graceful Restart","feature/bgp/gracefulrestart/ate_tests/bgp_graceful_restart_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/bgp/gracefulrestart/ate_tests/bgp_graceful_restart_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/RT-1.4.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "bgp/policybase","RT-1.2","BGP Policy & Route Installation","feature/bgp/policybase/ate_tests/route_installation_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/bgp/policybase/ate_tests/route_installation_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/RT-1.2.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gribi","TE-4.1","Base Leader Election","feature/gribi/ate_tests/base_leader_election_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/base_leader_election_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-4.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gribi","TE-5.1","gRIBI Get RPC","feature/gribi/ate_tests/get_rpc_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/get_rpc_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-5.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gribi","TE-3.5","Ordering: ACK Received","feature/gribi/ate_tests/ordering_ack_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/ordering_ack_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-3.5.json 2>&1

#echo running test "gribi","TE-3.2","Traffic Balancing According to Weights","feature/gribi/ate_tests/weighted_balancing_test" 
#go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/weighted_balancing_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-3.2.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gribi","TE-6.1","Route Removal via Flush","feature/gribi/ate_tests/route_removal_via_flush_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/route_removal_via_flush_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-6.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gribi","TE-3.1","Base Hierarchical Route Installation","feature/gribi/ate_tests/base_hierarchical_route_installation_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/base_hierarchical_route_installation_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-3.1.json 2>&1
