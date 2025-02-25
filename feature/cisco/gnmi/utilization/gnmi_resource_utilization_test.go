package gnmi_resource_utilization_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"

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

func prettyPrintObj(obj interface{}) string {
	return spew.Sprintf("%#v", obj)
}

func stressTestSystem(t testing.TB, dut *ondatra.DUTDevice, resource string) {
	stressTestComponent(t, dut, "", resource)
}

func stressTestComponent(t testing.TB, dut *ondatra.DUTDevice, location string, resource string) {
	t.Helper()
	switch resource {
	case "cpu":
		stressCPU(t, dut, location)
	case "memory":
		stressMem(t, dut, location)
	case "disk0":
		stressDisk0(t, dut, location)
	case "harddisk":
		stressHardDisk(t, dut, location)
	case "power":
		stressPower(t, dut, location)
	default:
		t.Errorf("unknown resource: %s", resource)
	}
}

func stressCPU(t testing.TB, dut *ondatra.DUTDevice, location string) {
	t.Helper()
	cmd := "run /var/xr/scratch/stress --cpu 200 --timeout 10"
	if location != "" {
		cmd = fmt.Sprintf("attach location %s \n ", location) + cmd
	}
	dut.CLI().RunResult(t, cmd)
}

func stressMem(t testing.TB, dut *ondatra.DUTDevice, location string) {
	t.Helper()
	// spawn 100 workers spinning on malloc()/free()
	cmd := "run /var/xr/scratch/stress --vm 100 --timeout 10"
	if location != "" {
		cmd = fmt.Sprintf("attach location %s \n ", location) + cmd
	}
	dut.CLI().RunResult(t, cmd)
}

func stressDisk0(t testing.TB, dut *ondatra.DUTDevice, location string) {
	t.Helper()
	// spawn 100 workers spinning on write()/unlink()
	cmd := "run fallocate -l 100G big_file.iso; sleep 60s; rm big_file.iso"
	if location != "" {
		cmd = fmt.Sprintf("attach location %s \n ", location) + cmd
	}
	dut.CLI().RunResult(t, cmd)
}

func stressHardDisk(t testing.TB, dut *ondatra.DUTDevice, location string) {
	t.Helper()
	// allocate very large file
	cmd := "run cd /harddisk:; fallocate -l 100G big_file.iso; sleep 60s; rm big_file.iso"
	if location != "" {
		cmd = fmt.Sprintf("attach location %s \n ", location) + cmd
	}
	dut.CLI().RunResult(t, cmd)
}

func stressPower(t testing.TB, dut *ondatra.DUTDevice, location string) {
	t.Helper()
	cmd := "./spi_envmon_test -x 6000W 15"
	// set current power consumption to 6000W for 15s
	dut.CLI().RunResult(t, cmd)
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

type targetInfo struct {
	dut     string
	sshIp   string
	sshPort string
	sshUser string
	sshPass string
}

func TestCopyFile(t *testing.T) {
	for _, d := range parseBindingFile(t) {
		dut := ondatra.DUT(t, d.dut)
		copyFileSCP(t, &d, binPath)
		t.Logf("Installing file to %s", dut.ID())
	}
}

func parseBindingFile(t *testing.T) []targetInfo {
	t.Helper()

	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}

	targets := []targetInfo{}
	for _, dut := range b.Duts {

		sshUser := dut.Ssh.Username
		if sshUser == "" {
			sshUser = dut.Options.Username
		}
		if sshUser == "" {
			sshUser = b.Options.Username
		}

		sshPass := dut.Ssh.Password
		if sshPass == "" {
			sshPass = dut.Options.Password
		}
		if sshPass == "" {
			sshPass = b.Options.Password
		}

		sshTarget := strings.Split(dut.Ssh.Target, ":")
		sshIp := sshTarget[0]
		sshPort := "22"
		if len(sshTarget) > 1 {
			sshPort = sshTarget[1]
		}

		targets = append(targets, targetInfo{
			dut:     dut.Id,
			sshIp:   sshIp,
			sshPort: sshPort,
			sshUser: sshUser,
			sshPass: sshPass,
		})
	}

	return targets
}

