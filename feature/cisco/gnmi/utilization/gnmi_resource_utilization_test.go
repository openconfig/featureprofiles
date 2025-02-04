package gnmi_resource_utilization_test

import (
	"context"
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
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/openconfig/ondatra"
)

const (
	DEFAULT_THRESHOLD uint8 = 80
	DEFAULT_THRESHOLD_CLEAR uint8 = 75 
	binPath = "/auto/ng_ott_auto/tools/stress/stress"
	binDestination = "/disk0:/stress"
	binCopyTimeout   = 1800 * time.Second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func prettyPrintObj(obj interface{}) string {
	return spew.Sprintf("%#v", obj)
}

func stressTestSystem(t testing.TB, dut *ondatra.DUTDevice, location string, resource string) {
	t.Helper()
	switch resource {
	case "cpu":
		stressCPU(t, dut)
	case "memory":
		stressMem(t, dut)
	case "disk0":
		stressDisk0(t, dut)
	case "harddisk":
		stressHardDisk(t, dut)
	case "power":
		stressPower(t, dut)
	default:
		t.Errorf("unknown resource: %s", resource)
	}
}

func stressCPU(t testing.TB, dut *ondatra.DUTDevice){
	t.Helper()
	// spawn 100 workers spinning on sqrt()
	dut.CLI().RunResult(t, "run /var/xr/scratch/stress --cpu 300 --timeout 15")
}
func stressMem(t testing.TB, dut *ondatra.DUTDevice){
	t.Helper()
	// spawn 100 workers spinning on malloc()/free()
	dut.CLI().RunResult(t, "run /var/xr/scratch/stress --vm 300 --timeout 15")
}
func stressDisk0(t testing.TB, dut *ondatra.DUTDevice){
	t.Helper()
	// spawn 100 workers spinning on write()/unlink()
	dut.CLI().RunResult(t, "run /var/xr/scratch/stress --hdd 500 --timeout 15")
}
func stressHardDisk(t testing.TB, dut *ondatra.DUTDevice){
	t.Helper()
	// spawn 100 workers spinning on write()/unlink()
	dut.CLI().RunResult(t, "cd /harddisk:; run /var/xr/scratch/stress --hdd 500 --timeout 15")
}
func stressPower(t testing.TB, dut *ondatra.DUTDevice){
	t.Helper()
	// set current power consumption to 6000W for 15s
	dut.CLI().RunResult(t, "./spi_envmon_test -x 6000W 15")
}

func stressTestLinecard(location string, resource string) {
	
}

func rollbackLastConfig(t testing.TB, dut *ondatra.DUTDevice) {
	t.Helper()
	cli := dut.RawAPIs().CLI(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, _ := cli.RunCommand(ctx, "rollback configuration last 1")
	if result.Error() != "" {
		t.Logf("Error rolling back config: %s", result.Error())
	}
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

// func TestSetSystemThresholdCLI(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	
// 	tests := []struct {
// 		name    string
// 		resource string
// 		threshold uint8
// 		thresholdClear uint8
// 		err string
// 	}{
// 		{
// 			name:    "CPU",
// 			resource: "cpu",
// 			threshold: 60,
// 			thresholdClear: 50,
// 			err:   "",
// 		},
// 		{
// 			name:    "Memory",
// 			resource: "memory",
// 			threshold: 60,
// 			thresholdClear: 50,
// 			err:   "",
// 		},
// 		{
// 			name:    "Disk0",
// 			resource: "disk0",
// 			threshold: 60,
// 			thresholdClear: 50,
// 			err:   "",
// 		},
// 		{
// 			name:    "HardDisk`",
// 			resource: "harddisk",
// 			threshold: 60,
// 			thresholdClear: 50,
// 			err:   "",
// 		},
// 	}
//
// 	
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			
// 			cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
//
// 			dut.CLI().RunResult(t, "config")
// 			res := dut.CLI().RunResult(t, cmd)
// 			dut.CLI().RunResult(t, "commit")
// 			dut.CLI().RunResult(t, "end")
// 			
// 			// cli := dut.RawAPIs().CLI(t)
// 			
// 			// _, err := cli.RunCommand(context.Background(), "config")
// 			// resp, err := cli.RunCommand(context.Background(), cmd)
// 			// _, err = cli.RunCommand(context.Background(), "commit")
// 			// _, err = cli.RunCommand(context.Background(), "end")
// 			
// 			if res.Error() != "" {
// 				t.Errorf("Error: %s", res.Error())
// 			}
//
// 			t.Logf("response: %+v", res.Output())
// 		})
// 	}
// 	
// }

// func TestSetSystemThresholdGNMI(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	
// 	tests := []struct {
// 		name    string
// 		resource string
// 		threshold uint8
// 		thresholdClear uint8
// 		err string
// 	}{
// 		{
// 			name:    "CPU",
// 			resource: "cpu",
// 			threshold: 20,
// 			thresholdClear: 15,
// 			err:   "",
// 		},
// 		{
// 			name:    "Memory",
// 			resource: "memory",
// 			threshold: 20,
// 			thresholdClear: 15,
// 			err:   "",
// 		},
// 		{
// 			name:    "Disk0",
// 			resource: "disk0",
// 			threshold: 20,
// 			thresholdClear: 15,
// 			err:   "",
// 		},
// 		{
// 			name:    "HardDisk`",
// 			resource: "harddisk",
// 			threshold: 20,
// 			thresholdClear: 15,
// 			err:   "",
// 		},
// 	}
//
// 	
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			d := &oc.Root{}
//
// 			config := d.GetOrCreateSystem().GetOrCreateUtilization().GetOrCreateResource(tt.resource)
//
// 			config.UsedThresholdUpper = ygot.Uint8(tt.threshold)
// 			config.UsedThresholdUpperClear = ygot.Uint8(tt.thresholdClear)
//
// 			resp := gnmi.Update(t, dut, gnmi.OC().System().Utilization().Resource(config.GetName()).Config(), config)
//
// 			t.Logf("response: %+v", resp)
// 			t.Logf("config: %+v", config)
// 		})
// 	}
// 	
// }

func TestSetSystemThreshold(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	tests := []struct {
		name    string
		resource string
		threshold uint8
		thresholdClear uint8
		err string
	}{
		{
			name:    "Disk0",
			resource: "disk0",
			threshold: 60,
			thresholdClear: 50,
			err:   "",
		},
		{
			name:    "HardDisk",
			resource: "harddisk",
			threshold: 60,
			thresholdClear: 50,
			err:   "",
		},
		{
			name:    "CPU",
			resource: "cpu",
			threshold: 60,
			thresholdClear: 50,
			err:   "",
		},
		{
			name:    "Memory",
			resource: "memory",
			threshold: 60,
			thresholdClear: 50,
			err:   "",
		},
	}

	
	for _, tt := range tests {
		t.Run("CLI/" + tt.name, func(t *testing.T) {
			
			cli := dut.RawAPIs().CLI(t)
			
			cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
			
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			
			util.GNMIWithText(ctx, t, dut, cmd)
			t.Logf("Configured %s via CLI", tt.resource)
			
			t.Run("VerifyCLI", func(t *testing.T) {
				t.Log("Verifying via CLI")
				
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				
				result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | begin %s", cmd))
				t.Logf("output: %s", result.Output())
				
				if !strings.Contains(result.Output(), cmd) {
					t.Errorf("error verifying config: %s", result.Error())
				}
			})
			
			t.Run("VerifyGNMI", func(t *testing.T) {
				
				t.Log("Verifying via GNMI")

				resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
				t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d, UsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

				if resp.GetUsedThresholdUpper() != tt.threshold || resp.GetUsedThresholdUpperClear() != tt.thresholdClear {
					t.Errorf("error verifying config: received %+v", resp)
				}
			})

			t.Run("UnconfigCLI", func(t *testing.T) {
				// // unconfig CLI
				// dut.CLI().RunResult(t, "config")
				unCfgCmd := fmt.Sprintf("no watchdog resource-utilization %s", tt.resource)
				
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				
				util.GNMIWithText(ctx, t, dut, unCfgCmd)
				t.Log("Finished running unconfig")
				
				t.Run("VerifyCLI", func(t *testing.T) {
					t.Log("Verifying via CLI")
					
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()
					
					result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | begin %s", cmd))
					t.Logf("output: %s", result.Output())
					
					if strings.Contains(result.Output(), cmd) {
						t.Errorf("error verifying config: %s", result.Error())
					}
				})
				
				t.Run("VerifyGNMI", func(t *testing.T) {
					
					t.Log("Verifying via GNMI")

					resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
					t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d, UsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

					if resp.GetUsedThresholdUpper() != DEFAULT_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_THRESHOLD_CLEAR {
						t.Errorf("error verifying config: received %+v", resp)
					}
				})
				
			})
		})
	}
	
	for _, tt := range tests {
		t.Run("GNMI/" + tt.name, func(t *testing.T) {
			d := &oc.Root{}

			config := d.GetOrCreateSystem().GetOrCreateUtilization().GetOrCreateResource(tt.resource)
			t.Logf("Attempting to apply config to %s: \n\tUsedThresholdUpper: %d\n\tUsedThresholdUpperClear: %d", config, config.GetName(), config.GetUsedThresholdUpper(), config.GetUsedThresholdUpperClear())

			config.UsedThresholdUpper = ygot.Uint8(tt.threshold)
			config.UsedThresholdUpperClear = ygot.Uint8(tt.thresholdClear)

			resp := gnmi.Update(t, dut, gnmi.OC().System().Utilization().Resource(config.GetName()).Config(), config)

			t.Logf("response: %+v", resp)
			
			
			t.Run("VerifyCLI", func(t *testing.T) {
				t.Log("Verifying via CLI")
				
				cli := dut.RawAPIs().CLI(t)
				cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
				
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				
				result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | begin %s", cmd))
				t.Logf("output: %s", result.Output())
				
				if !strings.Contains(result.Output(), cmd) {
					t.Errorf("error verifying config: %s", result.Error())
				}
			})
			
			t.Run("VerifyGNMI", func(t *testing.T) {
				
				t.Log("Verifying via GNMI")

				resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
				t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d, UsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

				if resp.GetUsedThresholdUpper() != tt.threshold || resp.GetUsedThresholdUpperClear() != tt.thresholdClear {
					t.Errorf("error verifying config: received %+v", resp)
				}
			})
			
			t.Run("UnconfigGNMI", func(t *testing.T) {
				t.Skip()
				
				gnmi.Delete(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).Config())
				
				t.Run("VerifyCLI", func(t *testing.T) {
					t.Log("Verifying via CLI")
					
					cli := dut.RawAPIs().CLI(t)
					cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d", tt.resource, tt.threshold, tt.thresholdClear)
					
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()
					
					result, _ := cli.RunCommand(ctx, fmt.Sprintf("show running-config | begin %s", cmd))
					t.Logf("output: %s", result.Output())
					
					if strings.Contains(result.Output(), cmd) {
						t.Errorf("error verifying config: %s", result.Error())
					}
				})
				
				t.Run("VerifyGNMI", func(t *testing.T) {
					
					t.Log("Verifying via GNMI")

					resp := gnmi.Get(t, dut, gnmi.OC().System().Utilization().Resource(tt.resource).State())
					t.Logf("GNMI output: %+v\nName: %s\nUsedThresholdUpper: %d, UsedThresholdUpperClear: %d", resp, resp.GetName(), resp.GetUsedThresholdUpper(), resp.GetUsedThresholdUpperClear())

					if resp.GetUsedThresholdUpper() != DEFAULT_THRESHOLD || resp.GetUsedThresholdUpperClear() != DEFAULT_THRESHOLD_CLEAR {
						t.Errorf("error verifying config: received %+v", resp)
					}
				})
				
			})
		})
	}
	
}

func TestReceiveSystemThresholdNotification(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	tests := []struct {
		name    string
		resource string
		threshold uint8
		thresholdClear uint8
		err string
	}{
		{
			name:    "CPU",
			resource: "cpu",
			threshold: 15,
			thresholdClear: 10,
			err:   "",
		},
		{
			name:    "Memory",
			resource: "memory",
			threshold: 70,
			thresholdClear: 50,
			err:   "",
		},
		{
			name:    "Disk0",
			resource: "disk0",
			threshold: 2,
			thresholdClear: 1,
			err:   "",
		},
		{
			name:    "HardDisk",
			resource: "harddisk",
			threshold: 2,
			thresholdClear: 1,
			err:   "",
		},
	}

	for _, tt := range tests {
		t.Run("GNMI/" + tt.name, func(t *testing.T) {
			d := &oc.Root{}

			config := d.GetOrCreateSystem().GetOrCreateUtilization().GetOrCreateResource(tt.resource)

			config.UsedThresholdUpper = ygot.Uint8(tt.threshold)
			config.UsedThresholdUpperClear = ygot.Uint8(tt.thresholdClear)
			
			t.Logf("Attempting to apply config to %s: \n\tUsedThresholdUpper: %d\n\tUsedThresholdUpperClear: %d", config, config.GetName(), config.GetUsedThresholdUpper(), config.GetUsedThresholdUpperClear())

			resp := gnmi.Update(t, dut, gnmi.OC().System().Utilization().Resource(config.GetName()).Config(), config)

			t.Logf("response: %+v", resp)
			t.Logf("config: %+v", config)
			
			watcher := gnmi.Watch(t,
				dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
				gnmi.OC().Component("0/RP0/CPU0").Linecard().Utilization().Resource(config.GetName()).State(), time.Minute*4, func(v *ygnmi.Value[*oc.Component_Linecard_Utilization_Resource]) bool {
					val, _ := v.Val()
					// t.Logf("received notification: \n %s\n", util.PrettyPrintJson(v))
					t.Logf("Received notification:\n%s\n", util.PrettyPrintJson(val))
					return val.GetUsedThresholdUpperExceeded() 
				})

			watcher2 := gnmi.Watch(t,
				dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
				gnmi.OC().Component("0/RP1/CPU0").Linecard().Utilization().Resource(config.GetName()).State(), time.Minute*4, func(v *ygnmi.Value[*oc.Component_Linecard_Utilization_Resource]) bool {
					val, _ := v.Val()
					// t.Logf("received notification: \n %s\n", util.PrettyPrintJson(v))
					t.Logf("Received notification:\n%s\n", util.PrettyPrintJson(val))
					return !val.GetUsedThresholdUpperExceeded() 
				})
			
			t.Logf("Stressing system %s", tt.resource)
			
			stressTestSystem(t, dut, "", tt.resource)	

			_, passed := watcher.Await(t)

			if !passed {
				t.Fatal("threshold was not exceeded")
			}

			t.Log("Threshold exceeded, waiting for threshold clear")
			
			_, passed2 := watcher2.Await(t)
			
			if !passed2 {
				t.Fatal("threshold was not cleared after being exceeded")
			}
			t.Log("Threshold cleared")
		})
	}
	
}

func TestSetLinecardThresholdCLI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	tests := []struct {
		name    string
		resource string
		location string
		threshold uint8
		thresholdClear uint8
		err string
	}{
		{
			name:    "CPU",
			resource: "cpu",
			location: "0/0/CPU0",
			threshold: 60,
			thresholdClear: 50,
			err:   "",
		},
		{
			name:    "Memory",
			resource: "memory",
			location: "0/0/CPU0",
			threshold: 60,
			thresholdClear: 50,
			err:   "",
		},
		{
			name:    "Disk0",
			resource: "disk0",
			location: "0/0/CPU0",
			threshold: 60,
			thresholdClear: 50,
			err:   "",
		},
		{
			name:    "HardDisk`",
			resource: "harddisk",
			location: "0/0/CPU0",
			threshold: 60,
			thresholdClear: 50,
			err:   "",
		},
	}

	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			
			cmd := fmt.Sprintf("watchdog resource-utilization %s set-threshold %d clear-threshold %d location %s", tt.resource, tt.threshold, tt.thresholdClear, tt.location)

			dut.CLI().RunResult(t, "config")
			res := dut.CLI().RunResult(t, cmd)
			dut.CLI().RunResult(t, "commit")
			dut.CLI().RunResult(t, "end")
			
			// cli := dut.RawAPIs().CLI(t)
			
			// _, err := cli.RunCommand(context.Background(), "config")
			// resp, err := cli.RunCommand(context.Background(), cmd)
			// _, err = cli.RunCommand(context.Background(), "commit")
			// _, err = cli.RunCommand(context.Background(), "end")
			
			if res.Error() != "" {
				t.Errorf("Error: %s", res.Error())
			}

			t.Logf("response: %+v", res.Output())
		})
	}
	
}

