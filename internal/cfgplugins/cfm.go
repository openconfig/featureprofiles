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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
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

var measurementTypeMap = map[oc.E_PmProfile_MeasurementType]string{
	oc.PmProfile_MeasurementType_UNSET: "",
	oc.PmProfile_MeasurementType_LMM:   "loss",
	oc.PmProfile_MeasurementType_SLM:   "loss synthetic",
	oc.PmProfile_MeasurementType_DMM:   "delay",
	oc.PmProfile_MeasurementType_CCM:   "continuity-check",
}

// ConfigureMeasurementProfile configures the cfm performance measurement paramaters
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
			helpers.GnmiCLIConfig(t, dut, cli)
		default:
			t.Errorf("Deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
		return nil
	} else {
		root := &oc.Root{}
		oam := root.GetOrCreateOam()
		cfm := oam.GetOrCreateCfm()
		profile := cfm.GetOrCreatePerformanceMeasurementProfile(params.ProfileName)
		profile.SetBurstInterval(params.BurstInterval)
		profile.SetIntervalsArchived(params.IntervalsArchived)
		profile.SetMeasurementInterval(params.MeasurementInterval)
		profile.SetMeasurementType(params.MeasurementType)
		profile.SetPacketPerBurst(params.PacketPerBurst)
		profile.SetPacketsPerMeaurementPeriod(params.PacketsPerMeaurementPeriod)
		profile.SetRepetitionPeriod(params.RepetitionPeriod)

		gnmi.BatchReplace(batch, gnmi.OC().Oam().Config(), oam)
		return oam
	}
}

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

// ConfigureCFMDomain configures the cfm domain
func ConfigureCFMDomain(t *testing.T, oam *oc.Oam, dut *ondatra.DUTDevice, cfg *MaintenanceDomainConfig) {
	cli := "cfm\n"
	if deviations.CfmOCUnsupported(dut) {
		t.Log("Configuring CFM configs")
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if cfg.RemoveDomain {
				rmDomain := fmt.Sprintf(`
					cfm
					no domain %v`, cfg.DomainName)
				helpers.GnmiCLIConfig(t, dut, rmDomain)
			}
			if cfg.Assocs[0].CcmInterval != oc.MaintenanceAssociation_CcmInterval_UNSET {
				cli += fmt.Sprintf(`
					profile %s
					continuity-check tx-interval %v`, cfg.ProfileName, ccmIntervalMap[cfg.Assocs[0].CcmInterval])
			}
			if cfg.DomainName != "" {
				cli += fmt.Sprintf("\ndomain %v ", cfg.DomainName)
				if cfg.Level != -1 {
					cli += fmt.Sprintf("level %v\n", cfg.Level)
				}
			}

			if cfg.MdID != "" {
				cli += fmt.Sprintf("association %v\n", cfg.MdID)
			}

			if cfg.Assocs[0].Direction != oc.MepEndpoint_Direction_UNSET {
				direction := ""
				switch cfg.Assocs[0].Direction {
				case oc.MepEndpoint_Direction_UP:
					direction = "up"
				}
				cli += fmt.Sprintf("direction %v\n", direction)
			}

			if cfg.ProfileName != "" {
				cli += fmt.Sprintf("profile %v\n", cfg.ProfileName)
			}

			if cfg.Assocs[0].LocalMEPID != 0 {
				cli += fmt.Sprintf("end-point %v\n", cfg.Assocs[0].LocalMEPID)
			}

			if cfg.IntfName != "" {
				cli += fmt.Sprintf("interface %v\n", cfg.IntfName)
			}

			if cfg.Assocs[0].RemoteMEPID != 0 {
				cli += fmt.Sprintf("remote end-point %v\n", cfg.Assocs[0].RemoteMEPID)
			}
			helpers.GnmiCLIConfig(t, dut, cli)
		default:
			t.Errorf("deviation CfmOCUnsupported is not handled for the dut: %v", dut.Vendor())
		}
	} else {
		domain := oam.GetOrCreateCfm().GetOrCreateMaintenanceDomain(cfg.DomainName)
		domain.SetLevel(uint8(cfg.Level))
		domain.SetMdId(cfg.MdID)
		domain.SetMdNameType(cfg.MdNameType)

		for _, assoc := range cfg.Assocs {
			assoc.InterfaceName = cfg.IntfName
			assoc.ProfileName = cfg.ProfileName
			configureCFMAssociation(domain, assoc)
		}
	}

}

