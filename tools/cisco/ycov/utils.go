package ycov

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"text/template"
)

/*
 * HELPER Functions
 */

type fdata map[string]interface{}

func fstring(format string, data fdata) (string, error) {
	t, err := template.New("fstring").Parse(format)
	if err != nil {
		return "", fmt.Errorf("error creating template: %v", err)
	}
	output := new(bytes.Buffer)
	if err := t.Execute(output, data); err != nil {
		return "", fmt.Errorf("error executing template: %v", err)
	}
	return output.String(), nil
}

func copy(src, dst string) error {
	srcp, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcp.Close()

	// Create new file
	dstp, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstp.Close()

	//This will copy
	_, err = io.Copy(dstp, srcp)
	if err != nil {
		return err
	}
	return nil
}

// pathExists returns whether the given file or directory exists
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, errors.New(path + " does not exists!!")
		}
		return false, err
	}
	return true, nil
}

func writeLogsToFile(logs, fname string) (int, string) {
	if logs == "" {
		return -1, "Coverage logs are empty!!"
	}

	// - Save the logs to file
	f, err := os.Create(fname)
	if err != nil {
		return -1, err.Error()
	}

	defer f.Close()

	_, err = f.WriteString(logs)
	if err != nil {
		return -1, err.Error()
	}
	return 0, ""
}
