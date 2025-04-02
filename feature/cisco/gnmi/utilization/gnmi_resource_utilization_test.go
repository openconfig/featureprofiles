package gnmi_resource_utilization_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/openconfig/featureprofiles/internal/cisco/stress"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/ondatra"
)

const (
	DEFAULT_THRESHOLD             uint8 = 80
	DEFAULT_THRESHOLD_CLEAR       uint8 = 75
	DEFAULT_POWER_THRESHOLD       uint8 = 50
	DEFAULT_POWER_THRESHOLD_CLEAR uint8 = 40
	binPath                             = "/auto/ng_ott_auto/tools/stress/stress"
	binDestination                      = "/disk0:/stress"
	binCopyTimeout                      = 1800 * time.Second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TODO: fix excessive logging of components.FindComponentsByType()
func findComponentsByTypeNoLogs(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	var s []string
	for _, c := range components {
		if c.GetType() == nil {
			continue
		}
		switch v := c.GetType().(type) {
		case oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
			if v == cType {
				s = append(s, c.GetName())
			}
		}
	}
	return s
}

func prettyPrintObj(obj interface{}) string {
	return spew.Sprintf("%#v", obj)
}

func stressTestSystem(t testing.TB, dut *ondatra.DUTDevice, resource string) {
	t.Helper()
	switch resource {
	case "cpu":
		stress.StressCPU(t, dut, 200, time.Second*40)
	case "memory":
		stress.StressMem(t, dut, 100, time.Second*40)
	case "disk0":
		stress.StressDisk0(t, dut, 100, time.Second*60)
	case "harddisk":
		stress.StressHardDisk(t, dut, 100, time.Second*60)
	case "power":
		stress.StressPower(t, dut, 6000, time.Second*15)
	default:
		t.Errorf("unknown resource: %s", resource)
	}
}

func rollbackLastConfig(t testing.TB, dut *ondatra.DUTDevice) error {
	t.Helper()
	cli := dut.RawAPIs().CLI(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := cli.RunCommand(ctx, "rollback configuration last 1")
	if err != nil {
		return err
	}
	if result.Error() != "" {
		return errors.New(result.Error())
	}
	return nil
}

// This needs to be separate from rollbackLastConfigCLI as the power CLI is done in two steps
func rollbackLastPowerConfigCLI(t testing.TB, dut *ondatra.DUTDevice) error {
	t.Helper()
	cli := dut.RawAPIs().CLI(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := cli.RunCommand(ctx, "rollback configuration last 2")
	if err != nil {
		return err
	}
	if result.Error() != "" {
		return errors.New(result.Error())
	}
	return nil
}

func deleteResourceUtilizationGNMI(t testing.TB, dut *ondatra.DUTDevice, resource string, location string) *ygnmi.Result {
	t.Helper()

	if location == "" {
		return gnmi.Delete(t, dut, gnmi.OC().System().Utilization().Resource(resource).Config())
	}

	if resource == "power" {
		return gnmi.Delete(t, dut, gnmi.OC().Component(location).Chassis().Utilization().Config())
	} else {
		return gnmi.Delete(t, dut, gnmi.OC().Component(location).Linecard().Utilization().Config())
	}
}

func unconfigResourceUtilizationCLI(t testing.TB, dut *ondatra.DUTDevice, resource string, location string) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if resource == "" {
		return errors.New("resource not provided")
	}

	if resource == "power" {

		powerCmd := "no power-mgmt used-threshold-upper"
		powerCmdClear := "no power-mgmt used-threshold-upper-clear"

		if location != "" {
			powerCmd = powerCmd + fmt.Sprintf(" location %s", location)
			powerCmdClear = powerCmdClear + fmt.Sprintf(" location %s", location)
		}

		util.GNMIWithText(ctx, t, dut, powerCmd)
		util.GNMIWithText(ctx, t, dut, powerCmdClear)

	} else {

		cmd := fmt.Sprintf("no watchdog resource-utilization %s", resource)

		if location != "" {
			cmd = cmd + fmt.Sprintf(" location %s", location)
		}

		util.GNMIWithText(ctx, t, dut, cmd)
	}

	return nil
}

func setResourceUtilizationGNMI(t testing.TB, dut *ondatra.DUTDevice, threshold uint8, thresholdClear uint8, resource string, location string) (*ygnmi.Result, error) {
	t.Helper()

	if resource == "" {
		return nil, errors.New("resource not provided")
	}

	if location == "" {
		d := &oc.Root{}

		config := d.GetOrCreateSystem().GetOrCreateUtilization().GetOrCreateResource(resource)

		config.Name = ygot.String(resource)
		config.UsedThresholdUpper = ygot.Uint8(threshold)
		config.UsedThresholdUpperClear = ygot.Uint8(thresholdClear)

		t.Logf("Attempting to apply config to %s: \n\tUsedThresholdUpper: %d\n\tUsedThresholdUpperClear: %d", config.GetName(), config.GetUsedThresholdUpper(), config.GetUsedThresholdUpperClear())

		resp := gnmi.Update(t, dut, gnmi.OC().System().Utilization().Resource(config.GetName()).Config(), config)

		return resp, nil

	}

	if resource == "power" {

		d := &oc.Root{}

		component := d.GetOrCreateComponent(location)

		resp := gnmi.Replace(t, dut, gnmi.OC().Component(location).Config(), component)

		t.Logf("Received response: %s", prettyPrintObj(resp))

		config := component.GetOrCreateChassis().GetOrCreateUtilization().GetOrCreateResource(resource)

		config.UsedThresholdUpper = ygot.Uint8(threshold)
		config.UsedThresholdUpperClear = ygot.Uint8(thresholdClear)

		t.Logf("Input config: %+v", prettyPrintObj(config))

		resp = gnmi.Replace(t, dut, gnmi.OC().Component(location).Chassis().Utilization().Resource(resource).Config(), config)

		return resp, nil

	} else {

		d := &oc.Root{}

		component := d.GetOrCreateComponent(location)

		resp := gnmi.Replace(t, dut, gnmi.OC().Component(location).Config(), component)

		t.Logf("Received response: %s", prettyPrintObj(resp))

		config := component.GetOrCreateLinecard().GetOrCreateUtilization().GetOrCreateResource(resource)

		config.UsedThresholdUpper = ygot.Uint8(threshold)
		config.UsedThresholdUpperClear = ygot.Uint8(thresholdClear)

		t.Logf("Input config: %+v", prettyPrintObj(config))

		resp = gnmi.Replace(t, dut, gnmi.OC().Component(location).Linecard().Utilization().Resource(resource).Config(), config)

		return resp, nil

	}

}

func setResourceUtilizationCLI(t testing.TB, dut *ondatra.DUTDevice, threshold uint8, thresholdClear uint8, resource string, location string) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if resource == "" {
		return errors.New("resource not provided")
	}

	if resource == "power" {

		if location != "" {
			location = "all"
		}

		powerCmd := fmt.Sprintf("power-mgmt used-threshold-upper %d location %s", threshold, location)
		powerCmdClear := fmt.Sprintf("power-mgmt used-threshold-upper-clear %d location %s", thresholdClear, location)

		util.GNMIWithText(ctx, t, dut, powerCmd)
		util.GNMIWithText(ctx, t, dut, powerCmdClear)

	} else {

		cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", resource, threshold, thresholdClear)

		if location != "" {
			cmd = cmd + fmt.Sprintf(" location %s", location)
		}

		util.GNMIWithText(ctx, t, dut, cmd)
	}

	return nil
}

func TestSetSystemThreshold(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	tests := []struct {
		name           string
		resource       string
		threshold      uint8
		thresholdClear uint8
		err            string
	}{
		{
			name:           "CPU",
			resource:       "cpu",
			threshold:      60,
			thresholdClear: 50,
		},
		{
			name:           "Memory",
			resource:       "memory",
			threshold:      60,
			thresholdClear: 50,
		},
		{
			name:           "Disk0",
			resource:       "disk0",
			threshold:      60,
			thresholdClear: 50,
		},
		{
			name:           "HardDisk",
			resource:       "harddisk",
			threshold:      60,
			thresholdClear: 50,
		},
		{
			name:           "Power",
			resource:       "power",
			threshold:      60,
			thresholdClear: 50,
		},
	}

	for _, tt := range tests {
		t.Run("CLI/"+tt.name, func(t *testing.T) {

			cli := dut.RawAPIs().CLI(t)

			cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
			powerCmd := fmt.Sprintf("power-mgmt used-threshold-upper %d location all", tt.threshold)
			powerCmdClear := fmt.Sprintf("power-mgmt used-threshold-upper-clear %d location all", tt.thresholdClear)

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			if tt.resource != "power" {
				util.GNMIWithText(ctx, t, dut, cmd)
			} else {
				util.GNMIWithText(ctx, t, dut, powerCmd)
				util.GNMIWithText(ctx, t, dut, powerCmdClear)
			}
			t.Logf("Configured %s via CLI", tt.resource)

			t.Run("VerifyCLI", func(t *testing.T) {
				t.Log("Verifying via CLI")

				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()

				if tt.resource != "power" {
					result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", cmd))
					t.Logf("output: %s", result.Output())

					if !strings.Contains(result.Output(), cmd) {
						t.Error("error verifying config: configuration not found")
					}
				} else {
					result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmd))
					t.Logf("output: %s", result.Output())

					if !strings.Contains(result.Output(), powerCmd) {
						t.Error("error verifying config: configuration not found")
					}

					result2, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmdClear))
					t.Logf("output: %s", result2.Output())

					if !strings.Contains(result2.Output(), powerCmdClear) {
						t.Error("error verifying config: configuration not found")
					}

				}
			})

			t.Run("VerifyGNMI", func(t *testing.T) {

				t.Log("Verifying via GNMI")

				resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
				t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d\nUsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

				if resp.GetUsedThresholdUpper() != tt.threshold || resp.GetUsedThresholdUpperClear() != tt.thresholdClear {
					t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
						resp.GetName(), tt.resource,
						resp.GetUsedThresholdUpper(), tt.threshold,
						resp.GetUsedThresholdUpperClear(), tt.thresholdClear,
						resp,
					)
				}
			})

			t.Run("UnconfigCLI", func(t *testing.T) {
				unCfgCmd := fmt.Sprintf("no watchdog resource-utilization %s", tt.resource)
				unCfgPowerCmd := "no power-mgmt used-threshold-upper"
				unCfgPowerCmdClear := "no power-mgmt used-threshold-upper-clear"

				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				defer cancel()

				if tt.resource != "power" {
					util.GNMIWithText(ctx, t, dut, unCfgCmd)
				} else {
					util.GNMIWithText(ctx, t, dut, unCfgPowerCmd)
					util.GNMIWithText(ctx, t, dut, unCfgPowerCmdClear)
				}
				t.Log("Finished running unconfig")

				t.Run("VerifyCLI", func(t *testing.T) {
					t.Log("Verifying via CLI")

					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()

					if tt.resource != "power" {
						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", cmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), cmd) {
							t.Error("error verifying config: configuration not unapplied")
						}
					} else {
						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), powerCmd) {
							t.Error("error verifying config: configuration not unapplied")
						}
						resultClear, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmdClear))
						t.Logf("output: %s", resultClear.Output())

						if strings.Contains(result.Output(), powerCmdClear) {
							t.Error("error verifying config: configuration not unapplied")
						}
					}
				})

				t.Run("VerifyGNMI", func(t *testing.T) {

					t.Log("Verifying via GNMI")

					resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
					t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d\nUsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

					if tt.resource != "power" {
						if resp.GetUsedThresholdUpper() != DEFAULT_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					} else {
						if resp.GetUsedThresholdUpper() != DEFAULT_POWER_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_POWER_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					}
				})

			})

			t.Run("Rollback/"+tt.name, func(t *testing.T) {

				cli := dut.RawAPIs().CLI(t)

				cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
				powerCmd := fmt.Sprintf("power-mgmt used-threshold-upper %d location all", tt.threshold)
				powerCmdClear := fmt.Sprintf("power-mgmt used-threshold-upper-clear %d location all", tt.thresholdClear)

				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()

				if tt.resource != "power" {
					util.GNMIWithText(ctx, t, dut, cmd)
				} else {
					util.GNMIWithText(ctx, t, dut, powerCmd)
					util.GNMIWithText(ctx, t, dut, powerCmdClear)
				}
				t.Logf("Configured %s via CLI", tt.resource)

				if tt.resource == "power" {
					err := rollbackLastPowerConfigCLI(t, dut)
					if err != nil {
						t.Errorf("config rollback failed: %s", err)
					}
				} else {
					err := rollbackLastConfig(t, dut)
					if err != nil {
						t.Errorf("config rollback failed: %s", err)
					}
				}

				t.Run("VerifyCLI", func(t *testing.T) {
					t.Log("Verifying via CLI")

					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()

					if tt.resource != "power" {
						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", cmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), cmd) {
							t.Error("error verifying config: configuration not unapplied")
						}
					} else {
						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), powerCmd) {
							t.Error("error verifying config: configuration not unapplied")
						}
						resultClear, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmdClear))
						t.Logf("output: %s", resultClear.Output())

						if strings.Contains(result.Output(), powerCmdClear) {
							t.Error("error verifying config: configuration not unapplied")
						}
					}
				})

				t.Run("VerifyGNMI", func(t *testing.T) {

					t.Log("Verifying via GNMI")

					resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
					t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d\nUsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

					if tt.resource != "power" {
						if resp.GetUsedThresholdUpper() != DEFAULT_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					} else {
						if resp.GetUsedThresholdUpper() != DEFAULT_POWER_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_POWER_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					}
				})
			})

		})

	}

	for _, tt := range tests {
		t.Run("GNMI/"+tt.name, func(t *testing.T) {

			d := &oc.Root{}

			config := d.GetOrCreateSystem().GetOrCreateUtilization().GetOrCreateResource(tt.resource)

			config.Name = ygot.String(tt.resource)
			config.UsedThresholdUpper = ygot.Uint8(tt.threshold)
			config.UsedThresholdUpperClear = ygot.Uint8(tt.thresholdClear)

			t.Logf("Attempting to apply config to %s: \n\tUsedThresholdUpper: %d\n\tUsedThresholdUpperClear: %d", config.GetName(), config.GetUsedThresholdUpper(), config.GetUsedThresholdUpperClear())

			resp := gnmi.Update(t, dut, gnmi.OC().System().Utilization().Resource(config.GetName()).Config(), config)

			t.Logf("response: %+v", resp)

			t.Run("VerifyCLI", func(t *testing.T) {
				t.Log("Verifying via CLI")

				cli := dut.RawAPIs().CLI(t)
				cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
				powerCmd := fmt.Sprintf("power-mgmt used-threshold-upper %d", tt.threshold)
				powerCmdClear := fmt.Sprintf("power-mgmt used-threshold-upper-clear %d", tt.thresholdClear)

				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()

				if tt.resource != "power" {
					result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", cmd))
					t.Logf("output: %s", result.Output())

					if !strings.Contains(result.Output(), cmd) {
						t.Error("error verifying config: configuration not found")
					}
				} else {
					result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmd))
					t.Logf("output: %s", result.Output())

					if !strings.Contains(result.Output(), powerCmd) {
						t.Error("error verifying config: configuration not found")
					}

					result2, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmdClear))
					t.Logf("output: %s", result2.Output())

					if !strings.Contains(result2.Output(), powerCmdClear) {
						t.Error("error verifying config: configuration not found")
					}

				}
			})

			t.Run("VerifyGNMI", func(t *testing.T) {

				t.Log("Verifying via GNMI")

				resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
				t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d\nUsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

				if resp.GetUsedThresholdUpper() != tt.threshold || resp.GetUsedThresholdUpperClear() != tt.thresholdClear {
					t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
						resp.GetName(), tt.resource,
						resp.GetUsedThresholdUpper(), tt.threshold,
						resp.GetUsedThresholdUpperClear(), tt.thresholdClear,
						resp,
					)
				}
			})

			t.Run("UnconfigGNMI", func(t *testing.T) {

				gnmi.Delete(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).Config())

				t.Run("VerifyCLI", func(t *testing.T) {
					t.Log("Verifying via CLI")

					cli := dut.RawAPIs().CLI(t)
					cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
					powerCmd := fmt.Sprintf("power-mgmt used-threshold-upper %d", tt.threshold)
					powerCmdClear := fmt.Sprintf("power-mgmt used-threshold-upper-clear %d", tt.thresholdClear)

					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()

					if tt.resource != "power" {
						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", cmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), cmd) {
							t.Error("error verifying config: configuration not unapplied")
						}
					} else {
						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), powerCmd) {
							t.Error("error verifying config: configuration not unapplied")
						}
						resultClear, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmdClear))
						t.Logf("output: %s", resultClear.Output())

						if strings.Contains(result.Output(), powerCmdClear) {
							t.Error("error verifying config: configuration not unapplied")
						}
					}
				})

				t.Run("VerifyGNMI", func(t *testing.T) {

					t.Log("Verifying via GNMI")

					resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
					t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d\nUsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

					if tt.resource != "power" {
						if resp.GetUsedThresholdUpper() != DEFAULT_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					} else {
						if resp.GetUsedThresholdUpper() != DEFAULT_POWER_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_POWER_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					}
				})

			})

			t.Run("Rollback/"+tt.name, func(t *testing.T) {
				d := &oc.Root{}

				config := d.GetOrCreateSystem().GetOrCreateUtilization().GetOrCreateResource(tt.resource)

				config.Name = ygot.String(tt.resource)
				config.UsedThresholdUpper = ygot.Uint8(tt.threshold)
				config.UsedThresholdUpperClear = ygot.Uint8(tt.thresholdClear)

				t.Logf("Attempting to apply config to %s: \n\tUsedThresholdUpper: %d\n\tUsedThresholdUpperClear: %d", config.GetName(), config.GetUsedThresholdUpper(), config.GetUsedThresholdUpperClear())

				resp := gnmi.Update(t, dut, gnmi.OC().System().Utilization().Resource(config.GetName()).Config(), config)

				t.Logf("response: %+v", resp)

				err := rollbackLastConfig(t, dut)
				if err != nil {
					t.Errorf("config rollback failed: %s", err)
				}

				t.Run("VerifyCLI", func(t *testing.T) {

					cli := dut.RawAPIs().CLI(t)

					cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
					powerCmd := fmt.Sprintf("power-mgmt used-threshold-upper %d", tt.threshold)
					powerCmdClear := fmt.Sprintf("power-mgmt used-threshold-upper-clear %d", tt.thresholdClear)

					t.Log("Verifying via CLI")

					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()

					if tt.resource != "power" {
						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", cmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), cmd) {
							t.Error("error verifying config: configuration not unapplied")
						}
					} else {
						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), powerCmd) {
							t.Error("error verifying config: configuration not unapplied")
						}
						resultClear, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmdClear))
						t.Logf("output: %s", resultClear.Output())

						if strings.Contains(result.Output(), powerCmdClear) {
							t.Error("error verifying config: configuration not unapplied")
						}
					}
				})

				t.Run("VerifyGNMI", func(t *testing.T) {

					t.Log("Verifying via GNMI")

					resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
					t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d\nUsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

					if tt.resource != "power" {
						if resp.GetUsedThresholdUpper() != DEFAULT_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					} else {
						if resp.GetUsedThresholdUpper() != DEFAULT_POWER_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_POWER_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					}
				})
			})
		})
	}

	t.Run("GNMISimultaneous", func(t *testing.T) {

		d := &oc.Root{}
		config := d.GetOrCreateSystem().GetOrCreateUtilization()

		for _, tt := range tests {
			config.GetOrCreateResource(tt.resource).Name = ygot.String(tt.resource)
			config.GetOrCreateResource(tt.resource).UsedThresholdUpper = ygot.Uint8(tt.threshold)
			config.GetOrCreateResource(tt.resource).UsedThresholdUpperClear = ygot.Uint8(tt.thresholdClear)
		}

		t.Logf("Attempting to apply configs at once: %+v", config)

		resp := gnmi.Update(t, dut, gnmi.OC().System().Utilization().Config(), config)

		t.Logf("response: %+v", resp)

		t.Run("VerifyCLI", func(t *testing.T) {
			t.Log("Verifying via CLI")

			cli := dut.RawAPIs().CLI(t)

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			for _, tt := range tests {
				if tt.resource != "power" {
					cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)

					result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", cmd))
					t.Logf("output: %s", result.Output())

					if !strings.Contains(result.Output(), cmd) {
						t.Error("error verifying config: configuration not found")
					}
				} else {
					powerCmd := fmt.Sprintf("power-mgmt used-threshold-upper %d location all", tt.threshold)
					powerCmdClear := fmt.Sprintf("power-mgmt used-threshold-upper-clear %d location all", tt.thresholdClear)

					result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmd))
					t.Logf("output: %s", result.Output())

					if !strings.Contains(result.Output(), powerCmd) {
						t.Error("error verifying config: configuration not found")
					}

					resultClear, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmdClear))
					t.Logf("output: %s", resultClear.Output())

					if !strings.Contains(resultClear.Output(), powerCmdClear) {
						t.Error("error verifying config: configuration not found")
					}
				}
			}

		})

		t.Run("VerifyGNMI", func(t *testing.T) {

			t.Log("Verifying via GNMI")

			for _, tt := range tests {
				resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
				t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d\nUsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

				if resp.GetUsedThresholdUpper() != tt.threshold || resp.GetUsedThresholdUpperClear() != tt.thresholdClear {
					t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
						resp.GetName(), tt.resource,
						resp.GetUsedThresholdUpper(), tt.threshold,
						resp.GetUsedThresholdUpperClear(), tt.thresholdClear,
						resp,
					)
				}
			}

		})

		t.Run("UnconfigGNMI", func(t *testing.T) {

			gnmi.Delete(t, dut, gnmi.OC().System().Utilization().Config())

			t.Run("VerifyCLI", func(t *testing.T) {
				t.Log("Verifying via CLI")

				cli := dut.RawAPIs().CLI(t)

				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()

				for _, tt := range tests {
					if tt.resource != "power" {
						cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", cmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), cmd) {
							t.Error("error verifying config: configuration not found")
						}
					} else {
						powerCmd := fmt.Sprintf("power-mgmt used-threshold-upper %d", tt.threshold)
						powerCmdClear := fmt.Sprintf("power-mgmt used-threshold-upper-clear %d", tt.thresholdClear)

						result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmd))
						t.Logf("output: %s", result.Output())

						if strings.Contains(result.Output(), powerCmd) {
							t.Error("error verifying config: configuration not found")
						}

						resultClear, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | include %s", powerCmdClear))
						t.Logf("output: %s", resultClear.Output())

						if strings.Contains(result.Output(), powerCmdClear) {
							t.Error("error verifying config: configuration not found")
						}

					}
				}

			})

			t.Run("VerifyGNMI", func(t *testing.T) {

				t.Log("Verifying via GNMI")

				for _, tt := range tests {
					resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
					t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d\nUsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

					if tt.resource != "power" {
						if resp.GetUsedThresholdUpper() != DEFAULT_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					} else {
						if resp.GetUsedThresholdUpper() != DEFAULT_POWER_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_POWER_THRESHOLD_CLEAR {
							t.Errorf("error verifying config:\nName: %s Expected: %s\nUsedThresholdUpper: %d Expected: %d\nUsedThresholdUpperClear: %d Expected: %d\n raw: %+v",
								resp.GetName(), tt.resource,
								resp.GetUsedThresholdUpper(), DEFAULT_THRESHOLD,
								resp.GetUsedThresholdUpperClear(), DEFAULT_THRESHOLD_CLEAR,
								resp,
							)
						}
					}
				}

			})
		})
	})
}