func TestSetLinecardThresholdGNMI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	tests := []struct {
		name    string
		resource string
		location string
		threshold uint8
		thresholdClear uint8
		err string
	}{
		{
			name:    "CPU",
			resource: "cpu",
			location: "0/0/CPU0",
			threshold: 30,
			thresholdClear: 25,
			err:   "",
		},
		{
			name:    "Memory",
			resource: "memory",
			location: "0/0/CPU0",
			threshold: 30,
			thresholdClear: 25,
			err:   "",
		},
		{
			name:    "Disk0",
			resource: "disk0",
			location: "0/0/CPU0",
			threshold: 30,
			thresholdClear: 25,
			err:   "",
		},
		{
			name:    "HardDisk`",
			resource: "harddisk",
			location: "0/0/CPU0",
			threshold: 30,
			thresholdClear: 25,
			err:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &oc.Root{}

			config := d.GetOrCreateComponent(tt.location).GetOrCreateLinecard().GetOrCreateUtilization().GetOrCreateResource(tt.resource)

			config.UsedThresholdUpper = ygot.Uint8(tt.threshold)
			config.UsedThresholdUpperClear = ygot.Uint8(tt.thresholdClear)

			resp := gnmi.Update(t, dut, gnmi.OC().Component(tt.location).Linecard().Utilization().Resource(tt.resource).Config(), config)

			t.Logf("Received response: %s", prettyPrintObj(resp))
			t.Logf("Input config: %+v", prettyPrintObj(config))

			
			// getResp := gnmi.Get(t, dut, gnmi.OC().Component(tt.location).Linecard().Utilization().Resource(tt.resource).State())
			// // 
			// t.Logf("Get response Name: %d", getResp.Name)
			// t.Logf("Get response UsedThresholdUpper: %d", getResp.UsedThresholdUpper)
			// t.Logf("Get response UsedThresholdUpperClear: %d", *getResp.UsedThresholdUpperClear)
			
		})
	}
	
}

