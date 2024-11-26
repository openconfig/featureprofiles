// Package gnxi populates a list of all RPCs related for featuresprofile tests.
// The below code is generated using ../gen/generate.go. Please do not modify.
package gnxi

type rpcs struct {
	AllRPC                                   *RPC
	GnmiAllRPC                               *RPC
	GnmiGet                                  *RPC
	GnmiSet                                  *RPC
	GnmiSubscribe                            *RPC
	GnmiCapabilities                         *RPC
	GnoiBgpAllRPC                            *RPC
	GnoiBgpClearBGPNeighbor                  *RPC
	GnoiDiagAllRPC                           *RPC
	GnoiDiagGetBERTResult                    *RPC
	GnoiDiagStopBERT                         *RPC
	GnoiDiagStartBERT                        *RPC
	GnoiFactoryresetAllRPC                   *RPC
	GnoiFactoryresetStart                    *RPC
	GnoiFileAllRPC                           *RPC
	GnoiFilePut                              *RPC
	GnoiFileRemove                           *RPC
	GnoiFileStat                             *RPC
	GnoiFileTransferToRemote                 *RPC
	GnoiFileGet                              *RPC
	GnoiHealthzAcknowledge                   *RPC
	GnoiHealthzAllRPC                        *RPC
	GnoiHealthzArtifact                      *RPC
	GnoiHealthzCheck                         *RPC
	GnoiHealthzList                          *RPC
	GnoiHealthzGet                           *RPC
	GnoiLayer2AllRPC                         *RPC
	GnoiLayer2ClearLLDPInterface             *RPC
	GnoiLayer2ClearSpanningTree              *RPC
	GnoiLayer2PerformBERT                    *RPC
	GnoiLayer2SendWakeOnLAN                  *RPC
	GnoiLayer2ClearNeighborDiscovery         *RPC
	GnoiLinkqualificationCreate              *RPC
	GnoiMplsAllRPC                           *RPC
	GnoiMplsClearLSPCounters                 *RPC
	GnoiMplsMPLSPing                         *RPC
	GnoiMplsClearLSP                         *RPC
	GnoiOtdrAllRPC                           *RPC
	GnoiWavelengthrouterAdjustSpectrum       *RPC
	GnoiWavelengthrouterAllRPC               *RPC
	GnoiWavelengthrouterCancelAdjustPSD      *RPC
	GnoiWavelengthrouterCancelAdjustSpectrum *RPC
	GnoiOsActivate                           *RPC
	GnoiOsAllRPC                             *RPC
	GnoiOsVerify                             *RPC
	GnoiOsInstall                            *RPC
	GnoiOtdrInitiate                         *RPC
	GnoiLinkqualificationAllRPC              *RPC
	GnoiLinkqualificationCapabilities        *RPC
	GnoiLinkqualificationDelete              *RPC
	GnoiLinkqualificationGet                 *RPC
	GnoiLinkqualificationList                *RPC
	GnoiSystemAllRPC                         *RPC
	GnoiSystemCancelReboot                   *RPC
	GnoiSystemKillProcess                    *RPC
	GnoiSystemReboot                         *RPC
	GnoiSystemRebootStatus                   *RPC
	GnoiSystemSetPackage                     *RPC
	GnoiSystemSwitchControlProcessor         *RPC
	GnoiSystemTime                           *RPC
	GnoiSystemTraceroute                     *RPC
	GnoiSystemPing                           *RPC
	GnoiWavelengthrouterAdjustPSD            *RPC
	GnsiAcctzAllRPC                          *RPC
	GnsiAcctzRecordSubscribe                 *RPC
	GnsiAuthzAllRPC                          *RPC
	GnsiAuthzGet                             *RPC
	GnsiAuthzProbe                           *RPC
	GnsiAuthzRotate                          *RPC
	GnsiCertzAddProfile                      *RPC
	GnsiCertzAllRPC                          *RPC
	GnsiCertzCanGenerateCSR                  *RPC
	GnsiCertzDeleteProfile                   *RPC
	GnsiCertzGetProfileList                  *RPC
	GnsiCertzRotate                          *RPC
	GnsiCredentialzAllRPC                    *RPC
	GnsiCredentialzCanGenerateKey            *RPC
	GnsiCredentialzGetPublicKeys             *RPC
	GnsiCredentialzRotateHostParameters      *RPC
	GnsiCredentialzRotateAccountCredentials  *RPC
	GnsiPathzAllRPC                          *RPC
	GnsiPathzGet                             *RPC
	GnsiPathzProbe                           *RPC
	GnsiPathzRotate                          *RPC
	GribiAllRPC                              *RPC
	GribiFlush                               *RPC
	GribiGet                                 *RPC
	GribiModify                              *RPC
	P4P4runtimeAllRPC                        *RPC
	P4P4runtimeCapabilities                  *RPC
	P4P4runtimeGetForwardingPipelineConfig   *RPC
	P4P4runtimeRead                          *RPC
	P4P4runtimeSetForwardingPipelineConfig   *RPC
	P4P4runtimeStreamChannel                 *RPC
	P4P4runtimeWrite                         *RPC
}

