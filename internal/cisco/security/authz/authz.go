package authz

import (
	"context"

	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/featureprofiles/internal/cisco/security/gnxi"

)



type AuthorizationPolicy struct {
	Name       string `json:"name"`
	AllowRules []Rule `json:"allow_rules,omitempty"`
	DenyRules []Rule  `json:"deny_rules,omitempty"`
}

type Rule struct {
	Name       string   `json:"name"`
	Source  struct {
		Principals []string `json:"principals"`
	}`json:"source"`
	Request    struct {
		Paths []string `json:"paths"`
	} `json:"request"`
}
func  createRule(users []*User, rpcs []*gnxi.RPC) Rule {
	rule:=Rule{}	
	for _,rpc := range rpcs {
		rule.Name=rule.Name+rpc.QFN
		rule.Request.Paths= append(rule.Request.Paths, rpc.Path)
	}
	for _,user := range users {
		userName:=user.SpiffeID
		if user.SpiffeID==""{
			userName=user.Name
		}
		rule.Source.Principals=append(rule.Source.Principals, userName)
	}
	return rule
}

func (p *AuthorizationPolicy)  AddAllowRules(users []*User, rpcs []*gnxi.RPC) {
	rule:=createRule(users,rpcs)
	p.AllowRules = append(p.AllowRules, rule)
}

func (p *AuthorizationPolicy)  AddDenyRules(users []*User, rpcs []*gnxi.RPC) {
	rule:=createRule(users,rpcs)
	p.DenyRules = append(p.DenyRules, rule)}

func (p *AuthorizationPolicy)  RestDenyRules() {
	p.DenyRules=[]Rule{}

}

func (p *AuthorizationPolicy)  RestAllowRules() {
	p.AllowRules=[]Rule{}
}

func (p *AuthorizationPolicy)  Unmarshal(jsonString string) error{
	return  json.Unmarshal([]byte(jsonString), p)
}

func (p *AuthorizationPolicy)  Marshal() ([]byte,error){
	return  json.Marshal(p)
}

func (p *AuthorizationPolicy)  Rotate(t *testing.T, dut *ondatra.DUTDevice) {
	t.Logf("Performing Authz.Rotate request on device %s",dut.Name())
	rotateStream, _ := dut.RawAPIs().GNSI().Default(t).Authz().Rotate(context.Background())
	defer rotateStream.CloseSend()
	policy, err:=p.Marshal(); if err!=nil {
		t.Fatalf("Could not marshal the policy %s", prettyPrint(policy))
	}
	autzRotateReq := &authz.RotateAuthzRequest_UploadRequest{
		UploadRequest: &authz.UploadRequest{
			Version: fmt.Sprintf("v0.%v",(time.Now().UnixMilli())),
			CreatedOn: uint64(time.Now().UnixMicro()),
			Policy: string(policy),
		},
	}
	t.Logf("Sending Authz.Rotate request on device: \n %s",prettyPrint(autzRotateReq))
	err = rotateStream.Send(&authz.RotateAuthzRequest{RotateRequest: autzRotateReq,})
	if err == nil {
		t.Logf("Authz.Rotate upload was successful, receiving response ...")
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply %v", err)
		}
		// validate Result
		tempPolicy:=NewAuthorizationPolicy()
		tempPolicy.Get(t,dut)
		if !cmp.Equal(p,tempPolicy) {
			t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(p,tempPolicy))
		}
		//p.Verify(t,dut, false)
		finalizeRotateReq:=&authz.RotateAuthzRequest_FinalizeRotation{FinalizeRotation: &authz.FinalizeRequest{}}
		err = rotateStream.Send(&authz.RotateAuthzRequest{RotateRequest: finalizeRotateReq })
		t.Logf("Sending Authz.Rotate FinalizeRotation request: \n%s", prettyPrint(finalizeRotateReq))
		if err != nil {
			t.Fatalf("Error while finalizing rotate request  %v", err)
		}
	} else  {
		t.Fatalf("Error while uploading prob request reply %v", err)
	}
	//validate Result
	finalPolicy:=NewAuthorizationPolicy()
	finalPolicy.Get(t,dut)
	if !cmp.Equal(p,finalPolicy) {
		t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(p,finalPolicy))
	}
	//p.Verify(t,dut, false)


}
func NewAuthorizationPolicy() *AuthorizationPolicy{
	return  &AuthorizationPolicy{}
}

