package secure_boot_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	"golang.org/x/crypto/ssh"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type iF struct {
	name             string
	ipv4Address      string
	ipv4PrefixLength uint8
	ipv6Address      string
	ipv6PrefixLength uint8
	disabled         bool
	mtu              uint16
}

var (
	loopback1 = iF{
		name:             "Loopback12",
		ipv4Address:      "192.0.2.50",
		ipv4PrefixLength: 30,
		ipv6Address:      "2001:DB8::50",
		ipv6PrefixLength: 64,
	}
)

var (
	device1  = "dut"
	showMgmt = map[ondatra.Vendor]string{
		ondatra.CISCO: "show ipv4 interface brief | inc Mg",
	}
	ipv4Pattern = "([0-9]{1,3}[\\.]){3}[0-9]{1,3}"
)

// buildInterfaceConfig implements a builder for interface config model
func buildInterfaceConfig(intf *iF, intftype oc.E_IETFInterfaces_InterfaceType) *oc.Interface {
	model := oc.Interface{
		Name:    &intf.name,
		Enabled: ygot.Bool(!intf.disabled),
		Type:    intftype,
	}
	if intf.disabled {
		model.Enabled = ygot.Bool(false)
	} else {
		model.Enabled = ygot.Bool(true)
	}
	if intf.mtu != 0 {
		model.Mtu = ygot.Uint16(intf.mtu)
	}
	model.Subinterface = map[uint32]*oc.Interface_Subinterface{}
	model.Subinterface[0] = &oc.Interface_Subinterface{
		Index: ygot.Uint32(0),
	}
	if intf.ipv4Address != "" && intf.ipv4PrefixLength != 0 {
		model.Subinterface[0].Ipv4 = buildSubintfIpv4(&intf.ipv4Address, intf.ipv4PrefixLength)
	}
	if intf.ipv6Address != "" && intf.ipv4PrefixLength != 0 {
		model.Subinterface[0].Ipv6 = buildSubintfIpv6(&intf.ipv6Address, intf.ipv6PrefixLength)
	}
	return &model
}

// buildSubintfIpv4 implements a builder for ipv4 subinterface model
func buildSubintfIpv4(v4ip *string, prefixlength uint8) *oc.Interface_Subinterface_Ipv4 {
	sb := &oc.Interface_Subinterface_Ipv4{
		Address: map[string]*oc.Interface_Subinterface_Ipv4_Address{
			*v4ip: {
				Ip:           ygot.String(*v4ip),
				PrefixLength: ygot.Uint8(prefixlength),
			},
		},
	}
	return sb
}

// buildSubintfIpv6 implements a builder for ipv4 subinterface model
func buildSubintfIpv6(v6ip *string, prefixlength uint8) *oc.Interface_Subinterface_Ipv6 {
	sb := &oc.Interface_Subinterface_Ipv6{
		Address: map[string]*oc.Interface_Subinterface_Ipv6_Address{
			*v6ip: {
				Ip:           ygot.String(*v6ip),
				PrefixLength: ygot.Uint8(prefixlength),
			},
		},
	}
	return sb
}

// mgmtAddress provides the management interface addresses from the device
func mgmtAddress(t *testing.T, dut *ondatra.DUTDevice, stc ondatra.StreamClient) []iF {
	var ifs []iF
	switch dut.Vendor() {
	case ondatra.CISCO:
		response, err := stc.SendCommand(context.Background(), showMgmt[ondatra.CISCO])
		if err != nil {
			t.Error("Unable to get the management IP address : %w", err)
		}
		exp, _ := regexp.Compile(ipv4Pattern)
		addresses := exp.FindAllString(response, 1)
		for _, address := range addresses {
			ifs = append(ifs, iF{
				ipv4Address: address,
			})
		}
	}
	return ifs
}

