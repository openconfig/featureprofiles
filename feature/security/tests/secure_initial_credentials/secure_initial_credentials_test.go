package secure_boot_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	expect "github.com/google/goexpect"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
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
		ipv4Address:      "1.1.1.1",
		ipv4PrefixLength: 24,
		ipv6Address:      "1::1",
		ipv6PrefixLength: 128,
	}
)

var (
	device1  = "dut"
	showMgmt = map[ondatra.Vendor]string{
		ondatra.CISCO: "show ipv4 interface brief | inc Mg",
	}
	ipv4Pattern = "([0-9]{1,3}[\\.]){3}[0-9]{1,3}"
)

// expectCLI creates an expect handle for CLI Executed over SSH
func expectCLI(client ondatra.StreamClient, timeout time.Duration, opts ...expect.Option) (expect.Expecter, <-chan error, error) {
	resCh := make(chan error)
	return expect.SpawnGeneric(&expect.GenOptions{
		In:  client.Stdin(),
		Out: client.Stdout(),
		Wait: func() error {
			return <-resCh
		},
		Close: func() error {
			close(resCh)
			return client.Close()
		},
		Check: func() bool { return true },
	}, timeout, opts...)
}

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

// verifyLogin verifies that login attempts with correct credentials are allowed
func verifyLogin(intf iF, t *testing.T, dut *ondatra.DUTDevice, userName string, password string) {
	stc := dut.RawAPIs().CLI(t)
	switch dut.Vendor() {
	case ondatra.CISCO:
		exp, _, err := expectCLI(stc, 10, expect.Verbose(true))
		if err != nil {
			t.Error("CLI Interactive channel creation failed: %w", err)
		}
		defer func() {
			if err := exp.Close(); err != nil {
				t.Error("CLI Interactive channel Close failed: %w", err)
			}
		}()
		timeout := 15 * time.Second
		loginCMD := fmt.Sprintf("ssh %s username %s", intf.ipv4Address, userName)
		_, err = exp.ExpectBatch([]expect.Batcher{
			&expect.BExp{R: "#"},
			&expect.BSnd{S: loginCMD + "\n"},
			&expect.BExp{R: "Password:"},
			&expect.BSnd{S: password + "\n\n"},
			&expect.BExp{R: "#"},
			&expect.BSnd{S: "show user " + "\n"},
			&expect.BExp{R: userName},
		}, timeout)
		if err != nil {
			t.Error("ssh verification  with correct credentials failed: %w", err)
		}
	}
}

// verifyLoginDenied verifies that login attempts with incorrect credentials are denied
func verifyLoginDenied(intf iF, t *testing.T, dut *ondatra.DUTDevice, userName string, password string) {
	switch dut.Vendor() {
	case ondatra.CISCO:
		stc := dut.RawAPIs().CLI(t)
		exp, _, err := expectCLI(stc, 10, expect.Verbose(true))
		if err != nil {
			t.Error("CLI Interactive channel creation failed: %w", err)
		}
		defer func() {
			if err := exp.Close(); err != nil {
				t.Error("CLI Interactive channel Close failed: %w", err)
			}
		}()
		timeout := 15 * time.Second
		loginCMD := fmt.Sprintf("ssh %s username %s", intf.ipv4Address, userName)
		res, err := exp.ExpectBatch([]expect.Batcher{
			&expect.BExp{R: "#"},
			&expect.BSnd{S: loginCMD + "\n"},
			&expect.BCas{C: []expect.Caser{
				&expect.Case{R: regexp.MustCompile(`Password`), S: password + "#$@#\n",
					T: expect.Continue(expect.NewStatus(codes.PermissionDenied, "wrong password")), Rt: 4}},
			},
		}, timeout)
		if err == nil || strings.Contains(fmt.Sprintf("%v", res), "denied") == false {
			t.Error("ssh verification  with incorrect password failed: %w ", err)
		}
	}
}

// verifyLoginRefused verifies that login attempts are refused
func verifyLoginRefused(intf iF, t *testing.T, dut *ondatra.DUTDevice, stc ondatra.StreamClient, userName string, password string) {
	switch dut.Vendor() {
	case ondatra.CISCO:
		defer stc.Close()
		exp, _, err := expectCLI(stc, 10, expect.Verbose(true))
		if err != nil {
			t.Error("CLI Interactive channel creation failed: %w", err)
		}
		defer func() {
			if err := exp.Close(); err != nil {
				t.Error("CLI Interactive channel Close failed: %w", err)
			}
		}()
		timeout := 15 * time.Second
		loginCMD := fmt.Sprintf("ssh %s username %s", intf.ipv4Address, userName)
		res, err := exp.ExpectBatch([]expect.Batcher{
			&expect.BExp{R: "#"},
			&expect.BSnd{S: loginCMD + "\n"},
			&expect.BCas{C: []expect.Caser{
				&expect.Case{R: regexp.MustCompile(`Password`), S: password + "#$@#\n",
					T: expect.Continue(expect.NewStatus(codes.PermissionDenied, "refused")), Rt: 4}},
			},
		}, timeout)
		if err == nil || strings.Contains(fmt.Sprintf("%v", res), "refused") == false {
			t.Errorf("cliSpawn.ExpectBatch for ssh verification  with incorrect password failed: %v , res: %v", err, res)
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
	addresses := mgmtAddress(t, dut, handle)
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
	for _, intf := range addresses {
		fmt.Println(intf.ipv4Address)
		time.Sleep(10 * time.Second)
		verifyLoginRefused(intf, t, dut, handle, userName, userPass)
	}
	time.Sleep(10 * time.Second)
	dut.Config().System().SshServer().Enable().Update(t, true)
	t.Logf("Attempting to Login with SSH Enabled ")
	dut.RawAPIs().CLI(t)
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
	addresses := mgmtAddress(t, dut, handle)
	t.Run("VerifyLogin", func(t *testing.T) {
		verifyLogin(loopback1, t, dut, userName, userPass)
		for _, intf := range addresses {
			verifyLogin(intf, t, dut, userName, userPass)
		}
	})
	t.Run("VerifyLoginDeniedIncorrectPassword", func(t *testing.T) {
		verifyLoginDenied(loopback1, t, dut, userName, userPass+"@#$!")
		for _, intf := range addresses {
			verifyLoginDenied(intf, t, dut, userName, userPass)
		}
	})
	t.Run("VerifyLoginDeniedIncorrectUsername", func(t *testing.T) {
		verifyLoginDenied(loopback1, t, dut, userName+"123", userPass)
		for _, intf := range addresses {
			verifyLoginDenied(intf, t, dut, userName, userPass)
		}
	})
}
