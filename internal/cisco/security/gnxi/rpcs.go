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
	GNSI_AUTHZ_ALL                                                        *RPC
	GNSI_AUTHZ_GET                                                        *RPC
	GNSI_AUTHZ_PROBE                                                      *RPC
	GNSI_AUTHZ_ROTATE                                                     *RPC
	GNSI_CERTZ_ADDPROFILE                                                 *RPC
	GNSI_CERTZ_ALL                                                        *RPC
	GNSI_CERTZ_CANGENERATECSR                                             *RPC
	GNSI_CERTZ_DELETEPROFILE                                              *RPC
	GNSI_CERTZ_GETPROFILELIST                                             *RPC
	GNSI_CERTZ_ROTATE                                                     *RPC
	GNSI_CREDENTIALZ_ALL                                                  *RPC
	GNSI_CREDENTIALZ_CANGENERATEKEY                                       *RPC
	GNSI_CREDENTIALZ_ROTATEHOSTCREDENTIALS                                *RPC
	GNSI_CREDENTIALZ_ROTATEACCOUNTCREDENTIALS                             *RPC
	GNSI_PATHZ_ALL                                                        *RPC
	GNSI_PATHZ_GET                                                        *RPC
	GNSI_PATHZ_PROBE                                                      *RPC
	GNSI_PATHZ_ROTATE                                                     *RPC
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
	ALL = &RPC{
		Name:     "*",
		Service:  "*",
		QFN:      "*",
		Path:     "*",
		ExecFunc: ExecAllrpc,
	}
	gnmiALL = &RPC{
		Name:     "*",
		Service:  "gnmi.gNMI",
		QFN:      "gnmi.gNMI.*",
		Path:     "/gnmi.gNMI/*",
		ExecFunc: ExecGnmiallrpc,
	}
	gnmiGet = &RPC{
		Name:     "Get",
		Service:  "gnmi.gNMI",
		QFN:      "gnmi.gNMI.Get",
		Path:     "/gnmi.gNMI/Get",
		ExecFunc: ExecGnmiget,
	}
	gnmiSet = &RPC{
		Name:     "Set",
		Service:  "gnmi.gNMI",
		QFN:      "gnmi.gNMI.Set",
		Path:     "/gnmi.gNMI/Set",
		ExecFunc: ExecGnmiset,
	}
	gnmiSubscribe = &RPC{
		Name:     "Subscribe",
		Service:  "gnmi.gNMI",
		QFN:      "gnmi.gNMI.Subscribe",
		Path:     "/gnmi.gNMI/Subscribe",
		ExecFunc: ExecGnmisubscribe,
	}
	gnmiCapabilities = &RPC{
		Name:     "Capabilities",
		Service:  "gnmi.gNMI",
		QFN:      "gnmi.gNMI.Capabilities",
		Path:     "/gnmi.gNMI/Capabilities",
		ExecFunc: ExecGnmicapabilities,
	}
	gnoibgpALL = &RPC{
		Name:     "*",
		Service:  "gnoi.bgp.BGP",
		QFN:      "gnoi.bgp.BGP.*",
		Path:     "/gnoi.bgp.BGP/*",
		ExecFunc: ExecGnoibgpallrpc,
	}
	gnoibgpClearBGPNeighbor = &RPC{
		Name:     "ClearBGPNeighbor",
		Service:  "gnoi.bgp.BGP",
		QFN:      "gnoi.bgp.BGP.ClearBGPNeighbor",
		Path:     "/gnoi.bgp.BGP/ClearBGPNeighbor",
		ExecFunc: ExecGnoibgpclearbgpneighbor,
	}
	gnoicertificateCertificateManagementALL = &RPC{
		Name:     "*",
		Service:  "gnoi.certificate.CertificateManagement",
		QFN:      "gnoi.certificate.CertificateManagement.*",
		Path:     "/gnoi.certificate.CertificateManagement/*",
		ExecFunc: ExecGnoicertificatecertificatemanagementallrpc,
	}
	gnoicertificateCertificateManagementCanGenerateCSR = &RPC{
		Name:     "CanGenerateCSR",
		Service:  "gnoi.certificate.CertificateManagement",
		QFN:      "gnoi.certificate.CertificateManagement.CanGenerateCSR",
		Path:     "/gnoi.certificate.CertificateManagement/CanGenerateCSR",
		ExecFunc: ExecGnoicertificatecertificatemanagementcangeneratecsr,
	}
	gnoicertificateCertificateManagementGenerateCSR = &RPC{
		Name:     "GenerateCSR",
		Service:  "gnoi.certificate.CertificateManagement",
		QFN:      "gnoi.certificate.CertificateManagement.GenerateCSR",
		Path:     "/gnoi.certificate.CertificateManagement/GenerateCSR",
		ExecFunc: ExecGnoicertificatecertificatemanagementgeneratecsr,
	}
	gnoicertificateCertificateManagementGetCertificates = &RPC{
		Name:     "GetCertificates",
		Service:  "gnoi.certificate.CertificateManagement",
		QFN:      "gnoi.certificate.CertificateManagement.GetCertificates",
		Path:     "/gnoi.certificate.CertificateManagement/GetCertificates",
		ExecFunc: ExecGnoicertificatecertificatemanagementgetcertificates,
	}
	gnoicertificateCertificateManagementInstall = &RPC{
		Name:     "Install",
		Service:  "gnoi.certificate.CertificateManagement",
		QFN:      "gnoi.certificate.CertificateManagement.Install",
		Path:     "/gnoi.certificate.CertificateManagement/Install",
		ExecFunc: ExecGnoicertificatecertificatemanagementinstall,
	}
	gnoicertificateCertificateManagementLoadCertificate = &RPC{
		Name:     "LoadCertificate",
		Service:  "gnoi.certificate.CertificateManagement",
		QFN:      "gnoi.certificate.CertificateManagement.LoadCertificate",
		Path:     "/gnoi.certificate.CertificateManagement/LoadCertificate",
		ExecFunc: ExecGnoicertificatecertificatemanagementloadcertificate,
	}
	gnoicertificateCertificateManagementLoadCertificateAuthorityBundle = &RPC{
		Name:     "LoadCertificateAuthorityBundle",
		Service:  "gnoi.certificate.CertificateManagement",
		QFN:      "gnoi.certificate.CertificateManagement.LoadCertificateAuthorityBundle",
		Path:     "/gnoi.certificate.CertificateManagement/LoadCertificateAuthorityBundle",
		ExecFunc: ExecGnoicertificatecertificatemanagementloadcertificateauthoritybundle,
	}
	gnoicertificateCertificateManagementRevokeCertificates = &RPC{
		Name:     "RevokeCertificates",
		Service:  "gnoi.certificate.CertificateManagement",
		QFN:      "gnoi.certificate.CertificateManagement.RevokeCertificates",
		Path:     "/gnoi.certificate.CertificateManagement/RevokeCertificates",
		ExecFunc: ExecGnoicertificatecertificatemanagementrevokecertificates,
	}
	gnoicertificateCertificateManagementRotate = &RPC{
		Name:     "Rotate",
		Service:  "gnoi.certificate.CertificateManagement",
		QFN:      "gnoi.certificate.CertificateManagement.Rotate",
		Path:     "/gnoi.certificate.CertificateManagement/Rotate",
		ExecFunc: ExecGnoicertificatecertificatemanagementrotate,
	}
	gnoidiagALL = &RPC{
		Name:     "*",
		Service:  "gnoi.diag.Diag",
		QFN:      "gnoi.diag.Diag.*",
		Path:     "/gnoi.diag.Diag/*",
		ExecFunc: ExecGnoidiagallrpc,
	}
	gnoidiagGetBERTResult = &RPC{
		Name:     "GetBERTResult",
		Service:  "gnoi.diag.Diag",
		QFN:      "gnoi.diag.Diag.GetBERTResult",
		Path:     "/gnoi.diag.Diag/GetBERTResult",
		ExecFunc: ExecGnoidiaggetbertresult,
	}
	gnoidiagStopBERT = &RPC{
		Name:     "StopBERT",
		Service:  "gnoi.diag.Diag",
		QFN:      "gnoi.diag.Diag.StopBERT",
		Path:     "/gnoi.diag.Diag/StopBERT",
		ExecFunc: ExecGnoidiagstopbert,
	}
	gnoidiagStartBERT = &RPC{
		Name:     "StartBERT",
		Service:  "gnoi.diag.Diag",
		QFN:      "gnoi.diag.Diag.StartBERT",
		Path:     "/gnoi.diag.Diag/StartBERT",
		ExecFunc: ExecGnoidiagstartbert,
	}
	gnoifactory_resetFactoryResetALL = &RPC{
		Name:     "*",
		Service:  "gnoi.factory_reset.FactoryReset",
		QFN:      "gnoi.factory_reset.FactoryReset.*",
		Path:     "/gnoi.factory_reset.FactoryReset/*",
		ExecFunc: ExecGnoifactory_resetfactoryresetallrpc,
	}
	gnoifactory_resetFactoryResetStart = &RPC{
		Name:     "Start",
		Service:  "gnoi.factory_reset.FactoryReset",
		QFN:      "gnoi.factory_reset.FactoryReset.Start",
		Path:     "/gnoi.factory_reset.FactoryReset/Start",
		ExecFunc: ExecGnoifactory_resetfactoryresetstart,
	}
	gnoifileALL = &RPC{
		Name:     "*",
		Service:  "gnoi.file.File",
		QFN:      "gnoi.file.File.*",
		Path:     "/gnoi.file.File/*",
		ExecFunc: ExecGnoifileallrpc,
	}
	gnoifilePut = &RPC{
		Name:     "Put",
		Service:  "gnoi.file.File",
		QFN:      "gnoi.file.File.Put",
		Path:     "/gnoi.file.File/Put",
		ExecFunc: ExecGnoifileput,
	}
	gnoifileRemove = &RPC{
		Name:     "Remove",
		Service:  "gnoi.file.File",
		QFN:      "gnoi.file.File.Remove",
		Path:     "/gnoi.file.File/Remove",
		ExecFunc: ExecGnoifileremove,
	}
	gnoifileStat = &RPC{
		Name:     "Stat",
		Service:  "gnoi.file.File",
		QFN:      "gnoi.file.File.Stat",
		Path:     "/gnoi.file.File/Stat",
		ExecFunc: ExecGnoifilestat,
	}
	gnoifileTransferToRemote = &RPC{
		Name:     "TransferToRemote",
		Service:  "gnoi.file.File",
		QFN:      "gnoi.file.File.TransferToRemote",
		Path:     "/gnoi.file.File/TransferToRemote",
		ExecFunc: ExecGnoifiletransfertoremote,
	}
	gnoifileGet = &RPC{
		Name:     "Get",
		Service:  "gnoi.file.File",
		QFN:      "gnoi.file.File.Get",
		Path:     "/gnoi.file.File/Get",
		ExecFunc: ExecGnoifileget,
	}
	gnoihealthzAcknowledge = &RPC{
		Name:     "Acknowledge",
		Service:  "gnoi.healthz.Healthz",
		QFN:      "gnoi.healthz.Healthz.Acknowledge",
		Path:     "/gnoi.healthz.Healthz/Acknowledge",
		ExecFunc: ExecGnoihealthzacknowledge,
	}
	gnoihealthzALL = &RPC{
		Name:     "*",
		Service:  "gnoi.healthz.Healthz",
		QFN:      "gnoi.healthz.Healthz.*",
		Path:     "/gnoi.healthz.Healthz/*",
		ExecFunc: ExecGnoihealthzallrpc,
	}
	gnoihealthzArtifact = &RPC{
		Name:     "Artifact",
		Service:  "gnoi.healthz.Healthz",
		QFN:      "gnoi.healthz.Healthz.Artifact",
		Path:     "/gnoi.healthz.Healthz/Artifact",
		ExecFunc: ExecGnoihealthzartifact,
	}
	gnoihealthzCheck = &RPC{
		Name:     "Check",
		Service:  "gnoi.healthz.Healthz",
		QFN:      "gnoi.healthz.Healthz.Check",
		Path:     "/gnoi.healthz.Healthz/Check",
		ExecFunc: ExecGnoihealthzcheck,
	}
	gnoihealthzList = &RPC{
		Name:     "List",
		Service:  "gnoi.healthz.Healthz",
		QFN:      "gnoi.healthz.Healthz.List",
		Path:     "/gnoi.healthz.Healthz/List",
		ExecFunc: ExecGnoihealthzlist,
	}
	gnoihealthzGet = &RPC{
		Name:     "Get",
		Service:  "gnoi.healthz.Healthz",
		QFN:      "gnoi.healthz.Healthz.Get",
		Path:     "/gnoi.healthz.Healthz/Get",
		ExecFunc: ExecGnoihealthzget,
	}
	gnoilayer2ALL = &RPC{
		Name:     "*",
		Service:  "gnoi.layer2.Layer2",
		QFN:      "gnoi.layer2.Layer2.*",
		Path:     "/gnoi.layer2.Layer2/*",
		ExecFunc: ExecGnoilayer2allrpc,
	}
	gnoilayer2ClearLLDPInterface = &RPC{
		Name:     "ClearLLDPInterface",
		Service:  "gnoi.layer2.Layer2",
		QFN:      "gnoi.layer2.Layer2.ClearLLDPInterface",
		Path:     "/gnoi.layer2.Layer2/ClearLLDPInterface",
		ExecFunc: ExecGnoilayer2clearlldpinterface,
	}
	gnoilayer2ClearSpanningTree = &RPC{
		Name:     "ClearSpanningTree",
		Service:  "gnoi.layer2.Layer2",
		QFN:      "gnoi.layer2.Layer2.ClearSpanningTree",
		Path:     "/gnoi.layer2.Layer2/ClearSpanningTree",
		ExecFunc: ExecGnoilayer2clearspanningtree,
	}
	gnoilayer2PerformBERT = &RPC{
		Name:     "PerformBERT",
		Service:  "gnoi.layer2.Layer2",
		QFN:      "gnoi.layer2.Layer2.PerformBERT",
		Path:     "/gnoi.layer2.Layer2/PerformBERT",
		ExecFunc: ExecGnoilayer2performbert,
	}
	gnoilayer2SendWakeOnLAN = &RPC{
		Name:     "SendWakeOnLAN",
		Service:  "gnoi.layer2.Layer2",
		QFN:      "gnoi.layer2.Layer2.SendWakeOnLAN",
		Path:     "/gnoi.layer2.Layer2/SendWakeOnLAN",
		ExecFunc: ExecGnoilayer2sendwakeonlan,
	}
	gnoilayer2ClearNeighborDiscovery = &RPC{
		Name:     "ClearNeighborDiscovery",
		Service:  "gnoi.layer2.Layer2",
		QFN:      "gnoi.layer2.Layer2.ClearNeighborDiscovery",
		Path:     "/gnoi.layer2.Layer2/ClearNeighborDiscovery",
		ExecFunc: ExecGnoilayer2clearneighbordiscovery,
	}
	gnoipacket_link_qualificationLinkQualificationCreate = &RPC{
		Name:     "Create",
		Service:  "gnoi.packet_link_qualification.LinkQualification",
		QFN:      "gnoi.packet_link_qualification.LinkQualification.Create",
		Path:     "/gnoi.packet_link_qualification.LinkQualification/Create",
		ExecFunc: ExecGnoipacket_link_qualificationlinkqualificationcreate,
	}
	gnoimplsALL = &RPC{
		Name:     "*",
		Service:  "gnoi.mpls.MPLS",
		QFN:      "gnoi.mpls.MPLS.*",
		Path:     "/gnoi.mpls.MPLS/*",
		ExecFunc: ExecGnoimplsallrpc,
	}
	gnoimplsClearLSPCounters = &RPC{
		Name:     "ClearLSPCounters",
		Service:  "gnoi.mpls.MPLS",
		QFN:      "gnoi.mpls.MPLS.ClearLSPCounters",
		Path:     "/gnoi.mpls.MPLS/ClearLSPCounters",
		ExecFunc: ExecGnoimplsclearlspcounters,
	}
	gnoimplsMPLSPing = &RPC{
		Name:     "MPLSPing",
		Service:  "gnoi.mpls.MPLS",
		QFN:      "gnoi.mpls.MPLS.MPLSPing",
		Path:     "/gnoi.mpls.MPLS/MPLSPing",
		ExecFunc: ExecGnoimplsmplsping,
	}
	gnoimplsClearLSP = &RPC{
		Name:     "ClearLSP",
		Service:  "gnoi.mpls.MPLS",
		QFN:      "gnoi.mpls.MPLS.ClearLSP",
		Path:     "/gnoi.mpls.MPLS/ClearLSP",
		ExecFunc: ExecGnoimplsclearlsp,
	}
	gnoiopticalOTDRALL = &RPC{
		Name:     "*",
		Service:  "gnoi.optical.OTDR",
		QFN:      "gnoi.optical.OTDR.*",
		Path:     "/gnoi.optical.OTDR/*",
		ExecFunc: ExecGnoiopticalotdrallrpc,
	}
	gnoiopticalWavelengthRouterAdjustSpectrum = &RPC{
		Name:     "AdjustSpectrum",
		Service:  "gnoi.optical.WavelengthRouter",
		QFN:      "gnoi.optical.WavelengthRouter.AdjustSpectrum",
		Path:     "/gnoi.optical.WavelengthRouter/AdjustSpectrum",
		ExecFunc: ExecGnoiopticalwavelengthrouteradjustspectrum,
	}
	gnoiopticalWavelengthRouterALL = &RPC{
		Name:     "*",
		Service:  "gnoi.optical.WavelengthRouter",
		QFN:      "gnoi.optical.WavelengthRouter.*",
		Path:     "/gnoi.optical.WavelengthRouter/*",
		ExecFunc: ExecGnoiopticalwavelengthrouterallrpc,
	}
	gnoiopticalWavelengthRouterCancelAdjustPSD = &RPC{
		Name:     "CancelAdjustPSD",
		Service:  "gnoi.optical.WavelengthRouter",
		QFN:      "gnoi.optical.WavelengthRouter.CancelAdjustPSD",
		Path:     "/gnoi.optical.WavelengthRouter/CancelAdjustPSD",
		ExecFunc: ExecGnoiopticalwavelengthroutercanceladjustpsd,
	}
	gnoiopticalWavelengthRouterCancelAdjustSpectrum = &RPC{
		Name:     "CancelAdjustSpectrum",
		Service:  "gnoi.optical.WavelengthRouter",
		QFN:      "gnoi.optical.WavelengthRouter.CancelAdjustSpectrum",
		Path:     "/gnoi.optical.WavelengthRouter/CancelAdjustSpectrum",
		ExecFunc: ExecGnoiopticalwavelengthroutercanceladjustspectrum,
	}
	gnoiosActivate = &RPC{
		Name:     "Activate",
		Service:  "gnoi.os.OS",
		QFN:      "gnoi.os.OS.Activate",
		Path:     "/gnoi.os.OS/Activate",
		ExecFunc: ExecGnoiosactivate,
	}
	gnoiosALL = &RPC{
		Name:     "*",
		Service:  "gnoi.os.OS",
		QFN:      "gnoi.os.OS.*",
		Path:     "/gnoi.os.OS/*",
		ExecFunc: ExecGnoiosallrpc,
	}
	gnoiosVerify = &RPC{
		Name:     "Verify",
		Service:  "gnoi.os.OS",
		QFN:      "gnoi.os.OS.Verify",
		Path:     "/gnoi.os.OS/Verify",
		ExecFunc: ExecGnoiosverify,
	}
	gnoiosInstall = &RPC{
		Name:     "Install",
		Service:  "gnoi.os.OS",
		QFN:      "gnoi.os.OS.Install",
		Path:     "/gnoi.os.OS/Install",
		ExecFunc: ExecGnoiosinstall,
	}
	gnoiopticalOTDRInitiate = &RPC{
		Name:     "Initiate",
		Service:  "gnoi.optical.OTDR",
		QFN:      "gnoi.optical.OTDR.Initiate",
		Path:     "/gnoi.optical.OTDR/Initiate",
		ExecFunc: ExecGnoiopticalotdrinitiate,
	}
	gnoipacket_link_qualificationLinkQualificationALL = &RPC{
		Name:     "*",
		Service:  "gnoi.packet_link_qualification.LinkQualification",
		QFN:      "gnoi.packet_link_qualification.LinkQualification.*",
		Path:     "/gnoi.packet_link_qualification.LinkQualification/*",
		ExecFunc: ExecGnoipacket_link_qualificationlinkqualificationallrpc,
	}
	gnoipacket_link_qualificationLinkQualificationCapabilities = &RPC{
		Name:     "Capabilities",
		Service:  "gnoi.packet_link_qualification.LinkQualification",
		QFN:      "gnoi.packet_link_qualification.LinkQualification.Capabilities",
		Path:     "/gnoi.packet_link_qualification.LinkQualification/Capabilities",
		ExecFunc: ExecGnoipacket_link_qualificationlinkqualificationcapabilities,
	}
	gnoipacket_link_qualificationLinkQualificationDelete = &RPC{
		Name:     "Delete",
		Service:  "gnoi.packet_link_qualification.LinkQualification",
		QFN:      "gnoi.packet_link_qualification.LinkQualification.Delete",
		Path:     "/gnoi.packet_link_qualification.LinkQualification/Delete",
		ExecFunc: ExecGnoipacket_link_qualificationlinkqualificationdelete,
	}
	gnoipacket_link_qualificationLinkQualificationGet = &RPC{
		Name:     "Get",
		Service:  "gnoi.packet_link_qualification.LinkQualification",
		QFN:      "gnoi.packet_link_qualification.LinkQualification.Get",
		Path:     "/gnoi.packet_link_qualification.LinkQualification/Get",
		ExecFunc: ExecGnoipacket_link_qualificationlinkqualificationget,
	}
	gnoipacket_link_qualificationLinkQualificationList = &RPC{
		Name:     "List",
		Service:  "gnoi.packet_link_qualification.LinkQualification",
		QFN:      "gnoi.packet_link_qualification.LinkQualification.List",
		Path:     "/gnoi.packet_link_qualification.LinkQualification/List",
		ExecFunc: ExecGnoipacket_link_qualificationlinkqualificationlist,
	}
	gnoisystemALL = &RPC{
		Name:     "*",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.*",
		Path:     "/gnoi.system.System/*",
		ExecFunc: ExecGnoisystemallrpc,
	}
	gnoisystemCancelReboot = &RPC{
		Name:     "CancelReboot",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.CancelReboot",
		Path:     "/gnoi.system.System/CancelReboot",
		ExecFunc: ExecGnoisystemcancelreboot,
	}
	gnoisystemKillProcess = &RPC{
		Name:     "KillProcess",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.KillProcess",
		Path:     "/gnoi.system.System/KillProcess",
		ExecFunc: ExecGnoisystemkillprocess,
	}
	gnoisystemReboot = &RPC{
		Name:     "Reboot",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.Reboot",
		Path:     "/gnoi.system.System/Reboot",
		ExecFunc: ExecGnoisystemreboot,
	}
	gnoisystemRebootStatus = &RPC{
		Name:     "RebootStatus",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.RebootStatus",
		Path:     "/gnoi.system.System/RebootStatus",
		ExecFunc: ExecGnoisystemrebootstatus,
	}
	gnoisystemSetPackage = &RPC{
		Name:     "SetPackage",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.SetPackage",
		Path:     "/gnoi.system.System/SetPackage",
		ExecFunc: ExecGnoisystemsetpackage,
	}
	gnoisystemSwitchControlProcessor = &RPC{
		Name:     "SwitchControlProcessor",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.SwitchControlProcessor",
		Path:     "/gnoi.system.System/SwitchControlProcessor",
		ExecFunc: ExecGnoisystemswitchcontrolprocessor,
	}
	gnoisystemTime = &RPC{
		Name:     "Time",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.Time",
		Path:     "/gnoi.system.System/Time",
		ExecFunc: ExecGnoisystemtime,
	}
	gnoisystemTraceroute = &RPC{
		Name:     "Traceroute",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.Traceroute",
		Path:     "/gnoi.system.System/Traceroute",
		ExecFunc: ExecGnoisystemtraceroute,
	}
	gnoisystemPing = &RPC{
		Name:     "Ping",
		Service:  "gnoi.system.System",
		QFN:      "gnoi.system.System.Ping",
		Path:     "/gnoi.system.System/Ping",
		ExecFunc: ExecGnoisystemping,
	}
	gnoiopticalWavelengthRouterAdjustPSD = &RPC{
		Name:     "AdjustPSD",
		Service:  "gnoi.optical.WavelengthRouter",
		QFN:      "gnoi.optical.WavelengthRouter.AdjustPSD",
		Path:     "/gnoi.optical.WavelengthRouter/AdjustPSD",
		ExecFunc: ExecGnoiopticalwavelengthrouteradjustpsd,
	}
	gnsiauthzALL = &RPC{
		Name:     "*",
		Service:  "gnsi.authz.Authz",
		QFN:      "gnsi.authz.Authz.*",
		Path:     "/gnsi.authz.Authz/*",
		ExecFunc: ExecGnsiauthzallrpc,
	}
	gnsiauthzGet = &RPC{
		Name:     "Get",
		Service:  "gnsi.authz.Authz",
		QFN:      "gnsi.authz.Authz.Get",
		Path:     "/gnsi.authz.Authz/Get",
		ExecFunc: ExecGnsiauthzget,
	}
	gnsiauthzProbe = &RPC{
		Name:     "Probe",
		Service:  "gnsi.authz.Authz",
		QFN:      "gnsi.authz.Authz.Probe",
		Path:     "/gnsi.authz.Authz/Probe",
		ExecFunc: ExecGnsiauthzprobe,
	}
	gnsiauthzRotate = &RPC{
		Name:     "Rotate",
		Service:  "gnsi.authz.Authz",
		QFN:      "gnsi.authz.Authz.Rotate",
		Path:     "/gnsi.authz.Authz/Rotate",
		ExecFunc: ExecGnsiauthzrotate,
	}
	gnsicertzAddProfile = &RPC{
		Name:     "AddProfile",
		Service:  "gnsi.certz.Certz",
		QFN:      "gnsi.certz.Certz.AddProfile",
		Path:     "/gnsi.certz.Certz/AddProfile",
		ExecFunc: ExecGnsicertzaddprofile,
	}
	gnsicertzALL = &RPC{
		Name:     "*",
		Service:  "gnsi.certz.Certz",
		QFN:      "gnsi.certz.Certz.*",
		Path:     "/gnsi.certz.Certz/*",
		ExecFunc: ExecGnsicertzallrpc,
	}
	gnsicertzCanGenerateCSR = &RPC{
		Name:     "CanGenerateCSR",
		Service:  "gnsi.certz.Certz",
		QFN:      "gnsi.certz.Certz.CanGenerateCSR",
		Path:     "/gnsi.certz.Certz/CanGenerateCSR",
		ExecFunc: ExecGnsicertzcangeneratecsr,
	}
	gnsicertzDeleteProfile = &RPC{
		Name:     "DeleteProfile",
		Service:  "gnsi.certz.Certz",
		QFN:      "gnsi.certz.Certz.DeleteProfile",
		Path:     "/gnsi.certz.Certz/DeleteProfile",
		ExecFunc: ExecGnsicertzdeleteprofile,
	}
	gnsicertzGetProfileList = &RPC{
		Name:     "GetProfileList",
		Service:  "gnsi.certz.Certz",
		QFN:      "gnsi.certz.Certz.GetProfileList",
		Path:     "/gnsi.certz.Certz/GetProfileList",
		ExecFunc: ExecGnsicertzgetprofilelist,
	}
	gnsicertzRotate = &RPC{
		Name:     "Rotate",
		Service:  "gnsi.certz.Certz",
		QFN:      "gnsi.certz.Certz.Rotate",
		Path:     "/gnsi.certz.Certz/Rotate",
		ExecFunc: ExecGnsicertzrotate,
	}
	gnsicredentialzALL = &RPC{
		Name:     "*",
		Service:  "gnsi.credentialz.Credentialz",
		QFN:      "gnsi.credentialz.Credentialz.*",
		Path:     "/gnsi.credentialz.Credentialz/*",
		ExecFunc: ExecGnsicredentialzallrpc,
	}
	gnsicredentialzCanGenerateKey = &RPC{
		Name:     "CanGenerateKey",
		Service:  "gnsi.credentialz.Credentialz",
		QFN:      "gnsi.credentialz.Credentialz.CanGenerateKey",
		Path:     "/gnsi.credentialz.Credentialz/CanGenerateKey",
		ExecFunc: ExecGnsicredentialzcangeneratekey,
	}
	gnsicredentialzRotateHostCredentials = &RPC{
		Name:     "RotateHostCredentials",
		Service:  "gnsi.credentialz.Credentialz",
		QFN:      "gnsi.credentialz.Credentialz.RotateHostCredentials",
		Path:     "/gnsi.credentialz.Credentialz/RotateHostCredentials",
		ExecFunc: ExecGnsicredentialzrotatehostcredentials,
	}
	gnsicredentialzRotateAccountCredentials = &RPC{
		Name:     "RotateAccountCredentials",
		Service:  "gnsi.credentialz.Credentialz",
		QFN:      "gnsi.credentialz.Credentialz.RotateAccountCredentials",
		Path:     "/gnsi.credentialz.Credentialz/RotateAccountCredentials",
		ExecFunc: ExecGnsicredentialzrotateaccountcredentials,
	}
	gnsipathzALL = &RPC{
		Name:     "*",
		Service:  "gnsi.pathz.Pathz",
		QFN:      "gnsi.pathz.Pathz.*",
		Path:     "/gnsi.pathz.Pathz/*",
		ExecFunc: ExecGnsipathzallrpc,
	}
	gnsipathzGet = &RPC{
		Name:     "Get",
		Service:  "gnsi.pathz.Pathz",
		QFN:      "gnsi.pathz.Pathz.Get",
		Path:     "/gnsi.pathz.Pathz/Get",
		ExecFunc: ExecGnsipathzget,
	}
	gnsipathzProbe = &RPC{
		Name:     "Probe",
		Service:  "gnsi.pathz.Pathz",
		QFN:      "gnsi.pathz.Pathz.Probe",
		Path:     "/gnsi.pathz.Pathz/Probe",
		ExecFunc: ExecGnsipathzprobe,
	}
	gnsipathzRotate = &RPC{
		Name:     "Rotate",
		Service:  "gnsi.pathz.Pathz",
		QFN:      "gnsi.pathz.Pathz.Rotate",
		Path:     "/gnsi.pathz.Pathz/Rotate",
		ExecFunc: ExecGnsipathzrotate,
	}
	gribiALL = &RPC{
		Name:     "*",
		Service:  "gribi.gRIBI",
		QFN:      "gribi.gRIBI.*",
		Path:     "/gribi.gRIBI/*",
		ExecFunc: ExecGribiallrpc,
	}
	gribiFlush = &RPC{
		Name:     "Flush",
		Service:  "gribi.gRIBI",
		QFN:      "gribi.gRIBI.Flush",
		Path:     "/gribi.gRIBI/Flush",
		ExecFunc: ExecGribiflush,
	}
	gribiGet = &RPC{
		Name:     "Get",
		Service:  "gribi.gRIBI",
		QFN:      "gribi.gRIBI.Get",
		Path:     "/gribi.gRIBI/Get",
		ExecFunc: ExecGribiget,
	}
	gribiModify = &RPC{
		Name:     "Modify",
		Service:  "gribi.gRIBI",
		QFN:      "gribi.gRIBI.Modify",
		Path:     "/gribi.gRIBI/Modify",
		ExecFunc: ExecGribimodify,
	}
	p4v1P4RuntimeALL = &RPC{
		Name:     "*",
		Service:  "p4.v1.P4Runtime",
		QFN:      "p4.v1.P4Runtime.*",
		Path:     "/p4.v1.P4Runtime/*",
		ExecFunc: ExecP4v1p4runtimeallrpc,
	}
	p4v1P4RuntimeCapabilities = &RPC{
		Name:     "Capabilities",
		Service:  "p4.v1.P4Runtime",
		QFN:      "p4.v1.P4Runtime.Capabilities",
		Path:     "/p4.v1.P4Runtime/Capabilities",
		ExecFunc: ExecP4v1p4runtimecapabilities,
	}
	p4v1P4RuntimeGetForwardingPipelineConfig = &RPC{
		Name:     "GetForwardingPipelineConfig",
		Service:  "p4.v1.P4Runtime",
		QFN:      "p4.v1.P4Runtime.GetForwardingPipelineConfig",
		Path:     "/p4.v1.P4Runtime/GetForwardingPipelineConfig",
		ExecFunc: ExecP4v1p4runtimegetforwardingpipelineconfig,
	}
	p4v1P4RuntimeRead = &RPC{
		Name:     "Read",
		Service:  "p4.v1.P4Runtime",
		QFN:      "p4.v1.P4Runtime.Read",
		Path:     "/p4.v1.P4Runtime/Read",
		ExecFunc: ExecP4v1p4runtimeread,
	}
	p4v1P4RuntimeSetForwardingPipelineConfig = &RPC{
		Name:     "SetForwardingPipelineConfig",
		Service:  "p4.v1.P4Runtime",
		QFN:      "p4.v1.P4Runtime.SetForwardingPipelineConfig",
		Path:     "/p4.v1.P4Runtime/SetForwardingPipelineConfig",
		ExecFunc: ExecP4v1p4runtimesetforwardingpipelineconfig,
	}
	p4v1P4RuntimeStreamChannel = &RPC{
		Name:     "StreamChannel",
		Service:  "p4.v1.P4Runtime",
		QFN:      "p4.v1.P4Runtime.StreamChannel",
		Path:     "/p4.v1.P4Runtime/StreamChannel",
		ExecFunc: ExecP4v1p4runtimestreamchannel,
	}
	p4v1P4RuntimeWrite = &RPC{
		Name:     "Write",
		Service:  "p4.v1.P4Runtime",
		QFN:      "p4.v1.P4Runtime.Write",
		Path:     "/p4.v1.P4Runtime/Write",
		ExecFunc: ExecP4v1p4runtimewrite,
	}

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
		GNOI_SYSTEM_ALL:                             gnoisystemALL,
		GNOI_SYSTEM_CANCELREBOOT:                    gnoisystemCancelReboot,
		GNOI_SYSTEM_KILLPROCESS:                     gnoisystemKillProcess,
		GNOI_SYSTEM_REBOOT:                          gnoisystemReboot,
		GNOI_SYSTEM_REBOOTSTATUS:                    gnoisystemRebootStatus,
		GNOI_SYSTEM_SETPACKAGE:                      gnoisystemSetPackage,
		GNOI_SYSTEM_SWITCHCONTROLPROCESSOR:          gnoisystemSwitchControlProcessor,
		GNOI_SYSTEM_TIME:                            gnoisystemTime,
		GNOI_SYSTEM_TRACEROUTE:                      gnoisystemTraceroute,
		GNOI_SYSTEM_PING:                            gnoisystemPing,
		GNOI_OPTICAL_WAVELENGTHROUTER_ADJUSTPSD:     gnoiopticalWavelengthRouterAdjustPSD,
		GNSI_AUTHZ_ALL:                              gnsiauthzALL,
		GNSI_AUTHZ_GET:                              gnsiauthzGet,
		GNSI_AUTHZ_PROBE:                            gnsiauthzProbe,
		GNSI_AUTHZ_ROTATE:                           gnsiauthzRotate,
		GNSI_CERTZ_ADDPROFILE:                       gnsicertzAddProfile,
		GNSI_CERTZ_ALL:                              gnsicertzALL,
		GNSI_CERTZ_CANGENERATECSR:                   gnsicertzCanGenerateCSR,
		GNSI_CERTZ_DELETEPROFILE:                    gnsicertzDeleteProfile,
		GNSI_CERTZ_GETPROFILELIST:                   gnsicertzGetProfileList,
		GNSI_CERTZ_ROTATE:                           gnsicertzRotate,
		GNSI_CREDENTIALZ_ALL:                        gnsicredentialzALL,
		GNSI_CREDENTIALZ_CANGENERATEKEY:             gnsicredentialzCanGenerateKey,
		GNSI_CREDENTIALZ_ROTATEHOSTCREDENTIALS:      gnsicredentialzRotateHostCredentials,
		GNSI_CREDENTIALZ_ROTATEACCOUNTCREDENTIALS:   gnsicredentialzRotateAccountCredentials,
		GNSI_PATHZ_ALL:                              gnsipathzALL,
		GNSI_PATHZ_GET:                              gnsipathzGet,
		GNSI_PATHZ_PROBE:                            gnsipathzProbe,
		GNSI_PATHZ_ROTATE:                           gnsipathzRotate,
		GRIBI_ALL:                                   gribiALL,
		GRIBI_FLUSH:                                 gribiFlush,
		GRIBI_GET:                                   gribiGet,
		GRIBI_MODIFY:                                gribiModify,
		P4_V1_P4RUNTIME_ALL:                         p4v1P4RuntimeALL,
		P4_V1_P4RUNTIME_CAPABILITIES:                p4v1P4RuntimeCapabilities,
		P4_V1_P4RUNTIME_GETFORWARDINGPIPELINECONFIG: p4v1P4RuntimeGetForwardingPipelineConfig,
		P4_V1_P4RUNTIME_READ:                        p4v1P4RuntimeRead,
		P4_V1_P4RUNTIME_SETFORWARDINGPIPELINECONFIG: p4v1P4RuntimeSetForwardingPipelineConfig,
		P4_V1_P4RUNTIME_STREAMCHANNEL:               p4v1P4RuntimeStreamChannel,
		P4_V1_P4RUNTIME_WRITE:                       p4v1P4RuntimeWrite,
	}
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
		"/gnsi.authz.Authz/*":                                            gnsiauthzALL,
		"/gnsi.authz.Authz/Get":                                          gnsiauthzGet,
		"/gnsi.authz.Authz/Probe":                                        gnsiauthzProbe,
		"/gnsi.authz.Authz/Rotate":                                       gnsiauthzRotate,
		"/gnsi.certz.Certz/AddProfile":                                   gnsicertzAddProfile,
		"/gnsi.certz.Certz/*":                                            gnsicertzALL,
		"/gnsi.certz.Certz/CanGenerateCSR":                               gnsicertzCanGenerateCSR,
		"/gnsi.certz.Certz/DeleteProfile":                                gnsicertzDeleteProfile,
		"/gnsi.certz.Certz/GetProfileList":                               gnsicertzGetProfileList,
		"/gnsi.certz.Certz/Rotate":                                       gnsicertzRotate,
		"/gnsi.credentialz.Credentialz/*":                                gnsicredentialzALL,
		"/gnsi.credentialz.Credentialz/CanGenerateKey":                   gnsicredentialzCanGenerateKey,
		"/gnsi.credentialz.Credentialz/RotateHostCredentials":            gnsicredentialzRotateHostCredentials,
		"/gnsi.credentialz.Credentialz/RotateAccountCredentials":         gnsicredentialzRotateAccountCredentials,
		"/gnsi.pathz.Pathz/*":                                            gnsipathzALL,
		"/gnsi.pathz.Pathz/Get":                                          gnsipathzGet,
		"/gnsi.pathz.Pathz/Probe":                                        gnsipathzProbe,
		"/gnsi.pathz.Pathz/Rotate":                                       gnsipathzRotate,
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