func (p *AuthorizationPolicy)  Get(t testing.TB, dut *ondatra.DUTDevice) *authz.GetResponse{
	p.RestAllowRules()
	p.RestDenyRules()
	t.Logf("Performing Authz.Get request on device %s",dut.Name())
	gnsiCLient:=dut.RawAPIs().GNSI().Default(t)
	resp, err:=gnsiCLient.Authz().Get(context.Background(), &authz.GetRequest{}); if err!=nil {
		t.Fatalf("Authz.Get request is failed on device %s",dut.Name())
	}

	t.Logf("Authz.Get response is %s",prettyPrint(resp))
	if resp.GetVersion()=="" {
		t.Errorf("Version is not set in Authz.Get response")
	}
	if resp.GetCreatedOn()>uint64(time.Now().UnixMicro()) {
		t.Errorf("CreatedOn value can not be larger than current time")
	}
	err = p.Unmarshal(resp.Policy); if err!=nil {
		t.Fatalf("Authz.Get response contains invalid policy %s",resp.GetPolicy())
	}
	// ensure all rules has principal and paths and the paths are valid
	checkRule:= func(t testing.TB,r Rule) {
		if len(r.Source.Principals)==0 {
			t.Errorf("rule %v has no principal",r)
		}
		if len(r.Request.Paths)==0 {
			t.Errorf("rule %v has no request path",r)
		}
	} 
	for _,rule:= range p.AllowRules {
		checkRule(t,rule)
	} 
	for _,rule:= range p.DenyRules {
		checkRule(t,rule)
	} 
	return resp
}

func prettyPrint(i interface{}) string {
    s, _ := json.MarshalIndent(i, "", "\t")
    return string(s)
}

func (p *AuthorizationPolicy)  PrettyPrint() string{
	prettyTex,err:=json.MarshalIndent(p,"", "    "); if err!=nil {
		glog.Warningf("PrettyPrint is failed due to err: %v", err)
		return ""
	}
	return string(prettyTex)
}

func (p *AuthorizationPolicy)  Verify(t *testing.T, dut *ondatra.DUTDevice, deepCheck bool) {
	// TODO: only draft code, do not use this function for now
	usersAccess:=map[string]map[string]bool{}
	allAccess:=map[string]bool{}
	allDeny:=map[string]bool{}
	for _,rule := range p.AllowRules {
		t.Logf("Processing Allow Rule %s",rule.Name)
		for _,user := range  rule.Source.Principals{
			if user == "*" {
				for _,path:=range rule.Request.Paths {
					allAccess[path]=true
				}
			} else {
				for _,path:=range rule.Request.Paths {
					_, ok := usersAccess["foo"]; if !ok {
						usersAccess[user]=map[string]bool{}
					}
					usersAccess[user][path]=true
				}
			}
		}
	} 
	// deny rules
	for _,rule := range p.DenyRules {
		t.Logf("Processing Deny Rule %s",rule.Name)
		for _,user := range  rule.Source.Principals{
			if user == "*" {
				for _,path:=range rule.Request.Paths {
					allDeny[path]=true
					_,ok:=allAccess[path]; if ok {
						allAccess[path]=false
					}
				}
			} else {
				for _,path:=range rule.Request.Paths {
					_, ok := usersAccess["foo"]; if !ok {
						usersAccess[user]=map[string]bool{}
					}
					usersAccess[user][path]=false
				}
			}
		}
	} 
	gnsiCLient:=dut.RawAPIs().GNSI().Default(t)
	for _,rule := range p.AllowRules {
		t.Logf("Verifying Allow Rule %s",rule.Name)
		for _,user := range  rule.Source.Principals{
				for _,path:=range rule.Request.Paths {
					resp,err:= gnsiCLient.Authz().Probe(context.Background(),&authz.ProbeRequest{User: user, Rpc: path}); if err!=nil {
						t.Fatalf("Prob Request %s failed on dut %s", prettyPrint(&authz.ProbeRequest{User: user, Rpc: path}),dut.Name())
					}
					expectedResult:= authz.ProbeResponse_ACTION_UNSPECIFIED
					ok:=allDeny[path]; if ok {
						expectedResult= authz.ProbeResponse_ACTION_DENY
					} else if !usersAccess[user][path] {
						expectedResult= authz.ProbeResponse_ACTION_DENY
					} else if  allAccess[path] || usersAccess[user][path] {
						expectedResult= authz.ProbeResponse_ACTION_PERMIT
					}
					if resp.GetAction()!=expectedResult {
						t.Fatalf("Prob response is wrong, want %v, got %v", expectedResult,resp.GetAction())
					}
				}
		}
	} 

}


type User struct {
	Name string
	SpiffeID string
}