func copyFileSCP(t testing.TB, d *targetInfo, imagePath string) {
	t.Helper()
	target := fmt.Sprintf("%s:%s", d.sshIp, d.sshPort)
	t.Logf("Copying file to %s (%s) over scp", d.dut, target)
	sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
	scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
	if err != nil {
		t.Fatalf("Error initializing scp client: %v", err)
	}
	defer scpClient.Close()

	ticker := time.NewTicker(1 * time.Minute)
	tickerQuit := make(chan bool)

	go func() {
		for {
			select {
			case <-ticker.C:
				t.Logf("Copying file...")
			case <-tickerQuit:
				return
			}
		}
	}()

	defer func() {
		ticker.Stop()
		tickerQuit <- true
	}()

	if err := scpClient.CopyFileToRemote(imagePath, binDestination, &scp.FileTransferOption{
		Timeout: binCopyTimeout,
	}); err != nil {
		t.Fatalf("Error copying image to target %s (%s:%s): %v", d.dut, d.sshIp, d.sshPort, err)
	}
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
			err:            "",
		},
		{
			name:           "Memory",
			resource:       "memory",
			threshold:      60,
			thresholdClear: 50,
			err:            "",
		},
		{
			name:           "Disk0",
			resource:       "disk0",
			threshold:      60,
			thresholdClear: 50,
			err:            "",
		},
		{
			name:           "HardDisk",
			resource:       "harddisk",
			threshold:      60,
			thresholdClear: 50,
			err:            "",
		},
		{
			name:           "Power",
			resource:       "power",
			threshold:      60,
			thresholdClear: 50,
			err:            "",
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
				// // unconfig CLI
				// dut.CLI().RunResult(t, "config")
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
					rollbackLastPowerConfigCLI(t, dut)
				} else {
					rollbackLastConfig(t, dut)
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

				rollbackLastConfig(t, dut)

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
	t.Skip()
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
			threshold:      25,
			thresholdClear: 25,
			err:            "",
		},
		{
			name:           "Memory",
			resource:       "memory",
			threshold:      30,
			thresholdClear: 30,
			err:            "",
		},
		{
			name:           "Disk0",
			resource:       "disk0",
			threshold:      15,
			thresholdClear: 15,
			err:            "",
		},
		{
			name:           "HardDisk",
			resource:       "harddisk",
			threshold:      15,
			thresholdClear: 15,
			err:            "",
		},
	}

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
				gnmi.OC().Component("0/RP0/CPU0").Linecard().Utilization().Resource(config.GetName()).State(), time.Minute*2, func(v *ygnmi.Value[*oc.Component_Linecard_Utilization_Resource]) bool {
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
				gnmi.OC().Component("0/RP0/CPU0").Linecard().Utilization().Resource(config.GetName()).State(), time.Minute*2, func(v *ygnmi.Value[*oc.Component_Linecard_Utilization_Resource]) bool {
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
		err            string
	}{
		{
			name:           "CPU",
			resource:       "cpu",
			location:       "0/0/CPU0",
			threshold:      30,
			thresholdClear: 25,
			err:            "",
		},
		{
			name:           "Memory",
			resource:       "memory",
			location:       "0/0/CPU0",
			threshold:      30,
			thresholdClear: 25,
			err:            "",
		},
		{
			name:           "Disk0",
			resource:       "disk0",
			location:       "0/0/CPU0",
			threshold:      30,
			thresholdClear: 25,
			err:            "",
		},
		{
			name:           "Power",
			resource:       "power",
			location:       "Rack 0",
			threshold:      30,
			thresholdClear: 25,
			err:            "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/GNMIWithDelete", func(t *testing.T) {

			setResourceUtilizationGNMI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)

			deleteResourceUtilizationGNMI(t, dut, tt.resource, tt.location)

		})

		t.Run(tt.name+"/GNMIWithRollback", func(t *testing.T) {

			setResourceUtilizationGNMI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)

			rollbackLastConfig(t, dut)

		})

		t.Run(tt.name+"/CLIWithUnconfig", func(t *testing.T) {

			setResourceUtilizationGNMI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)

			unconfigResourceUtilizationCLI(t, dut, tt.resource, tt.location)

		})

		t.Run(tt.name+"/CLIWithRollback", func(t *testing.T) {

			setResourceUtilizationGNMI(t, dut, tt.threshold, tt.thresholdClear, tt.resource, tt.location)

			rollbackLastConfig(t, dut)

		})
	}
}

// func TestSetLinecardThresholdCLI(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
//
// 	tests := []struct {
// 		name           string
// 		resource       string
// 		location       string
// 		threshold      uint8
// 		thresholdClear uint8
// 		err            string
// 	}{
// 		{
// 			name:           "CPU",
// 			resource:       "cpu",
// 			location:       "0/0/CPU0",
// 			threshold:      30,
// 			thresholdClear: 25,
// 			err:            "",
// 		},
// 		{
// 			name:           "Memory",
// 			resource:       "memory",
// 			location:       "0/0/CPU0",
// 			threshold:      30,
// 			thresholdClear: 25,
// 			err:            "",
// 		},
// 		{
// 			name:           "Disk0",
// 			resource:       "disk0",
// 			location:       "0/0/CPU0",
// 			threshold:      30,
// 			thresholdClear: 25,
// 			err:            "",
// 		},
// 		{
// 			name:           "Power",
// 			resource:       "power",
// 			location:       "0/0/CPU0",
// 			threshold:      30,
// 			thresholdClear: 25,
// 			err:            "",
// 		},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			d := &oc.Root{}
//
// 			component := d.GetOrCreateComponent(tt.location)
//
// 			resp := gnmi.Replace(t, dut, gnmi.OC().Component(tt.location).Config(), component)
//
// 			t.Logf("Received response: %s", prettyPrintObj(resp))
//
// 			config := component.GetOrCreateLinecard().GetOrCreateUtilization().GetOrCreateResource(tt.resource)
//
// 			config.UsedThresholdUpper = ygot.Uint8(tt.threshold)
// 			config.UsedThresholdUpperClear = ygot.Uint8(tt.thresholdClear)
//
// 			t.Logf("Input config: %+v", prettyPrintObj(config))
//
// 			resp = gnmi.Replace(t, dut, gnmi.OC().Component(tt.location).Linecard().Utilization().Resource(tt.resource).Config(), config)
//
// 			t.Logf("Received response: %s", prettyPrintObj(resp))
//
// 			getResp := gnmi.Get(t, dut, gnmi.OC().Component(tt.location).Linecard().Utilization().Resource(tt.resource).Config())
//
// 			t.Logf("Get response Name: %s", getResp.GetName())
// 			t.Logf("Get response UsedThresholdUpper: %d", getResp.GetUsedThresholdUpper())
// 			t.Logf("Get response UsedThresholdUpperClear: %d", getResp.GetUsedThresholdUpperClear())
//
// 		})
// 	}
//
// }
