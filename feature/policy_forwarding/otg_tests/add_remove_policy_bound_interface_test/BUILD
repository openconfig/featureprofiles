load("//ops/netops/lab/wbb/testbeds:testbeds.bzl", "featureprofiles_test_suite")

package(default_applicable_licenses = ["//third_party/openconfig/featureprofiles:license"])

featureprofiles_test_suite(
    name = "add_remove_policy_bound_interface_test",
    srcs = ["add_remove_policy_bound_interface_test.go"],
    data = [
        "metadata.textproto",
        "//ops/netops/lab/kne/guitar/functional_tests:topologies",
        "//third_party/openconfig/featureprofiles/topologies/kne:topologies",
    ],
    glaze_kind = "go_test",
    tags = ["external"],
    testbed = "//ops/netops/lab/wbb/testbeds:testbed_dut_ate_4links.textproto",
    deps = [
        "//third_party/golang/ygot/ygot",
        "//third_party/open_traffic_generator/gosnappi",
        "//third_party/openconfig/featureprofiles/internal/attrs",
        "//third_party/openconfig/featureprofiles/internal/cfgplugins",
        "//third_party/openconfig/featureprofiles/internal/deviations",
        "//third_party/openconfig/featureprofiles/internal/fptest",
        "//third_party/openconfig/featureprofiles/internal/otgutils",
        "//third_party/openconfig/ondatra",
        "//third_party/openconfig/ondatra/gnmi",
        "//third_party/openconfig/ondatra/gnmi/oc",
    ],
)

filegroup(
    name = "readmes",
    srcs = glob(["**/README.md"]),
    visibility = ["//third_party/openconfig/featureprofiles:internal"],
)

filegroup(
    name = "textprotos",
    srcs = glob(["*.textproto"]),
    visibility = ["//third_party/openconfig/featureprofiles:internal"],
)

filegroup(
    name = "gofiles",
    srcs = glob(["*.go"]),
    visibility = ["//third_party/openconfig/featureprofiles:internal"],
)
