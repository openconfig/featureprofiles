package debug

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/logger"
	"github.com/openconfig/testt"
)

func TestCollectCoreFiles(t *testing.T) {
	targets := NewTargets(t)
	if *outDirFlag == "" {
		logger.Logger.Error().Msg(fmt.Sprintf("out directory flag not set correctly: [%s]", *outDirFlag))
		t.FailNow()
	} else {
		outDir = *outDirFlag
		logger.Logger.Info().Msg(fmt.Sprintf("out directory flag is: [%s]", *outDirFlag))
		// TODO: router does not undestand time.RFC3339Nano - also what timestamp time do we want... 5 mins ago, rn???
		timestamp = *timestampFlag
	}
	commands := []string{
		"run rm -rf /" + techDirectory,
		"mkdir " + techDirectory,
		"run find /misc/disk1 -maxdepth 1 -type f -name '*core*' -newermt @" + timestamp + " -exec cp \"{}\" /" + techDirectory + "/  \\\\;",
		"run find /harddisk: -maxdepth 1 -type f -name '*core*' -newermt @" + timestamp + " -exec cp \"{}\" /" + techDirectory + "/  \\\\;",
	}

	// pipeCore := []string{"cd harddisk:", "dir | i *core*"}
	// for _, t := range pipeCore {
	// 	commands = append(commands, fmt.Sprintf("%s | file %s", t, getTechFileName(t)))
	// }

	for dutID, targetInfo := range targets.targetInfo {
		t.Logf("Collecting debug files on %s", dutID)

		ctx := context.Background()
		cli := targets.GetOndatraCLI(t, dutID)

		for _, cmd := range commands {
			logger.Logger.Info().Msg(fmt.Sprintf("Running current command logger: [%s]", cmd))
			testt.CaptureFatal(t, func(t testing.TB) {
				if result, err := cli.SendCommand(ctx, cmd); err == nil {
					logger.Logger.Error().Msg(fmt.Sprintf("Error while running [%s] : [%v]", cmd, err))
					t.Logf("> %s", cmd)
					t.Log(result)
				} else {
					logger.Logger.Info().Msg(fmt.Sprintf("Command [%s] ran successfully", cmd))
					t.Logf("> %s", cmd)
					t.Log(err.Error())
				}
				t.Logf("\n")
			})
		}

		copyDebugFiles(t, targetInfo, "CollectCoreFiles")
	}
	fmt.Println("Exiting TestCollectionDebugFiles")
}
