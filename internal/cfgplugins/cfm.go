// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgplugins

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/functional-translators/registrar"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

type CFMMeasurementProfile struct {
	ProfileName                string
	BurstInterval              uint32
	IntervalsArchived          uint16
	MeasurementInterval        uint32
	MeasurementType            oc.E_PmProfile_MeasurementType
	PacketPerBurst             uint32
	PacketsPerMeaurementPeriod uint16
	RepetitionPeriod           uint32
}

var ccmIntervalMap = map[oc.E_MaintenanceAssociation_CcmInterval]string{
	oc.MaintenanceAssociation_CcmInterval_300MS: "300 milli",
	oc.MaintenanceAssociation_CcmInterval_1S:    "1 seconds",
	oc.MaintenanceAssociation_CcmInterval_10S:   "10 seconds",
}

// ccmIntervalDuration maps the OC CCM interval to its duration.
var ccmIntervalDuration = map[oc.E_MaintenanceAssociation_CcmInterval]time.Duration{
	oc.MaintenanceAssociation_CcmInterval_300MS: 300 * time.Millisecond,
	oc.MaintenanceAssociation_CcmInterval_1S:    time.Second,
	oc.MaintenanceAssociation_CcmInterval_10S:   10 * time.Second,
}

var measurementTypeMap = map[oc.E_PmProfile_MeasurementType]string{
	oc.PmProfile_MeasurementType_UNSET: "",
	oc.PmProfile_MeasurementType_LMM:   "loss",
	oc.PmProfile_MeasurementType_SLM:   "loss synthetic",
	oc.PmProfile_MeasurementType_DMM:   "delay",
	oc.PmProfile_MeasurementType_CCM:   "continuity-check",
}

const (
	// cfmSessionTimeout bounds the wait for a CFM session or alarm state.
	cfmSessionTimeout = 60 * time.Second
	// pmMeasurementTimeout bounds the wait for a valid performance measurement:
	// one 60 second measurement interval plus margin.
	pmMeasurementTimeout = 90 * time.Second
	// maxTwoWayDelayMicros is a sanity upper bound (1s) for the
	// frame-delay-two-way-* leaves, which are in microseconds.
	maxTwoWayDelayMicros = 1000000
	// maxFrameLossRatio is the upper bound (100%) of the frame loss ratio
	// leaves, which are in milli-percent.
	maxFrameLossRatio = 100000
	// defaultCFMScaleCLIBatchSize is the number of CFM scale instances pushed in
	// a single CLI request when the CLI fallback is used.
	defaultCFMScaleCLIBatchSize = 100
)

// ConfigureMeasurementProfile configures the cfm performance measurement
// parameters. With OC, the profile is added to batch and the returned *oc.Oam
// is non-nil; the caller is responsible for calling batch.Set.
func ConfigureMeasurementProfile(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, params CFMMeasurementProfile) *oc.Oam {
	cli := ""
	if deviations.CfmOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cli = fmt.Sprintf(`
				cfm
					measurement loss inband
   					measurement loss synthetic
   					continuity-check loc-state action disable interface routing

   					profile %s
      				continuity-check
      				continuity-check alarm defect rdi-ccm loc-state
					measurement delay single-ended
      				measurement loss single-ended
      				measurement loss synthetic single-ended
					`,
				params.ProfileName)
			if params.MeasurementInterval != 0 {
				cli += fmt.Sprintf("measurement %s tx-interval %v milliseconds \n", measurementTypeMap[params.MeasurementType], params.MeasurementInterval)
			}
			if params.PacketsPerMeaurementPeriod != 0 {
				cli += fmt.Sprintf("measurement %s interval %v minutes \n", measurementTypeMap[params.MeasurementType], params.PacketsPerMeaurementPeriod/60)
			}
			t.Logf("%s: measurement profile CLI:\n%s", dut.Name(), cli)
			helpers.GnmiCLIConfig(t, dut, cli)
		default:
			t.Errorf("Deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return nil
	}
	root := &oc.Root{}
	oam := root.GetOrCreateOam()
	profile := oam.GetOrCreateCfm().GetOrCreatePerformanceMeasurementProfile(params.ProfileName)
	profile.SetBurstInterval(params.BurstInterval)
	profile.SetIntervalsArchived(params.IntervalsArchived)
	profile.SetMeasurementInterval(params.MeasurementInterval)
	profile.SetMeasurementType(params.MeasurementType)
	profile.SetPacketPerBurst(params.PacketPerBurst)
	profile.SetPacketsPerMeaurementPeriod(params.PacketsPerMeaurementPeriod)
	profile.SetRepetitionPeriod(params.RepetitionPeriod)

	// Update only this profile so that the CFM domains are not replaced.
	gnmi.BatchUpdate(batch, gnmi.OC().Oam().Cfm().PerformanceMeasurementProfile(params.ProfileName).Config(), profile)
	return oam
}

// MaintenanceDomainConfig holds the configuration of a CFM maintenance domain.
//
// The OC maintenance-domain is keyed by DomainName and the
// maintenance-association by MdID, which matches the Arista domain and
// association names used by the CLI fallback and the functional translators.
type MaintenanceDomainConfig struct {
	DomainName   string
	Level        int
	MdID         string
	MdNameType   oc.E_MaintenanceDomain_MdNameType
	Status       oc.E_OamCfm_OperationalStateType
	IntfName     string
	ProfileName  string
	RemoveDomain bool
	Assocs       []AssociationConfig
}

type AssociationConfig struct {
	GroupName        string
	CcmInterval      oc.E_MaintenanceAssociation_CcmInterval
	LossThreshold    int
	MaID             string
	MaNameType       oc.E_MaintenanceAssociation_MaNameType
	LocalMEPID       int
	CcmEnabled       bool
	Direction        oc.E_MepEndpoint_Direction
	InterfaceName    string
	ProfileName      string
	TransmitOnDefect bool
	RemoteMEPID      int
}

