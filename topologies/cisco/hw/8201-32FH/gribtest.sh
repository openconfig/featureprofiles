go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "experimental/backup_nhg","TE-11.1","Backup NHG: Single NH","feature/experimental/backup_nhg/ate_tests/backup_nhg_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/experimental/backup_nhg/ate_tests/backup_nhg_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-11.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "experimental/gribi","TE-3.6","ACK in the Presence of Other Routes","feature/experimental/gribi/ate_tests/ack_in_presence_other_routes" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/experimental/gribi/ate_tests/ack_in_presence_other_routes   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-3.6.json 2>&1


go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gribi","TE-4.1","Base Leader Election","feature/gribi/ate_tests/base_leader_election_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/base_leader_election_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-4.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gribi","TE-5.1","gRIBI Get RPC","feature/gribi/ate_tests/get_rpc_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/get_rpc_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-5.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gribi","TE-3.5","Ordering: ACK Received","feature/gribi/ate_tests/ordering_ack_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/ordering_ack_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-3.5.json 2>&1

echo running test "gribi","TE-6.1","Route Removal via Flush","feature/gribi/ate_tests/route_removal_via_flush_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/route_removal_via_flush_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-6.1.json 2>&1

go test -v    -p 1 -timeout 0  ../../pretests/pre_fp_gribi_test.go   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true
echo running test "gribi","TE-3.1","Base Hierarchical Route Installation","feature/gribi/ate_tests/base_hierarchical_route_installation_test" 
go test -v  -json  -p 1 -timeout 0  ../../../../feature/gribi/ate_tests/base_hierarchical_route_installation_test   -args -testbed $PWD/testbed  -binding  $PWD/binding  -v 5  -alsologtostderr true > ./trace/TE-3.1.json 2>&1