func TestSetSystemCPUThresholdGNMI(t *testing.T) {
	
	dut := ondatra.DUT(t, "dut")
	
	set_threshold := uint8(60)
	clear_threshold := uint8(50)
	
	// cmd := fmt.Sprintf("watchdog resource-utilization cpu set-threshold %d clear-threshold", set_threshold, clear_threshold)
	
	d := &oc.Root{}

	config := d.GetOrCreateSystem().GetOrCreateUtilization().GetOrCreateResource("cpu")

	config.UsedThresholdUpper = ygot.Uint8(set_threshold)
	config.UsedThresholdUpperClear = ygot.Uint8(clear_threshold)

	// resp := gnmi.Update(t, dut, gnmi.OC().Component(location).Linecard().Utilization().Resource(config.GetName()).Config(), config)
	resp := gnmi.Update(t, dut, gnmi.OC().System().Utilization().Resource(config.GetName()).Config(), config)

	t.Logf("response: %+v", resp)
	t.Logf("config: %+v", config)
	
}


func TestSetSystemMemoryThresholdCLI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	location := "0/RP0/CPU0"
	// show_cmd := fmt.Sprintf("show memory summary location <LC location>", location)
	
	set_threshold := uint8(60)
	clear_threshold := uint8(50)
	
	// cmd := fmt.Sprintf("watchdog resource-utilization memory set-threshold %d clear-threshold %d location %s", set_threshold, clear_threshold, location)

	config := gnmi.Get(t, dut, gnmi.OC().Component(location).Chassis().Utilization().Resource("CPU").Config())

	config.UsedThresholdUpper = ygot.Uint8(set_threshold)
	config.UsedThresholdUpperClear = ygot.Uint8(clear_threshold)

	t.Logf("config: \n %+v", config)

	// memoryThreshold := gnmi.Replace(t, dut, gnmi.OC().Component(location).Chassis().Utilization().Resource("CPU").Config(), config)
	//
	// cli := dut.RawAPIs().CLI(t)

	// cli.RunCommand(context.Background(), cmd)
	
}

