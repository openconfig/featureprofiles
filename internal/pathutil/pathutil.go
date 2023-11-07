// Package pathutil provides utilities for finding test-relative paths at runtime.
package pathutil

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	mu       sync.Mutex
	rootPath string
)

// RootPath returns an absolute path to the root "featureprofiles" directory.
func RootPath() (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if rootPath != "" {
		return rootPath, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	rootPath, err = computeRootPath(wd)
	if err != nil {
		return "", err
	}
	return rootPath, nil
}

func computeRootPath(wd string) (string, error) {
	const rootPart = "/featureprofiles/"
	rootIdx := strings.LastIndex(wd, rootPart)
	if rootIdx < 0 {
		return "", fmt.Errorf("root %q not in working directory %q", rootPart, wd)
	}
	return wd[0 : rootIdx+len(rootPart)-1], nil
}
