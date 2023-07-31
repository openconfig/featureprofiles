package authz_test

import (
	//"context"
	"strconv"
	"testing"
	"time"

	//"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra/gnmi"

	//"github.com/openconfig/ondatra/gnmi/oc"
	//"github.com/openconfig/ygot/ygot"

	//"github.com/openconfig/gnsi/authz"
	//authz "github.com/openconfig/featureprofiles/internal/cisco/security/authz"
	//"github.com/openconfig/featureprofiles/internal/cisco/security/gnxi"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGNMIUpdate(t *testing.T) {
	//t.Log("Test is Started")
	dut := ondatra.DUT(t, "dut")
	beforeTime := time.Now()
	for i := 0; i <= 100; i++ {
		gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test"+strconv.Itoa(i))
	}
	t.Logf("time to do 100 gnmi uodate is %s", time.Since(beforeTime).String())
	/*gnmi.Update(t,dut, gnmi.OC().System().Hostname().Config(),"test")
	authzPolicy:= authz.NewAuthorizationPolicy()
	authzPolicy.Get(t,dut)
	t.Logf("Authz Policy of the device %s is %s", dut.Name(),authzPolicy.PrettyPrint())*/
}