func TestSetSystemMemoryThresholdGNMI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	cmd := "watchdog resource-utilization cpu set-threshold 60 clear-threshold 50 location 0/RP0/CPU0"
	cli := dut.RawAPIs().CLI(t)

	cli.RunCommand(context.Background(), cmd)
	
}

func TestSetSystemDisk0ThresholdCLI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	cmd := "watchdog resource-utilization cpu set-threshold 60 clear-threshold 50 location 0/RP0/CPU0"
	cli := dut.RawAPIs().CLI(t)

	cli.RunCommand(context.Background(), cmd)
	
}

func TestSetSystemDisk0ThresholdGNMI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	cmd := "watchdog resource-utilization cpu set-threshold 60 clear-threshold 50 location 0/RP0/CPU0"
	cli := dut.RawAPIs().CLI(t)

	cli.RunCommand(context.Background(), cmd)
	
}


func TestSetSystemHardDiskThresholdCLI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	cmd := "watchdog resource-utilization cpu set-threshold 60 clear-threshold 50 location 0/RP0/CPU0"
	cli := dut.RawAPIs().CLI(t)

	cli.RunCommand(context.Background(), cmd)

}

func TestSetSystemHardDiskThresholdGNMI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	
	cmd := "watchdog resource-utilization cpu set-threshold 60 clear-threshold 50 location 0/RP0/CPU0"
	cli := dut.RawAPIs().CLI(t)

	cli.RunCommand(context.Background(), cmd)

}


