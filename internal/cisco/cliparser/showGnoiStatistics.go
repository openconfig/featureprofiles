package cliparser

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
)

// Define individual structs for each section
type MPLSStats struct {
	Ping             map[string]int
	ClearLSP         map[string]int
	ClearLSPCounters map[string]int
}

type Layer2Stats struct {
	ClearLLDPInterface map[string]int
	SetLoopbackMode    map[string]int
	GetLoopbackMode    map[string]int
}

type BGPStats struct {
	ClearBgpNeighbor map[string]int
}

type FactoryResetStats struct {
	FactoryReset map[string]int
}

type DiagStats struct {
	StartBERT      map[string]int
	StopBERT       map[string]int
	GetBERTResults map[string]int
}

type SystemStats struct {
	SetPackage             map[string]int
	Reboot                 map[string]int
	RebootStatus           map[string]int
	Traceroute             map[string]int
	Time                   map[string]int
	SwitchControlProcessor map[string]int
	Ping                   map[string]int
	KillProcess            map[string]int
	CancelReboot           map[string]int
}

type FileStats struct {
	Get              map[string]int
	Remove           map[string]int
	Put              map[string]int
	TransferToRemote map[string]int
	Stat             map[string]int
}

type CertificateStats struct {
	Install         map[string]int
	Rotate          map[string]int
	Get             map[string]int
	Revoke          map[string]int
	CanGenerateCSR  map[string]int
	InstallCABundle map[string]int
}

type OSPProtoStats struct {
	InstallRequests                           int
	ActivateRequests                          int
	VerifyRequests                            int
	VerifyResponses                           int
	ActivateOKAlreadyActivated                int
	ActivateOKActivated                       int
	InstallTransferRequestMessages            int
	InstallForcedTransferRequestMessages      int
	InstallSupervisorTransferRequestMessages  int
	InstallTransferContentMessages            int
	InstallTransferEndMessages                int
	InstallTransferReadyMessagesSent          int
	InstallImageTransfersToPartner            int
	InstallImageTransfersFromPartner          int
	InstallTransferValidateResponsesSent      int
	InstallTransferProgressMessagesSent       int
	InstallValidatedMessagesSentUponTransfer  int
	InstallValidatedMessagesUponImageTransfer int
	InstallTransferRequestSyncFailureErrors   int
	InstallTransferContentFileWriteErrors     int
	InstallTransferEndParseFailureErrors      int
	VerifyVersionVerificationAPIError         int
	VerifyVersionVerificationUnmarshallError  int
	ActivateErrorFailedVersionVerifications   int
	ActivateErrorNoImage                      int
	ActivateErrorActivationFailed             int
	ActivateErrorStandbyActivation            int
	ActivateErrorNoRebootSpecified            int
	UnexpectedSwitchoverMessagesSent          int
	Unauthenticated                           int
	InvalidArgument                           int
	Internal                                  int
	FailedPrecondition                        int
	PermissionDenied                          int
	ResourceExhausted                         int
	Aborted                                   int
}

type GnoiStats struct {
	System       SystemStats
	File         FileStats
	Certificate  CertificateStats
	OSProto      OSPProtoStats
	MPLS         MPLSStats
	Layer2       Layer2Stats
	BGP          BGPStats
	FactoryReset FactoryResetStats
	Diag         DiagStats
}

