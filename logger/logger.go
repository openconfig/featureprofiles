package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

// Global logger for all packages to access
var Logger zerolog.Logger

// constant fields for logger
const APPNAME = "application_name"
const SOFTWARENAME = "software_name"
const SOURCE = "source"
const LOGGER = "LOGGER"
const LOGGERREMOTE = "PROD"
const NA = "N/A"

// constants for setting log level
const LOGLEVEL = "LOGLEVEL"
const INFO = "INFO"
const ERROR = "ERROR"
const WARN = "WARN"
const DEBUG = "DEBUG"

func init() {
	Logger = Config()
}

// an interface that follows the zerolog library requirements to create multiwriters
type FilteredWriter struct {
	w     zerolog.LevelWriter
	level zerolog.Level
}

// The Write function required by the interface of zerolog, this just writes bytes to a zerolevel writer that is a sync writer.
func (w *FilteredWriter) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}

// The WriteLevel function required by the interface of zerolog, this is used for setting multilevel writters and is just a wrapper around the zerolog sync writers.
// Input: the zerolog level to write e.g. 0,1,2,3 , the bytes to write
// Output: the length of bytes to write or error if any
func (w *FilteredWriter) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	if level == w.level {
		return w.w.WriteLevel(level, p)
	}
	return len(p), nil
}

// Sets the level the logger will print to stdout or the respective files if remote logger is enabled
func setLevel() {
	switch os.Getenv(LOGLEVEL) {
	case INFO:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case DEBUG:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case ERROR:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case WARN:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.Disabled)
	}
}

// The remote logger creates four different log level files to write to. This files are written in jsonlines format.
// Input: Strings used to set the hostname, appName, softwareName in the remote logger fields
func setRemote(hostname, appName, softwareName string) {

	// create multiple files to write to, JSONLOGSDIR is a path to a new jsonlogs directory
	jsonDir := os.Getenv("JSONLOGSDIR")
	if jsonDir == "" {
		Logger.Warn().Msg("The path to store json logs in config for logger is not set please set your JSONLOGSDIR by doing: export JSONLOGSDIR=/fullpath/to/store/jsonlogs")
		return
	}
	err := os.MkdirAll(jsonDir, os.ModePerm)

	if err != nil {
		Logger.Err(err).Msg("failed creating json logs directory")
	}
	// add private and public directories in path given
	err = os.MkdirAll(jsonDir+`/private`, os.ModePerm)
	if err != nil {
		Logger.Err(err).Msg("Failed creating directory for private logs")
	}
	err = os.MkdirAll(jsonDir+`/public`, os.ModePerm)
	if err != nil {
		Logger.Err(err).Msg("Failed creating directory for public logs")
	}
	fDebug, err := os.OpenFile(jsonDir+"/private/debug.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		Logger.Err(err).Msg("failed creating file for debug logs")
	}
	fInfo, err := os.OpenFile(jsonDir+"/public/info.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		Logger.Err(err).Msg("failed creating file for info logs")
	}
	fWarn, err := os.OpenFile(jsonDir+"/public/warn.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		Logger.Err(err).Msg("failed creating file for warning logs")
	}
	fError, err := os.OpenFile(jsonDir+"/public/error.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		Logger.Err(err).Msg("failed creating file for error logs")
	}

	// set the multi lvl writers
	writerDebug := zerolog.MultiLevelWriter(fDebug)
	writerInfo := zerolog.MultiLevelWriter(fInfo)
	writerWarn := zerolog.MultiLevelWriter(fWarn)
	writerError := zerolog.MultiLevelWriter(fError)

	// set the filtered writers
	filteredWriterDebug := &FilteredWriter{writerDebug, zerolog.DebugLevel}
	filteredWriteInfo := &FilteredWriter{writerInfo, zerolog.InfoLevel}
	filteredWriterWarn := &FilteredWriter{writerWarn, zerolog.WarnLevel}
	filteredWriterError := &FilteredWriter{writerError, zerolog.ErrorLevel}
	// set all the writers for the logger here
	w := zerolog.MultiLevelWriter(os.Stdout,
		filteredWriterDebug, filteredWriteInfo, filteredWriterWarn, filteredWriterError)
	// finally add this to our main logger
	Logger = zerolog.New(w).With().
		Str(SOURCE, hostname).
		Str(APPNAME, appName).
		Str(SOFTWARENAME, softwareName).
		Caller().
		Timestamp().
		Logger()
}

// Standard function that configures the logger according to the standards set on the docs.
func Config() zerolog.Logger {

	// set standard time format
	// get hostname and set as source for standard logs
	hostname, err := os.Hostname()
	// log whenever the hostname is not found
	if err != nil {
		hostname = NA
	}

	// get app name, and software name from env
	appName := os.Getenv(APPNAME)
	if appName == "" {
		appName = NA
	}
	softwareName := os.Getenv(SOFTWARENAME)
	if softwareName == "" {
		softwareName = NA
	}
	// set default stack tracer and a pretty printer
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339Nano}).
		With().
		Caller().
		Str(SOURCE, hostname).
		Logger()

	if os.Getenv(LOGGER) == LOGGERREMOTE {
		setRemote(hostname, appName, softwareName)
	}
	// add logging messages here to avoid null pointer dereference
	if appName == "" {
		Logger.Warn().Msg("could not find env variable:" + SOFTWARENAME)
	}
	if softwareName == "" {
		Logger.Warn().Msg("could not find env variable:" + APPNAME)
	}
	if err != nil {
		Logger.Warn().Msg("could not find hostname for field: " + SOURCE)
	}

	// finally set the level to print
	setLevel()
	return Logger
}

// Prints an interface in a pretty format easy to read, good tool for debugging.
func Prettyprint(data interface{}) {
	pretty, err := prettyConvert(data)
	if err != nil {
		fmt.Println("Failed converting data:", err)
	}
	fmt.Println("\n" + pretty + "\n")
}

// Formats an interface in a pretty struct format, good for debugging deep nested data structures
// Input: Any data structure that is deeply nested
// Output: The formatted data structured with indentation as a string or error if any
func prettyConvert(data interface{}) (string, error) {
	var prettyJSON bytes.Buffer
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if err := json.Indent(&prettyJSON, []byte(bytes), "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}