// firstAssoc returns the association of cfg. The CFM helpers configure and
// validate a single association per maintenance domain, so an empty Assocs is a
// test configuration error.
func firstAssoc(t testing.TB, cfg *MaintenanceDomainConfig) AssociationConfig {
	t.Helper()
	if len(cfg.Assocs) == 0 {
		t.Fatalf("CFM maintenance domain %q: Assocs must not be empty", cfg.DomainName)
	}
	return cfg.Assocs[0]
}

// aristaCFMDomainCLI returns the Arista CLI that configures the CFM domain,
// association and local MEP described by cfg and its association assoc.
func aristaCFMDomainCLI(cfg *MaintenanceDomainConfig, assoc AssociationConfig) string {
	cli := "cfm\n"
	if assoc.CcmInterval != oc.MaintenanceAssociation_CcmInterval_UNSET {
		cli += fmt.Sprintf("   profile %s\n      continuity-check tx-interval %v\n", cfg.ProfileName, ccmIntervalMap[assoc.CcmInterval])
	}
	if cfg.DomainName != "" {
		cli += fmt.Sprintf("   domain %v", cfg.DomainName)
		if cfg.Level != -1 {
			cli += fmt.Sprintf(" level %v", cfg.Level)
		}
		cli += "\n"
	}
	if cfg.MdID != "" {
		cli += fmt.Sprintf("      association %v\n", cfg.MdID)
	}
	if assoc.Direction == oc.MepEndpoint_Direction_UP {
		cli += "         direction up\n"
	}
	if cfg.ProfileName != "" {
		cli += fmt.Sprintf("         profile %v\n", cfg.ProfileName)
	}
	if assoc.LocalMEPID != 0 {
		cli += fmt.Sprintf("         end-point %v\n", assoc.LocalMEPID)
	}
	if cfg.IntfName != "" {
		cli += fmt.Sprintf("            interface %v\n", cfg.IntfName)
	}
	if assoc.RemoteMEPID != 0 {
		cli += fmt.Sprintf("            remote end-point %v\n", assoc.RemoteMEPID)
	}
	return cli
}

// buildCFMDomainOC adds the OC CFM domain described by cfg to oam.
func buildCFMDomainOC(oam *oc.Oam, cfg *MaintenanceDomainConfig) *oc.Oam_Cfm_MaintenanceDomain {
	domain := oam.GetOrCreateCfm().GetOrCreateMaintenanceDomain(cfg.DomainName)
	domain.SetLevel(uint8(cfg.Level))
	domain.SetMdNameType(cfg.MdNameType)

	for _, assoc := range cfg.Assocs {
		assoc.InterfaceName = cfg.IntfName
		assoc.ProfileName = cfg.ProfileName
		configureCFMAssociation(domain, cfg.MdID, assoc)
	}
	return domain
}

// ConfigureCFMDomain configures the cfm domain on the DUT.
func ConfigureCFMDomain(t *testing.T, oam *oc.Oam, dut *ondatra.DUTDevice, cfg *MaintenanceDomainConfig) {
	t.Helper()
	assoc := firstAssoc(t, cfg)
	if deviations.CfmOCUnsupported(dut) {
		t.Log("Configuring CFM configs")
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if cfg.RemoveDomain {
				helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("cfm\n   no domain %v\n", cfg.DomainName))
			}
			helpers.GnmiCLIConfig(t, dut, aristaCFMDomainCLI(cfg, assoc))
		default:
			t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	domainPath := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName)
	if cfg.RemoveDomain {
		gnmi.Delete(t, dut, domainPath.Config())
	}
	if oam == nil {
		oam = &oc.Oam{}
	}
	gnmi.Update(t, dut, domainPath.Config(), buildCFMDomainOC(oam, cfg))
}

// RemoveCFMDomain removes the cfm domain from the DUT.
func RemoveCFMDomain(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	t.Helper()
	if deviations.CfmOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("cfm\n   no domain %v\n", cfg.DomainName))
		default:
			t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	gnmi.Delete(t, dut, gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).Config())
}

// ConfigureLossThreshold configures the CCM loss threshold.
func ConfigureLossThreshold(t *testing.T, dut *ondatra.DUTDevice, oam *oc.Oam, cfg MaintenanceDomainConfig, loss float64) {
	t.Helper()
	if deviations.CfmOCUnsupported(dut) {
		t.Log("Configuring CFM configs")
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf(`
cfm
   profile %s
      continuity-check timeout multiplier %v
`, cfg.ProfileName, loss))
		default:
			t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return
	}
	gnmi.Update(t, dut, gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).LossThreshold().Config(), uint16(loss))
}

// configureCFMAssociation configures cfm domain association
func configureCFMAssociation(domain *oc.Oam_Cfm_MaintenanceDomain, maID string, cfg AssociationConfig) {
	ma := domain.GetOrCreateMaintenanceAssociation(maID)
	ma.SetGroupName(cfg.GroupName)
	ma.SetCcmInterval(cfg.CcmInterval)
	ma.SetLossThreshold(uint16(cfg.LossThreshold))
	ma.SetMaNameType(cfg.MaNameType)

	mepEndpoint := ma.GetOrCreateMepEndpoint(uint16(cfg.LocalMEPID))
	mepEndpoint.SetCcmEnabled(cfg.CcmEnabled)
	mepEndpoint.SetDirection(cfg.Direction)
	mepEndpoint.SetInterface(cfg.InterfaceName)
	mepEndpoint.GetOrCreatePmProfile(cfg.ProfileName)
	mepEndpoint.GetOrCreateRdi().SetTransmitOnDefect(cfg.TransmitOnDefect)
	mepEndpoint.GetOrCreateRemoteMep(uint16(cfg.RemoteMEPID))
}