// Final

func TestLinecardCPUThreshold(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	location := "0/0/CPU0"
	// show_cmd := fmt.Sprintf("show memory summary location %s", location)
	
	set_threshold := uint8(60)
	clear_threshold := uint8(50)
	
	// cmd := fmt.Sprintf("watchdog resource-utilization memory set-threshold %d clear-threshold %d location %s", set_threshold, clear_threshold, location)

	d := &oc.Root{}

	config := d.GetOrCreateComponent("0/0/CPU0").GetOrCreateLinecard().GetOrCreateUtilization().GetOrCreateResource("cpu")

	config.UsedThresholdUpper = ygot.Uint8(set_threshold)
	config.UsedThresholdUpperClear = ygot.Uint8(clear_threshold)

	resp := gnmi.Update(t, dut, gnmi.OC().Component(location).Linecard().Utilization().Resource(config.GetName()).Config(), config)

	t.Logf("response: \n%+v", resp)

	t.Logf("config: \n %+v", config)

}

func TestLinecardMemoryThreshold(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	location := "0/0/CPU0"
	// show_cmd := fmt.Sprintf("show memory summary location %s", location)
	
	set_threshold := uint8(60)
	clear_threshold := uint8(50)
	
	// cmd := fmt.Sprintf("watchdog resource-utilization memory set-threshold %d clear-threshold %d location %s", set_threshold, clear_threshold, location)

	d := &oc.Root{}

	config := d.GetOrCreateComponent("0/0/CPU0").GetOrCreateLinecard().GetOrCreateUtilization().GetOrCreateResource("memory")

	config.UsedThresholdUpper = ygot.Uint8(set_threshold)
	config.UsedThresholdUpperClear = ygot.Uint8(clear_threshold)

	resp := gnmi.Update(t, dut, gnmi.OC().Component(location).Linecard().Utilization().Resource(config.GetName()).Config(), config)

	t.Logf("response: \n%+v", resp)

	t.Logf("config: \n %+v", config)

}

