package authz

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/openconfig/ondatra"
)

type ExecRPCFunction func(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params ...any) error

type RPC struct {
	Service, Name, QFN, Path string
	ExecFunc                 ExecRPCFunction
}

func (rpc *RPC) VerifyAccess(t testing.TB, dut *ondatra.DUTDevice, user User, expected, checkWithExec bool) {

}

type Rule struct {
	Action  string
	Service string
}

type User struct {
}

type policy struct {
	rules   []Rule
	history []string
}
