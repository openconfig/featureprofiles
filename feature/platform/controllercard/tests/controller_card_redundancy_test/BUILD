load("//ops/netops/lab/wbb/testbeds:testbeds.bzl", "featureprofiles_test_suite")

package(default_applicable_licenses = ["//third_party/openconfig/featureprofiles:license"])

featureprofiles_test_suite(
    name = "controller_card_redundancy_test",
    srcs = ["controller_card_redundancy_test.go"],
    data = [
        "metadata.textproto",
        "//ops/netops/lab/kne/guitar/functional_tests:topologies",
        "//third_party/openconfig/featureprofiles/topologies/kne:topologies",
    ],
    glaze_kind = "go_test",
    tags = ["external"],
    testbed = "//ops/netops/lab/wbb/testbeds:testbed_dut.textproto",
    deps = [
        "//third_party/openconfig/featureprofiles/internal/args",
        "//third_party/openconfig/featureprofiles/internal/components",
        "//third_party/openconfig/featureprofiles/internal/deviations",
        "//third_party/openconfig/featureprofiles/internal/fptest",
        "//third_party/openconfig/gnoi/system:system_go_proto",
        "//third_party/openconfig/gnoi/types:types_go_proto",
        "//third_party/openconfig/ondatra",
        "//third_party/openconfig/ondatra/gnmi",
        "//third_party/openconfig/ondatra/gnmi/oc",
        "//third_party/openconfig/testt",
        "//third_party/openconfig/ygnmi/ygnmi",
    ],
)

