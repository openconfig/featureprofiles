load("//third_party/openconfig/ondatra:ondatra.bzl", "ondatra_test")

package(default_visibility = ["//ops/netops/lab:__subpackages__"])

# blaze test \
#     --notest_loasd \
#     --test_arg=--topology=ops/netops/lab/ondatra/knegcebind/tests/gnmi/topology.textproto \
#     ops/netops/lab/ondatra/knegcebind/tests/gnmi:gnmi_test
ondatra_test(
    name = "gnmi_test",
    srcs = ["gnmi_test.go"],
    data = [
        "topology.textproto",
    ],
    glaze_kind = "go_test",
    testbed = "testbed.textproto",
    deps = [
        "//third_party/golang/ygot/ygot",
        "//third_party/openconfig/featureprofiles/internal/fptest",
        "//third_party/openconfig/ondatra",
        "//third_party/openconfig/ondatra/telemetry",
    ],
)

# blaze test \
#     --notest_loasd \
#     --test_arg=--topology=ops/netops/lab/ondatra/knegcebind/tests/gnmi/topology_local_config.textproto \
#     ops/netops/lab/ondatra/knegcebind/tests/gnmi/gnmi_test_local_config
ondatra_test(
    name = "gnmi_test_local_config",
    srcs = ["gnmi_test.go"],
    data = [
        "r1-config",
        "r2-config",
        "topology_local_config.textproto",
    ],
    glaze_kind = "go_test",
    testbed = "testbed.textproto",
    deps = [
        "//third_party/golang/ygot/ygot",
        "//third_party/openconfig/featureprofiles/internal/fptest",
        "//third_party/openconfig/ondatra",
        "//third_party/openconfig/ondatra/telemetry",
    ],
)
