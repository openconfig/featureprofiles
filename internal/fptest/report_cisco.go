package fptest

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	timeformat = "2006-01-02 15:04:05"
	reportDir  = os.Getenv("REPORTS_DIR")
)

// Observer watches for events and sends them to all listners
type Observer struct {
	name      string
	listeners []listner
	path      string
}

// AddCsvRecorder adds a CSV recorder
func (o *Observer) AddCsvRecorder(name string) *Observer {
	o.listeners = append(o.listeners, &csvListner{
		filepath: o.path,
		filename: name + ".csv",
	})
	return o
}

// RecordYgot records a ygot operation
func (o *Observer) RecordYgot(t *testing.T, operation string, pathstruct interface{}) {
	ygotEvents := newYgotEvent(o.name, t, operation, pathstruct)
	for _, event := range ygotEvents {
		for _, listner := range o.listeners {
			err := listner.record(event)
			if err != nil {
				t.Logf("Unable to record , logging instead Test Path: %s -- %s -- %s -- %s ", o.name, event.testname, event.operation, event.status)

			} else {
				t.Logf("Test Path: %s -- %s -- %s -- %s ", o.name, event.testname, event.operation, event.status)

			}

		}
	}
}

// NewObserver returns an Observer with given listners
func NewObserver(name string, listeners ...listner) *Observer {
	path := reportDir
	return &Observer{
		name:      name,
		path:      path,
		listeners: listeners,
	}
}

type listner interface {
	record(event event) error
}
type event interface {
	getCsvEvent() []string
}

type csvListner struct {
	filepath string
	filename string
}

func (fw *csvListner) record(event event) error {
	filePath := filepath.Join(fw.filepath, fw.filename)
	if *outputsDir != "" {
		filePath = filepath.Join(*outputsDir, fw.filename)

	}
	data := event.getCsvEvent()
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	err = w.Write(data)
	w.Flush()
	if err == io.EOF {
		defer f.Close()
	}
	return err
}

type ygotEvent struct {
	feature   string
	testname  string
	operation string
	path      string
	status    string
	timestamp string
}

func newYgotEvent(name string, t *testing.T, operation string, pathstruct interface{}) (events []ygotEvent) {
	status := "PASSED"
	if t.Failed() {
		status = "FAILED"
	}
	timestamp := getCurrentTime()
	events = append(events, ygotEvent{
		feature:   name,
		testname:  t.Name(),
		status:    status,
		operation: operation,
		timestamp: timestamp,
		path:      fmt.Sprintf("%v", pathstruct),
	})
	return
}
func (y ygotEvent) getCsvEvent() []string {
	return []string{
		y.feature,
		y.testname,
		y.operation,
		y.status,
		y.path,
		y.timestamp,
	}
}

func getCurrentTime() string {
	return time.Now().Format(timeformat)
}
