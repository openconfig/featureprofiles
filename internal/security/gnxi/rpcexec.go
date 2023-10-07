package gnxi

import (
	"context"
	"io"
	"strings"

	"github.com/openconfig/gnoi/system"
	gribi "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

// function AllRPC implements a sample request for service * to validate if authz works as expected.
func AllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC * is not implemented")
}

// function GnmiAllRPC implements a sample request for service /gnmi.gNMI/* to validate if authz works as expected.
func GnmiAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnmi.gNMI/* is not implemented")
}

// function GnmiGet implements a sample request for service /gnmi.gNMI/Get to validate if authz works as expected.
func GnmiGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	gnmiC, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx, opts...)
	if err != nil {
		return err
	}
	ygnmiC, err := ygnmi.NewClient(gnmiC)
	if err != nil {
		return err
	}
	yopts := []ygnmi.Option{
		ygnmi.WithUseGet(),
		ygnmi.WithEncoding(gpb.Encoding_JSON_IETF),
	}
	_, err = ygnmi.Get[string](ctx, ygnmiC, gnmi.OC().System().Hostname().Config(), yopts...)
	s := err.Error()
	if strings.Contains(s, "value not present") {
		return nil
	}
	return err
}

// function GnmiSet implements a sample request for service /gnmi.gNMI/Set to validate if authz works as expected.
func GnmiSet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	gnmiC, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx, opts...)
	if err != nil {
		return err
	}
	ygnmiC, err := ygnmi.NewClient(gnmiC)
	if err != nil {
		return err
	}
	yopts := []ygnmi.Option{
		ygnmi.WithUseGet(),
		ygnmi.WithEncoding(gpb.Encoding_JSON_IETF),
	}
	_, err = ygnmi.Replace[string](ctx, ygnmiC, gnmi.OC().System().Hostname().Config(), "test", yopts...)
	return err
}

// function GnmiSubscribe implements a sample request for service /gnmi.gNMI/Subscribe to validate if authz works as expected.
func GnmiSubscribe(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	gnmiC, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx, opts...)
	if err != nil {
		return err
	}
	ygnmiC, err := ygnmi.NewClient(gnmiC)
	if err != nil {
		return err
	}
	_, err = ygnmi.Get(ctx, ygnmiC, gnmi.OC().System().Hostname().State())
	return err
}

// function GnmiCapabilities implements a sample request for service /gnmi.gNMI/Capabilities to validate if authz works as expected.
func GnmiCapabilities(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	gnmiC, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx, opts...)
	if err != nil {
		return err
	}
	_, err = gnmiC.Capabilities(ctx, &gpb.CapabilityRequest{})
	return err
}

// function GnoiBgpAllRPC implements a sample request for service /gnoi.bgp.BGP/* to validate if authz works as expected.
func GnoiBgpAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.bgp.BGP/* is not implemented")
}

// function GnoiBgpClearBGPNeighbor implements a sample request for service /gnoi.bgp.BGP/ClearBGPNeighbor to validate if authz works as expected.
func GnoiBgpClearBGPNeighbor(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.bgp.BGP/ClearBGPNeighbor is not implemented")
}

// function GnoiCertificatemanagementAllRPC implements a sample request for service /gnoi.certificate.CertificateManagement/* to validate if authz works as expected.
func GnoiCertificatemanagementAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/* is not implemented")
}

// function GnoiCertificatemanagementCanGenerateCSR implements a sample request for service /gnoi.certificate.CertificateManagement/CanGenerateCSR to validate if authz works as expected.
func GnoiCertificatemanagementCanGenerateCSR(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/CanGenerateCSR is not implemented")
}

// function GnoiCertificatemanagementGenerateCSR implements a sample request for service /gnoi.certificate.CertificateManagement/GenerateCSR to validate if authz works as expected.
func GnoiCertificatemanagementGenerateCSR(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/GenerateCSR is not implemented")
}

// function GnoiCertificatemanagementGetCertificates implements a sample request for service /gnoi.certificate.CertificateManagement/GetCertificates to validate if authz works as expected.
func GnoiCertificatemanagementGetCertificates(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/GetCertificates is not implemented")
}

// function GnoiCertificatemanagementInstall implements a sample request for service /gnoi.certificate.CertificateManagement/Install to validate if authz works as expected.
func GnoiCertificatemanagementInstall(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/Install is not implemented")
}

// function GnoiCertificatemanagementLoadCertificate implements a sample request for service /gnoi.certificate.CertificateManagement/LoadCertificate to validate if authz works as expected.
func GnoiCertificatemanagementLoadCertificate(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/LoadCertificate is not implemented")
}

// function GnoiCertificatemanagementLoadCertificateAuthorityBundle implements a sample request for service /gnoi.certificate.CertificateManagement/LoadCertificateAuthorityBundle to validate if authz works as expected.
func GnoiCertificatemanagementLoadCertificateAuthorityBundle(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/LoadCertificateAuthorityBundle is not implemented")
}

// function GnoiCertificatemanagementRevokeCertificates implements a sample request for service /gnoi.certificate.CertificateManagement/RevokeCertificates to validate if authz works as expected.
func GnoiCertificatemanagementRevokeCertificates(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/RevokeCertificates is not implemented")
}

// function GnoiCertificatemanagementRotate implements a sample request for service /gnoi.certificate.CertificateManagement/Rotate to validate if authz works as expected.
func GnoiCertificatemanagementRotate(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/Rotate is not implemented")
}

// function GnoiDiagAllRPC implements a sample request for service /gnoi.diag.Diag/* to validate if authz works as expected.
func GnoiDiagAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.diag.Diag/* is not implemented")
}

// function GnoiDiagGetBERTResult implements a sample request for service /gnoi.diag.Diag/GetBERTResult to validate if authz works as expected.
func GnoiDiagGetBERTResult(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.diag.Diag/GetBERTResult is not implemented")
}

// function GnoiDiagStopBERT implements a sample request for service /gnoi.diag.Diag/StopBERT to validate if authz works as expected.
func GnoiDiagStopBERT(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.diag.Diag/StopBERT is not implemented")
}

// function GnoiDiagStartBERT implements a sample request for service /gnoi.diag.Diag/StartBERT to validate if authz works as expected.
func GnoiDiagStartBERT(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.diag.Diag/StartBERT is not implemented")
}

// function GnoiFactoryresetAllRPC implements a sample request for service /gnoi.factory_reset.FactoryReset/* to validate if authz works as expected.
func GnoiFactoryresetAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.factory_reset.FactoryReset/* is not implemented")
}

// function GnoiFactoryresetStart implements a sample request for service /gnoi.factory_reset.FactoryReset/Start to validate if authz works as expected.
func GnoiFactoryresetStart(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.factory_reset.FactoryReset/Start is not implemented")
}

// function GnoiFileAllRPC implements a sample request for service /gnoi.file.File/* to validate if authz works as expected.
func GnoiFileAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/* is not implemented")
}

// function GnoiFilePut implements a sample request for service /gnoi.file.File/Put to validate if authz works as expected.
func GnoiFilePut(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/Put is not implemented")
}

// function GnoiFileRemove implements a sample request for service /gnoi.file.File/Remove to validate if authz works as expected.
func GnoiFileRemove(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/Remove is not implemented")
}

// function GnoiFileStat implements a sample request for service /gnoi.file.File/Stat to validate if authz works as expected.
func GnoiFileStat(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/Stat is not implemented")
}

// function GnoiFileTransferToRemote implements a sample request for service /gnoi.file.File/TransferToRemote to validate if authz works as expected.
func GnoiFileTransferToRemote(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/TransferToRemote is not implemented")
}

// function GnoiFileGet implements a sample request for service /gnoi.file.File/Get to validate if authz works as expected.
func GnoiFileGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/Get is not implemented")
}

// function GnoiHealthzAcknowledge implements a sample request for service /gnoi.healthz.Healthz/Acknowledge to validate if authz works as expected.
func GnoiHealthzAcknowledge(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/Acknowledge is not implemented")
}

// function GnoiHealthzAllRPC implements a sample request for service /gnoi.healthz.Healthz/* to validate if authz works as expected.
func GnoiHealthzAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/* is not implemented")
}

// function GnoiHealthzArtifact implements a sample request for service /gnoi.healthz.Healthz/Artifact to validate if authz works as expected.
func GnoiHealthzArtifact(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/Artifact is not implemented")
}

// function GnoiHealthzCheck implements a sample request for service /gnoi.healthz.Healthz/Check to validate if authz works as expected.
func GnoiHealthzCheck(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/Check is not implemented")
}

// function GnoiHealthzList implements a sample request for service /gnoi.healthz.Healthz/List to validate if authz works as expected.
func GnoiHealthzList(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/List is not implemented")
}

// function GnoiHealthzGet implements a sample request for service /gnoi.healthz.Healthz/Get to validate if authz works as expected.
func GnoiHealthzGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/Get is not implemented")
}

// function GnoiLayer2AllRPC implements a sample request for service /gnoi.layer2.Layer2/* to validate if authz works as expected.
func GnoiLayer2AllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/* is not implemented")
}

// function GnoiLayer2ClearLLDPInterface implements a sample request for service /gnoi.layer2.Layer2/ClearLLDPInterface to validate if authz works as expected.
func GnoiLayer2ClearLLDPInterface(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/ClearLLDPInterface is not implemented")
}

// function GnoiLayer2ClearSpanningTree implements a sample request for service /gnoi.layer2.Layer2/ClearSpanningTree to validate if authz works as expected.
func GnoiLayer2ClearSpanningTree(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/ClearSpanningTree is not implemented")
}

// function GnoiLayer2PerformBERT implements a sample request for service /gnoi.layer2.Layer2/PerformBERT to validate if authz works as expected.
func GnoiLayer2PerformBERT(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/PerformBERT is not implemented")
}

// function GnoiLayer2SendWakeOnLAN implements a sample request for service /gnoi.layer2.Layer2/SendWakeOnLAN to validate if authz works as expected.
func GnoiLayer2SendWakeOnLAN(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/SendWakeOnLAN is not implemented")
}

// function GnoiLayer2ClearNeighborDiscovery implements a sample request for service /gnoi.layer2.Layer2/ClearNeighborDiscovery to validate if authz works as expected.
func GnoiLayer2ClearNeighborDiscovery(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/ClearNeighborDiscovery is not implemented")
}

// function GnoiLinkqualificationCreate implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Create to validate if authz works as expected.
func GnoiLinkqualificationCreate(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Create is not implemented")
}

// function GnoiMplsAllRPC implements a sample request for service /gnoi.mpls.MPLS/* to validate if authz works as expected.
func GnoiMplsAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.mpls.MPLS/* is not implemented")
}

// function GnoiMplsClearLSPCounters implements a sample request for service /gnoi.mpls.MPLS/ClearLSPCounters to validate if authz works as expected.
func GnoiMplsClearLSPCounters(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.mpls.MPLS/ClearLSPCounters is not implemented")
}

// function GnoiMplsMPLSPing implements a sample request for service /gnoi.mpls.MPLS/MPLSPing to validate if authz works as expected.
func GnoiMplsMPLSPing(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.mpls.MPLS/MPLSPing is not implemented")
}

// function GnoiMplsClearLSP implements a sample request for service /gnoi.mpls.MPLS/ClearLSP to validate if authz works as expected.
func GnoiMplsClearLSP(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.mpls.MPLS/ClearLSP is not implemented")
}

// function GnoiOtdrAllRPC implements a sample request for service /gnoi.optical.OTDR/* to validate if authz works as expected.
func GnoiOtdrAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.OTDR/* is not implemented")
}

// function GnoiWavelengthrouterAdjustSpectrum implements a sample request for service /gnoi.optical.WavelengthRouter/AdjustSpectrum to validate if authz works as expected.
func GnoiWavelengthrouterAdjustSpectrum(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/AdjustSpectrum is not implemented")
}

// function GnoiWavelengthrouterAllRPC implements a sample request for service /gnoi.optical.WavelengthRouter/* to validate if authz works as expected.
func GnoiWavelengthrouterAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/* is not implemented")
}

// function GnoiWavelengthrouterCancelAdjustPSD implements a sample request for service /gnoi.optical.WavelengthRouter/CancelAdjustPSD to validate if authz works as expected.
func GnoiWavelengthrouterCancelAdjustPSD(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/CancelAdjustPSD is not implemented")
}

// function GnoiWavelengthrouterCancelAdjustSpectrum implements a sample request for service /gnoi.optical.WavelengthRouter/CancelAdjustSpectrum to validate if authz works as expected.
func GnoiWavelengthrouterCancelAdjustSpectrum(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/CancelAdjustSpectrum is not implemented")
}

// function GnoiOsActivate implements a sample request for service /gnoi.os.OS/Activate to validate if authz works as expected.
func GnoiOsActivate(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.os.OS/Activate is not implemented")
}

// function GnoiOsAllRPC implements a sample request for service /gnoi.os.OS/* to validate if authz works as expected.
func GnoiOsAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.os.OS/* is not implemented")
}

// function GnoiOsVerify implements a sample request for service /gnoi.os.OS/Verify to validate if authz works as expected.
func GnoiOsVerify(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.os.OS/Verify is not implemented")
}

// function GnoiOsInstall implements a sample request for service /gnoi.os.OS/Install to validate if authz works as expected.
func GnoiOsInstall(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.os.OS/Install is not implemented")
}

// function GnoiOtdrInitiate implements a sample request for service /gnoi.optical.OTDR/Initiate to validate if authz works as expected.
func GnoiOtdrInitiate(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.OTDR/Initiate is not implemented")
}

// function GnoiLinkqualificationAllRPC implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/* to validate if authz works as expected.
func GnoiLinkqualificationAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/* is not implemented")
}

// function GnoiLinkqualificationCapabilities implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Capabilities to validate if authz works as expected.
func GnoiLinkqualificationCapabilities(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Capabilities is not implemented")
}

// function GnoiLinkqualificationDelete implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Delete to validate if authz works as expected.
func GnoiLinkqualificationDelete(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Delete is not implemented")
}

// function GnoiLinkqualificationGet implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Get to validate if authz works as expected.
func GnoiLinkqualificationGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Get is not implemented")
}

// function GnoiLinkqualificationList implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/List to validate if authz works as expected.
func GnoiLinkqualificationList(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/List is not implemented")
}

// function GnoiSystemAllRPC implements a sample request for service /gnoi.system.System/* to validate if authz works as expected.
func GnoiSystemAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/* is not implemented")
}

// function GnoiSystemCancelReboot implements a sample request for service /gnoi.system.System/CancelReboot to validate if authz works as expected.
func GnoiSystemCancelReboot(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/CancelReboot is not implemented")
}

// function GnoiSystemKillProcess implements a sample request for service /gnoi.system.System/KillProcess to validate if authz works as expected.
func GnoiSystemKillProcess(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/KillProcess is not implemented")
}

// function GnoiSystemReboot implements a sample request for service /gnoi.system.System/Reboot to validate if authz works as expected.
func GnoiSystemReboot(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/Reboot is not implemented")
}

// function GnoiSystemRebootStatus implements a sample request for service /gnoi.system.System/RebootStatus to validate if authz works as expected.
func GnoiSystemRebootStatus(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/RebootStatus is not implemented")
}

// function GnoiSystemSetPackage implements a sample request for service /gnoi.system.System/SetPackage to validate if authz works as expected.
func GnoiSystemSetPackage(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/SetPackage is not implemented")
}

// function GnoiSystemSwitchControlProcessor implements a sample request for service /gnoi.system.System/SwitchControlProcessor to validate if authz works as expected.
func GnoiSystemSwitchControlProcessor(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/SwitchControlProcessor is not implemented")
}

// function GnoiSystemTime implements a sample request for service /gnoi.system.System/Time to validate if authz works as expected.
func GnoiSystemTime(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/Time is not implemented")
}

// function GnoiSystemTraceroute implements a sample request for service /gnoi.system.System/Traceroute to validate if authz works as expected.
func GnoiSystemTraceroute(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/Traceroute is not implemented")
}

// function GnoiSystemPing implements a sample request for service /gnoi.system.System/Ping to validate if authz works as expected.
func GnoiSystemPing(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	gnoiC, err := dut.RawAPIs().BindingDUT().DialGNOI(ctx, opts...)
	if err != nil {
		return err
	}
	_, err = gnoiC.System().Ping(ctx, &system.PingRequest{Destination: "192.0.2.1"})
	return err
}

// function GnoiWavelengthrouterAdjustPSD implements a sample request for service /gnoi.optical.WavelengthRouter/AdjustPSD to validate if authz works as expected.
func GnoiWavelengthrouterAdjustPSD(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/AdjustPSD is not implemented")
}

// function GnsiAccountingpullAllRPC implements a sample request for service /gnsi.accounting.v1.AccountingPull/* to validate if authz works as expected.
func GnsiAccountingpullAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.accounting.v1.AccountingPull/* is not implemented")
}

// function GnsiAccountingpushRecordStream implements a sample request for service /gnsi.accounting.v1.AccountingPush/RecordStream to validate if authz works as expected.
func GnsiAccountingpushRecordStream(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.accounting.v1.AccountingPush/RecordStream is not implemented")
}

// function GnsiAccountingpushAllRPC implements a sample request for service /gnsi.accounting.v1.AccountingPush/* to validate if authz works as expected.
func GnsiAccountingpushAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.accounting.v1.AccountingPush/* is not implemented")
}

// function GnsiAccountingpullRecordStream implements a sample request for service /gnsi.accounting.v1.AccountingPull/RecordStream to validate if authz works as expected.
func GnsiAccountingpullRecordStream(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.accounting.v1.AccountingPull/RecordStream is not implemented")
}

// function GnsiAuthzAllRPC implements a sample request for service /gnsi.authz.v1.Authz/* to validate if authz works as expected.
func GnsiAuthzAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.authz.v1.Authz/* is not implemented")
}

// function GnsiAuthzGet implements a sample request for service /gnsi.authz.v1.Authz/Get to validate if authz works as expected.
func GnsiAuthzGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.authz.v1.Authz/Get is not implemented")
}

// function GnsiAuthzProbe implements a sample request for service /gnsi.authz.v1.Authz/Probe to validate if authz works as expected.
func GnsiAuthzProbe(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.authz.v1.Authz/Probe is not implemented")
}

// function GnsiAuthzRotate implements a sample request for service /gnsi.authz.v1.Authz/Rotate to validate if authz works as expected.
func GnsiAuthzRotate(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.authz.v1.Authz/Rotate is not implemented")
}

// function GnsiCertzAddProfile implements a sample request for service /gnsi.certz.v1.Certz/AddProfile to validate if authz works as expected.
func GnsiCertzAddProfile(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/AddProfile is not implemented")
}

// function GnsiCertzAllRPC implements a sample request for service /gnsi.certz.v1.Certz/* to validate if authz works as expected.
func GnsiCertzAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/* is not implemented")
}

// function GnsiCertzCanGenerateCSR implements a sample request for service /gnsi.certz.v1.Certz/CanGenerateCSR to validate if authz works as expected.
func GnsiCertzCanGenerateCSR(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/CanGenerateCSR is not implemented")
}

// function GnsiCertzDeleteProfile implements a sample request for service /gnsi.certz.v1.Certz/DeleteProfile to validate if authz works as expected.
func GnsiCertzDeleteProfile(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/DeleteProfile is not implemented")
}

// function GnsiCertzGetProfileList implements a sample request for service /gnsi.certz.v1.Certz/GetProfileList to validate if authz works as expected.
func GnsiCertzGetProfileList(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/GetProfileList is not implemented")
}

// function GnsiCertzRotate implements a sample request for service /gnsi.certz.v1.Certz/Rotate to validate if authz works as expected.
func GnsiCertzRotate(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/Rotate is not implemented")
}

// function GnsiCredentialzAllRPC implements a sample request for service /gnsi.credentialz.v1.Credentialz/* to validate if authz works as expected.
func GnsiCredentialzAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/* is not implemented")
}

// function GnsiCredentialzCanGenerateKey implements a sample request for service /gnsi.credentialz.v1.Credentialz/CanGenerateKey to validate if authz works as expected.
func GnsiCredentialzCanGenerateKey(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/CanGenerateKey is not implemented")
}

// function GnsiCredentialzGetPublicKeys implements a sample request for service /gnsi.credentialz.v1.Credentialz/GetPublicKeys to validate if authz works as expected.
func GnsiCredentialzGetPublicKeys(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/GetPublicKeys is not implemented")
}

// function GnsiCredentialzRotateHostCredentials implements a sample request for service /gnsi.credentialz.v1.Credentialz/RotateHostCredentials to validate if authz works as expected.
func GnsiCredentialzRotateHostCredentials(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/RotateHostCredentials is not implemented")
}

// function GnsiCredentialzRotateAccountCredentials implements a sample request for service /gnsi.credentialz.v1.Credentialz/RotateAccountCredentials to validate if authz works as expected.
func GnsiCredentialzRotateAccountCredentials(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/RotateAccountCredentials is not implemented")
}

// function GnsiPathzAllRPC implements a sample request for service /gnsi.pathz.v1.Pathz/* to validate if authz works as expected.
func GnsiPathzAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.pathz.v1.Pathz/* is not implemented")
}

// function GnsiPathzGet implements a sample request for service /gnsi.pathz.v1.Pathz/Get to validate if authz works as expected.
func GnsiPathzGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.pathz.v1.Pathz/Get is not implemented")
}

// function GnsiPathzProbe implements a sample request for service /gnsi.pathz.v1.Pathz/Probe to validate if authz works as expected.
func GnsiPathzProbe(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.pathz.v1.Pathz/Probe is not implemented")
}

// function GnsiPathzRotate implements a sample request for service /gnsi.pathz.v1.Pathz/Rotate to validate if authz works as expected.
func GnsiPathzRotate(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.pathz.v1.Pathz/Rotate is not implemented")
}

// function GribiAllRPC implements a sample request for service /gribi.gRIBI/* to validate if authz works as expected.
func GribiAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gribi.gRIBI/* is not implemented")
}

// function GribiFlush implements a sample request for service /gribi.gRIBI/Flush to validate if authz works as expected.
func GribiFlush(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	gribiC, err := dut.RawAPIs().BindingDUT().DialGRIBI(ctx, opts...)
	if err != nil {
		return err
	}
	_, err = gribiC.Flush(ctx, &gribi.FlushRequest{Election: &gribi.FlushRequest_Id{Id: &gribi.Uint128{Low: 1}}})
	return err
}

// function GribiGet implements a sample request for service /gribi.gRIBI/Get to validate if authz works as expected.
func GribiGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	gribiC, err := dut.RawAPIs().BindingDUT().DialGRIBI(ctx, opts...)
	if err != nil {
		return err
	}
	getReq := gribi.GetRequest{
		NetworkInstance: &gribi.GetRequest_All{},
		Aft:             gribi.AFTType_ALL,
	}
	getSteram, err := gribiC.Get(ctx, &getReq)
	if err != nil {
		return err
	}
	_, err = getSteram.Recv()
	if err == io.EOF {
		return nil
	}
	return err
}

// function GribiModify implements a sample request for service /gribi.gRIBI/Modify to validate if authz works as expected.
func GribiModify(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	gribiC, err := dut.RawAPIs().BindingDUT().DialGRIBI(ctx, opts...)
	if err != nil {
		return err
	}
	mStream, err := gribiC.Modify(ctx)
	if err != nil {
		return err
	}
	err = mStream.Send(&gribi.ModifyRequest{})
	if err != nil {
		return err
	}
	_, err = mStream.Recv()
	return err
}

// function P4P4runtimeAllRPC implements a sample request for service /p4.v1.P4Runtime/* to validate if authz works as expected.
func P4P4runtimeAllRPC(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/* is not implemented")
}

// function P4P4runtimeCapabilities implements a sample request for service /p4.v1.P4Runtime/Capabilities to validate if authz works as expected.
func P4P4runtimeCapabilities(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/Capabilities is not implemented")
}

// function P4P4runtimeGetForwardingPipelineConfig implements a sample request for service /p4.v1.P4Runtime/GetForwardingPipelineConfig to validate if authz works as expected.
func P4P4runtimeGetForwardingPipelineConfig(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/GetForwardingPipelineConfig is not implemented")
}

// function P4P4runtimeRead implements a sample request for service /p4.v1.P4Runtime/Read to validate if authz works as expected.
func P4P4runtimeRead(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/Read is not implemented")
}

// function P4P4runtimeSetForwardingPipelineConfig implements a sample request for service /p4.v1.P4Runtime/SetForwardingPipelineConfig to validate if authz works as expected.
func P4P4runtimeSetForwardingPipelineConfig(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/SetForwardingPipelineConfig is not implemented")
}

// function P4P4runtimeStreamChannel implements a sample request for service /p4.v1.P4Runtime/StreamChannel to validate if authz works as expected.
func P4P4runtimeStreamChannel(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/StreamChannel is not implemented")
}

// function P4P4runtimeWrite implements a sample request for service /p4.v1.P4Runtime/Write to validate if authz works as expected.
func P4P4runtimeWrite(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, params ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/Write is not implemented")
}