// CFMScaleParams describes a set of CFM sessions where each session runs on
// its own subinterface, attached to its own static MPLS pseudowire.
type CFMScaleParams struct {
	// IntfName is the parent (aggregate) interface of the subinterfaces.
	IntfName string
	// Subinterfaces are the subinterface indices. Each index is also used as
	// the client VLAN ID and as the pseudowire, patch and domain suffix.
	Subinterfaces []int
	// PseudowireName is the prefix of the pseudowire names.
	PseudowireName string
	// NexthopGroupName is the transport nexthop-group of the pseudowires.
	NexthopGroupName string
	// LocalLabelBase and RemoteLabelBase are added to the subinterface index to
	// derive the pseudowire labels.
	LocalLabelBase  int
	RemoteLabelBase int
	// DomainPrefix is the prefix of the CFM domain names.
	DomainPrefix string
	// CFM is the template of the CFM configuration of each session.
	CFM MaintenanceDomainConfig
	// CLIBatchSize is the number of sessions pushed per CLI request when the CLI
	// fallback is used. Defaults to 100.
	CLIBatchSize int
}

// DomainConfigs returns the CFM configuration of each scale session.
func (p CFMScaleParams) DomainConfigs() []MaintenanceDomainConfig {
	var cfgs []MaintenanceDomainConfig
	for _, idx := range p.Subinterfaces {
		cfg := p.CFM
		cfg.Assocs = append([]AssociationConfig(nil), p.CFM.Assocs...)
		cfg.DomainName = fmt.Sprintf("%s%d", p.DomainPrefix, idx)
		cfg.IntfName = fmt.Sprintf("%s.%d", p.IntfName, idx)
		cfg.RemoveDomain = false
		cfgs = append(cfgs, cfg)
	}
	return cfgs
}

func (p CFMScaleParams) pseudowire(idx int) MplsStaticPseudowire {
	return MplsStaticPseudowire{
		PseudowireName:   fmt.Sprintf("%s-%d", p.PseudowireName, idx),
		IntfName:         p.IntfName,
		Subinterface:     idx,
		NexthopGroupName: p.NexthopGroupName,
		LocalLabel:       strconv.Itoa(p.LocalLabelBase + idx),
		RemoteLabel:      strconv.Itoa(p.RemoteLabelBase + idx),
		PatchPanel:       fmt.Sprintf("patch-%d", idx),
	}
}

// useCFMScaleCLI reports whether all of the CFM scale configuration has to be
// pushed as CLI, in which case it is batched to reduce the number of RPCs.
func useCFMScaleCLI(dut *ondatra.DUTDevice) bool {
	return dut.Vendor() == ondatra.ARISTA &&
		deviations.MplsStaticPseudowireOcUnsupported(dut) &&
		deviations.VlanClientEncapsulationOcUnsupported(dut) &&
		deviations.CfmOCUnsupported(dut)
}

// pushBatchedCLI pushes the CLI snippets, batchSize snippets per request.
func pushBatchedCLI(t *testing.T, dut *ondatra.DUTDevice, snippets []string, batchSize int) {
	t.Helper()
	if batchSize <= 0 {
		batchSize = defaultCFMScaleCLIBatchSize
	}
	for start := 0; start < len(snippets); start += batchSize {
		end := min(start+batchSize, len(snippets))
		helpers.GnmiCLIConfig(t, dut, strings.Join(snippets[start:end], ""))
	}
}

// cfmDomainCLI returns the vendor CLI that configures the CFM domain cfg on
// devices with the CfmOCUnsupported deviation.
func cfmDomainCLI(t *testing.T, dut *ondatra.DUTDevice, cfg *MaintenanceDomainConfig) string {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return aristaCFMDomainCLI(cfg, firstAssoc(t, cfg))
	default:
		t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
		return ""
	}
}

// cfmRemoveDomainCLI returns the vendor CLI that removes the CFM domain cfg on
// devices with the CfmOCUnsupported deviation.
func cfmRemoveDomainCLI(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) string {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return fmt.Sprintf("cfm\n   no domain %v\n", cfg.DomainName)
	default:
		t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
		return ""
	}
}

// ConfigureCFMScale configures a static pseudowire, client VLAN encapsulation
// and CFM session for each subinterface in params. The subinterfaces must
// already exist.
//
// The OpenConfig configuration is appended to sb and the caller is responsible
// for calling sb.Set. Configuration that is only available as CLI (deviations)
// cannot be carried by a gnmi.SetBatch, so it is pushed by this function in
// batches of params.CLIBatchSize sessions per request. ConfigureCFMScale
// returns nil when nothing was appended to sb, i.e. when all of the
// configuration was pushed as CLI.
func ConfigureCFMScale(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, params CFMScaleParams) *gnmi.SetBatch {
	t.Helper()
	cfgs := params.DomainConfigs()

	if useCFMScaleCLI(dut) {
		var snippets []string
		for i, idx := range params.Subinterfaces {
			snippets = append(snippets,
				aristaMplsStaticPseudowireCLI(params.pseudowire(idx))+
					aristaVlanClientEncapsulationCLI(VlanClientEncapsulationParams{IntfName: params.IntfName, Subinterfaces: idx})+
					aristaCFMDomainCLI(&cfgs[i], firstAssoc(t, &cfgs[i])))
		}
		pushBatchedCLI(t, dut, snippets, params.CLIBatchSize)
		return nil
	}

	for _, idx := range params.Subinterfaces {
		ConfigureMplsStaticPseudowire(t, sb, dut, params.pseudowire(idx))
		VlanClientEncapsulation(t, sb, dut, VlanClientEncapsulationParams{IntfName: params.IntfName, Subinterfaces: idx})
	}
	if deviations.CfmOCUnsupported(dut) {
		var snippets []string
		for i := range cfgs {
			snippets = append(snippets, cfmDomainCLI(t, dut, &cfgs[i]))
		}
		pushBatchedCLI(t, dut, snippets, params.CLIBatchSize)
	} else {
		oam := &oc.Oam{}
		for i := range cfgs {
			firstAssoc(t, &cfgs[i])
			gnmi.BatchUpdate(sb, gnmi.OC().Oam().Cfm().MaintenanceDomain(cfgs[i].DomainName).Config(), buildCFMDomainOC(oam, &cfgs[i]))
		}
	}
	if deviations.CfmOCUnsupported(dut) && deviations.MplsStaticPseudowireOcUnsupported(dut) {
		return nil
	}
	return sb
}