var (
	// ALL defines all FP related RPCs
	ALL = &RPC{
		Name:    "*",
		Service: "*",
		FQN:     "*",
		Path:    "*",
		Exec:    AllRPC,
	}
	gnmiALL = &RPC{
		Name:    "*",
		Service: "gnmi.gNMI",
		FQN:     "gnmi.gNMI.*",
		Path:    "/gnmi.gNMI/*",
		Exec:    GnmiAllRPC,
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
		Exec:    GnoiBgpAllRPC,
	}
	gnoibgpClearBGPNeighbor = &RPC{
		Name:    "ClearBGPNeighbor",
		Service: "gnoi.bgp.BGP",
		FQN:     "gnoi.bgp.BGP.ClearBGPNeighbor",
		Path:    "/gnoi.bgp.BGP/ClearBGPNeighbor",
		Exec:    GnoiBgpClearBGPNeighbor,
	}
	gnoidiagALL = &RPC{
		Name:    "*",
		Service: "gnoi.diag.Diag",
		FQN:     "gnoi.diag.Diag.*",
		Path:    "/gnoi.diag.Diag/*",
		Exec:    GnoiDiagAllRPC,
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
	gnoifactory_resetFactoryResetALL = &RPC{ //revive:disable-line the name of the rpc includes _
		Name:    "*",
		Service: "gnoi.factory_reset.FactoryReset",
		FQN:     "gnoi.factory_reset.FactoryReset.*",
		Path:    "/gnoi.factory_reset.FactoryReset/*",
		Exec:    GnoiFactoryresetAllRPC,
	}
	gnoifactory_resetFactoryResetStart = &RPC{ //revive:disable-line the name of the rpc includes _
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
		Exec:    GnoiFileAllRPC,
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
		Exec:    GnoiHealthzAllRPC,
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
		Exec:    GnoiLayer2AllRPC,
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
	gnoipacket_link_qualificationLinkQualificationCreate = &RPC{ //revive:disable-line the name of the rpc includes _
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
		Exec:    GnoiMplsAllRPC,
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
		Exec:    GnoiOtdrAllRPC,
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
		Exec:    GnoiWavelengthrouterAllRPC,
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
		Exec:    GnoiOsAllRPC,
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
	gnoipacket_link_qualificationLinkQualificationALL = &RPC{ //revive:disable-line the name of the rpc includes _
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.*",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/*",
		Exec:    GnoiLinkqualificationAllRPC,
	}
	gnoipacket_link_qualificationLinkQualificationCapabilities = &RPC{ //revive:disable-line the name of the rpc includes _

		Name:    "Capabilities",
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.Capabilities",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/Capabilities",
		Exec:    GnoiLinkqualificationCapabilities,
	}
	gnoipacket_link_qualificationLinkQualificationDelete = &RPC{ //revive:disable-line the name of the rpc includes _

		Name:    "Delete",
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.Delete",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/Delete",
		Exec:    GnoiLinkqualificationDelete,
	}
	gnoipacket_link_qualificationLinkQualificationGet = &RPC{ //revive:disable-line the name of the rpc includes _
		Name:    "Get",
		Service: "gnoi.packet_link_qualification.LinkQualification",
		FQN:     "gnoi.packet_link_qualification.LinkQualification.Get",
		Path:    "/gnoi.packet_link_qualification.LinkQualification/Get",
		Exec:    GnoiLinkqualificationGet,
	}
	gnoipacket_link_qualificationLinkQualificationList = &RPC{ //revive:disable-line the name of the rpc includes _
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
		Exec:    GnoiSystemAllRPC,
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
	gnsiacctzv1AcctzALL = &RPC{
		Name:    "*",
		Service: "gnsi.acctz.v1.Acctz",
		FQN:     "gnsi.acctz.v1.Acctz.*",
		Path:    "/gnsi.acctz.v1.Acctz/*",
		Exec:    GnsiAcctzAllRPC,
	}
	gnsiacctzv1AcctzRecordSubscribe = &RPC{
		Name:    "RecordSubscribe",
		Service: "gnsi.acctz.v1.Acctz",
		FQN:     "gnsi.acctz.v1.Acctz.RecordSubscribe",
		Path:    "/gnsi.acctz.v1.Acctz/RecordSubscribe",
		Exec:    GnsiAcctzRecordSubscribe,
	}
	gnsiauthzv1AuthzALL = &RPC{
		Name:    "*",
		Service: "gnsi.authz.v1.Authz",
		FQN:     "gnsi.authz.v1.Authz.*",
		Path:    "/gnsi.authz.v1.Authz/*",
		Exec:    GnsiAuthzAllRPC,
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
		Exec:    GnsiCertzAllRPC,
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
		Exec:    GnsiCredentialzAllRPC,
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
	gnsicredentialzv1CredentialzRotateHostParameters = &RPC{
		Name:    "RotateHostParameters",
		Service: "gnsi.credentialz.v1.Credentialz",
		FQN:     "gnsi.credentialz.v1.Credentialz.RotateHostParameters",
		Path:    "/gnsi.credentialz.v1.Credentialz/RotateHostParameters",
		Exec:    GnsiCredentialzRotateHostParameters,
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
		Exec:    GnsiPathzAllRPC,
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
		Exec:    GribiAllRPC,
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
		Exec:    P4P4runtimeAllRPC,
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
		AllRPC:                                   ALL,
		GnmiAllRPC:                               gnmiALL,
		GnmiGet:                                  gnmiGet,
		GnmiSet:                                  gnmiSet,
		GnmiSubscribe:                            gnmiSubscribe,
		GnmiCapabilities:                         gnmiCapabilities,
		GnoiBgpAllRPC:                            gnoibgpALL,
		GnoiBgpClearBGPNeighbor:                  gnoibgpClearBGPNeighbor,
		GnoiDiagAllRPC:                           gnoidiagALL,
		GnoiDiagGetBERTResult:                    gnoidiagGetBERTResult,
		GnoiDiagStopBERT:                         gnoidiagStopBERT,
		GnoiDiagStartBERT:                        gnoidiagStartBERT,
		GnoiFactoryresetAllRPC:                   gnoifactory_resetFactoryResetALL,
		GnoiFactoryresetStart:                    gnoifactory_resetFactoryResetStart,
		GnoiFileAllRPC:                           gnoifileALL,
		GnoiFilePut:                              gnoifilePut,
		GnoiFileRemove:                           gnoifileRemove,
		GnoiFileStat:                             gnoifileStat,
		GnoiFileTransferToRemote:                 gnoifileTransferToRemote,
		GnoiFileGet:                              gnoifileGet,
		GnoiHealthzAcknowledge:                   gnoihealthzAcknowledge,
		GnoiHealthzAllRPC:                        gnoihealthzALL,
		GnoiHealthzArtifact:                      gnoihealthzArtifact,
		GnoiHealthzCheck:                         gnoihealthzCheck,
		GnoiHealthzList:                          gnoihealthzList,
		GnoiHealthzGet:                           gnoihealthzGet,
		GnoiLayer2AllRPC:                         gnoilayer2ALL,
		GnoiLayer2ClearLLDPInterface:             gnoilayer2ClearLLDPInterface,
		GnoiLayer2ClearSpanningTree:              gnoilayer2ClearSpanningTree,
		GnoiLayer2PerformBERT:                    gnoilayer2PerformBERT,
		GnoiLayer2SendWakeOnLAN:                  gnoilayer2SendWakeOnLAN,
		GnoiLayer2ClearNeighborDiscovery:         gnoilayer2ClearNeighborDiscovery,
		GnoiLinkqualificationCreate:              gnoipacket_link_qualificationLinkQualificationCreate,
		GnoiMplsAllRPC:                           gnoimplsALL,
		GnoiMplsClearLSPCounters:                 gnoimplsClearLSPCounters,
		GnoiMplsMPLSPing:                         gnoimplsMPLSPing,
		GnoiMplsClearLSP:                         gnoimplsClearLSP,
		GnoiOtdrAllRPC:                           gnoiopticalOTDRALL,
		GnoiWavelengthrouterAdjustSpectrum:       gnoiopticalWavelengthRouterAdjustSpectrum,
		GnoiWavelengthrouterAllRPC:               gnoiopticalWavelengthRouterALL,
		GnoiWavelengthrouterCancelAdjustPSD:      gnoiopticalWavelengthRouterCancelAdjustPSD,
		GnoiWavelengthrouterCancelAdjustSpectrum: gnoiopticalWavelengthRouterCancelAdjustSpectrum,
		GnoiOsActivate:                           gnoiosActivate,
		GnoiOsAllRPC:                             gnoiosALL,
		GnoiOsVerify:                             gnoiosVerify,
		GnoiOsInstall:                            gnoiosInstall,
		GnoiOtdrInitiate:                         gnoiopticalOTDRInitiate,
		GnoiLinkqualificationAllRPC:              gnoipacket_link_qualificationLinkQualificationALL,
		GnoiLinkqualificationCapabilities:        gnoipacket_link_qualificationLinkQualificationCapabilities,
		GnoiLinkqualificationDelete:              gnoipacket_link_qualificationLinkQualificationDelete,
		GnoiLinkqualificationGet:                 gnoipacket_link_qualificationLinkQualificationGet,
		GnoiLinkqualificationList:                gnoipacket_link_qualificationLinkQualificationList,
		GnoiSystemAllRPC:                         gnoisystemALL,
		GnoiSystemCancelReboot:                   gnoisystemCancelReboot,
		GnoiSystemKillProcess:                    gnoisystemKillProcess,
		GnoiSystemReboot:                         gnoisystemReboot,
		GnoiSystemRebootStatus:                   gnoisystemRebootStatus,
		GnoiSystemSetPackage:                     gnoisystemSetPackage,
		GnoiSystemSwitchControlProcessor:         gnoisystemSwitchControlProcessor,
		GnoiSystemTime:                           gnoisystemTime,
		GnoiSystemTraceroute:                     gnoisystemTraceroute,
		GnoiSystemPing:                           gnoisystemPing,
		GnoiWavelengthrouterAdjustPSD:            gnoiopticalWavelengthRouterAdjustPSD,
		GnsiAcctzAllRPC:                          gnsiacctzv1AcctzALL,
		GnsiAcctzRecordSubscribe:                 gnsiacctzv1AcctzRecordSubscribe,
		GnsiAuthzAllRPC:                          gnsiauthzv1AuthzALL,
		GnsiAuthzGet:                             gnsiauthzv1AuthzGet,
		GnsiAuthzProbe:                           gnsiauthzv1AuthzProbe,
		GnsiAuthzRotate:                          gnsiauthzv1AuthzRotate,
		GnsiCertzAddProfile:                      gnsicertzv1CertzAddProfile,
		GnsiCertzAllRPC:                          gnsicertzv1CertzALL,
		GnsiCertzCanGenerateCSR:                  gnsicertzv1CertzCanGenerateCSR,
		GnsiCertzDeleteProfile:                   gnsicertzv1CertzDeleteProfile,
		GnsiCertzGetProfileList:                  gnsicertzv1CertzGetProfileList,
		GnsiCertzRotate:                          gnsicertzv1CertzRotate,
		GnsiCredentialzAllRPC:                    gnsicredentialzv1CredentialzALL,
		GnsiCredentialzCanGenerateKey:            gnsicredentialzv1CredentialzCanGenerateKey,
		GnsiCredentialzGetPublicKeys:             gnsicredentialzv1CredentialzGetPublicKeys,
		GnsiCredentialzRotateHostParameters:      gnsicredentialzv1CredentialzRotateHostParameters,
		GnsiCredentialzRotateAccountCredentials:  gnsicredentialzv1CredentialzRotateAccountCredentials,
		GnsiPathzAllRPC:                          gnsipathzv1PathzALL,
		GnsiPathzGet:                             gnsipathzv1PathzGet,
		GnsiPathzProbe:                           gnsipathzv1PathzProbe,
		GnsiPathzRotate:                          gnsipathzv1PathzRotate,
		GribiAllRPC:                              gribiALL,
		GribiFlush:                               gribiFlush,
		GribiGet:                                 gribiGet,
		GribiModify:                              gribiModify,
		P4P4runtimeAllRPC:                        p4v1P4RuntimeALL,
		P4P4runtimeCapabilities:                  p4v1P4RuntimeCapabilities,
		P4P4runtimeGetForwardingPipelineConfig:   p4v1P4RuntimeGetForwardingPipelineConfig,
		P4P4runtimeRead:                          p4v1P4RuntimeRead,
		P4P4runtimeSetForwardingPipelineConfig:   p4v1P4RuntimeSetForwardingPipelineConfig,
		P4P4runtimeStreamChannel:                 p4v1P4RuntimeStreamChannel,
		P4P4runtimeWrite:                         p4v1P4RuntimeWrite,
	}

	// RPCMAP is a helper that  maps path to RPCs data that may be needed in tests.
	RPCMAP = map[string]*RPC{
		"*":                                                              ALL,
		"/gnmi.gNMI/*":                                                   gnmiALL,
		"/gnmi.gNMI/Get":                                                 gnmiGet,
		"/gnmi.gNMI/Set":                                                 gnmiSet,
		"/gnmi.gNMI/Subscribe":                                           gnmiSubscribe,
		"/gnmi.gNMI/Capabilities":                                        gnmiCapabilities,
		"/gnoi.bgp.BGP/*":                                                gnoibgpALL,
		"/gnoi.bgp.BGP/ClearBGPNeighbor":                                 gnoibgpClearBGPNeighbor,
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
		"/gnsi.acctz.v1.Acctz/*":                                         gnsiacctzv1AcctzALL,
		"/gnsi.acctz.v1.Acctz/RecordSubscribe":                           gnsiacctzv1AcctzRecordSubscribe,
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
		"/gnsi.credentialz.v1.Credentialz/RotateHostParameters":          gnsicredentialzv1CredentialzRotateHostParameters,
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