func TestReceiveSystemThresholdNotification(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	tests := []struct {
		name           string
		resource       string
		threshold      uint8
		thresholdClear uint8
		err            string
	}{
		{
			name:           "CPU",
			resource:       "cpu",
			threshold:      30,
			thresholdClear: 30,
		},
		{
			name:           "Memory",
			resource:       "memory",
			threshold:      40,
			thresholdClear: 40,
		},
		{
			name:           "Disk0",
			resource:       "disk0",
			threshold:      15,
			thresholdClear: 15,
		},
		{
			name:           "HardDisk",
			resource:       "harddisk",
			threshold:      15,
			thresholdClear: 15,
		},
	}

	rps := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	_, rpActive := components.FindStandbyRP(t, dut, rps)

	for _, tt := range tests {
		t.Run("GNMI/"+tt.name, func(t *testing.T) {
			d := &oc.Root{}

			config := d.GetOrCreateSystem().GetOrCreateUtilization().GetOrCreateResource(tt.resource)

			config.UsedThresholdUpper = ygot.Uint8(tt.threshold)
			config.UsedThresholdUpperClear = ygot.Uint8(tt.thresholdClear)

			t.Logf("Attempting to apply config to %s: \n\tUsedThresholdUpper: %d\n\tUsedThresholdUpperClear: %d", config.GetName(), config.GetUsedThresholdUpper(), config.GetUsedThresholdUpperClear())

			resp := gnmi.Update(t, dut, gnmi.OC().System().Utilization().Resource(config.GetName()).Config(), config)

			t.Logf("response: %+v", resp)
			t.Logf("config: %+v", config)

			defer gnmi.Delete(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).Config())

			watcher := gnmi.Watch(t,
				dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
				gnmi.OC().Component(rpActive).Linecard().Utilization().Resource(config.GetName()).State(), time.Minute*2, func(v *ygnmi.Value[*oc.Component_Linecard_Utilization_Resource]) bool {
					val, _ := v.Val()
					t.Logf("Received notification:\n%s\n", util.PrettyPrintJson(val))
					return val.GetUsedThresholdUpperExceeded()
				})

			t.Logf("Stressing system %s", tt.resource)

			stressTestSystem(t, dut, tt.resource)

			v, passed := watcher.Await(t)
			val, _ := v.Val()
			t.Logf("Received notification:\n%s\n", util.PrettyPrintJson(val))

			if !passed {
				t.Fatalf("threshold was not exceeded")
			}

			t.Log("Threshold exceeded, waiting for threshold clear")

			watcher2 := gnmi.Watch(t,
				dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
				gnmi.OC().Component(rpActive).Linecard().Utilization().Resource(config.GetName()).State(), time.Minute*2, func(v *ygnmi.Value[*oc.Component_Linecard_Utilization_Resource]) bool {
					val, _ := v.Val()
					t.Logf("Received notification:\n%s\n", util.PrettyPrintJson(val))
					return !val.GetUsedThresholdUpperExceeded()
				})

			v2, passed2 := watcher2.Await(t)
			val2, _ := v2.Val()
			t.Logf("Received notification:\n%s\n", util.PrettyPrintJson(val2))

			if !passed2 {
				t.Fatalf("threshold was not cleared after being exceeded")
			}
			t.Log("Threshold cleared")

			time.Sleep(time.Second * 5)

		})
	}

}