// RemoveCFMScale removes the CFM sessions, patches and pseudowires configured
// by ConfigureCFMScale. The subinterfaces are not removed.
//
// As with ConfigureCFMScale, the OpenConfig deletes are appended to sb for the
// caller to Set, CLI-only configuration is removed by this function in batches,
// and nil is returned when nothing was appended to sb.
func RemoveCFMScale(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, params CFMScaleParams) *gnmi.SetBatch {
	t.Helper()
	cfgs := params.DomainConfigs()

	if useCFMScaleCLI(dut) {
		var snippets []string
		for i, idx := range params.Subinterfaces {
			pw := params.pseudowire(idx)
			snippets = append(snippets, fmt.Sprintf(`
cfm
   no domain %s
patch panel
   no patch %s
mpls pseudowires
   static pseudowires
      no pseudowire %s
`, cfgs[i].DomainName, pw.PatchPanel, pw.PseudowireName))
		}
		pushBatchedCLI(t, dut, snippets, params.CLIBatchSize)
		return nil
	}

	// TODO: Remove the scale pseudowires once ConfigureMplsStaticPseudowire
	// supports one OC connection-point per pseudowire.
	if deviations.CfmOCUnsupported(dut) {
		var snippets []string
		for _, cfg := range cfgs {
			snippets = append(snippets, cfmRemoveDomainCLI(t, dut, cfg))
		}
		pushBatchedCLI(t, dut, snippets, params.CLIBatchSize)
		return nil
	}
	for _, cfg := range cfgs {
		gnmi.BatchDelete(sb, gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).Config())
	}
	return sb
}

// MonitorSessionConfig holds configuration for monitor session
type MonitorSessionConfig struct {
	SessionName       string
	SourcePort        string // Port to monitor
	DestinationDUTAte string // DUT port connected to ATE port where mirrored traffic is sent
}

// ConfigureMonitorSession configures SPAN/monitor session on device
func ConfigureMonitorSession(t *testing.T, dut *ondatra.DUTDevice, config MonitorSessionConfig) {
	cli := ""
	if deviations.CfmOCUnsupported(dut) {
		t.Log("Configuring CFM configs")
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cli += fmt.Sprintf(`
				monitor session %v source %v
				monitor session %v destination %v
			`, config.SessionName, config.SourcePort, config.SessionName, config.DestinationDUTAte)
			helpers.GnmiCLIConfig(t, dut, cli)
		default:
			t.Errorf("Deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
	}
}

// cfmFTWrapper adapts the keys emitted by the Arista CFM functional
// translators (domain-name, association-name, mep-id) to the OC list keys
// (md-id, ma-id, local-mep-id), and orders the Sysdb input paths first so that
// the translator caches are populated before the Smash telemetry arrives.
//
// TODO: Remove once the functional translators emit the OC list keys.
type cfmFTWrapper struct {
	ft ygnmi.FunctionalTranslator
}

func (w *cfmFTWrapper) Translate(sr *gpb.SubscribeResponse) (*gpb.SubscribeResponse, error) {
	out, err := w.ft.Translate(sr)
	if err != nil || out == nil || out.GetUpdate() == nil {
		return out, err
	}
	fixCFMPathKeys(out.GetUpdate().GetPrefix())
	for _, u := range out.GetUpdate().GetUpdate() {
		fixCFMPathKeys(u.GetPath())
	}
	for _, d := range out.GetUpdate().GetDelete() {
		fixCFMPathKeys(d)
	}
	return out, nil
}

func (w *cfmFTWrapper) OutputToInput(p *gpb.Path) (bool, []*gpb.Path, error) {
	matched, paths, err := w.ft.OutputToInput(p)
	if err != nil || !matched {
		return matched, paths, err
	}
	var sysdbPaths, otherPaths []*gpb.Path
	for _, path := range paths {
		if len(path.GetElem()) > 0 && path.GetElem()[0].GetName() == "Sysdb" {
			sysdbPaths = append(sysdbPaths, path)
		} else {
			otherPaths = append(otherPaths, path)
		}
	}
	return true, append(sysdbPaths, otherPaths...), nil
}

func fixCFMPathKeys(p *gpb.Path) {
	if p == nil {
		return
	}
	renames := map[string][2]string{
		"maintenance-domain":      {"domain-name", "md-id"},
		"maintenance-association": {"association-name", "ma-id"},
		"mep-endpoint":            {"mep-id", "local-mep-id"},
	}
	for _, elem := range p.GetElem() {
		r, ok := renames[elem.GetName()]
		if !ok {
			continue
		}
		if v, ok := elem.GetKey()[r[0]]; ok {
			elem.Key[r[1]] = v
			delete(elem.Key, r[0])
		}
	}
}

// cfmTelemetryOpts returns the gNMI options to query CFM telemetry, using the
// functional translator ftName when it is set.
func cfmTelemetryOpts(t *testing.T, dut *ondatra.DUTDevice, ftName string) *gnmi.Opts {
	t.Helper()
	if ftName == "" {
		return dut.GNMIOpts()
	}
	ft, ok := registrar.FunctionalTranslatorRegistry[ftName]
	if !ok {
		t.Fatalf("Functional translator %s is not registered", ftName)
	}
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithFT(&cfmFTWrapper{ft: ft}))
}

// lastValue formats the last value observed by a gnmi.Watch for error messages.
func lastValue[T any](v *ygnmi.Value[T]) string {
	if v == nil {
		return "none"
	}
	val, present := v.Val()
	if !present {
		return "not present"
	}
	return fmt.Sprint(val)
}

