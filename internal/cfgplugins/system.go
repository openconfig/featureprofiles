// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cfgplugins is a collection of OpenConfig configuration libraries for system and network instance.
//
// Each plugin function has parameters for a gnmi Batch, values to use in the configuration
// and an ondatra.DUTDevice.  Each function returns OpenConfig values.
//
// The configuration function will modify the batch which is passed in by reference. The caller may
// pass nil as the configuration values to use in which case the function will provide a set of
// default values.  The ondatra.DUTDevice is used to determine any configuration deviations which
// may be necessary.
//
// The caller may choose to use the returned OC value to customize the values set by this
// function or for use in a non-batch use case.
package cfgplugins

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// CreateGNMIServer creates a gNMI server on the DUT on a given network-instance.
func CreateGNMIServer(t testing.TB, d *ondatra.DUTDevice, batch *gnmi.SetBatch, nip *NetworkInstanceParams, port uint16, transportSec bool) {
	var niName string
	var gnmiServerName string

	if nip.Default {
		// It is expected that network-instance name and gRPC server name are "DEFAULT" for default network instance.
		niName = nip.Name
		gnmiServerName = nip.Name
		// If not aligned with OC, then use deviation flags to set network-instance name and gRPC server name.
		// This validation to be removed when the deviation flags are removed.
		if deviations.DefaultNetworkInstance(d) != "" {
			niName = deviations.DefaultNetworkInstance(d)
		}
		if deviations.DefaultNiGnmiServerName(d) != "" {
			gnmiServerName = deviations.DefaultNiGnmiServerName(d)
		}
	} else {
		niName, gnmiServerName = nip.Name, "gnxi-"+nip.Name
	}
	t.Logf("Creating gNMI server %s on network instance: %s", gnmiServerName, niName)
	gnmiServerPath := gnmi.OC().System().GrpcServer(gnmiServerName)
	gnmiServer := &oc.System_GrpcServer{
		Name:            ygot.String(gnmiServerName),
		Port:            ygot.Uint16(port),
		Enable:          ygot.Bool(true),
		NetworkInstance: ygot.String(niName),
	}
	if deviations.RequireTransportSecurity(d) {
		gnmiServer.TransportSecurity = ygot.Bool(true)
	}
	if !deviations.GrpcServerServicesUnsupported(d) {
		gnmiServer.Services = []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI}
	}
	gnmi.BatchUpdate(batch, gnmiServerPath.Config(), gnmiServer)
}

// FindLineCardParent traverses up the component hierarchy starting from the given component name
// to find the nearest ancestor component of type LINECARD. It returns the name of the LINECARD
// component if found, or an error otherwise.
func FindLineCardParent(t *testing.T, dut *ondatra.DUTDevice, startComponentName string) string {
	t.Helper()

	currentComponentName := startComponentName
	depth := 0
	maxDepth := 10
	for {
		if depth >= maxDepth {
			t.Fatalf("Exceeded maximum search depth while searching for LINECARD parent for starting component %s.", startComponentName)
		}
		componentTypePath := gnmi.OC().Component(currentComponentName).Type().State()
		currentType, ok := gnmi.Lookup(t, dut, componentTypePath).Val()
		if !ok {
			t.Logf("Component %s not found or missing state data. Continuing to search for parent", currentComponentName)
		}

		t.Logf("Component %s has type: %v", currentComponentName, currentType)
		if currentType == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD {
			t.Logf("Successfully traced to parent line card: %s", currentComponentName)
			return currentComponentName
		}

		parentPath := gnmi.OC().Component(currentComponentName).Parent().State()
		parentName, ok := gnmi.Lookup(t, dut, parentPath).Val()

		if !ok || parentName == "" {
			t.Fatalf("Failed to find a parent component of type LINECARD for starting component %s.", startComponentName)
		}
		currentComponentName = parentName
		depth++
	}
}

// NTPServerCfg defines parameters for an OpenConfig NTP server entry.
type NTPServerCfg struct {
	Address         string
	NetworkInstance string
	SourceAddress   string
}