func TestSetInvalid(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	tests := []struct {
		name           string
		resource       string
		location       string
		threshold      uint8
		thresholdClear uint8
	}{
		{
			name:           "InvalidSystemCPUThreshold",
			resource:       "cpu",
			location:       "",
			threshold:      30,
			thresholdClear: 40,
		},
		{
			name:           "InvalidSystemMemoryThreshold",
			resource:       "memory",
			location:       "",
			threshold:      30,
			thresholdClear: 40,
		},
		{
			name:           "InvalidSystemDisk0Threshold",
			resource:       "disk0",
			location:       "",
			threshold:      30,
			thresholdClear: 40,
		},
		{
			name:           "InvalidSystemHardDiskThreshold",
			resource:       "harddisk",
			location:       "",
			threshold:      30,
			thresholdClear: 40,
		},
		{
			name:           "InvalidSystemPowerThreshold",
			resource:       "power",
			location:       "",
			threshold:      30,
			thresholdClear: 40,
		},
		{
			name:           "InvalidLinecardCPUThreshold",
			resource:       "cpu",
			location:       "0/0/CPU0",
			threshold:      50,
			thresholdClear: 70,
		},
		{
			name:           "InvalidLinecardMemoryThreshold",
			resource:       "memory",
			location:       "0/0/CPU0",
			threshold:      50,
			thresholdClear: 70,
		},
		{
			name:           "InvalidLinecardDisk0Threshold",
			resource:       "disk0",
			location:       "0/0/CPU0",
			threshold:      50,
			thresholdClear: 70,
		},
		{
			name:           "InvalidChassisPowerThreshold",
			resource:       "power",
			location:       "Rack 0",
			threshold:      50,
			thresholdClear: 70,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name+"/GNMI", func(t *testing.T) {
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				setResourceUtilizationGNMI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)
			}); errMsg != nil {
				t.Logf("received expected error: %s", *errMsg)
			} else {
				t.Fatalf("did not receive expected error")
			}

		})

		t.Run(tt.name+"/CLI", func(t *testing.T) {
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				setResourceUtilizationCLI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)
			}); errMsg != nil {
				t.Logf("received error: %s", *errMsg)
			} else {
				t.Fatalf("did not receive expected error")
			}
		})
	}
}