// awaitLeaf waits for the leaf q to satisfy pred and returns its value.
func awaitLeaf[T any](t *testing.T, opts *gnmi.Opts, q ygnmi.SingletonQuery[T], timeout time.Duration, desc string, pred func(T) bool) (T, bool) {
	t.Helper()
	return awaitLeafSince(t, opts, q, time.Time{}, timeout, desc, pred)
}

// awaitLeafSince waits for the leaf q to satisfy pred with a value timestamped
// after since, and returns its value. A zero since accepts any timestamp.
func awaitLeafSince[T any](t *testing.T, opts *gnmi.Opts, q ygnmi.SingletonQuery[T], since time.Time, timeout time.Duration, desc string, pred func(T) bool) (T, bool) {
	t.Helper()
	val, ok := gnmi.Watch(t, opts, q, timeout, func(v *ygnmi.Value[T]) bool {
		got, present := v.Val()
		return present && v.Timestamp.After(since) && pred(got)
	}).Await(t)
	var got T
	if !ok {
		var ts time.Time
		if val != nil {
			ts = val.Timestamp
		}
		t.Errorf("%s: got %s (timestamp %v, want after %v), want condition to be met within %v", desc, lastValue(val), ts, since, timeout)
		return got, false
	}
	got, _ = val.Val()
	return got, true
}

// ValidateCFMSession validates that the remote MEP of cfg reaches cfg.Status
// and that the domain, association and MEP state reflect the configuration.
//
// Paths:
//
//	/oam/cfm/domains/maintenance-domain/state/md-id
//	/oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/state/ma-id
//	/oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/state/local-mep-id
//	/oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/remote-meps/remote-mep/state/id
func ValidateCFMSession(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	if deviations.CfmOCUnsupported(dut) {
		// TODO: Replace with a functional translator for the CFM session state
		// leaves, telemetry deviations should not depend on CLI output.
		validateCFMSessionCLI(t, dut, cfg)
		return
	}

	opts := dut.GNMIOpts()
	domain := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName)
	ma := domain.MaintenanceAssociation(cfg.MdID)
	mep := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).MepEndpoint(uint16(assoc.LocalMEPID))
	rmepID := uint16(assoc.RemoteMEPID)

	if _, ok := awaitLeaf(t, opts, mep.RemoteMep(rmepID).OperState().State(), cfmSessionTimeout,
		fmt.Sprintf("%s: remote MEP %d oper-state", dut.Name(), rmepID),
		func(v oc.E_OamCfm_OperationalStateType) bool { return v == cfg.Status }); ok {
		t.Logf("%s: remote MEP %d oper-state is %v as expected", dut.Name(), rmepID, cfg.Status)
	}
	if got := gnmi.Get(t, dut, domain.MdId().State()); got != cfg.DomainName {
		t.Errorf("%s: md-id state got %q, want %q", dut.Name(), got, cfg.DomainName)
	}
	if got := gnmi.Get(t, dut, ma.MaId().State()); got != cfg.MdID {
		t.Errorf("%s: ma-id state got %q, want %q", dut.Name(), got, cfg.MdID)
	}
	if got := gnmi.Get(t, dut, mep.LocalMepId().State()); got != uint16(assoc.LocalMEPID) {
		t.Errorf("%s: local-mep-id state got %d, want %d", dut.Name(), got, assoc.LocalMEPID)
	}
	if got := gnmi.Get(t, dut, mep.RemoteMep(rmepID).Id().State()); got != rmepID {
		t.Errorf("%s: remote-mep id state got %d, want %d", dut.Name(), got, rmepID)
	}
}

func validateCFMSessionCLI(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	opState := ""
	switch cfg.Status {
	case oc.OamCfm_OperationalStateType_ENABLED:
		opState = "active"
	case oc.OamCfm_OperationalStateType_DISABLED:
		opState = "deactive"
	case oc.OamCfm_OperationalStateType_UNKNOWN:
		opState = "unknown"
	default:
		opState = "unset"
	}

	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf("show cfm end-point domain %v association %v end-point %v", cfg.DomainName, cfg.MdID, assoc.LocalMEPID)
		output := helpers.ExecuteShowCLI(t, dut, cli).String()
		if !strings.Contains(output, strconv.Itoa(assoc.RemoteMEPID)) {
			t.Fatalf("Expected remote MEP ID %v not found in output", assoc.RemoteMEPID)
		}
		if !strings.Contains(output, opState) {
			t.Fatalf("Expected remote MEP status %s not found in output", opState)
		}
		t.Logf("Verified remote MEP ID %v with status %s", assoc.RemoteMEPID, cfg.Status)
	default:
		t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
	}
}

// ValidateDeadTimer validates the CCM interval from which the dead timer
// (3.5 times the CCM interval) is derived.
func ValidateDeadTimer(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	if deviations.CfmOCUnsupported(dut) {
		// TODO: Validate the dead timer once telemetry is available.
		t.Logf("%s: dead timer validation is not supported", dut.Name())
		return
	}
	want := assoc.CcmInterval
	got := gnmi.Get(t, dut, gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).CcmInterval().State())
	if got != want {
		t.Errorf("%s: ccm-interval state got %v, want %v", dut.Name(), got, want)
		return
	}
	t.Logf("%s: ccm-interval is %v, dead timer is %v", dut.Name(), got, time.Duration(3.5*float64(ccmIntervalDuration[got])))
}

// ValidateAlarmDetection validates that the local MEP of cfg reports a remote
// defect (present-rdi) within cfmSessionTimeout.
//
// Path:
//
//	/oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/state/present-rdi
func ValidateAlarmDetection(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	t.Helper()
	ValidateAlarmDetectionWithin(t, dut, cfg, cfmSessionTimeout)
}

// ValidateAlarmDetectionWithin is ValidateAlarmDetection with a custom timeout.
func ValidateAlarmDetectionWithin(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig, timeout time.Duration) {
	t.Helper()
	validatePresentRDI(t, dut, cfg, true, timeout)
}