// Configure loss threshold
func ConfigureLossThreshold(t *testing.T, dut *ondatra.DUTDevice, oam *oc.Oam, cfg MaintenanceDomainConfig, loss float64) {
	cli := "cfm\n"
	if deviations.CfmOCUnsupported(dut) {
		t.Log("Configuring CFM configs")
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cli += fmt.Sprintf(`
					profile %s
					continuity-check timeout multiplier %v`, cfg.ProfileName, loss)
			helpers.GnmiCLIConfig(t, dut, cli)
		}
	} else {
		domain := oam.GetOrCreateCfm().GetOrCreateMaintenanceDomain(cfg.DomainName)
		ma := domain.GetOrCreateMaintenanceAssociation(domain.GetMdId())
		ma.SetLossThreshold(uint16(loss))
	}
}

// configureCFMAssociation configures cfm domain association
func configureCFMAssociation(domain *oc.Oam_Cfm_MaintenanceDomain, cfg AssociationConfig) {
	ma := domain.GetOrCreateMaintenanceAssociation(domain.GetMdId())
	ma.SetGroupName(cfg.GroupName)
	ma.SetCcmInterval(cfg.CcmInterval)
	ma.SetLossThreshold(uint16(cfg.LossThreshold))
	ma.SetMaNameType(cfg.MaNameType)
	ma.SetMaId(cfg.MaID)

	mepEndpoint := ma.GetOrCreateMepEndpoint(uint16(cfg.LocalMEPID))
	mepEndpoint.SetCcmEnabled(cfg.CcmEnabled)
	mepEndpoint.SetDirection(cfg.Direction)
	mepEndpoint.SetInterface(cfg.InterfaceName)
	mepEndpoint.GetOrCreatePmProfile(cfg.ProfileName)
	mepEndpoint.GetOrCreateRdi().SetTransmitOnDefect(cfg.TransmitOnDefect)
	mepEndpoint.GetOrCreateRemoteMep(uint16(cfg.RemoteMEPID))
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

func ValidateCFMSession(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	if deviations.CfmOCUnsupported(dut) {
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

		cli := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cli = fmt.Sprintf(`
				show cfm end-point domain %v association %v end-point %v
				`, cfg.DomainName, cfg.MdID, cfg.Assocs[0].LocalMEPID)
			output := helpers.ExecuteShowCLI(t, dut, cli).String()

			if !strings.Contains(output, strconv.Itoa(cfg.Assocs[0].RemoteMEPID)) {
				t.Fatalf("Expected remote MEP ID %v not found in output", cfg.Assocs[0].RemoteMEPID)
			}
			if !strings.Contains(output, opState) {
				t.Fatalf("Expected remote MEP status %s not found in output", opState)
			}
			t.Logf("Verified remote MEP ID %v with status %s", cfg.Assocs[0].RemoteMEPID, cfg.Status)
		}
	} else {
		remoteMeps := gnmi.GetAll(t, dut, gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).MepEndpoint(uint16(cfg.Assocs[0].LocalMEPID)).RemoteMepAny().State())
		for _, rmep := range remoteMeps {
			if rmep.GetId() != uint16(cfg.Assocs[0].RemoteMEPID) {
				t.Errorf("remote MEP %v detected, expected: %v", rmep.GetId(), cfg.Assocs[0].RemoteMEPID)
			}
			if rmep.GetOperState() != cfg.Status {
				t.Errorf("remote MEP status got: %v, expected: %v", rmep.GetOperState(), cfg.Status)
			}
			t.Logf("remote mep %v detected, status: %v as expected", cfg.Assocs[0].RemoteMEPID, rmep.GetOperState())
		}
	}
}

func ValidateDeadTimer(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	t.Helper()

	if deviations.CfmOCUnsupported(dut) {
		//TODO: CLI is not there to validate it
	} else {
		expectedDeadTimer := time.Duration(float64(100) * 3.5)

		t.Logf("Verifying dead timer: expected ~%.1fms (3.5 * 100ms)", expectedDeadTimer.Seconds()*1000)
		ma1 := gnmi.Get(t, dut, gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).State())
		if ma1 == nil {
			t.Fatal("Cannot retrieve MA state from DUT")
		}

		if ma1.CcmInterval != 100 {
			t.Errorf("DUT: CCM interval is %v, expected INTERVAL_100MS", ma1.CcmInterval)
		} else {
			t.Log("DUT: CCM interval configured to 100ms")
		}
	}

}

