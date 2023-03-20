package yang_coverage

import (
	"github.com/openconfig/ondatra/eventlis"
	"fmt"
	"strings"
)

type yCov struct {
	sanityName string 
	prefixPaths string
	ws string
	verbose bool	
	yc *YangCoverage
}

func PreTest(a *eventlis.BeforeTestsEvent) error {
	fmt.Println("executing before any test")
	//TODO
	clearCovLogs()
	enableCovLogs()
	return nil
}

func PostTest(a *eventlis.AfterTestsEvent) error {
	fmt.Println("executing after any test")
	//TODO
	collectCovLogs()
	processYCov()
	return nil
}
func CreateInstance(sanityName string, models []string, prefixPaths []string, ws string, event eventlis.EventListener, verbose ...bool) (*yCov, error) {
	var err error
	ycObj := &yCov{
		sanityName: sanityName,
		ws: ws,
		prefixPaths: strings.Join(prefixPaths, " "),
	}
	if len(verbose) > 0 {
		ycObj.verbose = verbose[0]
	}
	ycObj.yc, err = New(ws, models, sanityName, IT, PRECOMMIT)
	if err != nil {
		return nil, err
	}
	event.AddBeforeTestsCallback(PreTest)
	event.AddAfterTestsCallback(PostTest)
	return ycObj, err
}

func clearCovLogs() {
	//TODO
}

func enableCovLogs() {
	//TODO
}

func collectCovLogs() {
	//TODO
}

func processYCov() {
	//TODO
}