// ValidateAlarmCleared validates that the local MEP of cfg no longer reports a
// remote defect (present-rdi is false) within timeout.
func ValidateAlarmCleared(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig, timeout time.Duration) {
	t.Helper()
	validatePresentRDI(t, dut, cfg, false, timeout)
}

func validatePresentRDI(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig, want bool, timeout time.Duration) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	if deviations.CfmStateFt(dut) == "" && deviations.CfmOCUnsupported(dut) {
		// TODO: Remove once all CfmOCUnsupported platforms set cfm_state_ft.
		validateAlarmDetectionCLI(t, dut, cfg, want)
		return
	}
	opts := cfmTelemetryOpts(t, dut, deviations.CfmStateFt(dut))
	if _, ok := awaitLeaf(t, opts, gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).MepEndpoint(uint16(assoc.LocalMEPID)).PresentRdi().State(), timeout,
		fmt.Sprintf("%s: present-rdi (want %v)", dut.Name(), want), func(v bool) bool { return v == want }); ok {
		t.Logf("%s: present-rdi is %v as expected", dut.Name(), want)
	}
}

func validateAlarmDetectionCLI(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig, want bool) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf("show cfm continuity-check end-point domain %v association %v end-point %v", cfg.DomainName, cfg.MdID, assoc.LocalMEPID)
		output := helpers.ExecuteShowCLI(t, dut, cli).String()
		rdiFlag := regexp.MustCompile(`TX RDI state:\s*(true|false)`).FindStringSubmatch(output)
		if len(rdiFlag) < 2 {
			t.Errorf("rdi state not found")
			return
		}
		if rdiStatus := strings.TrimSpace(rdiFlag[1]); rdiStatus != strconv.FormatBool(want) {
			t.Errorf("expected RDI state: %v, got: %s", want, rdiStatus)
			return
		}
		t.Logf("RDI state is %v as expected", want)
	default:
		t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
	}
}

// ValidateMeasurementProfile validates the state of the global performance
// measurement profile.
//
// Path:
//
//	/oam/cfm/performance-measurement-profiles-global/performance-measurement-profile/state/profile-name
func ValidateMeasurementProfile(t *testing.T, dut *ondatra.DUTDevice, profileName string) {
	t.Helper()
	if deviations.CfmOCUnsupported(dut) {
		// TODO: Validate once a functional translator covers this path.
		t.Logf("%s: performance-measurement-profile state is not supported", dut.Name())
		return
	}
	if _, ok := awaitLeaf(t, dut.GNMIOpts(), gnmi.OC().Oam().Cfm().PerformanceMeasurementProfile(profileName).ProfileName().State(), cfmSessionTimeout,
		fmt.Sprintf("%s: performance-measurement-profile profile-name", dut.Name()), func(v string) bool { return v == profileName }); ok {
		t.Logf("%s: performance-measurement-profile %q is present", dut.Name(), profileName)
	}
}

// validatePmMeasurementType validates the measurement type of the pm-profile
// of the local MEP of cfg.
//
// Path:
//
//	/oam/cfm/domains/maintenance-domain/maintenance-associations/maintenance-association/mep-endpoints/mep-endpoint/pm-profiles/pm-profile/state/measurement-type
func validatePmMeasurementType(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig, want oc.E_PmProfile_MeasurementType) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	if got := gnmi.Get(t, dut, gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).MepEndpoint(uint16(assoc.LocalMEPID)).PmProfile(cfg.ProfileName).MeasurementType().State()); got != want {
		t.Errorf("%s: pm-profile measurement-type got %v, want %v", dut.Name(), got, want)
	}
}

// ValidateDelayMeasurement validates the two-way delay measurement of the local
// MEP of cfg.
//
// Paths:
//
//	.../mep-endpoint/pm-profiles/pm-profile/state/delay-measurement-state/frame-delay-two-way-average
//	.../mep-endpoint/pm-profiles/pm-profile/state/delay-measurement-state/frame-delay-two-way-max
//	.../mep-endpoint/pm-profiles/pm-profile/state/delay-measurement-state/frame-delay-two-way-min
//	.../mep-endpoint/pm-profiles/pm-profile/state/measurement-type
//
// Only measurements reported after since are accepted, so that values from a
// previous measurement are not validated.
func ValidateDelayMeasurement(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig, since time.Time) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	ftName := deviations.CfmPmFt(dut)
	if ftName == "" && deviations.CfmOCUnsupported(dut) {
		// TODO: Remove once all CfmOCUnsupported platforms set cfm_pm_ft.
		validateDelayMeasurementCLI(t, dut, cfg)
		return
	}

	dmState := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).MepEndpoint(uint16(assoc.LocalMEPID)).PmProfile(cfg.ProfileName).DelayMeasurementState()
	// An average of 0 means that no delay has been measured yet.
	avg, ok := awaitLeafSince(t, cfmTelemetryOpts(t, dut, ftName), dmState.FrameDelayTwoWayAverage().State(), since, pmMeasurementTimeout,
		fmt.Sprintf("%s: frame-delay-two-way-average (µs)", dut.Name()),
		func(v uint32) bool { return v > 0 && v <= maxTwoWayDelayMicros })
	if !ok {
		if deviations.CfmOCUnsupported(dut) {
			// Log the native delay measurement to tell a measurement that is not
			// running apart from a functional translator issue.
			validateDelayMeasurementCLI(t, dut, cfg)
		}
		LogCFMDebug(t, dut, cfg, "delay measurement not reported")
		return
	}
	t.Logf("%s: two-way frame delay average: %d µs", dut.Name(), avg)

	if deviations.CfmOCUnsupported(dut) {
		// TODO: Validate frame-delay-two-way-min/max and measurement-type once
		// the functional translator supports them.
		return
	}
	minDelay := gnmi.Get(t, dut, dmState.FrameDelayTwoWayMin().State())
	maxDelay := gnmi.Get(t, dut, dmState.FrameDelayTwoWayMax().State())
	if minDelay == 0 || minDelay > avg || avg > maxDelay || maxDelay > maxTwoWayDelayMicros {
		t.Errorf("%s: two-way frame delay got min/avg/max %d/%d/%d µs, want 0 < min <= avg <= max <= %d", dut.Name(), minDelay, avg, maxDelay, maxTwoWayDelayMicros)
	} else {
		t.Logf("%s: two-way frame delay min/avg/max: %d/%d/%d µs", dut.Name(), minDelay, avg, maxDelay)
	}
	validatePmMeasurementType(t, dut, cfg, oc.PmProfile_MeasurementType_DMM)
}

