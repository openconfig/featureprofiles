package gnxi

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	grpb "github.com/openconfig/gribi/v1/proto/service"
)

// AllRPC implements a sample request for service * to validate if authz works as expected.
func AllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC * is not implemented")
}

// GnmiAllRPC implements a sample request for service /gnmi.gNMI/* to validate if authz works as expected.
func GnmiAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnmi.gNMI/* is not implemented")
}

// GnmiGet implements a sample request for service /gnmi.gNMI/Get to validate if authz works as expected.
func GnmiGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
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
	if err != nil && strings.Contains(err.Error(), "value not present") {
		return nil
	}
	return err
}

// GnmiSet implements a sample request for service /gnmi.gNMI/Set to validate if authz works as expected.
func GnmiSet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
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

// GnmiSubscribe implements a sample request for service /gnmi.gNMI/Subscribe to validate if authz works as expected.
func GnmiSubscribe(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
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

// GnmiCapabilities implements a sample request for service /gnmi.gNMI/Capabilities to validate if authz works as expected.
func GnmiCapabilities(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gnmiC, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx, opts...)
	if err != nil {
		return err
	}
	_, err = gnmiC.Capabilities(ctx, &gpb.CapabilityRequest{})
	return err
}

// GnoiBgpAllRPC implements a sample request for service /gnoi.bgp.BGP/* to validate if authz works as expected.
func GnoiBgpAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.bgp.BGP/* is not implemented")
}

// GnoiBgpClearBGPNeighbor implements a sample request for service /gnoi.bgp.BGP/ClearBGPNeighbor to validate if authz works as expected.
func GnoiBgpClearBGPNeighbor(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.bgp.BGP/ClearBGPNeighbor is not implemented")
}

// GnoiCertificatemanagementAllRPC implements a sample request for service /gnoi.certificate.CertificateManagement/* to validate if authz works as expected.
func GnoiCertificatemanagementAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/* is not implemented")
}

// GnoiCertificatemanagementCanGenerateCSR implements a sample request for service /gnoi.certificate.CertificateManagement/CanGenerateCSR to validate if authz works as expected.
func GnoiCertificatemanagementCanGenerateCSR(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/CanGenerateCSR is not implemented")
}

// GnoiCertificatemanagementGenerateCSR implements a sample request for service /gnoi.certificate.CertificateManagement/GenerateCSR to validate if authz works as expected.
func GnoiCertificatemanagementGenerateCSR(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/GenerateCSR is not implemented")
}

// GnoiCertificatemanagementGetCertificates implements a sample request for service /gnoi.certificate.CertificateManagement/GetCertificates to validate if authz works as expected.
func GnoiCertificatemanagementGetCertificates(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/GetCertificates is not implemented")
}

// GnoiCertificatemanagementInstall implements a sample request for service /gnoi.certificate.CertificateManagement/Install to validate if authz works as expected.
func GnoiCertificatemanagementInstall(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/Install is not implemented")
}

// GnoiCertificatemanagementLoadCertificate implements a sample request for service /gnoi.certificate.CertificateManagement/LoadCertificate to validate if authz works as expected.
func GnoiCertificatemanagementLoadCertificate(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/LoadCertificate is not implemented")
}

// GnoiCertificatemanagementLoadCertificateAuthorityBundle implements a sample request for service /gnoi.certificate.CertificateManagement/LoadCertificateAuthorityBundle to validate if authz works as expected.
func GnoiCertificatemanagementLoadCertificateAuthorityBundle(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/LoadCertificateAuthorityBundle is not implemented")
}

// GnoiCertificatemanagementRevokeCertificates implements a sample request for service /gnoi.certificate.CertificateManagement/RevokeCertificates to validate if authz works as expected.
func GnoiCertificatemanagementRevokeCertificates(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/RevokeCertificates is not implemented")
}

// GnoiCertificatemanagementRotate implements a sample request for service /gnoi.certificate.CertificateManagement/Rotate to validate if authz works as expected.
func GnoiCertificatemanagementRotate(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.certificate.CertificateManagement/Rotate is not implemented")
}

// GnoiDiagAllRPC implements a sample request for service /gnoi.diag.Diag/* to validate if authz works as expected.
func GnoiDiagAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.diag.Diag/* is not implemented")
}

// GnoiDiagGetBERTResult implements a sample request for service /gnoi.diag.Diag/GetBERTResult to validate if authz works as expected.
func GnoiDiagGetBERTResult(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.diag.Diag/GetBERTResult is not implemented")
}

// GnoiDiagStopBERT implements a sample request for service /gnoi.diag.Diag/StopBERT to validate if authz works as expected.
func GnoiDiagStopBERT(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.diag.Diag/StopBERT is not implemented")
}

// GnoiDiagStartBERT implements a sample request for service /gnoi.diag.Diag/StartBERT to validate if authz works as expected.
func GnoiDiagStartBERT(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.diag.Diag/StartBERT is not implemented")
}

// GnoiFactoryresetAllRPC implements a sample request for service /gnoi.factory_reset.FactoryReset/* to validate if authz works as expected.
func GnoiFactoryresetAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.factory_reset.FactoryReset/* is not implemented")
}

// GnoiFactoryresetStart implements a sample request for service /gnoi.factory_reset.FactoryReset/Start to validate if authz works as expected.
func GnoiFactoryresetStart(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.factory_reset.FactoryReset/Start is not implemented")
}

// GnoiFileAllRPC implements a sample request for service /gnoi.file.File/* to validate if authz works as expected.
func GnoiFileAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/* is not implemented")
}

// GnoiFilePut implements a sample request for service /gnoi.file.File/Put to validate if authz works as expected.
func GnoiFilePut(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/Put is not implemented")
}

// GnoiFileRemove implements a sample request for service /gnoi.file.File/Remove to validate if authz works as expected.
func GnoiFileRemove(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/Remove is not implemented")
}

// GnoiFileStat implements a sample request for service /gnoi.file.File/Stat to validate if authz works as expected.
func GnoiFileStat(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/Stat is not implemented")
}

// GnoiFileTransferToRemote implements a sample request for service /gnoi.file.File/TransferToRemote to validate if authz works as expected.
func GnoiFileTransferToRemote(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/TransferToRemote is not implemented")
}

// GnoiFileGet implements a sample request for service /gnoi.file.File/Get to validate if authz works as expected.
func GnoiFileGet(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.file.File/Get is not implemented")
}

// GnoiHealthzAcknowledge implements a sample request for service /gnoi.healthz.Healthz/Acknowledge to validate if authz works as expected.
func GnoiHealthzAcknowledge(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/Acknowledge is not implemented")
}

// GnoiHealthzAllRPC implements a sample request for service /gnoi.healthz.Healthz/* to validate if authz works as expected.
func GnoiHealthzAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/* is not implemented")
}

// GnoiHealthzArtifact implements a sample request for service /gnoi.healthz.Healthz/Artifact to validate if authz works as expected.
func GnoiHealthzArtifact(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/Artifact is not implemented")
}

// GnoiHealthzCheck implements a sample request for service /gnoi.healthz.Healthz/Check to validate if authz works as expected.
func GnoiHealthzCheck(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/Check is not implemented")
}

// GnoiHealthzList implements a sample request for service /gnoi.healthz.Healthz/List to validate if authz works as expected.
func GnoiHealthzList(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/List is not implemented")
}

// GnoiHealthzGet implements a sample request for service /gnoi.healthz.Healthz/Get to validate if authz works as expected.
func GnoiHealthzGet(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.healthz.Healthz/Get is not implemented")
}

// GnoiLayer2AllRPC implements a sample request for service /gnoi.layer2.Layer2/* to validate if authz works as expected.
func GnoiLayer2AllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/* is not implemented")
}

// GnoiLayer2ClearLLDPInterface implements a sample request for service /gnoi.layer2.Layer2/ClearLLDPInterface to validate if authz works as expected.
func GnoiLayer2ClearLLDPInterface(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/ClearLLDPInterface is not implemented")
}

// GnoiLayer2ClearSpanningTree implements a sample request for service /gnoi.layer2.Layer2/ClearSpanningTree to validate if authz works as expected.
func GnoiLayer2ClearSpanningTree(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/ClearSpanningTree is not implemented")
}

// GnoiLayer2PerformBERT implements a sample request for service /gnoi.layer2.Layer2/PerformBERT to validate if authz works as expected.
func GnoiLayer2PerformBERT(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/PerformBERT is not implemented")
}

// GnoiLayer2SendWakeOnLAN implements a sample request for service /gnoi.layer2.Layer2/SendWakeOnLAN to validate if authz works as expected.
func GnoiLayer2SendWakeOnLAN(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/SendWakeOnLAN is not implemented")
}

// GnoiLayer2ClearNeighborDiscovery implements a sample request for service /gnoi.layer2.Layer2/ClearNeighborDiscovery to validate if authz works as expected.
func GnoiLayer2ClearNeighborDiscovery(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.layer2.Layer2/ClearNeighborDiscovery is not implemented")
}

// GnoiLinkqualificationCreate implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Create to validate if authz works as expected.
func GnoiLinkqualificationCreate(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Create is not implemented")
}

// GnoiMplsAllRPC implements a sample request for service /gnoi.mpls.MPLS/* to validate if authz works as expected.
func GnoiMplsAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.mpls.MPLS/* is not implemented")
}

// GnoiMplsClearLSPCounters implements a sample request for service /gnoi.mpls.MPLS/ClearLSPCounters to validate if authz works as expected.
func GnoiMplsClearLSPCounters(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.mpls.MPLS/ClearLSPCounters is not implemented")
}

// GnoiMplsMPLSPing implements a sample request for service /gnoi.mpls.MPLS/MPLSPing to validate if authz works as expected.
func GnoiMplsMPLSPing(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.mpls.MPLS/MPLSPing is not implemented")
}

// GnoiMplsClearLSP implements a sample request for service /gnoi.mpls.MPLS/ClearLSP to validate if authz works as expected.
func GnoiMplsClearLSP(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.mpls.MPLS/ClearLSP is not implemented")
}

// GnoiOtdrAllRPC implements a sample request for service /gnoi.optical.OTDR/* to validate if authz works as expected.
func GnoiOtdrAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.OTDR/* is not implemented")
}

// GnoiWavelengthrouterAdjustSpectrum implements a sample request for service /gnoi.optical.WavelengthRouter/AdjustSpectrum to validate if authz works as expected.
func GnoiWavelengthrouterAdjustSpectrum(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/AdjustSpectrum is not implemented")
}

// GnoiWavelengthrouterAllRPC implements a sample request for service /gnoi.optical.WavelengthRouter/* to validate if authz works as expected.
func GnoiWavelengthrouterAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/* is not implemented")
}

// GnoiWavelengthrouterCancelAdjustPSD implements a sample request for service /gnoi.optical.WavelengthRouter/CancelAdjustPSD to validate if authz works as expected.
func GnoiWavelengthrouterCancelAdjustPSD(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/CancelAdjustPSD is not implemented")
}

// GnoiWavelengthrouterCancelAdjustSpectrum implements a sample request for service /gnoi.optical.WavelengthRouter/CancelAdjustSpectrum to validate if authz works as expected.
func GnoiWavelengthrouterCancelAdjustSpectrum(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/CancelAdjustSpectrum is not implemented")
}

// GnoiOsActivate implements a sample request for service /gnoi.os.OS/Activate to validate if authz works as expected.
func GnoiOsActivate(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.os.OS/Activate is not implemented")
}

// GnoiOsAllRPC implements a sample request for service /gnoi.os.OS/* to validate if authz works as expected.
func GnoiOsAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.os.OS/* is not implemented")
}

// GnoiOsVerify implements a sample request for service /gnoi.os.OS/Verify to validate if authz works as expected.
func GnoiOsVerify(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.os.OS/Verify is not implemented")
}

// GnoiOsInstall implements a sample request for service /gnoi.os.OS/Install to validate if authz works as expected.
func GnoiOsInstall(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.os.OS/Install is not implemented")
}

// GnoiOtdrInitiate implements a sample request for service /gnoi.optical.OTDR/Initiate to validate if authz works as expected.
func GnoiOtdrInitiate(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.OTDR/Initiate is not implemented")
}

// GnoiLinkqualificationAllRPC implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/* to validate if authz works as expected.
func GnoiLinkqualificationAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/* is not implemented")
}

// GnoiLinkqualificationCapabilities implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Capabilities to validate if authz works as expected.
func GnoiLinkqualificationCapabilities(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Capabilities is not implemented")
}

// GnoiLinkqualificationDelete implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Delete to validate if authz works as expected.
func GnoiLinkqualificationDelete(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Delete is not implemented")
}

// GnoiLinkqualificationGet implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/Get to validate if authz works as expected.
func GnoiLinkqualificationGet(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/Get is not implemented")
}

// GnoiLinkqualificationList implements a sample request for service /gnoi.packet_link_qualification.LinkQualification/List to validate if authz works as expected.
func GnoiLinkqualificationList(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.packet_link_qualification.LinkQualification/List is not implemented")
}

// GnoiSystemAllRPC implements a sample request for service /gnoi.system.System/* to validate if authz works as expected.
func GnoiSystemAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/* is not implemented")
}

// GnoiSystemCancelReboot implements a sample request for service /gnoi.system.System/CancelReboot to validate if authz works as expected.
func GnoiSystemCancelReboot(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/CancelReboot is not implemented")
}

// GnoiSystemKillProcess implements a sample request for service /gnoi.system.System/KillProcess to validate if authz works as expected.
func GnoiSystemKillProcess(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/KillProcess is not implemented")
}

// GnoiSystemReboot implements a sample request for service /gnoi.system.System/Reboot to validate if authz works as expected.
func GnoiSystemReboot(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/Reboot is not implemented")
}

// GnoiSystemRebootStatus implements a sample request for service /gnoi.system.System/RebootStatus to validate if authz works as expected.
func GnoiSystemRebootStatus(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/RebootStatus is not implemented")
}

// GnoiSystemSetPackage implements a sample request for service /gnoi.system.System/SetPackage to validate if authz works as expected.
func GnoiSystemSetPackage(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/SetPackage is not implemented")
}

// GnoiSystemSwitchControlProcessor implements a sample request for service /gnoi.system.System/SwitchControlProcessor to validate if authz works as expected.
func GnoiSystemSwitchControlProcessor(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/SwitchControlProcessor is not implemented")
}

// GnoiSystemTime implements a sample request for service /gnoi.system.System/Time to validate if authz works as expected.
func GnoiSystemTime(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gnoiC, err := dut.RawAPIs().BindingDUT().DialGNOI(ctx, opts...)
	if err != nil {
		return err
	}
	_, err = gnoiC.System().Time(ctx, &spb.TimeRequest{})
	return err
}

// GnoiSystemTraceroute implements a sample request for service /gnoi.system.System/Traceroute to validate if authz works as expected.
func GnoiSystemTraceroute(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.system.System/Traceroute is not implemented")
}

// GnoiSystemPing implements a sample request for service /gnoi.system.System/Ping to validate if authz works as expected.
func GnoiSystemPing(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gnoiC, err := dut.RawAPIs().BindingDUT().DialGNOI(ctx, opts...)
	if err != nil {
		return err
	}
	pingC, err := gnoiC.System().Ping(ctx, &spb.PingRequest{Destination: "192.0.2.1"})
	if err != nil {
		return err

	}
	_, err = pingC.Recv()
	return err
}

// GnoiWavelengthrouterAdjustPSD implements a sample request for service /gnoi.optical.WavelengthRouter/AdjustPSD to validate if authz works as expected.
func GnoiWavelengthrouterAdjustPSD(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnoi.optical.WavelengthRouter/AdjustPSD is not implemented")
}

// GnsiAccountingpullAllRPC implements a sample request for service /gnsi.accounting.v1.AccountingPull/* to validate if authz works as expected.
func GnsiAccountingpullAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.accounting.v1.AccountingPull/* is not implemented")
}

// GnsiAccountingpushRecordStream implements a sample request for service /gnsi.accounting.v1.AccountingPush/RecordStream to validate if authz works as expected.
func GnsiAccountingpushRecordStream(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.accounting.v1.AccountingPush/RecordStream is not implemented")
}

// GnsiAccountingpushAllRPC implements a sample request for service /gnsi.accounting.v1.AccountingPush/* to validate if authz works as expected.
func GnsiAccountingpushAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.accounting.v1.AccountingPush/* is not implemented")
}

// GnsiAccountingpullRecordStream implements a sample request for service /gnsi.accounting.v1.AccountingPull/RecordStream to validate if authz works as expected.
func GnsiAccountingpullRecordStream(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.accounting.v1.AccountingPull/RecordStream is not implemented")
}

// GnsiAuthzAllRPC implements a sample request for service /gnsi.authz.v1.Authz/* to validate if authz works as expected.
func GnsiAuthzAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.authz.v1.Authz/* is not implemented")
}

// GnsiAuthzGet implements a sample request for service /gnsi.authz.v1.Authz/Get to validate if authz works as expected.
func GnsiAuthzGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx, opts...)
	if err != nil {
		return err
	}
	_, err = gnsiC.Authz().Get(ctx, &authzpb.GetRequest{})
	return err
}

// GnsiAuthzProbe implements a sample request for service /gnsi.authz.v1.Authz/Probe to validate if authz works as expected.
func GnsiAuthzProbe(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx, opts...)
	if err != nil {
		return err
	}
	_, err = gnsiC.Authz().Probe(ctx, &authzpb.ProbeRequest{User: "dummy", Rpc: "*"})
	return err
}

// GnsiAuthzRotate implements a sample request for service /gnsi.authz.v1.Authz/Rotate to validate if authz works as expected.
func GnsiAuthzRotate(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx, opts...)
	if err != nil {
		return err
	}
	gnsiCStream, err := gnsiC.Authz().Rotate(ctx)
	if err != nil {
		return err
	}
	// TODO: send valid policy for postive cases
	err = gnsiCStream.Send(&authzpb.RotateAuthzRequest{
		RotateRequest: &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   "0.0",
				CreatedOn: uint64(time.Now().Nanosecond()),
				Policy:    "",
			},
		},
	})
	if err != nil {
		return err
	}
	_, err = gnsiCStream.Recv()
	// invalid policy is expected since the empty policy is not allowed
	if strings.Contains(err.Error(), "invalid policy") {
		return nil
	}
	return err
}

// GnsiCertzAddProfile implements a sample request for service /gnsi.certz.v1.Certz/AddProfile to validate if authz works as expected.
func GnsiCertzAddProfile(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/AddProfile is not implemented")
}

// GnsiCertzAllRPC implements a sample request for service /gnsi.certz.v1.Certz/* to validate if authz works as expected.
func GnsiCertzAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/* is not implemented")
}

// GnsiCertzCanGenerateCSR implements a sample request for service /gnsi.certz.v1.Certz/CanGenerateCSR to validate if authz works as expected.
func GnsiCertzCanGenerateCSR(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/CanGenerateCSR is not implemented")
}

// GnsiCertzDeleteProfile implements a sample request for service /gnsi.certz.v1.Certz/DeleteProfile to validate if authz works as expected.
func GnsiCertzDeleteProfile(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/DeleteProfile is not implemented")
}

// GnsiCertzGetProfileList implements a sample request for service /gnsi.certz.v1.Certz/GetProfileList to validate if authz works as expected.
func GnsiCertzGetProfileList(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/GetProfileList is not implemented")
}

// GnsiCertzRotate implements a sample request for service /gnsi.certz.v1.Certz/Rotate to validate if authz works as expected.
func GnsiCertzRotate(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.certz.v1.Certz/Rotate is not implemented")
}

// GnsiCredentialzAllRPC implements a sample request for service /gnsi.credentialz.v1.Credentialz/* to validate if authz works as expected.
func GnsiCredentialzAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/* is not implemented")
}

// GnsiCredentialzCanGenerateKey implements a sample request for service /gnsi.credentialz.v1.Credentialz/CanGenerateKey to validate if authz works as expected.
func GnsiCredentialzCanGenerateKey(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/CanGenerateKey is not implemented")
}

// GnsiCredentialzGetPublicKeys implements a sample request for service /gnsi.credentialz.v1.Credentialz/GetPublicKeys to validate if authz works as expected.
func GnsiCredentialzGetPublicKeys(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/GetPublicKeys is not implemented")
}

// GnsiCredentialzRotateHostCredentials implements a sample request for service /gnsi.credentialz.v1.Credentialz/RotateHostCredentials to validate if authz works as expected.
func GnsiCredentialzRotateHostCredentials(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/RotateHostCredentials is not implemented")
}

// GnsiCredentialzRotateAccountCredentials implements a sample request for service /gnsi.credentialz.v1.Credentialz/RotateAccountCredentials to validate if authz works as expected.
func GnsiCredentialzRotateAccountCredentials(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.credentialz.v1.Credentialz/RotateAccountCredentials is not implemented")
}

// GnsiPathzAllRPC implements a sample request for service /gnsi.pathz.v1.Pathz/* to validate if authz works as expected.
func GnsiPathzAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.pathz.v1.Pathz/* is not implemented")
}

// GnsiPathzGet implements a sample request for service /gnsi.pathz.v1.Pathz/Get to validate if authz works as expected.
func GnsiPathzGet(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.pathz.v1.Pathz/Get is not implemented")
}

// GnsiPathzProbe implements a sample request for service /gnsi.pathz.v1.Pathz/Probe to validate if authz works as expected.
func GnsiPathzProbe(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.pathz.v1.Pathz/Probe is not implemented")
}

// GnsiPathzRotate implements a sample request for service /gnsi.pathz.v1.Pathz/Rotate to validate if authz works as expected.
func GnsiPathzRotate(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.pathz.v1.Pathz/Rotate is not implemented")
}

// GribiAllRPC implements a sample request for service /gribi.gRIBI/* to validate if authz works as expected.
func GribiAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gribi.gRIBI/* is not implemented")
}

// GribiFlush implements a sample request for service /gribi.gRIBI/Flush to validate if authz works as expected.
func GribiFlush(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gribiC, err := dut.RawAPIs().BindingDUT().DialGRIBI(ctx, opts...)
	if err != nil {
		return err
	}
	_, err = gribiC.Flush(ctx, &grpb.FlushRequest{Election: &grpb.FlushRequest_Id{Id: &grpb.Uint128{Low: 1}}})
	return err
}

// GribiGet implements a sample request for service /gribi.gRIBI/Get to validate if authz works as expected.
func GribiGet(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gribiC, err := dut.RawAPIs().BindingDUT().DialGRIBI(ctx, opts...)
	if err != nil {
		return err
	}
	getReq := grpb.GetRequest{
		NetworkInstance: &grpb.GetRequest_All{},
		Aft:             grpb.AFTType_ALL,
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

// GribiModify implements a sample request for service /gribi.gRIBI/Modify to validate if authz works as expected.
func GribiModify(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gribiC, err := dut.RawAPIs().BindingDUT().DialGRIBI(ctx, opts...)
	if err != nil {
		return err
	}
	mStream, err := gribiC.Modify(ctx)
	if err != nil {
		return err
	}
	err = mStream.Send(&grpb.ModifyRequest{
		Params: &grpb.SessionParameters{Redundancy: grpb.SessionParameters_SINGLE_PRIMARY,
			Persistence: grpb.SessionParameters_PRESERVE},
	})
	if err != nil {
		return err
	}
	_, err = mStream.Recv()
	return err
}

// P4P4runtimeAllRPC implements a sample request for service /p4.v1.P4Runtime/* to validate if authz works as expected.
func P4P4runtimeAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/* is not implemented")
}

// P4P4runtimeCapabilities implements a sample request for service /p4.v1.P4Runtime/Capabilities to validate if authz works as expected.
func P4P4runtimeCapabilities(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/Capabilities is not implemented")
}

// P4P4runtimeGetForwardingPipelineConfig implements a sample request for service /p4.v1.P4Runtime/GetForwardingPipelineConfig to validate if authz works as expected.
func P4P4runtimeGetForwardingPipelineConfig(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/GetForwardingPipelineConfig is not implemented")
}

// P4P4runtimeRead implements a sample request for service /p4.v1.P4Runtime/Read to validate if authz works as expected.
func P4P4runtimeRead(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/Read is not implemented")
}

// P4P4runtimeSetForwardingPipelineConfig implements a sample request for service /p4.v1.P4Runtime/SetForwardingPipelineConfig to validate if authz works as expected.
func P4P4runtimeSetForwardingPipelineConfig(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/SetForwardingPipelineConfig is not implemented")
}

// P4P4runtimeStreamChannel implements a sample request for service /p4.v1.P4Runtime/StreamChannel to validate if authz works as expected.
func P4P4runtimeStreamChannel(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/StreamChannel is not implemented")
}

// P4P4runtimeWrite implements a sample request for service /p4.v1.P4Runtime/Write to validate if authz works as expected.
func P4P4runtimeWrite(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /p4.v1.P4Runtime/Write is not implemented")
}

// GnsiAcctzAllRPC implements a sample request for service /gnsi.acctz.v1.Acctz/* to validate if authz works as expected.
func GnsiAcctzAllRPC(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.acctz.v1.Acctz/* is not implemented")
}

// GnsiAcctzRecordSubscribe implements a sample request for service /gnsi.acctz.v1.Acctz/RecordSubscribe to validate if authz works as expected.
func GnsiAcctzRecordSubscribe(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.acctz.v1.Acctz/RecordSubscribe is not implemented")
}

// GnsiCredentialzRotateHostParameters implements a sample request for service /gnsi.acctz.v1.Acctz/RecordSubscribe to validate if authz works as expected.
func GnsiCredentialzRotateHostParameters(_ context.Context, _ *ondatra.DUTDevice, _ []grpc.DialOption, _ ...any) error {
	return status.Errorf(codes.Unimplemented, "exec function for RPC /gnsi.acctz.v1.Acctz/RecordSubscribe is not implemented")
}