func ValidateAlarmDetection(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	if deviations.CfmStateFt(dut) != "" || !deviations.CfmOCUnsupported(dut) {
		opts := fptest.GetOptsForFunctionalTranslator(t, deviations.CfmStateFt(dut))
		mepPath := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).MepEndpoint(uint16(cfg.Assocs[0].LocalMEPID))
		_, ok := gnmi.Watch(t, dut.GNMIOpts().WithYGNMIOpts(opts...), mepPath.PresentRdi().State(), 30*time.Second, func(v *ygnmi.Value[bool]) bool {
			val, present := v.Val()
			return present && val
		}).Await(t)
		if !ok {
			t.Errorf("defect or fault condition has been detected on %s, expected present-rdi: true, got: false", dut.Name())
		} else {
			t.Logf("Verified present-rdi is true on %s", dut.Name())
		}
		if !deviations.CfmOCUnsupported(dut) {
			localmepState := gnmi.Get(t, dut, mepPath.State())
			if localmepState.GetRdi().GetTransmitOnDefect() {
				t.Log("no defect or fault condition has been detected, as expected RDI state: true")
			} else {
				t.Errorf("defect or fault condition has been detected, expected RDI state: true, got: false")
			}
		}
		return
	}

	cli := ""
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli = fmt.Sprintf(`
			show cfm continuity-check end-point domain %v association %v end-point %v
			`, cfg.DomainName, cfg.MdID, cfg.Assocs[0].LocalMEPID)
		output := helpers.ExecuteShowCLI(t, dut, cli).String()

		re := regexp.MustCompile(`TX RDI state:\s*(true|false)`)
		rdiFlag := re.FindStringSubmatch(output)
		if len(rdiFlag) > 1 {
			rdiStatus := strings.TrimSpace(rdiFlag[1])
			if rdiStatus != "true" {
				t.Errorf("defect or fault condition has been detected, expected RDI state: true, got: %s", rdiStatus)
			} else {
				t.Log("no defect or fault condition has been detected, as expected RDI state: true")
			}
		} else {
			t.Errorf("rdi state not found")
		}
	}
}

func ValidateDelayMeasurement(t *testing.T, dut *ondatra.DUTDevice, cfg MaintenanceDomainConfig) {
	if deviations.CfmPmFt(dut) != "" {
		opts := fptest.GetOptsForFunctionalTranslator(t, deviations.CfmPmFt(dut))
		avgDelayPath := gnmi.OC().Oam().Cfm().
			MaintenanceDomain(cfg.DomainName).
			MaintenanceAssociation(cfg.MdID).
			MepEndpoint(uint16(cfg.Assocs[0].LocalMEPID)).
			PmProfile(cfg.ProfileName).
			DelayMeasurementState().
			FrameDelayTwoWayAverage().State()
		val, ok := gnmi.Watch(t, dut.GNMIOpts().WithYGNMIOpts(opts...), avgDelayPath, 30*time.Second, func(v *ygnmi.Value[uint32]) bool {
			avg, present := v.Val()
			return present && avg > 0
		}).Await(t)
		if !ok {
			t.Errorf("Could not retrieve non-zero frame-delay-two-way-average on %s", dut.Name())
			return
		}
		avg, _ := val.Val()
		t.Logf("Two-way Frame Delay Average (µs) on %s: %d", dut.Name(), avg)
		return
	}

	if deviations.CfmOCUnsupported(dut) {
		cli := ""
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cli = fmt.Sprintf(`
				show cfm measurement delay proactive domain %s association %v end-point %v
				`, cfg.DomainName, cfg.MdID, cfg.Assocs[0].LocalMEPID)
			output := helpers.ExecuteShowCLI(t, dut, cli).String()

			delayMeasurementRe := regexp.MustCompile(`Two-way delay \(usec\)\s+min/max/avg:\s+([\d.]+)/([\d.]+)/([\d.]+)`)
			delayMeasurement := delayMeasurementRe.FindStringSubmatch(output)

			if len(delayMeasurement) > 1 {
				delayMin, _ := strconv.ParseFloat(delayMeasurement[1], 64)
				delayMax, _ := strconv.ParseFloat(delayMeasurement[2], 64)
				delayAvg, _ := strconv.ParseFloat(delayMeasurement[3], 64)

				t.Logf("two-way delay (µs) - Min: %.3f, Max: %.3f, Avg: %.3f\n", delayMin, delayMax, delayAvg)
			} else {
				t.Errorf("delay measurements not found")
			}
		}
	} else {
		pmPath := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg.DomainName).MaintenanceAssociation(cfg.MdID).MepEndpoint(uint16(cfg.Assocs[0].LocalMEPID)).PmProfile(cfg.ProfileName).DelayMeasurementState()
		max := gnmi.Get(t, dut, pmPath.FrameDelayTwoWayMax().State())
		min := gnmi.Get(t, dut, pmPath.FrameDelayTwoWayMin().State())
		avg := gnmi.Get(t, dut, pmPath.FrameDelayTwoWayAverage().State())
		dmmCounters := gnmi.Get(t, dut, pmPath.Counters().DmmReceived().State())

		if max == 0 || min == 0 || avg == 0 || dmmCounters == 0 {
			t.Fatal("Could not retrieve one or more delay measurement values")
		}

		t.Logf("Two-way Frame Delay (ms) - Min: %d, Max: %d, Avg: %d\n, dmm: %d", min, max, avg, dmmCounters)
	}
}