func validateDelayMeasurementCLI(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf("show cfm measurement delay proactive domain %s association %v end-point %v", cfg.DomainName, cfg.MdID, assoc.LocalMEPID)
		output := helpers.ExecuteShowCLI(t, dut, cli).String()
		m := regexp.MustCompile(`Two-way delay \(usec\)\s+min/max/avg:\s+([\d.]+)/([\d.]+)/([\d.]+)`).FindStringSubmatch(output)
		if len(m) < 4 {
			t.Errorf("%s: delay measurements not found in %q output:\n%s", dut.Name(), cli, output)
			return
		}
		t.Logf("%s: two-way delay (µs) - Min: %s, Max: %s, Avg: %s", dut.Name(), m[1], m[2], m[3])
		if avg, err := strconv.ParseFloat(m[3], 64); err != nil || avg == 0 {
			// Log the whole output, which includes the DMM/DMR counters and every
			// measurement interval, to debug a measurement that is not running.
			t.Logf("%s: no two-way delay measured, %q output:\n%s", dut.Name(), cli, output)
		}
	default:
		t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
	}
}

// ValidateLossMeasurement validates the synthetic loss measurement of the local
// MEP of each cfg on the corresponding DUT.
//
// Paths:
//
//	.../mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/far-end-average-frame-loss-ratio
//	.../mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/far-end-max-frame-loss-ratio
//	.../mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/far-end-min-frame-loss-ratio
//	.../mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/near-end-average-frame-loss-ratio
//	.../mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/near-end-max-frame-loss-ratio
//	.../mep-endpoint/pm-profiles/pm-profile/state/loss-measurement-state/near-end-min-frame-loss-ratio
//	.../mep-endpoint/pm-profiles/pm-profile/state/measurement-type
//
// Only measurements reported after since are accepted, so that values from a
// previous measurement are not validated.
func ValidateLossMeasurement(t *testing.T, duts []*ondatra.DUTDevice, cfgs []MaintenanceDomainConfig, since time.Time) {
	t.Helper()
	for i, dut := range duts {
		validateLossMeasurement(t, dut, cfgs[i], since)
	}
}

func validateLossMeasurement(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig, since time.Time) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	ftName := deviations.CfmPmFt(dut)
	if ftName == "" && deviations.CfmOCUnsupported(dut) {
		// TODO: Remove once all CfmOCUnsupported platforms set cfm_pm_ft.
		validateLossMeasurementCLI(t, dut, cfg)
		return
	}

	lmState := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).MepEndpoint(uint16(assoc.LocalMEPID)).PmProfile(cfg.ProfileName).LossMeasurementState()
	opts := cfmTelemetryOpts(t, dut, ftName)
	validRatio := func(v uint32) bool { return v <= maxFrameLossRatio }

	if !deviations.CfmOCUnsupported(dut) {
		// Wait for SLM replies before reading the loss ratios.
		if _, ok := awaitLeafSince(t, opts, lmState.Counters().SlmReceived().State(), since, pmMeasurementTimeout,
			fmt.Sprintf("%s: slm-received", dut.Name()), func(v uint64) bool { return v > 0 }); !ok {
			return
		}
	}

	for _, side := range []struct {
		name          string
		avg, min, max ygnmi.SingletonQuery[uint32]
	}{
		{"far-end", lmState.FarEndAverageFrameLossRatio().State(), lmState.FarEndMinFrameLossRatio().State(), lmState.FarEndMaxFrameLossRatio().State()},
		{"near-end", lmState.NearEndAverageFrameLossRatio().State(), lmState.NearEndMinFrameLossRatio().State(), lmState.NearEndMaxFrameLossRatio().State()},
	} {
		avg, ok := awaitLeafSince(t, opts, side.avg, since, pmMeasurementTimeout,
			fmt.Sprintf("%s: %s-average-frame-loss-ratio (milli-percent)", dut.Name(), side.name), validRatio)
		if !ok {
			LogCFMDebug(t, dut, cfg, side.name+" loss measurement not reported")
			continue
		}
		if deviations.CfmOCUnsupported(dut) {
			// TODO: Validate the min/max frame loss ratios once the functional
			// translator supports them.
			t.Logf("%s: %s average frame loss ratio: %d", dut.Name(), side.name, avg)
			continue
		}
		minRatio := gnmi.Get(t, dut, side.min)
		maxRatio := gnmi.Get(t, dut, side.max)
		if minRatio > avg || avg > maxRatio || maxRatio > maxFrameLossRatio {
			t.Errorf("%s: %s frame loss ratio got min/avg/max %d/%d/%d, want min <= avg <= max <= %d", dut.Name(), side.name, minRatio, avg, maxRatio, maxFrameLossRatio)
		} else {
			t.Logf("%s: %s frame loss ratio min/avg/max: %d/%d/%d", dut.Name(), side.name, minRatio, avg, maxRatio)
		}
	}
	if !deviations.CfmOCUnsupported(dut) {
		validatePmMeasurementType(t, dut, cfg, oc.PmProfile_MeasurementType_SLM)
	}
}