func TestLinecardDisk0Threshold(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	location := "0/0/CPU0"
	// show_cmd := fmt.Sprintf("show memory summary location %s", location)
	
	set_threshold := uint8(60)
	clear_threshold := uint8(50)
	
	// cmd := fmt.Sprintf("watchdog resource-utilization memory set-threshold %d clear-threshold %d location %s", set_threshold, clear_threshold, location)

	d := &oc.Root{}

	config := d.GetOrCreateComponent("0/0/CPU0").GetOrCreateLinecard().GetOrCreateUtilization().GetOrCreateResource("disk0")

	config.UsedThresholdUpper = ygot.Uint8(set_threshold)
	config.UsedThresholdUpperClear = ygot.Uint8(clear_threshold)

	resp := gnmi.Update(t, dut, gnmi.OC().Component(location).Linecard().Utilization().Resource(config.GetName()).Config(), config)

	t.Logf("response: \n%+v", resp)

	t.Logf("config: \n %+v", config)

}

func TestLinecardHardDiskThreshold(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	location := "0/0/CPU0"
	// show_cmd := fmt.Sprintf("show memory summary location %s", location)
	
	set_threshold := uint8(60)
	clear_threshold := uint8(50)
	
	// cmd := fmt.Sprintf("watchdog resource-utilization memory set-threshold %d clear-threshold %d location %s", set_threshold, clear_threshold, location)

	d := &oc.Root{}

	config := d.GetOrCreateComponent("0/0/CPU0").GetOrCreateLinecard().GetOrCreateUtilization().GetOrCreateResource("cpu")

	config.UsedThresholdUpper = ygot.Uint8(set_threshold)
	config.UsedThresholdUpperClear = ygot.Uint8(clear_threshold)

	resp := gnmi.Update(t, dut, gnmi.OC().Component(location).Linecard().Utilization().Resource(config.GetName()).Config(), config)

	t.Logf("response: \n%+v", resp)

	t.Logf("config: \n %+v", config)

}

func TestSetLinecardCPUThreshold(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	location := "0/0/CPU0"
	// show_cmd := fmt.Sprintf("show memory summary location %s", location)
	
	set_threshold := uint8(60)
	clear_threshold := uint8(50)
	
	// cmd := fmt.Sprintf("watchdog resource-utilization memory set-threshold %d clear-threshold %d location %s", set_threshold, clear_threshold, location)

	d := &oc.Root{}

	config := d.GetOrCreateComponent("0/0/CPU0").GetOrCreateLinecard().GetOrCreateUtilization().GetOrCreateResource("cpu")

	config.UsedThresholdUpper = ygot.Uint8(set_threshold)
	config.UsedThresholdUpperClear = ygot.Uint8(clear_threshold)

	resp := gnmi.Update(t, dut, gnmi.OC().Component(location).Linecard().Utilization().Resource(config.GetName()).Config(), config)

	t.Logf("response: \n%+v", resp)

	t.Logf("config: \n %+v", config)

}
