load("//ops/netops/lab/wbb/testbeds:testbeds.bzl", "featureprofiles_test_suite")

package(default_applicable_licenses = ["//third_party/openconfig/featureprofiles:license"])

featureprofiles_test_suite(
    name = "lacp_interval_test",
    srcs = ["lacp_interval_test.go"],
    data = [
        "metadata.textproto",
        "//ops/netops/lab/kne/guitar/functional_tests:topologies",
        "//third_party/openconfig/featureprofiles/topologies/kne:topologies",
    ],
    glaze_kind = "go_test",
    tags = ["external"],
    testbed = "//ops/netops/lab/wbb/testbeds:testbed_dut_dut_4links.textproto",
    deps = [
        "//third_party/golang/ygot/ygot",
        "//third_party/open_traffic_generator/gosnappi",
        "//third_party/openconfig/featureprofiles/internal/attrs",
        "//third_party/openconfig/featureprofiles/internal/deviations",
        "//third_party/openconfig/featureprofiles/internal/fptest",
        "//third_party/openconfig/ondatra",
        "//third_party/openconfig/ondatra/gnmi",
        "//third_party/openconfig/ondatra/gnmi/oc",
        "//third_party/openconfig/ondatra/netutil",
    ],
)

