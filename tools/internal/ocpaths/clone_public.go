package ocpaths

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// ClonePublicRepo clones the openconfig/public repo at the given path.
//
//   - branch is the branch to be downloaded (passed to git clone -b). If it is empty then the argument will be omitted.
//
// # Note
//
//   - If the "public" folder already exists, then no additional downloads will
//     be made.
//   - A manual deletion of the downloadPath folder is required if no longer used.
func ClonePublicRepo(downloadPath, branch string) (string, error) {
	if downloadPath == "" {
		return "", fmt.Errorf("must provide download path")
	}
	publicPath := filepath.Join(downloadPath, "public")

	if _, err := os.Stat(publicPath); err == nil { // If NO error
		return publicPath, nil
	}

	args := []string{"clone", "--depth", "1", "https://github.com/openconfig/public.git", publicPath}
	if branch != "" {
		args = append(args, "-b", branch, "--single-branch")
	}
	cmd := exec.Command("git", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to clone public repo: %v, command failed to start: %q", err, cmd.String())
	}
	stderrOutput, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("failed to clone public repo: %v, command failed during execution: %q\n%s", err, cmd.String(), stderrOutput)
	}
	return publicPath, nil
}