func TestSetComponentThreshold(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	tests := []struct {
		name           string
		resource       string
		location       string
		threshold      uint8
		thresholdClear uint8
	}{
		{
			name:           "CPU",
			resource:       "cpu",
			location:       "0/0/CPU0",
			threshold:      30,
			thresholdClear: 25,
		},
		{
			name:           "Memory",
			resource:       "memory",
			location:       "0/0/CPU0",
			threshold:      30,
			thresholdClear: 25,
		},
		{
			name:           "Disk0",
			resource:       "disk0",
			location:       "0/0/CPU0",
			threshold:      30,
			thresholdClear: 25,
		},
		{
			name:           "Power",
			resource:       "power",
			location:       "Rack 0",
			threshold:      30,
			thresholdClear: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/GNMIWithDelete", func(t *testing.T) {

			setResourceUtilizationGNMI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)

			deleteResourceUtilizationGNMI(t, dut, tt.resource, tt.location)

		})

		t.Run(tt.name+"/GNMIWithRollback", func(t *testing.T) {

			setResourceUtilizationGNMI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)

			err := rollbackLastConfig(t, dut)
			if err != nil {
				t.Errorf("config rollback failed: %s", err)
			}

		})

		t.Run(tt.name+"/CLIWithUnconfig", func(t *testing.T) {

			setResourceUtilizationGNMI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)

			unconfigResourceUtilizationCLI(t, dut, tt.resource, tt.location)

		})

		t.Run(tt.name+"/CLIWithRollback", func(t *testing.T) {

			setResourceUtilizationGNMI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)

			err := rollbackLastConfig(t, dut)
			if err != nil {
				t.Errorf("config rollback failed: %s", err)
			}

		})
	}
}
