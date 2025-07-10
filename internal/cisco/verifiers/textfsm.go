package verifiers

import (
	"fmt"
	"github.com/sirikothe/gotextfsm"
	"os"
)

// ProcessTextFSM takes a templatePath for a .textfsm file and CLI output string,
// parses the output, and returns a slice of variable maps.
func ProcessTextFSM(templatePath, cliOutput string) ([]map[string]any, error) {
	// Load template file
	templateFile, osErr := os.ReadFile(templatePath)
	if osErr != nil {
		panic(fmt.Errorf("failed to open template file: %v", osErr))
	}
	templateString := string(templateFile)
	fsm := gotextfsm.TextFSM{}
	err := fsm.ParseString(templateString)
	if err != nil {
		fmt.Printf("Error while parsing template '%s'\n", err.Error())
	}
	parser := gotextfsm.ParserOutput{}
	err = parser.ParseTextString(cliOutput, fsm, true)
	if err != nil {
		fmt.Printf("Error while parsing input '%s'\n", err.Error())
	}
	fmt.Printf("Parsed output: %v\n", parser.Dict)
	return parser.Dict, nil
}
