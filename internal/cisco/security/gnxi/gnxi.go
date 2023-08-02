package gnxi

import (
	//"testing"
	"crypto/tls"
	"crypto/x509"
	//"github.com/openconfig/ondatra"
	//
)

type ExecRPCFunction func(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params ...any) error

type RPC struct {
	Service, Name, QFN, Path string
	ExecFunc                 ExecRPCFunction
}