// verifyLogin verifies that login attempts are allowed/denied
func verifyLogin(ipssh string, t *testing.T, dut *ondatra.DUTDevice, userName string, password string, exceptPass bool, sshdisable bool) {
	config := &ssh.ClientConfig{
		User: userName,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", ipssh, config)
	if exceptPass {
		if err != nil {
			t.Fatalf("Error dialing server")
		}
		session, err := client.NewSession()
		if err != nil {
			t.Fatalf("Error Creating NewSession")
		}
		defer session.Close()
	} else {
		if sshdisable {
			if strings.Contains(string(err.Error()), "connection refused") == false {
				t.Fatalf("ssh verification with ssh disabled failed")
			}
		} else {
			if strings.Contains(string(err.Error()), "unable to authenticate") == false {
				t.Fatalf("ssh verification with incorrect credentials failed")
			}
		}
	}
}

func TestDisableSSH(t *testing.T) {
	// To Do: Verify disabling telent
	// To Do: Enable / Disble SSH via Console
	dut := ondatra.DUT(t, device1)
	dut.Config().System().SshServer().Enable().Update(t, true)
	defer dut.Config().System().SshServer().Enable().Update(t, true)
	handle := dut.RawAPIs().CLI(t)
	defer handle.Close()
	mgmtinterface := mgmtAddress(t, dut, handle)
	dut.Config().System().SshServer().Enable().Update(t, false)
	userName := "tempUser"
	userPass := "tempPassword"
	pass := "071B24415E3918160405041E00"
	config := &oc.System_Aaa_Authentication_User{
		Username: ygot.String(userName),
		Password: ygot.String(pass),
		Role:     oc.UnionString("root-lr"),
	}
	dut.Config().System().Aaa().Authentication().User(userName).Update(t, config)
	for _, intf := range mgmtinterface {
		addresses := intf.ipv4Address
		verifyLogin(addresses+":22", t, dut, userName, userPass, false, true)
	}

	time.Sleep(10 * time.Second)
	dut.Config().System().SshServer().Enable().Update(t, true)
	t.Logf("Attempting to Login with SSH Enabled ")
	for _, intf := range mgmtinterface {
		addresses := intf.ipv4Address
		verifyLogin(addresses+":22", t, dut, userName, userPass, true, false)
	}
}

func TestAddUsernamePassword(t *testing.T) {
	// To Do: Verify ssh from remote
	// To Do: Verify configuring via console
	dut := ondatra.DUT(t, device1)
	ifConfig := buildInterfaceConfig(&loopback1, oc.IETFInterfaces_InterfaceType_softwareLoopback)
	dut.Config().Interface(loopback1.name).Update(t, ifConfig)
	userName := "tempUser"
	userPass := "tempPassword"
	pass := "071B24415E3918160405041E00"
	config := &oc.System_Aaa_Authentication_User{
		Username: ygot.String(userName),
		Password: ygot.String(pass),
		Role:     oc.UnionString("root-lr"),
	}
	dut.Config().System().Aaa().Authentication().User(userName).Update(t, config)
	handle := dut.RawAPIs().CLI(t)
	defer handle.Close()
	mgmtinterface := mgmtAddress(t, dut, handle)
	t.Run("VerifyLogin", func(t *testing.T) {
		for _, intf := range mgmtinterface {
			addresses := intf.ipv4Address
			verifyLogin(addresses+":22", t, dut, userName, userPass, true, false)
		}
	})
	t.Run("VerifyLoginDeniedIncorrectPassword", func(t *testing.T) {
		for _, intf := range mgmtinterface {
			addresses := intf.ipv4Address
			verifyLogin(addresses+":22", t, dut, userName, userPass+"@#$!", false, false)
		}
	})
	t.Run("VerifyLoginDeniedIncorrectUsername", func(t *testing.T) {
		for _, intf := range mgmtinterface {
			addresses := intf.ipv4Address
			verifyLogin(addresses+":22", t, dut, userName+"123", userPass, false, false)
		}
	})
}
