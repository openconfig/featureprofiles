package fptest

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openconfig/ygot/ygot"
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
	path := filepath.Join(o.path, name) + ".csv"
	o.listeners = append(o.listeners, &csvListner{
		filepath: path,
	})
	return o
}

// RecordYgot records a ygot operation
func (o *Observer) RecordYgot(t *testing.T, operation string, pathstruct ygot.PathStruct) {
	ygotEvents := newYgotEvent(o.name, t, operation, pathstruct)
	for _, event := range ygotEvents {
		for _, listner := range o.listeners {
			listner.Record(event)
		}
	}
}

// NewObserver returns an Observer with given listners
func NewObserver(listeners ...listner) *Observer {
	path := reportDir
	return &Observer{
		path:      path,
		listeners: listeners,
	}
}

type listner interface {
	Record(event event) error
}
type event interface {
	getCsvEvent() []string
}

func (*Observer) RegisterObserver() {

}

type csvListner struct {
	filepath string
}

func (fw *csvListner) Record(event event) error {
	data := event.getCsvEvent()
	f, err := os.OpenFile(fw.filepath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
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

func newYgotEvent(name string, t *testing.T, operation string, pathstruct ygot.PathStruct) (events []ygotEvent) {
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
		path:      pathToText(pathstruct),
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
