// This following provide a list of all RPCs related to FP testing.
// The below code is generated using ../gen/generate.go. Please do not modify.
package gnxi

type rpcs struct {
	ALL                                                                   *RPC
	GNMI_ALL                                                              *RPC
	GNMI_GET                                                              *RPC
	GNMI_SET                                                              *RPC
	GNMI_SUBSCRIBE                                                        *RPC
	GNMI_CAPABILITIES                                                     *RPC
	GNOI_BGP_ALL                                                          *RPC
	GNOI_BGP_CLEARBGPNEIGHBOR                                             *RPC
	GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_ALL                            *RPC
	GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_CANGENERATECSR                 *RPC
	GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_GENERATECSR                    *RPC
	GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_GETCERTIFICATES                *RPC
	GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_INSTALL                        *RPC
	GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_LOADCERTIFICATE                *RPC
	GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_LOADCERTIFICATEAUTHORITYBUNDLE *RPC
	GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_REVOKECERTIFICATES             *RPC
	GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_ROTATE                         *RPC
	GNOI_DIAG_ALL                                                         *RPC
	GNOI_DIAG_GETBERTRESULT                                               *RPC
	GNOI_DIAG_STOPBERT                                                    *RPC
	GNOI_DIAG_STARTBERT                                                   *RPC
	GNOI_FACTORY_RESET_FACTORYRESET_ALL                                   *RPC
	GNOI_FACTORY_RESET_FACTORYRESET_START                                 *RPC
	GNOI_FILE_ALL                                                         *RPC
	GNOI_FILE_PUT                                                         *RPC
	GNOI_FILE_REMOVE                                                      *RPC
	GNOI_FILE_STAT                                                        *RPC
	GNOI_FILE_TRANSFERTOREMOTE                                            *RPC
	GNOI_FILE_GET                                                         *RPC
	GNOI_HEALTHZ_ACKNOWLEDGE                                              *RPC
	GNOI_HEALTHZ_ALL                                                      *RPC
	GNOI_HEALTHZ_ARTIFACT                                                 *RPC
	GNOI_HEALTHZ_CHECK                                                    *RPC
	GNOI_HEALTHZ_LIST                                                     *RPC
	GNOI_HEALTHZ_GET                                                      *RPC
	GNOI_LAYER2_ALL                                                       *RPC
	GNOI_LAYER2_CLEARLLDPINTERFACE                                        *RPC
	GNOI_LAYER2_CLEARSPANNINGTREE                                         *RPC
	GNOI_LAYER2_PERFORMBERT                                               *RPC
	GNOI_LAYER2_SENDWAKEONLAN                                             *RPC
	GNOI_LAYER2_CLEARNEIGHBORDISCOVERY                                    *RPC
	GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_CREATE               *RPC
	GNOI_MPLS_ALL                                                         *RPC
	GNOI_MPLS_CLEARLSPCOUNTERS                                            *RPC
	GNOI_MPLS_MPLSPING                                                    *RPC
	GNOI_MPLS_CLEARLSP                                                    *RPC
	GNOI_OPTICAL_OTDR_ALL                                                 *RPC
	GNOI_OPTICAL_WAVELENGTHROUTER_ADJUSTSPECTRUM                          *RPC
	GNOI_OPTICAL_WAVELENGTHROUTER_ALL                                     *RPC
	GNOI_OPTICAL_WAVELENGTHROUTER_CANCELADJUSTPSD                         *RPC
	GNOI_OPTICAL_WAVELENGTHROUTER_CANCELADJUSTSPECTRUM                    *RPC
	GNOI_OS_ACTIVATE                                                      *RPC
	GNOI_OS_ALL                                                           *RPC
	GNOI_OS_VERIFY                                                        *RPC
	GNOI_OS_INSTALL                                                       *RPC
	GNOI_OPTICAL_OTDR_INITIATE                                            *RPC
	GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_ALL                  *RPC
	GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_CAPABILITIES         *RPC
	GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_DELETE               *RPC
	GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_GET                  *RPC
	GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_LIST                 *RPC
	GNOI_SYSTEM_ALL                                                       *RPC
	GNOI_SYSTEM_CANCELREBOOT                                              *RPC
	GNOI_SYSTEM_KILLPROCESS                                               *RPC
	GNOI_SYSTEM_REBOOT                                                    *RPC
	GNOI_SYSTEM_REBOOTSTATUS                                              *RPC
	GNOI_SYSTEM_SETPACKAGE                                                *RPC
	GNOI_SYSTEM_SWITCHCONTROLPROCESSOR                                    *RPC
	GNOI_SYSTEM_TIME                                                      *RPC
	GNOI_SYSTEM_TRACEROUTE                                                *RPC
	GNOI_SYSTEM_PING                                                      *RPC
	GNOI_OPTICAL_WAVELENGTHROUTER_ADJUSTPSD                               *RPC
	GNSI_ACCOUNTING_V1_ACCOUNTINGPULL_ALL                                 *RPC
	GNSI_ACCOUNTING_V1_ACCOUNTINGPUSH_RECORDSTREAM                        *RPC
	GNSI_ACCOUNTING_V1_ACCOUNTINGPUSH_ALL                                 *RPC
	GNSI_ACCOUNTING_V1_ACCOUNTINGPULL_RECORDSTREAM                        *RPC
	GNSI_AUTHZ_V1_AUTHZ_ALL                                               *RPC
	GNSI_AUTHZ_V1_AUTHZ_GET                                               *RPC
	GNSI_AUTHZ_V1_AUTHZ_PROBE                                             *RPC
	GNSI_AUTHZ_V1_AUTHZ_ROTATE                                            *RPC
	GNSI_CERTZ_V1_CERTZ_ADDPROFILE                                        *RPC
	GNSI_CERTZ_V1_CERTZ_ALL                                               *RPC
	GNSI_CERTZ_V1_CERTZ_CANGENERATECSR                                    *RPC
	GNSI_CERTZ_V1_CERTZ_DELETEPROFILE                                     *RPC
	GNSI_CERTZ_V1_CERTZ_GETPROFILELIST                                    *RPC
	GNSI_CERTZ_V1_CERTZ_ROTATE                                            *RPC
	GNSI_CREDENTIALZ_V1_CREDENTIALZ_ALL                                   *RPC
	GNSI_CREDENTIALZ_V1_CREDENTIALZ_CANGENERATEKEY                        *RPC
	GNSI_CREDENTIALZ_V1_CREDENTIALZ_GETPUBLICKEYS                         *RPC
	GNSI_CREDENTIALZ_V1_CREDENTIALZ_ROTATEHOSTCREDENTIALS                 *RPC
	GNSI_CREDENTIALZ_V1_CREDENTIALZ_ROTATEACCOUNTCREDENTIALS              *RPC
	GNSI_PATHZ_V1_PATHZ_ALL                                               *RPC
	GNSI_PATHZ_V1_PATHZ_GET                                               *RPC
	GNSI_PATHZ_V1_PATHZ_PROBE                                             *RPC
	GNSI_PATHZ_V1_PATHZ_ROTATE                                            *RPC
	GRIBI_ALL                                                             *RPC
	GRIBI_FLUSH                                                           *RPC
	GRIBI_GET                                                             *RPC
	GRIBI_MODIFY                                                          *RPC
	P4_V1_P4RUNTIME_ALL                                                   *RPC
	P4_V1_P4RUNTIME_CAPABILITIES                                          *RPC
	P4_V1_P4RUNTIME_GETFORWARDINGPIPELINECONFIG                           *RPC
	P4_V1_P4RUNTIME_READ                                                  *RPC
	P4_V1_P4RUNTIME_SETFORWARDINGPIPELINECONFIG                           *RPC
	P4_V1_P4RUNTIME_STREAMCHANNEL                                         *RPC
	P4_V1_P4RUNTIME_WRITE                                                 *RPC
}

