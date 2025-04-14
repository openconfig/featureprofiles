load("//ops/netops/lab/wbb/testbeds:testbeds.bzl", "featureprofiles_test_suite")

package(default_applicable_licenses = ["//third_party/openconfig/featureprofiles:license"])

featureprofiles_test_suite(
    name = "control_plane_traffic_ingress_acl_test",
    srcs = ["control_plane_traffic_ingress_acl_test.go"],
    data = [
        "metadata.textproto",
        "//ops/netops/lab/kne/guitar/functional_tests:topologies",
        "//third_party/openconfig/featureprofiles/topologies/kne:topologies",
    ],
    glaze_kind = "go_test",
    tags = [
        "external",
        "tags_aggregation",
    ],
    testbed = "//ops/netops/lab/wbb/testbeds:testbed_dut_ate_2links.textproto",
    deps = [
        "//third_party/golang/gopacket",
        "//third_party/golang/gopacket/layers",
        "//third_party/golang/gopacket/pcap",
        "//third_party/golang/ygot/ygot",
        "//third_party/open_traffic_generator/gosnappi",
        "//third_party/openconfig/featureprofiles/internal/attrs",
        "//third_party/openconfig/featureprofiles/internal/deviations",
        "//third_party/openconfig/featureprofiles/internal/fptest",
        "//third_party/openconfig/gnmi/proto/gnmi:gnmi_go_proto",
        "//third_party/openconfig/ondatra",
        "//third_party/openconfig/ondatra/gnmi",
        "//third_party/openconfig/ondatra/gnmi/oc",
        "//third_party/openconfig/ondatra/gnmi/oc:acl",
        "//third_party/openconfig/featureprofiles/internal/otgutils",
    ],
)