func validateLossMeasurementCLI(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf("show cfm measurement loss synthetic proactive domain %s association %v end-point %v", cfg.DomainName, cfg.MdID, assoc.LocalMEPID)
		output := helpers.ExecuteShowCLI(t, dut, cli).String()
		for _, side := range []string{"Far-end", "Near-end"} {
			m := regexp.MustCompile(side + ` frame .*?min/max/avg: (\d+\.\d+)/(\d+\.\d+)/(\d+\.\d+)`).FindStringSubmatch(output)
			if len(m) < 4 {
				t.Errorf("%s loss measurements not found", side)
				continue
			}
			t.Logf("%s loss ratio - Min: %s, Max: %s, Avg: %s", side, m[1], m[2], m[3])
		}
	default:
		t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
	}
}

// cfmDebugMaxOutputLen bounds the output logged for a single debug command.
const cfmDebugMaxOutputLen = 20000

// LogCFMDebug logs the native CFM configuration, MEP state and performance
// measurement counters of the local MEP of cfg, prefixed with label. It never
// fails the test: commands that are rejected by the DUT are logged and
// skipped. extraCmds are additional show commands to log. It is used to debug
// performance measurements that are not reported.
func LogCFMDebug(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig, label string, extraCmds ...string) {
	t.Helper()
	assoc := firstAssoc(t, &cfg)
	if dut.Vendor() != ondatra.ARISTA {
		t.Logf("%s: [%s] CFM debug is not implemented for %v", dut.Name(), label, dut.Vendor())
		return
	}
	ep := fmt.Sprintf("domain %s association %v end-point %v", cfg.DomainName, cfg.MdID, assoc.LocalMEPID)
	cmds := []string{
		"show version",
		"show clock",
		"show running-config section cfm",
		"show cfm end-point " + ep,
		"show cfm continuity-check end-point " + ep,
		"show cfm measurement delay proactive " + ep,
		"show cfm measurement delay proactive " + ep + " detail",
		"show cfm measurement loss synthetic proactive " + ep,
		"show cfm measurement loss synthetic proactive " + ep + " detail",
		"show patch panel detail",
	}
	if cfg.IntfName != "" {
		cmds = append(cmds, "show interfaces "+cfg.IntfName)
	}
	cmds = append(cmds, extraCmds...)
	for _, cmd := range cmds {
		out, err := cliShowNonFatal(t, dut, cmd)
		if err != nil {
			t.Logf("%s: [%s] %q failed: %v", dut.Name(), label, cmd, err)
			continue
		}
		t.Logf("%s: [%s] %q output:\n%s", dut.Name(), label, cmd, truncateDebugOutput(out))
	}
	// Raw native state read by the arista-cfm-pm-ft and arista-cfm-state-ft
	// functional translators.
	for _, elems := range [][]string{
		{"Smash", "cfm", "mepSmashTable"},
		{"Sysdb", "cfm", "config"},
		{"Sysdb", "cfm", "status"},
	} {
		p := "/" + strings.Join(elems, "/")
		out, err := nativeGetNonFatal(t, dut, elems)
		if err != nil {
			t.Logf("%s: [%s] eos_native %s get failed: %v", dut.Name(), label, p, err)
			continue
		}
		t.Logf("%s: [%s] eos_native %s:\n%s", dut.Name(), label, p, truncateDebugOutput(out))
	}
}

// cliShowNonFatal runs a show command over gNMI and returns its output.
func cliShowNonFatal(t *testing.T, dut *ondatra.DUTDevice, cmd string) (string, error) {
	t.Helper()
	return gnmiGetText(t, dut, &gpb.GetRequest{
		Path:     []*gpb.Path{{Origin: "cli", Elem: []*gpb.PathElem{{Name: cmd}}}},
		Encoding: gpb.Encoding_ASCII,
		Type:     gpb.GetRequest_ALL,
	})
}

// nativeGetNonFatal reads an eos_native path over gNMI and returns its JSON.
func nativeGetNonFatal(t *testing.T, dut *ondatra.DUTDevice, elems []string) (string, error) {
	t.Helper()
	path := &gpb.Path{Origin: "eos_native"}
	for _, e := range elems {
		path.Elem = append(path.Elem, &gpb.PathElem{Name: e})
	}
	return gnmiGetText(t, dut, &gpb.GetRequest{
		Path:     []*gpb.Path{path},
		Encoding: gpb.Encoding_JSON,
		Type:     gpb.GetRequest_ALL,
	})
}

func gnmiGetText(t *testing.T, dut *ondatra.DUTDevice, req *gpb.GetRequest) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := dut.RawAPIs().GNMI(t).Get(ctx, req)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, n := range resp.GetNotification() {
		for _, u := range n.GetUpdate() {
			v := u.GetVal()
			switch {
			case v.GetAsciiVal() != "":
				sb.WriteString(v.GetAsciiVal())
			case len(v.GetJsonVal()) > 0:
				fmt.Fprintf(&sb, "%s: %s\n", pathString(n.GetPrefix(), u.GetPath()), v.GetJsonVal())
			case len(v.GetJsonIetfVal()) > 0:
				fmt.Fprintf(&sb, "%s: %s\n", pathString(n.GetPrefix(), u.GetPath()), v.GetJsonIetfVal())
			default:
				fmt.Fprintf(&sb, "%s: %v\n", pathString(n.GetPrefix(), u.GetPath()), v)
			}
		}
	}
	return sb.String(), nil
}

func pathString(prefix, path *gpb.Path) string {
	var parts []string
	for _, p := range []*gpb.Path{prefix, path} {
		for _, e := range p.GetElem() {
			parts = append(parts, e.GetName())
		}
	}
	return "/" + strings.Join(parts, "/")
}

func truncateDebugOutput(s string) string {
	if len(s) <= cfmDebugMaxOutputLen {
		return s
	}
	return fmt.Sprintf("%s\n... (truncated %d bytes)", s[:cfmDebugMaxOutputLen], len(s)-cfmDebugMaxOutputLen)
}