func ValidateLossMeasurement(t *testing.T, dutData []*ondatra.DUTDevice, cfg []MaintenanceDomainConfig) {
	if deviations.CfmPmFt(dutData[0]) != "" {
		for i, dut := range dutData {
			opts := fptest.GetOptsForFunctionalTranslator(t, deviations.CfmPmFt(dut))
			lossStatePath := gnmi.OC().Oam().Cfm().
				MaintenanceDomain(cfg[i].DomainName).
				MaintenanceAssociation(cfg[i].MdID).
				MepEndpoint(uint16(cfg[i].Assocs[0].LocalMEPID)).
				PmProfile(cfg[i].ProfileName).
				LossMeasurementState()

			farVal, ok := gnmi.Watch(t, dut.GNMIOpts().WithYGNMIOpts(opts...), lossStatePath.FarEndAverageFrameLossRatio().State(), 30*time.Second, func(v *ygnmi.Value[uint32]) bool {
				_, present := v.Val()
				return present
			}).Await(t)
			if !ok {
				t.Errorf("Could not retrieve far-end-average-frame-loss-ratio on %s", dut.Name())
			} else {
				farAvg, _ := farVal.Val()
				t.Logf("Far-end average frame loss ratio on %s: %d", dut.Name(), farAvg)
			}

			nearVal, ok := gnmi.Watch(t, dut.GNMIOpts().WithYGNMIOpts(opts...), lossStatePath.NearEndAverageFrameLossRatio().State(), 30*time.Second, func(v *ygnmi.Value[uint32]) bool {
				_, present := v.Val()
				return present
			}).Await(t)
			if !ok {
				t.Errorf("Could not retrieve near-end-average-frame-loss-ratio on %s", dut.Name())
			} else {
				nearAvg, _ := nearVal.Val()
				t.Logf("Near-end average frame loss ratio on %s: %d", dut.Name(), nearAvg)
			}
		}
		return
	}

	var lastMeasurement1, lastMeasurement2 int
	end := time.Now().Add(10 * time.Second)
	if deviations.CfmOCUnsupported(dutData[0]) {
		cli := ""
		switch dutData[0].Vendor() {
		case ondatra.ARISTA:
			var output1, output2 string
			measurementCountRe := regexp.MustCompile(`Number of measurements:\s+(\d+)`)
			for time.Now().Before(end) {
				cli = fmt.Sprintf(`show cfm measurement loss synthetic proactive domain %s association %v end-point %v`, cfg[0].DomainName, cfg[0].MdID, cfg[0].Assocs[0].LocalMEPID)
				output1 := helpers.ExecuteShowCLI(t, dutData[0], cli).String()

				cli = fmt.Sprintf(`show cfm measurement loss synthetic proactive domain %s association %v end-point %v`, cfg[1].DomainName, cfg[1].MdID, cfg[1].Assocs[0].LocalMEPID)
				output2 := helpers.ExecuteShowCLI(t, dutData[1], cli).String()

				measurementCount1 := measurementCountRe.FindStringSubmatch(output1)
				measurementCount2 := measurementCountRe.FindStringSubmatch(output2)
				if len(measurementCount1) > 1 && len(measurementCount2) > 1 {
					measurementVal1, _ := strconv.Atoi(measurementCount1[1])
					measurementVal2, _ := strconv.Atoi(measurementCount2[1])
					if measurementVal1 > lastMeasurement1 {
						lastMeasurement1 = measurementVal1
					} else {
						t.Errorf("DUT1: measurement count is not increasing, previous: %d, new: %d", lastMeasurement1, measurementVal1)
					}
					if measurementVal2 > lastMeasurement2 {
						lastMeasurement2 = measurementVal2
					} else {
						t.Errorf("DUT2: measurement count is not increasing, previous: %d, new: %d", lastMeasurement2, measurementVal2)
					}
				} else {
					t.Errorf("not able to fetch number of measurements")
				}
			}

			farEndRe := regexp.MustCompile(`Far-end frame .*?min/max/avg: (\d+\.\d+)/(\d+\.\d+)/(\d+\.\d+)`)

			for i := range dutData {
				var output string
				switch i {
				case 0:
					output = output1
				case 1:
					output = output2
				}

				farEndMeasurement := farEndRe.FindStringSubmatch(output)
				if len(farEndMeasurement) > 1 {
					farMin, _ := strconv.ParseFloat(farEndMeasurement[1], 64)
					farMax, _ := strconv.ParseFloat(farEndMeasurement[2], 64)
					farAvg, _ := strconv.ParseFloat(farEndMeasurement[3], 64)

					t.Logf("Farend loss ratio - Min: %.3f, Max: %.3f, Avg: %.3f\n", farMin, farMax, farAvg)
				} else {
					t.Errorf("farend loss measurements not found")
				}

				nearEndRe := regexp.MustCompile(`Near-end frame .*?min/max/avg: (\d+\.\d+)/(\d+\.\d+)/(\d+\.\d+)`)
				nearEndMeasurement := nearEndRe.FindStringSubmatch(output)
				if len(nearEndMeasurement) > 1 {
					nearMin, _ := strconv.ParseFloat(nearEndMeasurement[1], 64)
					nearMax, _ := strconv.ParseFloat(nearEndMeasurement[2], 64)
					nearAvg, _ := strconv.ParseFloat(nearEndMeasurement[3], 64)

					t.Logf("Near-end frame ratio - Min: %.3f, Max: %.3f, Avg: %.3f\n", nearMin, nearMax, nearAvg)
				} else {
					t.Errorf("Near-end frame ratio measurements not found")
				}
			}

		}
	} else {
		for time.Now().Before(end) {
			lossState0 := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg[0].DomainName).MaintenanceAssociation(cfg[0].MdID).MepEndpoint(uint16(cfg[0].Assocs[0].LocalMEPID)).PmProfile(cfg[0].ProfileName).LossMeasurementState()
			lossState1 := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg[1].DomainName).MaintenanceAssociation(cfg[1].MdID).MepEndpoint(uint16(cfg[1].Assocs[0].LocalMEPID)).PmProfile(cfg[1].ProfileName).LossMeasurementState()
			slmReceived1 := gnmi.Get(t, dutData[0], lossState0.Counters().SlmReceived().State())
			slmReceived2 := gnmi.Get(t, dutData[1], lossState1.Counters().SlmReceived().State())
			if slmReceived1 > 0 && slmReceived2 > 0 {
				if slmReceived1 > uint64(lastMeasurement1) {
					lastMeasurement1 = int(slmReceived1)
				} else {
					t.Errorf("DUT1: measurement count is not increasing, previous: %d, new: %d", lastMeasurement1, slmReceived1)
				}
				if slmReceived2 > uint64(lastMeasurement2) {
					lastMeasurement2 = int(slmReceived2)
				} else {
					t.Errorf("DUT2: measurement count is not increasing, previous: %d, new: %d", lastMeasurement2, slmReceived2)
				}
			} else {
				t.Errorf("slm measurement not found, got: %d, %d, expected > 1", slmReceived1, slmReceived2)
			}
			for i := range dutData {
				lossState := gnmi.OC().Oam().Cfm().MaintenanceDomain(cfg[i].DomainName).MaintenanceAssociation(cfg[i].MdID).MepEndpoint(uint16(cfg[i].Assocs[0].LocalMEPID)).PmProfile(cfg[i].ProfileName).LossMeasurementState()
				max := gnmi.Get(t, dutData[i], lossState.FarEndMaxFrameLossRatio().State())
				min := gnmi.Get(t, dutData[i], lossState.FarEndMinFrameLossRatio().State())
				avg := gnmi.Get(t, dutData[i], lossState.FarEndAverageFrameLossRatio().State())

				if max == 0 || min == 0 || avg == 0 {
					t.Fatal("Could not retrieve one or more farend loss measurement values")
				}

				t.Logf("Farend loss ratio - Min: %d, Max: %d, Avg: %d\n", min, max, avg)

				max = gnmi.Get(t, dutData[i], lossState.NearEndMaxFrameLossRatio().State())
				min = gnmi.Get(t, dutData[i], lossState.NearEndMinFrameLossRatio().State())
				avg = gnmi.Get(t, dutData[i], lossState.NearEndAverageFrameLossRatio().State())

				if max == 0 || min == 0 || avg == 0 {
					t.Fatal("Could not retrieve one or more near-end loss measurement values")
				}

				t.Logf("near end loss ratio - Min: %d, Max: %d, Avg: %d\n", min, max, avg)
			}
		}
	}
}
