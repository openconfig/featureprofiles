// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package core provides a validator for being able to
// check for core files on DUT's before and after test
// modules runs.
package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"text/template"
	"time"

	"github.com/golang/glog"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/eventlis"

	fpb "github.com/openconfig/gnoi/file"
	opb "github.com/openconfig/ondatra/proto"
)

var (
	vendorCoreFilePath = map[opb.Device_Vendor]string{
		opb.Device_JUNIPER: "/var/core/",
		opb.Device_CISCO:   "/misc/disk1/",
		opb.Device_NOKIA:   "/var/core/",
		opb.Device_ARISTA:  "/var/core/",
	}
	vendorCoreFileNamePattern = map[opb.Device_Vendor]*regexp.Regexp{
		opb.Device_JUNIPER: regexp.MustCompile(".*.tar.gz"),
		opb.Device_CISCO:   regexp.MustCompile("/misc/disk1/.*core.*"),
		opb.Device_NOKIA:   regexp.MustCompile("/var/core/coredump-.*"),
		opb.Device_ARISTA:  regexp.MustCompile("/var/core/core.*"),
	}
)

var (
	validator validatorImpl
)

type fileInfo struct {
	Name     string
	Path     string
	Modified uint64
}

type dutCoreFiles struct {
	DUT    string
	Files  coreFiles
	Status string
}

type coreFiles map[string]fileInfo

type checker struct {
	dut        binding.DUT
	fileClient fpb.FileClient

	mu        sync.Mutex
	startTime time.Time
	prevCores coreFiles
}

func newChecker(dut binding.DUT) (*checker, error) {
	dutVendor := dut.Vendor()
	// vendorCoreFilePath and vendorCoreProcName should be provided to fetch core file on dut.
	if _, ok := vendorCoreFilePath[dutVendor]; !ok {
		return nil, fmt.Errorf("add support for vendor %v in var vendorCoreFilePath", dutVendor)
	}
	if _, ok := vendorCoreFileNamePattern[dutVendor]; !ok {
		return nil, fmt.Errorf("add support for vendor %v in var vendorCoreFileNamePattern", dutVendor)
	}
	gClients, err := dut.DialGNOI(context.Background())
	if err != nil {
		return nil, err
	}
	return &checker{
		dut:        dut,
		fileClient: gClients.File(),
		prevCores:  coreFiles{},
		startTime:  time.Now(),
	}, nil
}

func (c *checker) check() (coreFiles, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cores, err := c.checkCores()
	if err != nil {
		return nil, err
	}
	delta := coreFiles{}
	for k, v := range cores {
		if _, ok := c.prevCores[k]; !ok {
			delta[k] = v
		}
	}
	c.prevCores = cores
	return delta, nil
}

type validatorImpl struct {
	mu   sync.Mutex
	duts map[string]*checker
}

func (v *validatorImpl) check() map[string]dutCoreFiles {
	var wg sync.WaitGroup
	var mu sync.Mutex
	dutCores := map[string]dutCoreFiles{}
	for _, c := range v.duts {
		wg.Add(1)
		go func(c *checker) {
			defer wg.Done()
			cores, err := c.check()
			status := "OK"
			if err != nil {
				status = fmt.Sprintf("DUT %q failed to check cores: %v", c.dut.Name(), err)
				glog.Warning(status)
			}
			mu.Lock()
			defer mu.Unlock()
			dutCores[c.dut.Name()] = dutCoreFiles{
				DUT:    c.dut.Name(),
				Files:  cores,
				Status: status,
			}
		}(c)
	}
	wg.Wait()
	return dutCores
}

// start starts a core file watcher for the provided DUT.
func (v *validatorImpl) start(duts map[string]binding.DUT) map[string]dutCoreFiles {
	v.mu.Lock()
	defer v.mu.Unlock()
	for k, dut := range duts {
		glog.Infof("Registering core file checking for DUT %q", k)
		c, err := newChecker(dut)
		if err != nil {
			glog.Warningf("Failed to register core file checking for DUT %q: %v", k, err)
			continue
		}
		v.duts[k] = c
	}
	return v.check()
}

// Stop ends the validator and returns a list of all DUTs that
// found core files.
func (v *validatorImpl) stop() map[string]dutCoreFiles {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.check()
}

func registerBefore(e *eventlis.BeforeTestsEvent) error {
	cores := validator.start(e.Reservation.DUTs)
	ondatra.Report().AddSuiteProperty("validator.core", "enabled")
	report := createReport(cores)
	ondatra.Report().AddSuiteProperty("validator.core.initial", report)
	return nil
}

const (
	coreFmt = `
Delta Core Files by DUT:{{range $key, $dut := .}} 
DUT: {{$key}}{{ range $key, $cores := $dut.Files }}
  {{ $key }}{{ end }}{{ end }}`
)

var coreTemplate = template.Must(template.New("errorMsg").Parse(coreFmt))

func createReport(d map[string]dutCoreFiles) string {
	b := new(bytes.Buffer)
	if err := coreTemplate.Execute(b, d); err != nil {
		b.Reset()
		fmt.Fprintf(b, "parse error on retrieving core files: %v", err)
	}
	return b.String()
}

func registerAfter(_ *eventlis.AfterTestsEvent) error {
	cores := validator.stop()
	foundCores := false
	for _, files := range cores {
		if len(files.Files) > 0 {
			foundCores = true
			break
		}
	}
	report := createReport(cores)
	msg := fmt.Sprintf("core file check found cores:\n%s", report)
	glog.Infof(msg)
	ondatra.Report().AddSuiteProperty("validator.core.end", report)
	if foundCores {
		return errors.New(msg)
	}
	return nil
}

// Register will register core file watcher with the caller.
// This will allow the event listener to fire on test module start and end.
// All DUTs in the reservation will be monitored.
func Register() {
	validator = validatorImpl{
		duts: map[string]*checker{},
	}
	ondatra.EventListener().AddBeforeTestsCallback(registerBefore)
	ondatra.EventListener().AddAfterTestsCallback(registerAfter)
}

// coreFileCheck function is used to check if cores are found on the DUT.
func (c *checker) checkCores() (coreFiles, error) {
	dutVendor := c.dut.Vendor()
	corePath := vendorCoreFilePath[dutVendor]
	fileMatch := vendorCoreFileNamePattern[dutVendor]
	in := &fpb.StatRequest{
		Path: corePath,
	}
	validResponse, err := c.fileClient.Stat(context.Background(), in)
	if err != nil {
		return nil, fmt.Errorf("DUT %q: %w", corePath, err)
	}
	cores := coreFiles{}
	// Check cores creation time is greater than test start time.
	for _, fileStatsInfo := range validResponse.GetStats() {
		// Get the exact file.
		in = &fpb.StatRequest{
			Path: fileStatsInfo.GetPath(),
		}
		validResponse, err := c.fileClient.Stat(context.Background(), in)
		if err != nil {
			return nil, fmt.Errorf("DUT %q: unable to stat file %q, %v", c.dut.Name(), fileStatsInfo.GetPath(), err)
		}
		for _, filesMatched := range validResponse.GetStats() {
			coreFileName := filesMatched.GetPath()
			if fileMatch.MatchString(coreFileName) {
				cores[coreFileName] = fileInfo{
					Name:     coreFileName,
					Modified: fileStatsInfo.GetLastModified(),
				}
			}
		}
	}
	return cores, nil
}
