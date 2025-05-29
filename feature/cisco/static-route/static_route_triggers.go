package static_route_test

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func FlapBulkInterfaces(t *testing.T, dut *ondatra.DUTDevice, intfList []string) {

	var flapDuration time.Duration = 2
	var adminState bool

	adminState = false
	SetInterfaceStateScale(t, dut, intfList, adminState)
	time.Sleep(flapDuration * time.Second)
	adminState = true
	SetInterfaceStateScale(t, dut, intfList, adminState)
}

func SetInterfaceStateScale(t *testing.T, dut *ondatra.DUTDevice, intfList []string,
	adminState bool) {

	var intfType oc.E_IETFInterfaces_InterfaceType
	batchConfig := &gnmi.SetBatch{}

	for i := 0; i < len(intfList); i++ {
		if intfList[i][:6] == "Bundle" {
			intfType = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		} else {
			intfType = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		}
		j := &oc.Interface{
			Enabled: ygot.Bool(adminState),
			Name:    ygot.String(intfList[i]),
			Type:    intfType,
		}
		gnmi.BatchUpdate(batchConfig, gnmi.OC().Interface(intfList[i]).Config(), j)
	}
	batchConfig.Set(t, dut)
}

func DelAddMemberPort(t *testing.T, dut *ondatra.DUTDevice,
	dutPorts []string, bundlePort ...[]string) {

	batchConfig := &gnmi.SetBatch{}

	if len(bundlePort) > 0 {
		for i := 0; i < len(bundlePort); i++ {
			BE := generateBundleMemberInterfaceConfig(dutPorts[i], bundlePort[i][0])
			pathb1m1 := gnmi.OC().Interface(dutPorts[i])
			gnmi.BatchReplace(batchConfig, pathb1m1.Config(), BE)
		}
	} else {
		for i := 0; i < len(dutPorts); i++ {
			pathb1m1 := gnmi.OC().Interface(dutPorts[i])
			gnmi.BatchDelete(batchConfig, pathb1m1.Config())
		}
	}
	batchConfig.Set(t, dut)
}