var (
	// definition of all FP related RPCs
	ALL = &RPC{
		Name:    "*",
		Service: "*",
		FQN:     "*",
		Path:    "*",
		Exec:    AllRPc,
	}
	gnmiALL = &RPC{
		Name:    "*",
		Service: "gnmi.gNMI",
		FQN:     "gnmi.gNMI.*",
		Path:    "/gnmi.gNMI/*",
		Exec:    GnmiAllRPc,
	}
	gnmiGet = &RPC{
		Name:    "Get",
		Service: "gnmi.gNMI",
		FQN:     "gnmi.gNMI.Get",
		Path:    "/gnmi.gNMI/Get",
		Exec:    GnmiGet,
	}
	gnmiSet = &RPC{
		Name:    "Set",
		Service: "gnmi.gNMI",
		FQN:     "gnmi.gNMI.Set",
		Path:    "/gnmi.gNMI/Set",
		Exec:    GnmiSet,
	}
	gnmiSubscribe = &RPC{
		Name:    "Subscribe",
		Service: "gnmi.gNMI",
		FQN:     "gnmi.gNMI.Subscribe",
		Path:    "/gnmi.gNMI/Subscribe",
		Exec:    GnmiSubscribe,
	}
	gnmiCapabilities = &RPC{
		Name:    "Capabilities",
		Service: "gnmi.gNMI",
		FQN:     "gnmi.gNMI.Capabilities",
		Path:    "/gnmi.gNMI/Capabilities",
		Exec:    GnmiCapabilities,
	}
	gnoibgpALL = &RPC{
		Name:    "*",
		Service: "gnoi.bgp.BGP",
		FQN:     "gnoi.bgp.BGP.*",
		Path:    "/gnoi.bgp.BGP/*",
		Exec:    GnoiBgpAllRPc,
	}
	gnoibgpClearBGPNeighbor = &RPC{
		Name:    "ClearBGPNeighbor",
		Service: "gnoi.bgp.BGP",
		FQN:     "gnoi.bgp.BGP.ClearBGPNeighbor",
		Path:    "/gnoi.bgp.BGP/ClearBGPNeighbor",
		Exec:    GnoiBgpClearBGPNeighbor,
	}
	gnoicertificateCertificateManagementALL = &RPC{
		Name:    "*",
		Service: "gnoi.certificate.CertificateManagement",
		FQN:     "gnoi.certificate.CertificateManagement.*",
		Path:    "/gnoi.certificate.CertificateManagement/*",
		Exec:    GnoiCertificatemanagementAllRPc,
	}
	gnoicertificateCertificateManagementCanGenerateCSR = &RPC{
		Name:    "CanGenerateCSR",
		Service: "gnoi.certificate.CertificateManagement",
		FQN:     "gnoi.certificate.CertificateManagement.CanGenerateCSR",
		Path:    "/gnoi.certificate.CertificateManagement/CanGenerateCSR",
		Exec:    GnoiCertificatemanagementCanGenerateCSR,
	}
	gnoicertificateCertificateManagementGenerateCSR = &RPC{
		Name:    "GenerateCSR",
		Service: "gnoi.certificate.CertificateManagement",
		FQN:     "gnoi.certificate.CertificateManagement.GenerateCSR",
		Path:    "/gnoi.certificate.CertificateManagement/GenerateCSR",
		Exec:    GnoiCertificatemanagementGenerateCSR,
	}
	gnoicertificateCertificateManagementGetCertificates = &RPC{
		Name:    "GetCertificates",
		Service: "gnoi.certificate.CertificateManagement",
		FQN:     "gnoi.certificate.CertificateManagement.GetCertificates",
		Path:    "/gnoi.certificate.CertificateManagement/GetCertificates",
		Exec:    GnoiCertificatemanagementGetCertificates,
	}
	gnoicertificateCertificateManagementInstall = &RPC{
		Name:    "Install",
		Service: "gnoi.certificate.CertificateManagement",
		FQN:     "gnoi.certificate.CertificateManagement.Install",
		Path:    "/gnoi.certificate.CertificateManagement/Install",
		Exec:    GnoiCertificatemanagementInstall,
	}
	gnoicertificateCertificateManagementLoadCertificate = &RPC{
		Name:    "LoadCertificate",
		Service: "gnoi.certificate.CertificateManagement",
		FQN:     "gnoi.certificate.CertificateManagement.LoadCertificate",
		Path:    "/gnoi.certificate.CertificateManagement/LoadCertificate",
		Exec:    GnoiCertificatemanagementLoadCertificate,
	}
	gnoicertificateCertificateManagementLoadCertificateAuthorityBundle = &RPC{
		Name:    "LoadCertificateAuthorityBundle",
		Service: "gnoi.certificate.CertificateManagement",
		FQN:     "gnoi.certificate.CertificateManagement.LoadCertificateAuthorityBundle",
		Path:    "/gnoi.certificate.CertificateManagement/LoadCertificateAuthorityBundle",
		Exec:    GnoiCertificatemanagementLoadCertificateAuthorityBundle,
	}
	gnoicertificateCertificateManagementRevokeCertificates = &RPC{
		Name:    "RevokeCertificates",
		Service: "gnoi.certificate.CertificateManagement",
		FQN:     "gnoi.certificate.CertificateManagement.RevokeCertificates",
		Path:    "/gnoi.certificate.CertificateManagement/RevokeCertificates",
		Exec:    GnoiCertificatemanagementRevokeCertificates,
	}
	gnoicertificateCertificateManagementRotate = &RPC{
		Name:    "Rotate",
		Service: "gnoi.certificate.CertificateManagement",
		FQN:     "gnoi.certificate.CertificateManagement.Rotate",
		Path:    "/gnoi.certificate.CertificateManagement/Rotate",
		Exec:    GnoiCertificatemanagementRotate,
	}
	gnoidiagALL = &RPC{
		Name:    "*",
		Service: "gnoi.diag.Diag",
		FQN:     "gnoi.diag.Diag.*",
		Path:    "/gnoi.diag.Diag/*",
		Exec:    GnoiDiagAllRPc,
	}
	gnoidiagGetBERTResult = &RPC{
		Name:    "GetBERTResult",
		Service: "gnoi.diag.Diag",
		FQN:     "gnoi.diag.Diag.GetBERTResult",
		Path:    "/gnoi.diag.Diag/GetBERTResult",
		Exec:    GnoiDiagGetBERTResult,
	}
	gnoidiagStopBERT = &RPC{
		Name:    "StopBERT",
		Service: "gnoi.diag.Diag",
		FQN:     "gnoi.diag.Diag.StopBERT",
		Path:    "/gnoi.diag.Diag/StopBERT",
		Exec:    GnoiDiagStopBERT,
	}
	gnoidiagStartBERT = &RPC{
		Name:    "StartBERT",
		Service: "gnoi.diag.Diag",
		FQN:     "gnoi.diag.Diag.StartBERT",
		Path:    "/gnoi.diag.Diag/StartBERT",
		Exec:    GnoiDiagStartBERT,
	}
	gnoifactory_resetFactoryResetALL = &RPC{
		Name:    "*",
		Service: "gnoi.factory_reset.FactoryReset",
		FQN:     "gnoi.factory_reset.FactoryReset.*",
		Path:    "/gnoi.factory_reset.FactoryReset/*",
		Exec:    GnoiFactoryresetAllRPc,
	}
	gnoifactory_resetFactoryResetStart = &RPC{
		Name:    "Start",
		Service: "gnoi.factory_reset.FactoryReset",
		FQN:     "gnoi.factory_reset.FactoryReset.Start",
		Path:    "/gnoi.factory_reset.FactoryReset/Start",
		Exec:    GnoiFactoryresetStart,
	}
	gnoifileALL = &RPC{
		Name:    "*",
		Service: "gnoi.file.File",
		FQN:     "gnoi.file.File.*",
		Path:    "/gnoi.file.File/*",
		Exec:    GnoiFileAllRPc,
	}
	gnoifilePut = &RPC{
		Name:    "Put",
		Service: "gnoi.file.File",
		FQN:     "gnoi.file.File.Put",
		Path:    "/gnoi.file.File/Put",
		Exec:    GnoiFilePut,
	}
	gnoifileRemove = &RPC{
		Name:    "Remove",
		Service: "gnoi.file.File",
		FQN:     "gnoi.file.File.Remove",
		Path:    "/gnoi.file.File/Remove",
		Exec:    GnoiFileRemove,
	}
	gnoifileStat = &RPC{
		Name:    "Stat",
		Service: "gnoi.file.File",
		FQN:     "gnoi.file.File.Stat",
		Path:    "/gnoi.file.File/Stat",
		Exec:    GnoiFileStat,
	}
	gnoifileTransferToRemote = &RPC{
		Name:    "TransferToRemote",
		Service: "gnoi.file.File",
		FQN:     "gnoi.file.File.TransferToRemote",
		Path:    "/gnoi.file.File/TransferToRemote",
		Exec:    GnoiFileTransferToRemote,
	}
	gnoifileGet = &RPC{
		Name:    "Get",
		Service: "gnoi.file.File",
		FQN:     "gnoi.file.File.Get",
		Path:    "/gnoi.file.File/Get",
		Exec:    GnoiFileGet,
	}
	gnoihealthzAcknowledge = &RPC{
		Name:    "Acknowledge",
		Service: "gnoi.healthz.Healthz",
		FQN:     "gnoi.healthz.Healthz.Acknowledge",
		Path:    "/gnoi.healthz.Healthz/Acknowledge",
		Exec:    GnoiHealthzAcknowledge,
	}
	gnoihealthzALL = &RPC{
		Name:    "*",
		Service: "gnoi.healthz.Healthz",
		FQN:     "gnoi.healthz.Healthz.*",
		Path:    "/gnoi.healthz.Healthz/*",
		Exec:    GnoiHealthzAllRPc,
	}
	gnoihealthzArtifact = &RPC{
		Name:    "Artifact",
		Service: "gnoi.healthz.Healthz",
		FQN:     "gnoi.healthz.Healthz.Artifact",
		Path:    "/gnoi.healthz.Healthz/Artifact",
		Exec:    GnoiHealthzArtifact,
	}
	gnoihealthzCheck = &RPC{
		Name:    "Check",
		Service: "gnoi.healthz.Healthz",
		FQN:     "gnoi.healthz.Healthz.Check",
		Path:    "/gnoi.healthz.Healthz/Check",
		Exec:    GnoiHealthzCheck,
	}
	gnoihealthzList = &RPC{
		Name:    "List",
		Service: "gnoi.healthz.Healthz",
		FQN:     "gnoi.healthz.Healthz.List",
		Path:    "/gnoi.healthz.Healthz/List",
		Exec:    GnoiHealthzList,
	}
	gnoihealthzGet = &RPC{
		Name:    "Get",
		Service: "gnoi.healthz.Healthz",
		FQN:     "gnoi.healthz.Healthz.Get",
		Path:    "/gnoi.healthz.Healthz/Get",
		Exec:    GnoiHealthzGet,
	}
	gnoilayer2ALL = &RPC{
		Name:    "*",
		Service: "gnoi.layer2.Layer2",
		FQN:     "gnoi.layer2.Layer2.*",
		Path:    "/gnoi.layer2.Layer2/*",
		Exec:    GnoiLayer2AllRPc,
	}
	gnoilayer2ClearLLDPInterface = &RPC{
		Name:    "ClearLLDPInterface",
		Service: "gnoi.layer2.Layer2",
		FQN:     "gnoi.layer2.Layer2.ClearLLDPInterface",
		Path:    "/gnoi.layer2.Layer2/ClearLLDPInterface",
		Exec:    GnoiLayer2ClearLLDPInterface,
	}
	gnoilayer2ClearSpanningTree = &RPC{
		Name:    "ClearSpanningTree",
		Service: "gnoi.layer2.Layer2",
		FQN:     "gnoi.layer2.Layer2.ClearSpanningTree",
		Path:    "/gnoi.layer2.Layer2/ClearSpanningTree",
		Exec:    GnoiLayer2ClearSpanningTree,
	}
	gnoilayer2PerformBERT = &RPC{
		Name:    "PerformBERT",
		Service: "gnoi.layer2.Layer2",
		FQN:     "gnoi.layer2.Layer2.PerformBERT",
		Path:    "/gnoi.layer2.Layer2/PerformBERT",
		Exec:    GnoiLayer2PerformBERT,
	}
	gnoilayer2SendWakeOnLAN = &RPC{
		Name:    "SendWakeOnLAN",
		Service: "gnoi.layer2.Layer2",
		FQN:     "gnoi.layer2.Layer2.SendWakeOnLAN",
		Path:    "/gnoi.layer2.Layer2/SendWakeOnLAN",
		Exec:    GnoiLayer2SendWakeOnLAN,
	}
	gnoilayer2ClearNeighborDiscovery = &RPC{
		Name:    "ClearNeighborDiscovery",
		Service: "gnoi.layer2.Layer2",
		FQN:     "gnoi.layer2.Layer2.ClearNeighborDiscovery",
		Path:    "/gnoi.layer2.Layer2/ClearNeighborDiscovery",
		Exec:    GnoiLayer2ClearNeighborDiscovery,
	}
	gnoipacket_link_qualificationLinkQualificationCreate = &RPC{
		Name:    "Create",
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.Create",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/Create",
		Exec:    GnoiLinkqualificationCreate,
	}
	gnoimplsALL = &RPC{
		Name:    "*",
		Service: "gnoi.mpls.MPLS",
		FQN:     "gnoi.mpls.MPLS.*",
		Path:    "/gnoi.mpls.MPLS/*",
		Exec:    GnoiMplsAllRPc,
	}
	gnoimplsClearLSPCounters = &RPC{
		Name:    "ClearLSPCounters",
		Service: "gnoi.mpls.MPLS",
		FQN:     "gnoi.mpls.MPLS.ClearLSPCounters",
		Path:    "/gnoi.mpls.MPLS/ClearLSPCounters",
		Exec:    GnoiMplsClearLSPCounters,
	}
	gnoimplsMPLSPing = &RPC{
		Name:    "MPLSPing",
		Service: "gnoi.mpls.MPLS",
		FQN:     "gnoi.mpls.MPLS.MPLSPing",
		Path:    "/gnoi.mpls.MPLS/MPLSPing",
		Exec:    GnoiMplsMPLSPing,
	}
	gnoimplsClearLSP = &RPC{
		Name:    "ClearLSP",
		Service: "gnoi.mpls.MPLS",
		FQN:     "gnoi.mpls.MPLS.ClearLSP",
		Path:    "/gnoi.mpls.MPLS/ClearLSP",
		Exec:    GnoiMplsClearLSP,
	}
	gnoiopticalOTDRALL = &RPC{
		Name:    "*",
		Service: "gnoi.optical.OTDR",
		FQN:     "gnoi.optical.OTDR.*",
		Path:    "/gnoi.optical.OTDR/*",
		Exec:    GnoiOtdrAllRPc,
	}
	gnoiopticalWavelengthRouterAdjustSpectrum = &RPC{
		Name:    "AdjustSpectrum",
		Service: "gnoi.optical.WavelengthRouter",
		FQN:     "gnoi.optical.WavelengthRouter.AdjustSpectrum",
		Path:    "/gnoi.optical.WavelengthRouter/AdjustSpectrum",
		Exec:    GnoiWavelengthrouterAdjustSpectrum,
	}
	gnoiopticalWavelengthRouterALL = &RPC{
		Name:    "*",
		Service: "gnoi.optical.WavelengthRouter",
		FQN:     "gnoi.optical.WavelengthRouter.*",
		Path:    "/gnoi.optical.WavelengthRouter/*",
		Exec:    GnoiWavelengthrouterAllRPc,
	}
	gnoiopticalWavelengthRouterCancelAdjustPSD = &RPC{
		Name:    "CancelAdjustPSD",
		Service: "gnoi.optical.WavelengthRouter",
		FQN:     "gnoi.optical.WavelengthRouter.CancelAdjustPSD",
		Path:    "/gnoi.optical.WavelengthRouter/CancelAdjustPSD",
		Exec:    GnoiWavelengthrouterCancelAdjustPSD,
	}
	gnoiopticalWavelengthRouterCancelAdjustSpectrum = &RPC{
		Name:    "CancelAdjustSpectrum",
		Service: "gnoi.optical.WavelengthRouter",
		FQN:     "gnoi.optical.WavelengthRouter.CancelAdjustSpectrum",
		Path:    "/gnoi.optical.WavelengthRouter/CancelAdjustSpectrum",
		Exec:    GnoiWavelengthrouterCancelAdjustSpectrum,
	}
	gnoiosActivate = &RPC{
		Name:    "Activate",
		Service: "gnoi.os.OS",
		FQN:     "gnoi.os.OS.Activate",
		Path:    "/gnoi.os.OS/Activate",
		Exec:    GnoiOsActivate,
	}
	gnoiosALL = &RPC{
		Name:    "*",
		Service: "gnoi.os.OS",
		FQN:     "gnoi.os.OS.*",
		Path:    "/gnoi.os.OS/*",
		Exec:    GnoiOsAllRPc,
	}
	gnoiosVerify = &RPC{
		Name:    "Verify",
		Service: "gnoi.os.OS",
		FQN:     "gnoi.os.OS.Verify",
		Path:    "/gnoi.os.OS/Verify",
		Exec:    GnoiOsVerify,
	}
	gnoiosInstall = &RPC{
		Name:    "Install",
		Service: "gnoi.os.OS",
		FQN:     "gnoi.os.OS.Install",
		Path:    "/gnoi.os.OS/Install",
		Exec:    GnoiOsInstall,
	}
	gnoiopticalOTDRInitiate = &RPC{
		Name:    "Initiate",
		Service: "gnoi.optical.OTDR",
		FQN:     "gnoi.optical.OTDR.Initiate",
		Path:    "/gnoi.optical.OTDR/Initiate",
		Exec:    GnoiOtdrInitiate,
	}
	gnoipacket_link_qualificationLinkQualificationALL = &RPC{
		Name:    "*",
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.*",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/*",
		Exec:    GnoiLinkqualificationAllRPc,
	}
	gnoipacket_link_qualificationLinkQualificationCapabilities = &RPC{
		Name:    "Capabilities",
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.Capabilities",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/Capabilities",
		Exec:    GnoiLinkqualificationCapabilities,
	}
	gnoipacket_link_qualificationLinkQualificationDelete = &RPC{
		Name:    "Delete",
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.Delete",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/Delete",
		Exec:    GnoiLinkqualificationDelete,
	}
	gnoipacket_link_qualificationLinkQualificationGet = &RPC{
		Name:    "Get",
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.Get",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/Get",
		Exec:    GnoiLinkqualificationGet,
	}
	gnoipacket_link_qualificationLinkQualificationList = &RPC{
		Name:    "List",
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.List",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/List",
		Exec:    GnoiLinkqualificationList,
	}
	gnoisystemALL = &RPC{
		Name:    "*",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.*",
		Path:    "/gnoi.system.System/*",
		Exec:    GnoiSystemAllRPc,
	}
	gnoisystemCancelReboot = &RPC{
		Name:    "CancelReboot",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.CancelReboot",
		Path:    "/gnoi.system.System/CancelReboot",
		Exec:    GnoiSystemCancelReboot,
	}
	gnoisystemKillProcess = &RPC{
		Name:    "KillProcess",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.KillProcess",
		Path:    "/gnoi.system.System/KillProcess",
		Exec:    GnoiSystemKillProcess,
	}
	gnoisystemReboot = &RPC{
		Name:    "Reboot",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.Reboot",
		Path:    "/gnoi.system.System/Reboot",
		Exec:    GnoiSystemReboot,
	}
	gnoisystemRebootStatus = &RPC{
		Name:    "RebootStatus",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.RebootStatus",
		Path:    "/gnoi.system.System/RebootStatus",
		Exec:    GnoiSystemRebootStatus,
	}
	gnoisystemSetPackage = &RPC{
		Name:    "SetPackage",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.SetPackage",
		Path:    "/gnoi.system.System/SetPackage",
		Exec:    GnoiSystemSetPackage,
	}
	gnoisystemSwitchControlProcessor = &RPC{
		Name:    "SwitchControlProcessor",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.SwitchControlProcessor",
		Path:    "/gnoi.system.System/SwitchControlProcessor",
		Exec:    GnoiSystemSwitchControlProcessor,
	}
	gnoisystemTime = &RPC{
		Name:    "Time",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.Time",
		Path:    "/gnoi.system.System/Time",
		Exec:    GnoiSystemTime,
	}
	gnoisystemTraceroute = &RPC{
		Name:    "Traceroute",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.Traceroute",
		Path:    "/gnoi.system.System/Traceroute",
		Exec:    GnoiSystemTraceroute,
	}
	gnoisystemPing = &RPC{
		Name:    "Ping",
		Service: "gnoi.system.System",
		FQN:     "gnoi.system.System.Ping",
		Path:    "/gnoi.system.System/Ping",
		Exec:    GnoiSystemPing,
	}
	gnoiopticalWavelengthRouterAdjustPSD = &RPC{
		Name:    "AdjustPSD",
		Service: "gnoi.optical.WavelengthRouter",
		FQN:     "gnoi.optical.WavelengthRouter.AdjustPSD",
		Path:    "/gnoi.optical.WavelengthRouter/AdjustPSD",
		Exec:    GnoiWavelengthrouterAdjustPSD,
	}
	gnsiaccountingv1AccountingPullALL = &RPC{
		Name:    "*",
		Service: "gnsi.accounting.v1.AccountingPull",
		FQN:     "gnsi.accounting.v1.AccountingPull.*",
		Path:    "/gnsi.accounting.v1.AccountingPull/*",
		Exec:    GnsiAccountingpullAllRPc,
	}
	gnsiaccountingv1AccountingPushRecordStream = &RPC{
		Name:    "RecordStream",
		Service: "gnsi.accounting.v1.AccountingPush",
		FQN:     "gnsi.accounting.v1.AccountingPush.RecordStream",
		Path:    "/gnsi.accounting.v1.AccountingPush/RecordStream",
		Exec:    GnsiAccountingpushRecordStream,
	}
	gnsiaccountingv1AccountingPushALL = &RPC{
		Name:    "*",
		Service: "gnsi.accounting.v1.AccountingPush",
		FQN:     "gnsi.accounting.v1.AccountingPush.*",
		Path:    "/gnsi.accounting.v1.AccountingPush/*",
		Exec:    GnsiAccountingpushAllRPc,
	}
	gnsiaccountingv1AccountingPullRecordStream = &RPC{
		Name:    "RecordStream",
		Service: "gnsi.accounting.v1.AccountingPull",
		FQN:     "gnsi.accounting.v1.AccountingPull.RecordStream",
		Path:    "/gnsi.accounting.v1.AccountingPull/RecordStream",
		Exec:    GnsiAccountingpullRecordStream,
	}
	gnsiauthzv1AuthzALL = &RPC{
		Name:    "*",
		Service: "gnsi.authz.v1.Authz",
		FQN:     "gnsi.authz.v1.Authz.*",
		Path:    "/gnsi.authz.v1.Authz/*",
		Exec:    GnsiAuthzAllRPc,
	}
	gnsiauthzv1AuthzGet = &RPC{
		Name:    "Get",
		Service: "gnsi.authz.v1.Authz",
		FQN:     "gnsi.authz.v1.Authz.Get",
		Path:    "/gnsi.authz.v1.Authz/Get",
		Exec:    GnsiAuthzGet,
	}
	gnsiauthzv1AuthzProbe = &RPC{
		Name:    "Probe",
		Service: "gnsi.authz.v1.Authz",
		FQN:     "gnsi.authz.v1.Authz.Probe",
		Path:    "/gnsi.authz.v1.Authz/Probe",
		Exec:    GnsiAuthzProbe,
	}
	gnsiauthzv1AuthzRotate = &RPC{
		Name:    "Rotate",
		Service: "gnsi.authz.v1.Authz",
		FQN:     "gnsi.authz.v1.Authz.Rotate",
		Path:    "/gnsi.authz.v1.Authz/Rotate",
		Exec:    GnsiAuthzRotate,
	}
	gnsicertzv1CertzAddProfile = &RPC{
		Name:    "AddProfile",
		Service: "gnsi.certz.v1.Certz",
		FQN:     "gnsi.certz.v1.Certz.AddProfile",
		Path:    "/gnsi.certz.v1.Certz/AddProfile",
		Exec:    GnsiCertzAddProfile,
	}
	gnsicertzv1CertzALL = &RPC{
		Name:    "*",
		Service: "gnsi.certz.v1.Certz",
		FQN:     "gnsi.certz.v1.Certz.*",
		Path:    "/gnsi.certz.v1.Certz/*",
		Exec:    GnsiCertzAllRPc,
	}
	gnsicertzv1CertzCanGenerateCSR = &RPC{
		Name:    "CanGenerateCSR",
		Service: "gnsi.certz.v1.Certz",
		FQN:     "gnsi.certz.v1.Certz.CanGenerateCSR",
		Path:    "/gnsi.certz.v1.Certz/CanGenerateCSR",
		Exec:    GnsiCertzCanGenerateCSR,
	}
	gnsicertzv1CertzDeleteProfile = &RPC{
		Name:    "DeleteProfile",
		Service: "gnsi.certz.v1.Certz",
		FQN:     "gnsi.certz.v1.Certz.DeleteProfile",
		Path:    "/gnsi.certz.v1.Certz/DeleteProfile",
		Exec:    GnsiCertzDeleteProfile,
	}
	gnsicertzv1CertzGetProfileList = &RPC{
		Name:    "GetProfileList",
		Service: "gnsi.certz.v1.Certz",
		FQN:     "gnsi.certz.v1.Certz.GetProfileList",
		Path:    "/gnsi.certz.v1.Certz/GetProfileList",
		Exec:    GnsiCertzGetProfileList,
	}
	gnsicertzv1CertzRotate = &RPC{
		Name:    "Rotate",
		Service: "gnsi.certz.v1.Certz",
		FQN:     "gnsi.certz.v1.Certz.Rotate",
		Path:    "/gnsi.certz.v1.Certz/Rotate",
		Exec:    GnsiCertzRotate,
	}
	gnsicredentialzv1CredentialzALL = &RPC{
		Name:    "*",
		Service: "gnsi.credentialz.v1.Credentialz",
		FQN:     "gnsi.credentialz.v1.Credentialz.*",
		Path:    "/gnsi.credentialz.v1.Credentialz/*",
		Exec:    GnsiCredentialzAllRPc,
	}
	gnsicredentialzv1CredentialzCanGenerateKey = &RPC{
		Name:    "CanGenerateKey",
		Service: "gnsi.credentialz.v1.Credentialz",
		FQN:     "gnsi.credentialz.v1.Credentialz.CanGenerateKey",
		Path:    "/gnsi.credentialz.v1.Credentialz/CanGenerateKey",
		Exec:    GnsiCredentialzCanGenerateKey,
	}
	gnsicredentialzv1CredentialzGetPublicKeys = &RPC{
		Name:    "GetPublicKeys",
		Service: "gnsi.credentialz.v1.Credentialz",
		FQN:     "gnsi.credentialz.v1.Credentialz.GetPublicKeys",
		Path:    "/gnsi.credentialz.v1.Credentialz/GetPublicKeys",
		Exec:    GnsiCredentialzGetPublicKeys,
	}
	gnsicredentialzv1CredentialzRotateHostCredentials = &RPC{
		Name:    "RotateHostCredentials",
		Service: "gnsi.credentialz.v1.Credentialz",
		FQN:     "gnsi.credentialz.v1.Credentialz.RotateHostCredentials",
		Path:    "/gnsi.credentialz.v1.Credentialz/RotateHostCredentials",
		Exec:    GnsiCredentialzRotateHostCredentials,
	}
	gnsicredentialzv1CredentialzRotateAccountCredentials = &RPC{
		Name:    "RotateAccountCredentials",
		Service: "gnsi.credentialz.v1.Credentialz",
		FQN:     "gnsi.credentialz.v1.Credentialz.RotateAccountCredentials",
		Path:    "/gnsi.credentialz.v1.Credentialz/RotateAccountCredentials",
		Exec:    GnsiCredentialzRotateAccountCredentials,
	}
	gnsipathzv1PathzALL = &RPC{
		Name:    "*",
		Service: "gnsi.pathz.v1.Pathz",
		FQN:     "gnsi.pathz.v1.Pathz.*",
		Path:    "/gnsi.pathz.v1.Pathz/*",
		Exec:    GnsiPathzAllRPc,
	}
	gnsipathzv1PathzGet = &RPC{
		Name:    "Get",
		Service: "gnsi.pathz.v1.Pathz",
		FQN:     "gnsi.pathz.v1.Pathz.Get",
		Path:    "/gnsi.pathz.v1.Pathz/Get",
		Exec:    GnsiPathzGet,
	}
	gnsipathzv1PathzProbe = &RPC{
		Name:    "Probe",
		Service: "gnsi.pathz.v1.Pathz",
		FQN:     "gnsi.pathz.v1.Pathz.Probe",
		Path:    "/gnsi.pathz.v1.Pathz/Probe",
		Exec:    GnsiPathzProbe,
	}
	gnsipathzv1PathzRotate = &RPC{
		Name:    "Rotate",
		Service: "gnsi.pathz.v1.Pathz",
		FQN:     "gnsi.pathz.v1.Pathz.Rotate",
		Path:    "/gnsi.pathz.v1.Pathz/Rotate",
		Exec:    GnsiPathzRotate,
	}
	gribiALL = &RPC{
		Name:    "*",
		Service: "gribi.gRIBI",
		FQN:     "gribi.gRIBI.*",
		Path:    "/gribi.gRIBI/*",
		Exec:    GribiAllRPc,
	}
	gribiFlush = &RPC{
		Name:    "Flush",
		Service: "gribi.gRIBI",
		FQN:     "gribi.gRIBI.Flush",
		Path:    "/gribi.gRIBI/Flush",
		Exec:    GribiFlush,
	}
	gribiGet = &RPC{
		Name:    "Get",
		Service: "gribi.gRIBI",
		FQN:     "gribi.gRIBI.Get",
		Path:    "/gribi.gRIBI/Get",
		Exec:    GribiGet,
	}
	gribiModify = &RPC{
		Name:    "Modify",
		Service: "gribi.gRIBI",
		FQN:     "gribi.gRIBI.Modify",
		Path:    "/gribi.gRIBI/Modify",
		Exec:    GribiModify,
	}
	p4v1P4RuntimeALL = &RPC{
		Name:    "*",
		Service: "p4.v1.P4Runtime",
		FQN:     "p4.v1.P4Runtime.*",
		Path:    "/p4.v1.P4Runtime/*",
		Exec:    P4P4runtimeAllRPc,
	}
	p4v1P4RuntimeCapabilities = &RPC{
		Name:    "Capabilities",
		Service: "p4.v1.P4Runtime",
		FQN:     "p4.v1.P4Runtime.Capabilities",
		Path:    "/p4.v1.P4Runtime/Capabilities",
		Exec:    P4P4runtimeCapabilities,
	}
	p4v1P4RuntimeGetForwardingPipelineConfig = &RPC{
		Name:    "GetForwardingPipelineConfig",
		Service: "p4.v1.P4Runtime",
		FQN:     "p4.v1.P4Runtime.GetForwardingPipelineConfig",
		Path:    "/p4.v1.P4Runtime/GetForwardingPipelineConfig",
		Exec:    P4P4runtimeGetForwardingPipelineConfig,
	}
	p4v1P4RuntimeRead = &RPC{
		Name:    "Read",
		Service: "p4.v1.P4Runtime",
		FQN:     "p4.v1.P4Runtime.Read",
		Path:    "/p4.v1.P4Runtime/Read",
		Exec:    P4P4runtimeRead,
	}
	p4v1P4RuntimeSetForwardingPipelineConfig = &RPC{
		Name:    "SetForwardingPipelineConfig",
		Service: "p4.v1.P4Runtime",
		FQN:     "p4.v1.P4Runtime.SetForwardingPipelineConfig",
		Path:    "/p4.v1.P4Runtime/SetForwardingPipelineConfig",
		Exec:    P4P4runtimeSetForwardingPipelineConfig,
	}
	p4v1P4RuntimeStreamChannel = &RPC{
		Name:    "StreamChannel",
		Service: "p4.v1.P4Runtime",
		FQN:     "p4.v1.P4Runtime.StreamChannel",
		Path:    "/p4.v1.P4Runtime/StreamChannel",
		Exec:    P4P4runtimeStreamChannel,
	}
	p4v1P4RuntimeWrite = &RPC{
		Name:    "Write",
		Service: "p4.v1.P4Runtime",
		FQN:     "p4.v1.P4Runtime.Write",
		Path:    "/p4.v1.P4Runtime/Write",
		Exec:    P4P4runtimeWrite,
	}

	// RPCs is a list of all FP related RPCs
	RPCs = rpcs{
		ALL:                       ALL,
		GNMI_ALL:                  gnmiALL,
		GNMI_GET:                  gnmiGet,
		GNMI_SET:                  gnmiSet,
		GNMI_SUBSCRIBE:            gnmiSubscribe,
		GNMI_CAPABILITIES:         gnmiCapabilities,
		GNOI_BGP_ALL:              gnoibgpALL,
		GNOI_BGP_CLEARBGPNEIGHBOR: gnoibgpClearBGPNeighbor,
		GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_ALL:                            gnoicertificateCertificateManagementALL,
		GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_CANGENERATECSR:                 gnoicertificateCertificateManagementCanGenerateCSR,
		GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_GENERATECSR:                    gnoicertificateCertificateManagementGenerateCSR,
		GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_GETCERTIFICATES:                gnoicertificateCertificateManagementGetCertificates,
		GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_INSTALL:                        gnoicertificateCertificateManagementInstall,
		GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_LOADCERTIFICATE:                gnoicertificateCertificateManagementLoadCertificate,
		GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_LOADCERTIFICATEAUTHORITYBUNDLE: gnoicertificateCertificateManagementLoadCertificateAuthorityBundle,
		GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_REVOKECERTIFICATES:             gnoicertificateCertificateManagementRevokeCertificates,
		GNOI_CERTIFICATE_CERTIFICATEMANAGEMENT_ROTATE:                         gnoicertificateCertificateManagementRotate,
		GNOI_DIAG_ALL:                         gnoidiagALL,
		GNOI_DIAG_GETBERTRESULT:               gnoidiagGetBERTResult,
		GNOI_DIAG_STOPBERT:                    gnoidiagStopBERT,
		GNOI_DIAG_STARTBERT:                   gnoidiagStartBERT,
		GNOI_FACTORY_RESET_FACTORYRESET_ALL:   gnoifactory_resetFactoryResetALL,
		GNOI_FACTORY_RESET_FACTORYRESET_START: gnoifactory_resetFactoryResetStart,
		GNOI_FILE_ALL:                         gnoifileALL,
		GNOI_FILE_PUT:                         gnoifilePut,
		GNOI_FILE_REMOVE:                      gnoifileRemove,
		GNOI_FILE_STAT:                        gnoifileStat,
		GNOI_FILE_TRANSFERTOREMOTE:            gnoifileTransferToRemote,
		GNOI_FILE_GET:                         gnoifileGet,
		GNOI_HEALTHZ_ACKNOWLEDGE:              gnoihealthzAcknowledge,
		GNOI_HEALTHZ_ALL:                      gnoihealthzALL,
		GNOI_HEALTHZ_ARTIFACT:                 gnoihealthzArtifact,
		GNOI_HEALTHZ_CHECK:                    gnoihealthzCheck,
		GNOI_HEALTHZ_LIST:                     gnoihealthzList,
		GNOI_HEALTHZ_GET:                      gnoihealthzGet,
		GNOI_LAYER2_ALL:                       gnoilayer2ALL,
		GNOI_LAYER2_CLEARLLDPINTERFACE:        gnoilayer2ClearLLDPInterface,
		GNOI_LAYER2_CLEARSPANNINGTREE:         gnoilayer2ClearSpanningTree,
		GNOI_LAYER2_PERFORMBERT:               gnoilayer2PerformBERT,
		GNOI_LAYER2_SENDWAKEONLAN:             gnoilayer2SendWakeOnLAN,
		GNOI_LAYER2_CLEARNEIGHBORDISCOVERY:    gnoilayer2ClearNeighborDiscovery,
		GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_CREATE: gnoipacket_link_qualificationLinkQualificationCreate,
		GNOI_MPLS_ALL:                                        gnoimplsALL,
		GNOI_MPLS_CLEARLSPCOUNTERS:                           gnoimplsClearLSPCounters,
		GNOI_MPLS_MPLSPING:                                   gnoimplsMPLSPing,
		GNOI_MPLS_CLEARLSP:                                   gnoimplsClearLSP,
		GNOI_OPTICAL_OTDR_ALL:                                gnoiopticalOTDRALL,
		GNOI_OPTICAL_WAVELENGTHROUTER_ADJUSTSPECTRUM:         gnoiopticalWavelengthRouterAdjustSpectrum,
		GNOI_OPTICAL_WAVELENGTHROUTER_ALL:                    gnoiopticalWavelengthRouterALL,
		GNOI_OPTICAL_WAVELENGTHROUTER_CANCELADJUSTPSD:        gnoiopticalWavelengthRouterCancelAdjustPSD,
		GNOI_OPTICAL_WAVELENGTHROUTER_CANCELADJUSTSPECTRUM:   gnoiopticalWavelengthRouterCancelAdjustSpectrum,
		GNOI_OS_ACTIVATE:                                     gnoiosActivate,
		GNOI_OS_ALL:                                          gnoiosALL,
		GNOI_OS_VERIFY:                                       gnoiosVerify,
		GNOI_OS_INSTALL:                                      gnoiosInstall,
		GNOI_OPTICAL_OTDR_INITIATE:                           gnoiopticalOTDRInitiate,
		GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_ALL: gnoipacket_link_qualificationLinkQualificationALL,
		GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_CAPABILITIES: gnoipacket_link_qualificationLinkQualificationCapabilities,
		GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_DELETE:       gnoipacket_link_qualificationLinkQualificationDelete,
		GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_GET:          gnoipacket_link_qualificationLinkQualificationGet,
		GNOI_PACKET_LINK_QUALIFICATION_LINKQUALIFICATION_LIST:         gnoipacket_link_qualificationLinkQualificationList,
		GNOI_SYSTEM_ALL:                                          gnoisystemALL,
		GNOI_SYSTEM_CANCELREBOOT:                                 gnoisystemCancelReboot,
		GNOI_SYSTEM_KILLPROCESS:                                  gnoisystemKillProcess,
		GNOI_SYSTEM_REBOOT:                                       gnoisystemReboot,
		GNOI_SYSTEM_REBOOTSTATUS:                                 gnoisystemRebootStatus,
		GNOI_SYSTEM_SETPACKAGE:                                   gnoisystemSetPackage,
		GNOI_SYSTEM_SWITCHCONTROLPROCESSOR:                       gnoisystemSwitchControlProcessor,
		GNOI_SYSTEM_TIME:                                         gnoisystemTime,
		GNOI_SYSTEM_TRACEROUTE:                                   gnoisystemTraceroute,
		GNOI_SYSTEM_PING:                                         gnoisystemPing,
		GNOI_OPTICAL_WAVELENGTHROUTER_ADJUSTPSD:                  gnoiopticalWavelengthRouterAdjustPSD,
		GNSI_ACCOUNTING_V1_ACCOUNTINGPULL_ALL:                    gnsiaccountingv1AccountingPullALL,
		GNSI_ACCOUNTING_V1_ACCOUNTINGPUSH_RECORDSTREAM:           gnsiaccountingv1AccountingPushRecordStream,
		GNSI_ACCOUNTING_V1_ACCOUNTINGPUSH_ALL:                    gnsiaccountingv1AccountingPushALL,
		GNSI_ACCOUNTING_V1_ACCOUNTINGPULL_RECORDSTREAM:           gnsiaccountingv1AccountingPullRecordStream,
		GNSI_AUTHZ_V1_AUTHZ_ALL:                                  gnsiauthzv1AuthzALL,
		GNSI_AUTHZ_V1_AUTHZ_GET:                                  gnsiauthzv1AuthzGet,
		GNSI_AUTHZ_V1_AUTHZ_PROBE:                                gnsiauthzv1AuthzProbe,
		GNSI_AUTHZ_V1_AUTHZ_ROTATE:                               gnsiauthzv1AuthzRotate,
		GNSI_CERTZ_V1_CERTZ_ADDPROFILE:                           gnsicertzv1CertzAddProfile,
		GNSI_CERTZ_V1_CERTZ_ALL:                                  gnsicertzv1CertzALL,
		GNSI_CERTZ_V1_CERTZ_CANGENERATECSR:                       gnsicertzv1CertzCanGenerateCSR,
		GNSI_CERTZ_V1_CERTZ_DELETEPROFILE:                        gnsicertzv1CertzDeleteProfile,
		GNSI_CERTZ_V1_CERTZ_GETPROFILELIST:                       gnsicertzv1CertzGetProfileList,
		GNSI_CERTZ_V1_CERTZ_ROTATE:                               gnsicertzv1CertzRotate,
		GNSI_CREDENTIALZ_V1_CREDENTIALZ_ALL:                      gnsicredentialzv1CredentialzALL,
		GNSI_CREDENTIALZ_V1_CREDENTIALZ_CANGENERATEKEY:           gnsicredentialzv1CredentialzCanGenerateKey,
		GNSI_CREDENTIALZ_V1_CREDENTIALZ_GETPUBLICKEYS:            gnsicredentialzv1CredentialzGetPublicKeys,
		GNSI_CREDENTIALZ_V1_CREDENTIALZ_ROTATEHOSTCREDENTIALS:    gnsicredentialzv1CredentialzRotateHostCredentials,
		GNSI_CREDENTIALZ_V1_CREDENTIALZ_ROTATEACCOUNTCREDENTIALS: gnsicredentialzv1CredentialzRotateAccountCredentials,
		GNSI_PATHZ_V1_PATHZ_ALL:                                  gnsipathzv1PathzALL,
		GNSI_PATHZ_V1_PATHZ_GET:                                  gnsipathzv1PathzGet,
		GNSI_PATHZ_V1_PATHZ_PROBE:                                gnsipathzv1PathzProbe,
		GNSI_PATHZ_V1_PATHZ_ROTATE:                               gnsipathzv1PathzRotate,
		GRIBI_ALL:                                                gribiALL,
		GRIBI_FLUSH:                                              gribiFlush,
		GRIBI_GET:                                                gribiGet,
		GRIBI_MODIFY:                                             gribiModify,
		P4_V1_P4RUNTIME_ALL:                                      p4v1P4RuntimeALL,
		P4_V1_P4RUNTIME_CAPABILITIES:                             p4v1P4RuntimeCapabilities,
		P4_V1_P4RUNTIME_GETFORWARDINGPIPELINECONFIG:              p4v1P4RuntimeGetForwardingPipelineConfig,
		P4_V1_P4RUNTIME_READ:                                     p4v1P4RuntimeRead,
		P4_V1_P4RUNTIME_SETFORWARDINGPIPELINECONFIG:              p4v1P4RuntimeSetForwardingPipelineConfig,
		P4_V1_P4RUNTIME_STREAMCHANNEL:                            p4v1P4RuntimeStreamChannel,
		P4_V1_P4RUNTIME_WRITE:                                    p4v1P4RuntimeWrite,
	}

	// RPCMAP is a helper that  maps path to RPCs data that may be needed in tests.
	RPCMAP = map[string]*RPC{
		"*":                              ALL,
		"/gnmi.gNMI/*":                   gnmiALL,
		"/gnmi.gNMI/Get":                 gnmiGet,
		"/gnmi.gNMI/Set":                 gnmiSet,
		"/gnmi.gNMI/Subscribe":           gnmiSubscribe,
		"/gnmi.gNMI/Capabilities":        gnmiCapabilities,
		"/gnoi.bgp.BGP/*":                gnoibgpALL,
		"/gnoi.bgp.BGP/ClearBGPNeighbor": gnoibgpClearBGPNeighbor,
		"/gnoi.certificate.CertificateManagement/*":                              gnoicertificateCertificateManagementALL,
		"/gnoi.certificate.CertificateManagement/CanGenerateCSR":                 gnoicertificateCertificateManagementCanGenerateCSR,
		"/gnoi.certificate.CertificateManagement/GenerateCSR":                    gnoicertificateCertificateManagementGenerateCSR,
		"/gnoi.certificate.CertificateManagement/GetCertificates":                gnoicertificateCertificateManagementGetCertificates,
		"/gnoi.certificate.CertificateManagement/Install":                        gnoicertificateCertificateManagementInstall,
		"/gnoi.certificate.CertificateManagement/LoadCertificate":                gnoicertificateCertificateManagementLoadCertificate,
		"/gnoi.certificate.CertificateManagement/LoadCertificateAuthorityBundle": gnoicertificateCertificateManagementLoadCertificateAuthorityBundle,
		"/gnoi.certificate.CertificateManagement/RevokeCertificates":             gnoicertificateCertificateManagementRevokeCertificates,
		"/gnoi.certificate.CertificateManagement/Rotate":                         gnoicertificateCertificateManagementRotate,
		"/gnoi.diag.Diag/*":                                              gnoidiagALL,
		"/gnoi.diag.Diag/GetBERTResult":                                  gnoidiagGetBERTResult,
		"/gnoi.diag.Diag/StopBERT":                                       gnoidiagStopBERT,
		"/gnoi.diag.Diag/StartBERT":                                      gnoidiagStartBERT,
		"/gnoi.factory_reset.FactoryReset/*":                             gnoifactory_resetFactoryResetALL,
		"/gnoi.factory_reset.FactoryReset/Start":                         gnoifactory_resetFactoryResetStart,
		"/gnoi.file.File/*":                                              gnoifileALL,
		"/gnoi.file.File/Put":                                            gnoifilePut,
		"/gnoi.file.File/Remove":                                         gnoifileRemove,
		"/gnoi.file.File/Stat":                                           gnoifileStat,
		"/gnoi.file.File/TransferToRemote":                               gnoifileTransferToRemote,
		"/gnoi.file.File/Get":                                            gnoifileGet,
		"/gnoi.healthz.Healthz/Acknowledge":                              gnoihealthzAcknowledge,
		"/gnoi.healthz.Healthz/*":                                        gnoihealthzALL,
		"/gnoi.healthz.Healthz/Artifact":                                 gnoihealthzArtifact,
		"/gnoi.healthz.Healthz/Check":                                    gnoihealthzCheck,
		"/gnoi.healthz.Healthz/List":                                     gnoihealthzList,
		"/gnoi.healthz.Healthz/Get":                                      gnoihealthzGet,
		"/gnoi.layer2.Layer2/*":                                          gnoilayer2ALL,
		"/gnoi.layer2.Layer2/ClearLLDPInterface":                         gnoilayer2ClearLLDPInterface,
		"/gnoi.layer2.Layer2/ClearSpanningTree":                          gnoilayer2ClearSpanningTree,
		"/gnoi.layer2.Layer2/PerformBERT":                                gnoilayer2PerformBERT,
		"/gnoi.layer2.Layer2/SendWakeOnLAN":                              gnoilayer2SendWakeOnLAN,
		"/gnoi.layer2.Layer2/ClearNeighborDiscovery":                     gnoilayer2ClearNeighborDiscovery,
		"/gnoi.packet_link_qualification.LinkQualification/Create":       gnoipacket_link_qualificationLinkQualificationCreate,
		"/gnoi.mpls.MPLS/*":                                              gnoimplsALL,
		"/gnoi.mpls.MPLS/ClearLSPCounters":                               gnoimplsClearLSPCounters,
		"/gnoi.mpls.MPLS/MPLSPing":                                       gnoimplsMPLSPing,
		"/gnoi.mpls.MPLS/ClearLSP":                                       gnoimplsClearLSP,
		"/gnoi.optical.OTDR/*":                                           gnoiopticalOTDRALL,
		"/gnoi.optical.WavelengthRouter/AdjustSpectrum":                  gnoiopticalWavelengthRouterAdjustSpectrum,
		"/gnoi.optical.WavelengthRouter/*":                               gnoiopticalWavelengthRouterALL,
		"/gnoi.optical.WavelengthRouter/CancelAdjustPSD":                 gnoiopticalWavelengthRouterCancelAdjustPSD,
		"/gnoi.optical.WavelengthRouter/CancelAdjustSpectrum":            gnoiopticalWavelengthRouterCancelAdjustSpectrum,
		"/gnoi.os.OS/Activate":                                           gnoiosActivate,
		"/gnoi.os.OS/*":                                                  gnoiosALL,
		"/gnoi.os.OS/Verify":                                             gnoiosVerify,
		"/gnoi.os.OS/Install":                                            gnoiosInstall,
		"/gnoi.optical.OTDR/Initiate":                                    gnoiopticalOTDRInitiate,
		"/gnoi.packet_link_qualification.LinkQualification/*":            gnoipacket_link_qualificationLinkQualificationALL,
		"/gnoi.packet_link_qualification.LinkQualification/Capabilities": gnoipacket_link_qualificationLinkQualificationCapabilities,
		"/gnoi.packet_link_qualification.LinkQualification/Delete":       gnoipacket_link_qualificationLinkQualificationDelete,
		"/gnoi.packet_link_qualification.LinkQualification/Get":          gnoipacket_link_qualificationLinkQualificationGet,
		"/gnoi.packet_link_qualification.LinkQualification/List":         gnoipacket_link_qualificationLinkQualificationList,
		"/gnoi.system.System/*":                                          gnoisystemALL,
		"/gnoi.system.System/CancelReboot":                               gnoisystemCancelReboot,
		"/gnoi.system.System/KillProcess":                                gnoisystemKillProcess,
		"/gnoi.system.System/Reboot":                                     gnoisystemReboot,
		"/gnoi.system.System/RebootStatus":                               gnoisystemRebootStatus,
		"/gnoi.system.System/SetPackage":                                 gnoisystemSetPackage,
		"/gnoi.system.System/SwitchControlProcessor":                     gnoisystemSwitchControlProcessor,
		"/gnoi.system.System/Time":                                       gnoisystemTime,
		"/gnoi.system.System/Traceroute":                                 gnoisystemTraceroute,
		"/gnoi.system.System/Ping":                                       gnoisystemPing,
		"/gnoi.optical.WavelengthRouter/AdjustPSD":                       gnoiopticalWavelengthRouterAdjustPSD,
		"/gnsi.accounting.v1.AccountingPull/*":                           gnsiaccountingv1AccountingPullALL,
		"/gnsi.accounting.v1.AccountingPush/RecordStream":                gnsiaccountingv1AccountingPushRecordStream,
		"/gnsi.accounting.v1.AccountingPush/*":                           gnsiaccountingv1AccountingPushALL,
		"/gnsi.accounting.v1.AccountingPull/RecordStream":                gnsiaccountingv1AccountingPullRecordStream,
		"/gnsi.authz.v1.Authz/*":                                         gnsiauthzv1AuthzALL,
		"/gnsi.authz.v1.Authz/Get":                                       gnsiauthzv1AuthzGet,
		"/gnsi.authz.v1.Authz/Probe":                                     gnsiauthzv1AuthzProbe,
		"/gnsi.authz.v1.Authz/Rotate":                                    gnsiauthzv1AuthzRotate,
		"/gnsi.certz.v1.Certz/AddProfile":                                gnsicertzv1CertzAddProfile,
		"/gnsi.certz.v1.Certz/*":                                         gnsicertzv1CertzALL,
		"/gnsi.certz.v1.Certz/CanGenerateCSR":                            gnsicertzv1CertzCanGenerateCSR,
		"/gnsi.certz.v1.Certz/DeleteProfile":                             gnsicertzv1CertzDeleteProfile,
		"/gnsi.certz.v1.Certz/GetProfileList":                            gnsicertzv1CertzGetProfileList,
		"/gnsi.certz.v1.Certz/Rotate":                                    gnsicertzv1CertzRotate,
		"/gnsi.credentialz.v1.Credentialz/*":                             gnsicredentialzv1CredentialzALL,
		"/gnsi.credentialz.v1.Credentialz/CanGenerateKey":                gnsicredentialzv1CredentialzCanGenerateKey,
		"/gnsi.credentialz.v1.Credentialz/GetPublicKeys":                 gnsicredentialzv1CredentialzGetPublicKeys,
		"/gnsi.credentialz.v1.Credentialz/RotateHostCredentials":         gnsicredentialzv1CredentialzRotateHostCredentials,
		"/gnsi.credentialz.v1.Credentialz/RotateAccountCredentials":      gnsicredentialzv1CredentialzRotateAccountCredentials,
		"/gnsi.pathz.v1.Pathz/*":                                         gnsipathzv1PathzALL,
		"/gnsi.pathz.v1.Pathz/Get":                                       gnsipathzv1PathzGet,
		"/gnsi.pathz.v1.Pathz/Probe":                                     gnsipathzv1PathzProbe,
		"/gnsi.pathz.v1.Pathz/Rotate":                                    gnsipathzv1PathzRotate,
		"/gribi.gRIBI/*":                                                 gribiALL,
		"/gribi.gRIBI/Flush":                                             gribiFlush,
		"/gribi.gRIBI/Get":                                               gribiGet,
		"/gribi.gRIBI/Modify":                                            gribiModify,
		"/p4.v1.P4Runtime/*":                                             p4v1P4RuntimeALL,
		"/p4.v1.P4Runtime/Capabilities":                                  p4v1P4RuntimeCapabilities,
		"/p4.v1.P4Runtime/GetForwardingPipelineConfig":                   p4v1P4RuntimeGetForwardingPipelineConfig,
		"/p4.v1.P4Runtime/Read":                                          p4v1P4RuntimeRead,
		"/p4.v1.P4Runtime/SetForwardingPipelineConfig":                   p4v1P4RuntimeSetForwardingPipelineConfig,
		"/p4.v1.P4Runtime/StreamChannel":                                 p4v1P4RuntimeStreamChannel,
		"/p4.v1.P4Runtime/Write":                                         p4v1P4RuntimeWrite,
	}
)