// ConfigureNTPServers configures the system NTP client and servers in the provided SetBatch.
func ConfigureNTPServers(t testing.TB, d *ondatra.DUTDevice, batch *gnmi.SetBatch, enabled bool, servers []NTPServerCfg) *oc.System_Ntp {
	t.Helper()
	ntp := &oc.System_Ntp{
		Enabled: ygot.Bool(enabled),
	}
	for _, s := range servers {
		srv := ntp.GetOrCreateServer(s.Address)
		srv.SetAddress(s.Address)
		if s.NetworkInstance != "" {
			srv.SetNetworkInstance(s.NetworkInstance)
		}
		if s.SourceAddress != "" && !deviations.NtpSourceAddressUnsupported(d) {
			srv.SetSourceAddress(s.SourceAddress)
		}
	}
	gnmi.BatchReplace(batch, gnmi.OC().System().Ntp().Config(), ntp)
	return ntp
}

// DNSServerCfg defines parameters for an OpenConfig DNS server entry.
type DNSServerCfg struct {
	Address string
	Port    uint16
}

// ConfigureDNSServers configures system DNS resolver servers in the provided SetBatch.
func ConfigureDNSServers(t testing.TB, batch *gnmi.SetBatch, servers []DNSServerCfg) *oc.System_Dns {
	t.Helper()
	dns := &oc.System_Dns{}
	for _, s := range servers {
		srv, err := dns.AppendNewServer(s.Address)
		if err != nil {
			t.Fatalf("Failed to append DNS server %s: %v", s.Address, err)
		}
		srv.SetAddress(s.Address)
		if s.Port != 0 {
			srv.SetPort(s.Port)
		}
	}
	gnmi.BatchUpdate(batch, gnmi.OC().System().Dns().Config(), dns)
	return dns
}

// TACACSServerCfg defines parameters for an OpenConfig TACACS+ server entry.
type TACACSServerCfg struct {
	Address       string
	Timeout       uint16
	Port          uint16
	SecretKey     string
	SourceAddress string
}

// TACACSGroupCfg defines parameters for an OpenConfig TACACS+ AAA server-group and authentication configuration.
type TACACSGroupCfg struct {
	GroupName string
	Servers   []TACACSServerCfg
}

// ConfigureTACACS configures AAA authentication with TACACS_ALL and a TACACS+ server group in the provided SetBatch.
func ConfigureTACACS(t testing.TB, batch *gnmi.SetBatch, cfg *TACACSGroupCfg) *oc.System_Aaa {
	t.Helper()
	aaa := &oc.System_Aaa{}
	auth := aaa.GetOrCreateAuthentication()
	authMethod, err := auth.To_System_Aaa_Authentication_AuthenticationMethod_Union(oc.AaaTypes_AAA_METHOD_TYPE_TACACS_ALL)
	if err != nil {
		t.Fatalf("Failed to convert TACACS_ALL to authentication method union: %v", err)
	}
	auth.SetAuthenticationMethod([]oc.System_Aaa_Authentication_AuthenticationMethod_Union{authMethod})
	sg := aaa.GetOrCreateServerGroup(cfg.GroupName)
	sg.SetName(cfg.GroupName)
	sg.SetType(oc.AaaTypes_AAA_SERVER_TYPE_TACACS)
	for _, s := range cfg.Servers {
		srv := sg.GetOrCreateServer(s.Address)
		srv.SetAddress(s.Address)
		if s.Timeout != 0 {
			srv.SetTimeout(s.Timeout)
		}
		tac := srv.GetOrCreateTacacs()
		if s.Port != 0 {
			tac.SetPort(s.Port)
		}
		if s.SecretKey != "" {
			tac.SetSecretKey(s.SecretKey)
		}
		if s.SourceAddress != "" {
			tac.SetSourceAddress(s.SourceAddress)
		}
	}
	gnmi.BatchUpdate(batch, gnmi.OC().System().Aaa().Config(), aaa)
	return aaa
}

