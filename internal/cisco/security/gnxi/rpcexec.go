package gnxi

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)
// function ExecAllrpc implements a sample request for service * to validate if authz works as expected.
func ExecAllrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC * is not implemented")
}

// function ExecGnmiallrpc implements a sample request for service /gnmi.gNMI/* to validate if authz works as expected.
func ExecGnmiallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnmi.gNMI/* is not implemented")
}

// function ExecGnmiget implements a sample request for service /gnmi.gNMI/Get to validate if authz works as expected.
func ExecGnmiget(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnmi.gNMI/Get is not implemented")
}

// function ExecGnmiset implements a sample request for service /gnmi.gNMI/Set to validate if authz works as expected.
func ExecGnmiset(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnmi.gNMI/Set is not implemented")
}

// function ExecGnmisubscribe implements a sample request for service /gnmi.gNMI/Subscribe to validate if authz works as expected.
func ExecGnmisubscribe(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnmi.gNMI/Subscribe is not implemented")
}

// function ExecGnmicapabilities implements a sample request for service /gnmi.gNMI/Capabilities to validate if authz works as expected.
func ExecGnmicapabilities(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnmi.gNMI/Capabilities is not implemented")
}

// function ExecGnoibgpallrpc implements a sample request for service /gnoi.bgp.BGP/* to validate if authz works as expected.
func ExecGnoibgpallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.bgp.BGP/* is not implemented")
}

// function ExecGnoibgpclearbgpneighbor implements a sample request for service /gnoi.bgp.BGP/ClearBGPNeighbor to validate if authz works as expected.
func ExecGnoibgpclearbgpneighbor(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.bgp.BGP/ClearBGPNeighbor is not implemented")
}

// function ExecGnoicertificatecertificatemanagementallrpc implements a sample request for service /gnoi.certificate.CertificateManagement/* to validate if authz works as expected.
func ExecGnoicertificatecertificatemanagementallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.certificate.CertificateManagement/* is not implemented")
}

// function ExecGnoicertificatecertificatemanagementcangeneratecsr implements a sample request for service /gnoi.certificate.CertificateManagement/CanGenerateCSR to validate if authz works as expected.
func ExecGnoicertificatecertificatemanagementcangeneratecsr(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.certificate.CertificateManagement/CanGenerateCSR is not implemented")
}

// function ExecGnoicertificatecertificatemanagementgeneratecsr implements a sample request for service /gnoi.certificate.CertificateManagement/GenerateCSR to validate if authz works as expected.
func ExecGnoicertificatecertificatemanagementgeneratecsr(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.certificate.CertificateManagement/GenerateCSR is not implemented")
}

// function ExecGnoicertificatecertificatemanagementgetcertificates implements a sample request for service /gnoi.certificate.CertificateManagement/GetCertificates to validate if authz works as expected.
func ExecGnoicertificatecertificatemanagementgetcertificates(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.certificate.CertificateManagement/GetCertificates is not implemented")
}

// function ExecGnoicertificatecertificatemanagementinstall implements a sample request for service /gnoi.certificate.CertificateManagement/Install to validate if authz works as expected.
func ExecGnoicertificatecertificatemanagementinstall(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.certificate.CertificateManagement/Install is not implemented")
}

// function ExecGnoicertificatecertificatemanagementloadcertificate implements a sample request for service /gnoi.certificate.CertificateManagement/LoadCertificate to validate if authz works as expected.
func ExecGnoicertificatecertificatemanagementloadcertificate(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.certificate.CertificateManagement/LoadCertificate is not implemented")
}

// function ExecGnoicertificatecertificatemanagementloadcertificateauthoritybundle implements a sample request for service /gnoi.certificate.CertificateManagement/LoadCertificateAuthorityBundle to validate if authz works as expected.
func ExecGnoicertificatecertificatemanagementloadcertificateauthoritybundle(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.certificate.CertificateManagement/LoadCertificateAuthorityBundle is not implemented")
}

// function ExecGnoicertificatecertificatemanagementrevokecertificates implements a sample request for service /gnoi.certificate.CertificateManagement/RevokeCertificates to validate if authz works as expected.
func ExecGnoicertificatecertificatemanagementrevokecertificates(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.certificate.CertificateManagement/RevokeCertificates is not implemented")
}

// function ExecGnoicertificatecertificatemanagementrotate implements a sample request for service /gnoi.certificate.CertificateManagement/Rotate to validate if authz works as expected.
func ExecGnoicertificatecertificatemanagementrotate(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.certificate.CertificateManagement/Rotate is not implemented")
}

// function ExecGnoidiagallrpc implements a sample request for service /gnoi.diag.Diag/* to validate if authz works as expected.
func ExecGnoidiagallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.diag.Diag/* is not implemented")
}

// function ExecGnoidiaggetbertresult implements a sample request for service /gnoi.diag.Diag/GetBERTResult to validate if authz works as expected.
func ExecGnoidiaggetbertresult(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.diag.Diag/GetBERTResult is not implemented")
}

// function ExecGnoidiagstopbert implements a sample request for service /gnoi.diag.Diag/StopBERT to validate if authz works as expected.
func ExecGnoidiagstopbert(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.diag.Diag/StopBERT is not implemented")
}

// function ExecGnoidiagstartbert implements a sample request for service /gnoi.diag.Diag/StartBERT to validate if authz works as expected.
func ExecGnoidiagstartbert(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.diag.Diag/StartBERT is not implemented")
}

// function ExecGnoifactory_resetfactoryresetallrpc implements a sample request for service /gnoi.factory_reset.FactoryReset/* to validate if authz works as expected.
func ExecGnoifactory_resetfactoryresetallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.factory_reset.FactoryReset/* is not implemented")
}

// function ExecGnoifactory_resetfactoryresetstart implements a sample request for service /gnoi.factory_reset.FactoryReset/Start to validate if authz works as expected.
func ExecGnoifactory_resetfactoryresetstart(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.factory_reset.FactoryReset/Start is not implemented")
}

// function ExecGnoifileallrpc implements a sample request for service /gnoi.file.File/* to validate if authz works as expected.
func ExecGnoifileallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.file.File/* is not implemented")
}

// function ExecGnoifileput implements a sample request for service /gnoi.file.File/Put to validate if authz works as expected.
func ExecGnoifileput(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.file.File/Put is not implemented")
}

// function ExecGnoifileremove implements a sample request for service /gnoi.file.File/Remove to validate if authz works as expected.
func ExecGnoifileremove(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.file.File/Remove is not implemented")
}

// function ExecGnoifilestat implements a sample request for service /gnoi.file.File/Stat to validate if authz works as expected.
func ExecGnoifilestat(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.file.File/Stat is not implemented")
}

// function ExecGnoifiletransfertoremote implements a sample request for service /gnoi.file.File/TransferToRemote to validate if authz works as expected.
func ExecGnoifiletransfertoremote(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.file.File/TransferToRemote is not implemented")
}

// function ExecGnoifileget implements a sample request for service /gnoi.file.File/Get to validate if authz works as expected.
func ExecGnoifileget(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.file.File/Get is not implemented")
}

// function ExecGnoihealthzacknowledge implements a sample request for service /gnoi.healthz.Healthz/Acknowledge to validate if authz works as expected.
func ExecGnoihealthzacknowledge(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.healthz.Healthz/Acknowledge is not implemented")
}

// function ExecGnoihealthzallrpc implements a sample request for service /gnoi.healthz.Healthz/* to validate if authz works as expected.
func ExecGnoihealthzallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.healthz.Healthz/* is not implemented")
}

// function ExecGnoihealthzartifact implements a sample request for service /gnoi.healthz.Healthz/Artifact to validate if authz works as expected.
func ExecGnoihealthzartifact(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.healthz.Healthz/Artifact is not implemented")
}

// function ExecGnoihealthzcheck implements a sample request for service /gnoi.healthz.Healthz/Check to validate if authz works as expected.
func ExecGnoihealthzcheck(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.healthz.Healthz/Check is not implemented")
}

// function ExecGnoihealthzlist implements a sample request for service /gnoi.healthz.Healthz/List to validate if authz works as expected.
func ExecGnoihealthzlist(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.healthz.Healthz/List is not implemented")
}

// function ExecGnoihealthzget implements a sample request for service /gnoi.healthz.Healthz/Get to validate if authz works as expected.
func ExecGnoihealthzget(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.healthz.Healthz/Get is not implemented")
}

// function ExecGnoilayer2allrpc implements a sample request for service /gnoi.layer2.Layer2/* to validate if authz works as expected.
func ExecGnoilayer2allrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.layer2.Layer2/* is not implemented")
}

// function ExecGnoilayer2clearlldpinterface implements a sample request for service /gnoi.layer2.Layer2/ClearLLDPInterface to validate if authz works as expected.
func ExecGnoilayer2clearlldpinterface(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.layer2.Layer2/ClearLLDPInterface is not implemented")
}

// function ExecGnoilayer2clearspanningtree implements a sample request for service /gnoi.layer2.Layer2/ClearSpanningTree to validate if authz works as expected.
func ExecGnoilayer2clearspanningtree(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.layer2.Layer2/ClearSpanningTree is not implemented")
}

// function ExecGnoilayer2performbert implements a sample request for service /gnoi.layer2.Layer2/PerformBERT to validate if authz works as expected.
func ExecGnoilayer2performbert(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.layer2.Layer2/PerformBERT is not implemented")
}

// function ExecGnoilayer2sendwakeonlan implements a sample request for service /gnoi.layer2.Layer2/SendWakeOnLAN to validate if authz works as expected.
func ExecGnoilayer2sendwakeonlan(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.layer2.Layer2/SendWakeOnLAN is not implemented")
}

// function ExecGnoilayer2clearneighbordiscovery implements a sample request for service /gnoi.layer2.Layer2/ClearNeighborDiscovery to validate if authz works as expected.
func ExecGnoilayer2clearneighbordiscovery(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.layer2.Layer2/ClearNeighborDiscovery is not implemented")
}

// function ExecGnoipacket_link_qualificationlinkqualificationcreate implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Create to validate if authz works as expected.
func ExecGnoipacket_link_qualificationlinkqualificationcreate(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Create is not implemented")
}

// function ExecGnoimplsallrpc implements a sample request for service /gnoi.mpls.MPLS/* to validate if authz works as expected.
func ExecGnoimplsallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.mpls.MPLS/* is not implemented")
}

// function ExecGnoimplsclearlspcounters implements a sample request for service /gnoi.mpls.MPLS/ClearLSPCounters to validate if authz works as expected.
func ExecGnoimplsclearlspcounters(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.mpls.MPLS/ClearLSPCounters is not implemented")
}

// function ExecGnoimplsmplsping implements a sample request for service /gnoi.mpls.MPLS/MPLSPing to validate if authz works as expected.
func ExecGnoimplsmplsping(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.mpls.MPLS/MPLSPing is not implemented")
}

// function ExecGnoimplsclearlsp implements a sample request for service /gnoi.mpls.MPLS/ClearLSP to validate if authz works as expected.
func ExecGnoimplsclearlsp(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.mpls.MPLS/ClearLSP is not implemented")
}

// function ExecGnoiopticalotdrallrpc implements a sample request for service /gnoi.optical.OTDR/* to validate if authz works as expected.
func ExecGnoiopticalotdrallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.optical.OTDR/* is not implemented")
}

// function ExecGnoiopticalwavelengthrouteradjustspectrum implements a sample request for service /gnoi.optical.WavelengthRouter/AdjustSpectrum to validate if authz works as expected.
func ExecGnoiopticalwavelengthrouteradjustspectrum(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.optical.WavelengthRouter/AdjustSpectrum is not implemented")
}

// function ExecGnoiopticalwavelengthrouterallrpc implements a sample request for service /gnoi.optical.WavelengthRouter/* to validate if authz works as expected.
func ExecGnoiopticalwavelengthrouterallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.optical.WavelengthRouter/* is not implemented")
}

// function ExecGnoiopticalwavelengthroutercanceladjustpsd implements a sample request for service /gnoi.optical.WavelengthRouter/CancelAdjustPSD to validate if authz works as expected.
func ExecGnoiopticalwavelengthroutercanceladjustpsd(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.optical.WavelengthRouter/CancelAdjustPSD is not implemented")
}

// function ExecGnoiopticalwavelengthroutercanceladjustspectrum implements a sample request for service /gnoi.optical.WavelengthRouter/CancelAdjustSpectrum to validate if authz works as expected.
func ExecGnoiopticalwavelengthroutercanceladjustspectrum(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.optical.WavelengthRouter/CancelAdjustSpectrum is not implemented")
}

// function ExecGnoiosactivate implements a sample request for service /gnoi.os.OS/Activate to validate if authz works as expected.
func ExecGnoiosactivate(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.os.OS/Activate is not implemented")
}

// function ExecGnoiosallrpc implements a sample request for service /gnoi.os.OS/* to validate if authz works as expected.
func ExecGnoiosallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.os.OS/* is not implemented")
}

// function ExecGnoiosverify implements a sample request for service /gnoi.os.OS/Verify to validate if authz works as expected.
func ExecGnoiosverify(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.os.OS/Verify is not implemented")
}

// function ExecGnoiosinstall implements a sample request for service /gnoi.os.OS/Install to validate if authz works as expected.
func ExecGnoiosinstall(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.os.OS/Install is not implemented")
}

// function ExecGnoiopticalotdrinitiate implements a sample request for service /gnoi.optical.OTDR/Initiate to validate if authz works as expected.
func ExecGnoiopticalotdrinitiate(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.optical.OTDR/Initiate is not implemented")
}

// function ExecGnoipacket_link_qualificationlinkqualificationallrpc implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/* to validate if authz works as expected.
func ExecGnoipacket_link_qualificationlinkqualificationallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.packet_link_qualification.LinkQualification/* is not implemented")
}

// function ExecGnoipacket_link_qualificationlinkqualificationcapabilities implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Capabilities to validate if authz works as expected.
func ExecGnoipacket_link_qualificationlinkqualificationcapabilities(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Capabilities is not implemented")
}

// function ExecGnoipacket_link_qualificationlinkqualificationdelete implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Delete to validate if authz works as expected.
func ExecGnoipacket_link_qualificationlinkqualificationdelete(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Delete is not implemented")
}

// function ExecGnoipacket_link_qualificationlinkqualificationget implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Get to validate if authz works as expected.
func ExecGnoipacket_link_qualificationlinkqualificationget(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Get is not implemented")
}

// function ExecGnoipacket_link_qualificationlinkqualificationlist implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/List to validate if authz works as expected.
func ExecGnoipacket_link_qualificationlinkqualificationlist(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.packet_link_qualification.LinkQualification/List is not implemented")
}

// function ExecGnoisystemallrpc implements a sample request for service /gnoi.system.System/* to validate if authz works as expected.
func ExecGnoisystemallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/* is not implemented")
}

// function ExecGnoisystemcancelreboot implements a sample request for service /gnoi.system.System/CancelReboot to validate if authz works as expected.
func ExecGnoisystemcancelreboot(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/CancelReboot is not implemented")
}

// function ExecGnoisystemkillprocess implements a sample request for service /gnoi.system.System/KillProcess to validate if authz works as expected.
func ExecGnoisystemkillprocess(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/KillProcess is not implemented")
}

// function ExecGnoisystemreboot implements a sample request for service /gnoi.system.System/Reboot to validate if authz works as expected.
func ExecGnoisystemreboot(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/Reboot is not implemented")
}

// function ExecGnoisystemrebootstatus implements a sample request for service /gnoi.system.System/RebootStatus to validate if authz works as expected.
func ExecGnoisystemrebootstatus(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/RebootStatus is not implemented")
}

// function ExecGnoisystemsetpackage implements a sample request for service /gnoi.system.System/SetPackage to validate if authz works as expected.
func ExecGnoisystemsetpackage(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/SetPackage is not implemented")
}

// function ExecGnoisystemswitchcontrolprocessor implements a sample request for service /gnoi.system.System/SwitchControlProcessor to validate if authz works as expected.
func ExecGnoisystemswitchcontrolprocessor(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/SwitchControlProcessor is not implemented")
}

// function ExecGnoisystemtime implements a sample request for service /gnoi.system.System/Time to validate if authz works as expected.
func ExecGnoisystemtime(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/Time is not implemented")
}

// function ExecGnoisystemtraceroute implements a sample request for service /gnoi.system.System/Traceroute to validate if authz works as expected.
func ExecGnoisystemtraceroute(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/Traceroute is not implemented")
}

// function ExecGnoisystemping implements a sample request for service /gnoi.system.System/Ping to validate if authz works as expected.
func ExecGnoisystemping(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.system.System/Ping is not implemented")
}

// function ExecGnoiopticalwavelengthrouteradjustpsd implements a sample request for service /gnoi.optical.WavelengthRouter/AdjustPSD to validate if authz works as expected.
func ExecGnoiopticalwavelengthrouteradjustpsd(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnoi.optical.WavelengthRouter/AdjustPSD is not implemented")
}

// function ExecGnsiauthzallrpc implements a sample request for service /gnsi.authz.Authz/* to validate if authz works as expected.
func ExecGnsiauthzallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.authz.Authz/* is not implemented")
}

// function ExecGnsiauthzget implements a sample request for service /gnsi.authz.Authz/Get to validate if authz works as expected.
func ExecGnsiauthzget(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.authz.Authz/Get is not implemented")
}

// function ExecGnsiauthzprobe implements a sample request for service /gnsi.authz.Authz/Probe to validate if authz works as expected.
func ExecGnsiauthzprobe(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.authz.Authz/Probe is not implemented")
}

// function ExecGnsiauthzrotate implements a sample request for service /gnsi.authz.Authz/Rotate to validate if authz works as expected.
func ExecGnsiauthzrotate(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.authz.Authz/Rotate is not implemented")
}

// function ExecGnsicertzaddprofile implements a sample request for service /gnsi.certz.Certz/AddProfile to validate if authz works as expected.
func ExecGnsicertzaddprofile(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.certz.Certz/AddProfile is not implemented")
}

// function ExecGnsicertzallrpc implements a sample request for service /gnsi.certz.Certz/* to validate if authz works as expected.
func ExecGnsicertzallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.certz.Certz/* is not implemented")
}

// function ExecGnsicertzcangeneratecsr implements a sample request for service /gnsi.certz.Certz/CanGenerateCSR to validate if authz works as expected.
func ExecGnsicertzcangeneratecsr(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.certz.Certz/CanGenerateCSR is not implemented")
}

// function ExecGnsicertzdeleteprofile implements a sample request for service /gnsi.certz.Certz/DeleteProfile to validate if authz works as expected.
func ExecGnsicertzdeleteprofile(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.certz.Certz/DeleteProfile is not implemented")
}

// function ExecGnsicertzgetprofilelist implements a sample request for service /gnsi.certz.Certz/GetProfileList to validate if authz works as expected.
func ExecGnsicertzgetprofilelist(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.certz.Certz/GetProfileList is not implemented")
}

// function ExecGnsicertzrotate implements a sample request for service /gnsi.certz.Certz/Rotate to validate if authz works as expected.
func ExecGnsicertzrotate(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.certz.Certz/Rotate is not implemented")
}

// function ExecGnsicredentialzallrpc implements a sample request for service /gnsi.credentialz.Credentialz/* to validate if authz works as expected.
func ExecGnsicredentialzallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.credentialz.Credentialz/* is not implemented")
}

// function ExecGnsicredentialzcangeneratekey implements a sample request for service /gnsi.credentialz.Credentialz/CanGenerateKey to validate if authz works as expected.
func ExecGnsicredentialzcangeneratekey(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.credentialz.Credentialz/CanGenerateKey is not implemented")
}

// function ExecGnsicredentialzrotatehostcredentials implements a sample request for service /gnsi.credentialz.Credentialz/RotateHostCredentials to validate if authz works as expected.
func ExecGnsicredentialzrotatehostcredentials(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.credentialz.Credentialz/RotateHostCredentials is not implemented")
}

// function ExecGnsicredentialzrotateaccountcredentials implements a sample request for service /gnsi.credentialz.Credentialz/RotateAccountCredentials to validate if authz works as expected.
func ExecGnsicredentialzrotateaccountcredentials(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.credentialz.Credentialz/RotateAccountCredentials is not implemented")
}

// function ExecGnsipathzallrpc implements a sample request for service /gnsi.pathz.Pathz/* to validate if authz works as expected.
func ExecGnsipathzallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.pathz.Pathz/* is not implemented")
}

// function ExecGnsipathzget implements a sample request for service /gnsi.pathz.Pathz/Get to validate if authz works as expected.
func ExecGnsipathzget(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.pathz.Pathz/Get is not implemented")
}

// function ExecGnsipathzprobe implements a sample request for service /gnsi.pathz.Pathz/Probe to validate if authz works as expected.
func ExecGnsipathzprobe(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.pathz.Pathz/Probe is not implemented")
}

// function ExecGnsipathzrotate implements a sample request for service /gnsi.pathz.Pathz/Rotate to validate if authz works as expected.
func ExecGnsipathzrotate(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gnsi.pathz.Pathz/Rotate is not implemented")
}

// function ExecGribiallrpc implements a sample request for service /gribi.gRIBI/* to validate if authz works as expected.
func ExecGribiallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gribi.gRIBI/* is not implemented")
}

// function ExecGribiflush implements a sample request for service /gribi.gRIBI/Flush to validate if authz works as expected.
func ExecGribiflush(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gribi.gRIBI/Flush is not implemented")
}

// function ExecGribiget implements a sample request for service /gribi.gRIBI/Get to validate if authz works as expected.
func ExecGribiget(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gribi.gRIBI/Get is not implemented")
}

// function ExecGribimodify implements a sample request for service /gribi.gRIBI/Modify to validate if authz works as expected.
func ExecGribimodify(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /gribi.gRIBI/Modify is not implemented")
}

// function ExecP4v1p4runtimeallrpc implements a sample request for service /p4.v1.P4Runtime/* to validate if authz works as expected.
func ExecP4v1p4runtimeallrpc(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /p4.v1.P4Runtime/* is not implemented")
}

// function ExecP4v1p4runtimecapabilities implements a sample request for service /p4.v1.P4Runtime/Capabilities to validate if authz works as expected.
func ExecP4v1p4runtimecapabilities(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /p4.v1.P4Runtime/Capabilities is not implemented")
}

// function ExecP4v1p4runtimegetforwardingpipelineconfig implements a sample request for service /p4.v1.P4Runtime/GetForwardingPipelineConfig to validate if authz works as expected.
func ExecP4v1p4runtimegetforwardingpipelineconfig(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /p4.v1.P4Runtime/GetForwardingPipelineConfig is not implemented")
}

// function ExecP4v1p4runtimeread implements a sample request for service /p4.v1.P4Runtime/Read to validate if authz works as expected.
func ExecP4v1p4runtimeread(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /p4.v1.P4Runtime/Read is not implemented")
}

// function ExecP4v1p4runtimesetforwardingpipelineconfig implements a sample request for service /p4.v1.P4Runtime/SetForwardingPipelineConfig to validate if authz works as expected.
func ExecP4v1p4runtimesetforwardingpipelineconfig(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /p4.v1.P4Runtime/SetForwardingPipelineConfig is not implemented")
}

// function ExecP4v1p4runtimestreamchannel implements a sample request for service /p4.v1.P4Runtime/StreamChannel to validate if authz works as expected.
func ExecP4v1p4runtimestreamchannel(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /p4.v1.P4Runtime/StreamChannel is not implemented")
}

// function ExecP4v1p4runtimewrite implements a sample request for service /p4.v1.P4Runtime/Write to validate if authz works as expected.
func ExecP4v1p4runtimewrite(deviceAddress string, certificate tls.Certificate, trustBundle *x509.CertPool, params... any)  error  {
	return fmt.Errorf("exec function for RPC /p4.v1.P4Runtime/Write is not implemented")
}