// Function to parse CLI output and return a GnoiStats instance
func ParseShowGnoiStats(t *testing.T, dut *ondatra.DUTDevice) GnoiStats {
	// Initialize the GnoiStats structure
	stats := GnoiStats{
		System: SystemStats{
			SetPackage:             make(map[string]int),
			Reboot:                 make(map[string]int),
			RebootStatus:           make(map[string]int),
			Traceroute:             make(map[string]int),
			Time:                   make(map[string]int),
			SwitchControlProcessor: make(map[string]int),
			Ping:                   make(map[string]int),
			KillProcess:            make(map[string]int),
			CancelReboot:           make(map[string]int),
		},
		File: FileStats{
			Get:              make(map[string]int),
			Remove:           make(map[string]int),
			Put:              make(map[string]int),
			TransferToRemote: make(map[string]int),
			Stat:             make(map[string]int),
		},
		Certificate: CertificateStats{
			Install:         make(map[string]int),
			Rotate:          make(map[string]int),
			Get:             make(map[string]int),
			Revoke:          make(map[string]int),
			CanGenerateCSR:  make(map[string]int),
			InstallCABundle: make(map[string]int),
		},
		OSProto: OSPProtoStats{},
		MPLS: MPLSStats{
			Ping:             make(map[string]int),
			ClearLSP:         make(map[string]int),
			ClearLSPCounters: make(map[string]int),
		},
		Layer2: Layer2Stats{
			ClearLLDPInterface: make(map[string]int),
			SetLoopbackMode:    make(map[string]int),
			GetLoopbackMode:    make(map[string]int),
		},
		BGP: BGPStats{
			ClearBgpNeighbor: make(map[string]int),
		},
		FactoryReset: FactoryResetStats{
			FactoryReset: make(map[string]int),
		},
		Diag: DiagStats{
			StartBERT:      make(map[string]int),
			StopBERT:       make(map[string]int),
			GetBERTResults: make(map[string]int),
		},
	}

	cmd := "show gnoi statistics"
	output := util.SshRunCommand(t, dut, cmd)

	// Define regular expressions to identify section headings and key-value pairs
	headingRegex := regexp.MustCompile(`^[A-Za-z ,\-]+$`)
	counterRegex := regexp.MustCompile(`^\s*([A-Za-z ,\-]+?)\s*:\s*(\d+)$`)

	// Track the current section and subsection being parsed
	var currentSection, currentSubsection string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line != "" {
			if headingRegex.MatchString(line) && !counterRegex.MatchString(line) {
				// If it's a heading, update the current section
				if strings.HasPrefix(line, "MPLS") || strings.HasPrefix(line, "Layer2") || strings.HasPrefix(line, "BGP") || strings.HasPrefix(line, "Factory Reset") || strings.HasPrefix(line, "Diag") {
					currentSubsection = line
				} else {
					currentSection = line
					currentSubsection = ""
				}
			} else if matches := counterRegex.FindStringSubmatch(line); matches != nil {
				// If it's a counter line, extract the key and value
				key := strings.TrimSpace(matches[1])
				value, err := strconv.Atoi(matches[2])
				if err == nil {
					switch currentSection {
					case "System":
						switch currentSubsection {
						case "SetPackage":
							stats.System.SetPackage[key] = value
						case "Reboot":
							stats.System.Reboot[key] = value
						case "RebootStatus":
							stats.System.RebootStatus[key] = value
						case "Traceroute":
							stats.System.Traceroute[key] = value
						case "Time":
							stats.System.Time[key] = value
						case "SwitchControlProcessor":
							stats.System.SwitchControlProcessor[key] = value
						case "Ping":
							stats.System.Ping[key] = value
						case "Kill Process":
							stats.System.KillProcess[key] = value
						case "Cancel Reboot":
							stats.System.CancelReboot[key] = value
						}
					case "File":
						switch currentSubsection {
						case "Get":
							stats.File.Get[key] = value
						case "Remove":
							stats.File.Remove[key] = value
						case "Put":
							stats.File.Put[key] = value
						case "TransferToRemote":
							stats.File.TransferToRemote[key] = value
						case "Stat":
							stats.File.Stat[key] = value
						}
					case "Certificate":
						switch currentSubsection {
						case "Install":
							stats.Certificate.Install[key] = value
						case "Rotate":
							stats.Certificate.Rotate[key] = value
						case "Get":
							stats.Certificate.Get[key] = value
						case "Revoke":
							stats.Certificate.Revoke[key] = value
						case "Can Generate CSR":
							stats.Certificate.CanGenerateCSR[key] = value
						case "Install CA Bundle":
							stats.Certificate.InstallCABundle[key] = value
						}
					case "OSProto":
						// Example of assigning to struct fields, assuming the keys match field names
						switch key {
						case "Install requests":
							stats.OSProto.InstallRequests = value
						case "Activate requests":
							stats.OSProto.ActivateRequests = value
						case "Verify requests":
							stats.OSProto.VerifyRequests = value
						case "Verify responses":
							stats.OSProto.VerifyResponses = value
						case "Activate OK - Already Activated":
							stats.OSProto.ActivateOKAlreadyActivated = value
						case "Activate OK - Activated":
							stats.OSProto.ActivateOKActivated = value
						case "Install - Transfer Request messages":
							stats.OSProto.InstallTransferRequestMessages = value
						case "Install - Forced Transfer Request messages":
							stats.OSProto.InstallForcedTransferRequestMessages = value
						case "Install - Supervisor Transfer Request messages":
							stats.OSProto.InstallSupervisorTransferRequestMessages = value
						case "Install - Transfer Content messages":
							stats.OSProto.InstallTransferContentMessages = value
						case "Install - Transfer End messages":
							stats.OSProto.InstallTransferEndMessages = value
						case "Install - Transfer Ready messages sent":
							stats.OSProto.InstallTransferReadyMessagesSent = value
						case "Install - Image transfers to partner":
							stats.OSProto.InstallImageTransfersToPartner = value
						case "Install - Image transfers from partner":
							stats.OSProto.InstallImageTransfersFromPartner = value
						case "Install - Transfer Validate responses sent as image exists":
							stats.OSProto.InstallTransferValidateResponsesSent = value
						case "Install - Transfer Progress messages sent":
							stats.OSProto.InstallTransferProgressMessagesSent = value
						case "Install - Validated messages sent upon Transfer End":
							stats.OSProto.InstallValidatedMessagesSentUponTransfer = value
						case "Install - Validated messages sent upon image transfer from partner":
							stats.OSProto.InstallValidatedMessagesUponImageTransfer = value
						case "Install - Transfer Request, sync failure errors":
							stats.OSProto.InstallTransferRequestSyncFailureErrors = value
						case "Install - Transfer Content, file write errors":
							stats.OSProto.InstallTransferContentFileWriteErrors = value
						case "Install - Transfer End, parse failure errors":
							stats.OSProto.InstallTransferEndParseFailureErrors = value
						case "Verify - Version verification API error":
							stats.OSProto.VerifyVersionVerificationAPIError = value
						case "Verify - Version verification unmarshall Error":
							stats.OSProto.VerifyVersionVerificationUnmarshallError = value
						case "Activate Error - Failed version verifications":
							stats.OSProto.ActivateErrorFailedVersionVerifications = value
						case "Activate Error - No Image":
							stats.OSProto.ActivateErrorNoImage = value
						case "Activate Error - Activation Failed":
							stats.OSProto.ActivateErrorActivationFailed = value
						case "Activate Error - Standby Activation":
							stats.OSProto.ActivateErrorStandbyActivation = value
						case "Activate Error - No reboot specified":
							stats.OSProto.ActivateErrorNoRebootSpecified = value
						case "Unexpected switchover messages sent":
							stats.OSProto.UnexpectedSwitchoverMessagesSent = value
						case "Unauthenticated":
							stats.OSProto.Unauthenticated = value
						case "Invalid argument":
							stats.OSProto.InvalidArgument = value
						case "Internal":
							stats.OSProto.Internal = value
						case "Failed precondition":
							stats.OSProto.FailedPrecondition = value
						case "Permission denied":
							stats.OSProto.PermissionDenied = value
						case "Resource exhausted":
							stats.OSProto.ResourceExhausted = value
						case "Aborted":
							stats.OSProto.Aborted = value
						}
					case "MPLS":
						switch currentSubsection {
						case "MPLS Ping":
							stats.MPLS.Ping[key] = value
						case "MPLS ClearLSP":
							stats.MPLS.ClearLSP[key] = value
						case "MPLS ClearLSPCounters":
							stats.MPLS.ClearLSPCounters[key] = value
						}
					case "Layer2":
						switch currentSubsection {
						case "Clear lldp Interface":
							stats.Layer2.ClearLLDPInterface[key] = value
						case "Set Loopback Mode":
							stats.Layer2.SetLoopbackMode[key] = value
						case "Get Loopback Mode":
							stats.Layer2.GetLoopbackMode[key] = value
						}
					case "BGP":
						if currentSubsection == "ClearBgpNeighbor" {
							stats.BGP.ClearBgpNeighbor[key] = value
						}
					case "FactoryReset":
						if currentSubsection == "Factory Reset" {
							stats.FactoryReset.FactoryReset[key] = value
						}
					case "Diag":
						switch currentSubsection {
						case "Start BERT":
							stats.Diag.StartBERT[key] = value
						case "Stop BERT":
							stats.Diag.StopBERT[key] = value
						case "Get BERT Results":
							stats.Diag.GetBERTResults[key] = value
						}
					}
				}
			}
		}
	}

	return stats
}