// SyslogServerCfg defines parameters for an OpenConfig remote syslog server entry.
type SyslogServerCfg struct {
	Host            string
	NetworkInstance string
	RemotePort      uint16
	SourceAddress   string
	Facility        oc.E_SystemLogging_SYSLOG_FACILITY
	Severity        oc.E_SystemLogging_SyslogSeverity
}

// ConfigureRemoteSyslogServers configures remote syslog servers in the provided SetBatch.
func ConfigureRemoteSyslogServers(t testing.TB, batch *gnmi.SetBatch, servers []SyslogServerCfg) *oc.System_Logging {
	t.Helper()
	logging := &oc.System_Logging{}
	for _, s := range servers {
		rs := logging.GetOrCreateRemoteServer(s.Host)
		rs.SetHost(s.Host)
		if s.NetworkInstance != "" {
			rs.SetNetworkInstance(s.NetworkInstance)
		}
		if s.RemotePort != 0 {
			rs.SetRemotePort(s.RemotePort)
		}
		if s.SourceAddress != "" {
			rs.SetSourceAddress(s.SourceAddress)
		}
		if s.Facility != oc.SystemLogging_SYSLOG_FACILITY_UNSET && s.Severity != oc.SystemLogging_SyslogSeverity_UNSET {
			sel := rs.GetOrCreateSelector(s.Facility, s.Severity)
			sel.SetFacility(s.Facility)
			sel.SetSeverity(s.Severity)
		}
	}
	gnmi.BatchReplace(batch, gnmi.OC().System().Logging().Config(), logging)
	return logging
}

// ConfigureSSHServer configures the system SSH server enable state in the provided SetBatch.
func ConfigureSSHServer(t testing.TB, batch *gnmi.SetBatch, enable bool) *oc.System_SshServer {
	t.Helper()
	ssh := &oc.System_SshServer{
		Enable: ygot.Bool(enable),
	}
	gnmi.BatchUpdate(batch, gnmi.OC().System().SshServer().Config(), ssh)
	return ssh
}

// GRPCServerCfg defines parameters for an OpenConfig gRPC server instance.
type GRPCServerCfg struct {
	Name            string
	Enable          bool
	Port            uint16
	NetworkInstance string
	ListenAddresses []string
	Services        []oc.E_SystemGrpc_GRPC_SERVICE
}

// ConfigureGRPCServer configures a gRPC server instance on the DUT in the provided SetBatch.
func ConfigureGRPCServer(t testing.TB, d *ondatra.DUTDevice, batch *gnmi.SetBatch, cfg *GRPCServerCfg) *oc.System_GrpcServer {
	t.Helper()
	grpcServer := &oc.System_GrpcServer{
		Name:   ygot.String(cfg.Name),
		Enable: ygot.Bool(cfg.Enable),
		Port:   ygot.Uint16(cfg.Port),
	}
	if cfg.NetworkInstance != "" {
		grpcServer.SetNetworkInstance(cfg.NetworkInstance)
	}
	if len(cfg.ListenAddresses) > 0 {
		var addrs []oc.System_GrpcServer_ListenAddresses_Union
		for _, addr := range cfg.ListenAddresses {
			uAddr, err := grpcServer.To_System_GrpcServer_ListenAddresses_Union(addr)
			if err != nil {
				t.Fatalf("Failed to convert listen address %s to union: %v", addr, err)
			}
			addrs = append(addrs, uAddr)
		}
		grpcServer.SetListenAddresses(addrs)
	}
	if !deviations.GrpcServerServicesUnsupported(d) && len(cfg.Services) > 0 {
		grpcServer.SetServices(cfg.Services)
	}
	if deviations.RequireTransportSecurity(d) {
		grpcServer.SetTransportSecurity(true)
	}
	gnmi.BatchUpdate(batch, gnmi.OC().System().GrpcServer(cfg.Name).Config(), grpcServer)
	return grpcServer
}
