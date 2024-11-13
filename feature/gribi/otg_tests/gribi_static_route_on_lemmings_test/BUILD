load("//ops/netops/lab/wbb/testbeds:testbeds.bzl", "featureprofiles_test_suite")

package(default_applicable_licenses = ["//third_party/openconfig/featureprofiles:license"])

featureprofiles_test_suite(
    name = "gribi_static_route_on_lemmings_test",
    srcs = ["gribi_static_route_on_lemmings_test.go"],
    data = [
        "metadata.textproto",
        "//ops/netops/lab/kne/guitar/functional_tests:topologies",
        "//third_party/openconfig/featureprofiles/topologies/kne:topologies",
    ],
    glaze_kind = "go_test",
    tags = [
        "external",
        "featureprofiles-skip-J2",
        "featureprofiles-skip-cisco_8808",
        "featureprofiles-skip-cisco_8818_2",
        "featureprofiles-skip-nokia_7250_DCGATE",
        "tags_datacenter_edge",
    ],
    testbed = "//ops/netops/lab/wbb/testbeds:testbed_dut_ate_2links.textproto",
    deps = [
        "//third_party/open_traffic_generator/gosnappi",
        "//third_party/openconfig/featureprofiles/internal/attrs",
        "//third_party/openconfig/featureprofiles/internal/deviations",
        "//third_party/openconfig/featureprofiles/internal/fptest",
        "//third_party/openconfig/featureprofiles/internal/gribi",
        "//third_party/openconfig/gribigo/chk",
        "//third_party/openconfig/gribigo/constants",
        "//third_party/openconfig/gribigo/fluent",
        "//third_party/openconfig/ondatra",
        "//third_party/openconfig/ondatra/gnmi",
        "//third_party/openconfig/ondatra/otg",
    ],
)
