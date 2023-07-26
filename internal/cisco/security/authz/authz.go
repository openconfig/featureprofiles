package authz

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"encoding/json"

	"github.com/openconfig/ondatra"
)

type ExecRPCFunction func(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params ...any) error

type RPC struct {
	Service, Name, QFN, Path string
	ExecFunc                 ExecRPCFunction
}

func (rpc *RPC) VerifyAccess(t testing.TB, dut *ondatra.DUTDevice, user User, expected, checkWithExec bool) {

}

type AuthorizationPolicy struct {
	Name       string `json:"name"`
	AllowRules []Rule `json:"allow_rules,omitempty"`
	DenyRules []Rule  `json:"deny_rules,omitempty"`
}

type Rule struct {
	Name       string   `json:"name"`
	Principals []string `json:"principals"`
	Request    struct {
		Paths []string `json:"paths"`
	} `json:"request"`
}
func  createRule(users []User, rpcs... RPC) Rule {
	rule:=Rule{}	
	for _,rpc := range rpcs {
		rule.Name=rule.Name+rpc.QFN
		rule.Request.Paths= append(rule.Request.Paths, rpc.Path)
	}
	for _,user := range users {
		userName:=user.Spiffeid
		if user.Spiffeid==""{
			userName=user.Name
		}
		rule.Principals=append(rule.Principals, userName)
	}
	return rule
}

func (p *AuthorizationPolicy)  AddAllowRules(users []User, rpcs... RPC) {
	rule:=createRule(users,rpcs...)
	p.AllowRules = append(p.AllowRules, rule)
}

func (p *AuthorizationPolicy)  AddDenyRules(users []User, rpcs... RPC) {
	rule:=createRule(users,rpcs...)
	p.AllowRules = append(p.AllowRules, rule)}

func (p *AuthorizationPolicy)  RestDenyRules(rules... Rule) {
	p.DenyRules=[]Rule{}

}

func (p *AuthorizationPolicy)  RestAllowRules(rules... Rule) {
	p.AllowRules=[]Rule{}
}

func (p *AuthorizationPolicy)  Unmarshal(jsonString string) error{
	return  json.Unmarshal([]byte(jsonString), p)
}

func (p *AuthorizationPolicy)  Marshal() ([]byte,error){
	return  json.Marshal(p)
}

func (p *AuthorizationPolicy)  Rotate(t *testing.T, dut ondatra.DUTDevice) {

}

func (p *AuthorizationPolicy)  Verify(t *testing.T, dut ondatra.DUTDevice, deepCheck bool, users ...User) {

}


type User struct {
	Name string
	Spiffeid string
}

type policy struct {
	rules   []Rule
	history []string
}
