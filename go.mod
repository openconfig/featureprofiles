module github.com/openconfig/featureprofiles

go 1.17

require (
	github.com/golang/glog v1.0.0
	github.com/google/go-cmp v0.5.8
	github.com/google/gopacket v1.1.19
	github.com/open-traffic-generator/snappi/gosnappi v0.8.5
	github.com/openconfig/gnmi v0.0.0-20220617175856-41246b1b3507
	github.com/openconfig/gnoi v0.0.0-20220131192435-7dd3a95a4f1e
	github.com/openconfig/gocloser v0.0.0-20220310182203-c6c950ed3b0b
	github.com/openconfig/goyang v1.1.0
	github.com/openconfig/gribi v0.1.1-0.20220622162620-08d53dffce45
	github.com/openconfig/gribigo v0.0.0-20220802181317-805e943d8714
	github.com/openconfig/lemming v0.0.0-20220404232244-1dbc2b4c6179
	github.com/openconfig/ondatra v0.0.0-20220629205534-35d4f8159d8f
	github.com/openconfig/testt v0.0.0-20220311054427-efbb1a32ec07
	github.com/openconfig/ygot v0.24.2
	github.com/p4lang/p4runtime v1.3.0
	github.com/protocolbuffers/txtpbfmt v0.0.0-20220608084003-fc78c767cd6a
	github.com/vishvananda/netlink v1.1.1-0.20201029203352-d40f9887b852
	go.uber.org/atomic v1.7.0
	golang.org/x/crypto v0.0.0-20220131195533-30dcbda58838
	google.golang.org/grpc v1.48.0
	google.golang.org/protobuf v1.28.0
	k8s.io/klog/v2 v2.60.1
)

require (
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.2.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/openconfig/kne v0.1.1 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/vishvananda/netns v0.0.0-20200728191858-db3c7e526aae // indirect
	golang.org/x/net v0.0.0-20220403103023-749bd193bc2b // indirect
	golang.org/x/sys v0.0.0-20220406163625-3f8b81556e12 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220405205423-9d709892a2bf // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	lukechampine.com/uint128 v1.1.1 // indirect
)

replace github.com/openconfig/ondatra => /usr/local/google/home/robjs/go/src/github.com/openconfig/ondatra

replace github.com/openconfig/lemming => /usr/local/google/home/robjs/go/src/github.com/openconfig/lemming

replace github.com/openconfig/ygot => /usr/local/google/home/robjs/go/src/github.com/openconfig/ygot
