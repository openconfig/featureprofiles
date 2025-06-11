load("//tools/build_defs/go:go_library.bzl", "go_library")

go_library(
    name = "packetvalidationhelpers",
    testonly = True,
    srcs = ["packetvalidationhelpers.go"],
    visibility = ["//visibility:public"],
    deps = [
        "//third_party/golang/gopacket",
        "//third_party/golang/gopacket/layers",
        "//third_party/golang/gopacket/pcap",
        "//third_party/open_traffic_generator/gosnappi",
        "//third_party/openconfig/ondatra",
    ],
)

