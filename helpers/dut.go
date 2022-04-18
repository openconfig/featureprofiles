package helpers

import (
	"fmt"
	"log"
	"os/exec"
)

func PushDUTConfig(confPath string, dutNodeName string) (string, error) {
	cmd := exec.Command(
		kneBindConfig.CLIPath,
		"-v", "trace",
		"--kubecfg", kneBindConfig.KubecfgPath,
		"topology",
		"push", kneBindConfig.TopoPath,
		dutNodeName, confPath,
	)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not execute command: %s", err)
	}

	return string(out), nil
}

func ConfigDUTs(dutConfMap map[string]string) {
	for nodeName, confPath := range dutConfMap {
		log.Printf("Setting DUT config %s on node %s ...\n", confPath, nodeName)
		out, err := PushDUTConfig(confPath, nodeName)
		if err != nil {
			log.Fatalf("Failed setting DUT config: %v", err)
			return
		}
		log.Printf("Done setting DUT config, output:\n%s\n", out)
	}
}
